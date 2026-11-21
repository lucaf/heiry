package heiry_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lucaf/heiry"
)

// r1hidden and R1Shown embed the holder under an unexported and an exported
// alias name. Reflection gives the library no usable pointer to a field
// with an unexported name, so only the exported name can hold a scope.
type (
	r1hidden = heiry.Node
	R1Shown  = heiry.Node
)

type (
	r1Read  interface{ Read() int }
	r1Empty interface{}
)

type r1HiddenRoot struct {
	r1hidden
	value int
}

func (r *r1HiddenRoot) Read() int { return r.value }

type r1HiddenRootLeaf struct {
	r1HiddenRoot
	heiry.Node
}

type r1ShownRoot struct {
	R1Shown
	value int
}

func (r *r1ShownRoot) Read() int { return r.value }

type r1ShownRootLeaf struct {
	r1ShownRoot
	heiry.Node
}

type r1DirectRoot struct {
	heiry.Node
	value int
}

func (r *r1DirectRoot) Read() int { return r.value }

type r1DirectRootLeaf struct {
	r1DirectRoot
	heiry.Node
}

type r1HiddenStore struct {
	r1hidden
	value int
}

func (s *r1HiddenStore) Read() int { return s.value }

type r1ShownStore struct {
	R1Shown
	value int
}

func (s *r1ShownStore) Read() int { return s.value }

type r1DirectStore struct {
	heiry.Node
	value int
}

func (s *r1DirectStore) Read() int { return s.value }

// r1BareRoot and r1BareStore have no holder field at all.
type r1BareRoot struct{ value int }

func (r *r1BareRoot) Read() int { return r.value }

type r1BareRootLeaf struct {
	r1BareRoot
	heiry.Node
}

type r1BareStore struct{ value int }

func (s *r1BareStore) Read() int { return s.value }

// r1TwoRoot and r1TwoStore embed two holders: heiry.Node and an unexported
// alias of it.
type r1TwoRoot struct {
	heiry.Node
	r1hidden
	value int
}

func (r *r1TwoRoot) Read() int { return r.value }

type r1TwoRootLeaf struct {
	r1TwoRoot
	heiry.Node
}

type r1TwoStore struct {
	heiry.Node
	r1hidden
	value int
}

func (s *r1TwoStore) Read() int { return s.value }

type r1TwoOwner struct {
	heiry.Node
	store r1TwoStore
}

type r1TwoOwnerLeaf struct {
	r1TwoOwner
	heiry.Node
}

// Each owner keeps its component in an unexported field, which is supported.
type r1HiddenOwner struct {
	heiry.Node
	store r1HiddenStore
}

type r1HiddenOwnerLeaf struct {
	r1HiddenOwner
	heiry.Node
}

type r1ShownOwner struct {
	heiry.Node
	store r1ShownStore
}

type r1ShownOwnerLeaf struct {
	r1ShownOwner
	heiry.Node
}

type r1BareOwner struct {
	heiry.Node
	store r1BareStore
}

type r1BareOwnerLeaf struct {
	r1BareOwner
	heiry.Node
}

type r1DirectOwner struct {
	heiry.Node
	store r1DirectStore
}

type r1DirectOwnerLeaf struct {
	r1DirectOwner
	heiry.Node
}

// r1Holder is one declaration case: fresh builds a new, unprepared
// definition, and object allocates and initializes one object through it,
// returning the holder the registered provider dispatches through. initTwice
// initializes two objects through one definition.
type r1Holder struct {
	name      string
	validate  func() error
	prepare   func() error
	object    func() (holder *heiry.Node, provider r1Read, err error)
	initTwice func() (first, second error)
}

func r1Case[Leaf any](name string, fresh func() *heiry.Definition[Leaf], object func(*heiry.Definition[Leaf]) (*heiry.Node, r1Read, error)) r1Holder {
	return r1Holder{
		name:     name,
		validate: func() error { return fresh().Validate() },
		prepare: func() error {
			plan, err := fresh().Prepare()
			if err == nil && plan == nil {
				return errors.New("Prepare returned neither a Plan nor an error")
			}
			if err != nil && plan != nil {
				return errors.New("Prepare returned a Plan with its error")
			}
			return err
		},
		object: func() (*heiry.Node, r1Read, error) { return object(fresh()) },
		initTwice: func() (error, error) {
			d := fresh()
			_, _, first := object(d)
			_, _, second := object(d)
			return first, second
		},
	}
}

// r1Rejected lists the rejected cases. whole, when set, is everything
// Validate and Prepare report: for a level node, validation stops at the
// holder check, and the same layout without a holder reports the same
// diagnostic apart from its path and its own leaf type.
func r1Rejected() []struct {
	r1Holder
	level int
	path  string
	whole *heiry.Error
} {
	return []struct {
		r1Holder
		level int
		path  string
		whole *heiry.Error
	}{
		{r1Case("level node", func() *heiry.Definition[r1HiddenRootLeaf] {
			d := heiry.Define[r1HiddenRootLeaf]()
			d.Level[r1HiddenRoot, r1Read, r1Empty]("").BindNode[r1Read]((*r1HiddenRoot)(nil)).Extend[r1HiddenRootLeaf]()
			return d
		}, func(d *heiry.Definition[r1HiddenRootLeaf]) (*heiry.Node, r1Read, error) {
			x := &r1HiddenRootLeaf{r1HiddenRoot: r1HiddenRoot{value: 1}}
			err := d.Init(x, d.NodeProvider(&x.r1HiddenRoot), d.NodeProvider(x))
			return &x.r1HiddenRoot.r1hidden, &x.r1HiddenRoot, err
		}), 0, "r1HiddenRoot.r1hidden", &heiry.Error{Code: heiry.MissingNode, Level: 0, Target: reflect.TypeFor[r1HiddenRootLeaf](), Path: "r1HiddenRoot.r1hidden", Expected: reflect.TypeFor[heiry.Node]()}},
		{r1Case("component", func() *heiry.Definition[r1HiddenOwnerLeaf] {
			d := heiry.Define[r1HiddenOwnerLeaf]()
			d.Level[r1HiddenOwner, r1Read, r1Empty]("").Bind[r1Read]((*r1HiddenStore)(nil), "store").Extend[r1HiddenOwnerLeaf]()
			return d
		}, func(d *heiry.Definition[r1HiddenOwnerLeaf]) (*heiry.Node, r1Read, error) {
			x := &r1HiddenOwnerLeaf{r1HiddenOwner: r1HiddenOwner{store: r1HiddenStore{value: 1}}}
			err := d.Init(x, d.NodeProvider(&x.r1HiddenOwner), d.ComponentProvider[r1HiddenOwner]("store", &x.store), d.NodeProvider(x))
			return &x.store.r1hidden, &x.store, err
		}), 0, "r1HiddenOwner.store.r1hidden", nil},
		// Without a holder field the path keeps naming the Node it lacks.
		{r1Case("level node without a holder", func() *heiry.Definition[r1BareRootLeaf] {
			d := heiry.Define[r1BareRootLeaf]()
			d.Level[r1BareRoot, r1Read, r1Empty]("").BindNode[r1Read]((*r1BareRoot)(nil)).Extend[r1BareRootLeaf]()
			return d
		}, func(d *heiry.Definition[r1BareRootLeaf]) (*heiry.Node, r1Read, error) {
			x := &r1BareRootLeaf{r1BareRoot: r1BareRoot{value: 1}}
			err := d.Init(x, d.NodeProvider(&x.r1BareRoot), d.NodeProvider(x))
			return &x.Node, &x.r1BareRoot, err
		}), 0, "r1BareRoot.Node", &heiry.Error{Code: heiry.MissingNode, Level: 0, Target: reflect.TypeFor[r1BareRootLeaf](), Path: "r1BareRoot.Node", Expected: reflect.TypeFor[heiry.Node]()}},
		{r1Case("component without a holder", func() *heiry.Definition[r1BareOwnerLeaf] {
			d := heiry.Define[r1BareOwnerLeaf]()
			d.Level[r1BareOwner, r1Read, r1Empty]("").Bind[r1Read]((*r1BareStore)(nil), "store").Extend[r1BareOwnerLeaf]()
			return d
		}, func(d *heiry.Definition[r1BareOwnerLeaf]) (*heiry.Node, r1Read, error) {
			x := &r1BareOwnerLeaf{r1BareOwner: r1BareOwner{store: r1BareStore{value: 1}}}
			err := d.Init(x, d.NodeProvider(&x.r1BareOwner), d.ComponentProvider[r1BareOwner]("store", &x.store), d.NodeProvider(x))
			return &x.r1BareOwner.Node, &x.store, err
		}), 0, "r1BareOwner.store.Node", nil},
		// With two holders the path also names Node, not one of them.
		{r1Case("level node with two holders", func() *heiry.Definition[r1TwoRootLeaf] {
			d := heiry.Define[r1TwoRootLeaf]()
			d.Level[r1TwoRoot, r1Read, r1Empty]("").BindNode[r1Read]((*r1TwoRoot)(nil)).Extend[r1TwoRootLeaf]()
			return d
		}, func(d *heiry.Definition[r1TwoRootLeaf]) (*heiry.Node, r1Read, error) {
			x := &r1TwoRootLeaf{r1TwoRoot: r1TwoRoot{value: 1}}
			err := d.Init(x, d.NodeProvider(&x.r1TwoRoot), d.NodeProvider(x))
			return &x.r1TwoRoot.Node, &x.r1TwoRoot, err
		}), 0, "r1TwoRoot.Node", nil},
		{r1Case("component with two holders", func() *heiry.Definition[r1TwoOwnerLeaf] {
			d := heiry.Define[r1TwoOwnerLeaf]()
			d.Level[r1TwoOwner, r1Read, r1Empty]("").Bind[r1Read]((*r1TwoStore)(nil), "store").Extend[r1TwoOwnerLeaf]()
			return d
		}, func(d *heiry.Definition[r1TwoOwnerLeaf]) (*heiry.Node, r1Read, error) {
			x := &r1TwoOwnerLeaf{r1TwoOwner: r1TwoOwner{store: r1TwoStore{value: 1}}}
			err := d.Init(x, d.NodeProvider(&x.r1TwoOwner), d.ComponentProvider[r1TwoOwner]("store", &x.store), d.NodeProvider(x))
			return &x.store.Node, &x.store, err
		}), 0, "r1TwoOwner.store.Node", nil},
	}
}

func r1Accepted() []r1Holder {
	return []r1Holder{
		r1Case("exported alias level node", func() *heiry.Definition[r1ShownRootLeaf] {
			d := heiry.Define[r1ShownRootLeaf]()
			d.Level[r1ShownRoot, r1Read, r1Empty]("").BindNode[r1Read]((*r1ShownRoot)(nil)).Extend[r1ShownRootLeaf]()
			return d
		}, func(d *heiry.Definition[r1ShownRootLeaf]) (*heiry.Node, r1Read, error) {
			x := &r1ShownRootLeaf{r1ShownRoot: r1ShownRoot{value: 1}}
			err := d.Init(x, d.NodeProvider(&x.r1ShownRoot), d.NodeProvider(x))
			return &x.r1ShownRoot.R1Shown, &x.r1ShownRoot, err
		}),
		r1Case("direct level node", func() *heiry.Definition[r1DirectRootLeaf] {
			d := heiry.Define[r1DirectRootLeaf]()
			d.Level[r1DirectRoot, r1Read, r1Empty]("").BindNode[r1Read]((*r1DirectRoot)(nil)).Extend[r1DirectRootLeaf]()
			return d
		}, func(d *heiry.Definition[r1DirectRootLeaf]) (*heiry.Node, r1Read, error) {
			x := &r1DirectRootLeaf{r1DirectRoot: r1DirectRoot{value: 1}}
			err := d.Init(x, d.NodeProvider(&x.r1DirectRoot), d.NodeProvider(x))
			return &x.r1DirectRoot.Node, &x.r1DirectRoot, err
		}),
		r1Case("exported alias component in an unexported field", func() *heiry.Definition[r1ShownOwnerLeaf] {
			d := heiry.Define[r1ShownOwnerLeaf]()
			d.Level[r1ShownOwner, r1Read, r1Empty]("").Bind[r1Read]((*r1ShownStore)(nil), "store").Extend[r1ShownOwnerLeaf]()
			return d
		}, func(d *heiry.Definition[r1ShownOwnerLeaf]) (*heiry.Node, r1Read, error) {
			x := &r1ShownOwnerLeaf{r1ShownOwner: r1ShownOwner{store: r1ShownStore{value: 1}}}
			err := d.Init(x, d.NodeProvider(&x.r1ShownOwner), d.ComponentProvider[r1ShownOwner]("store", &x.store), d.NodeProvider(x))
			return &x.store.R1Shown, &x.store, err
		}),
		r1Case("direct component in an unexported field", func() *heiry.Definition[r1DirectOwnerLeaf] {
			d := heiry.Define[r1DirectOwnerLeaf]()
			d.Level[r1DirectOwner, r1Read, r1Empty]("").Bind[r1Read]((*r1DirectStore)(nil), "store").Extend[r1DirectOwnerLeaf]()
			return d
		}, func(d *heiry.Definition[r1DirectOwnerLeaf]) (*heiry.Node, r1Read, error) {
			x := &r1DirectOwnerLeaf{r1DirectOwner: r1DirectOwner{store: r1DirectStore{value: 1}}}
			err := d.Init(x, d.NodeProvider(&x.r1DirectOwner), d.ComponentProvider[r1DirectOwner]("store", &x.store), d.NodeProvider(x))
			return &x.store.Node, &x.store, err
		}),
	}
}

// r1MissingNode returns the MissingNode diagnostics in err, in order.
func r1MissingNode(err error) []*heiry.Error {
	var found []*heiry.Error
	var walk func(error)
	walk = func(err error) {
		if diagnostic, ok := err.(*heiry.Error); ok && diagnostic.Code == heiry.MissingNode {
			found = append(found, diagnostic)
		}
		switch wrapped := err.(type) {
		case interface{ Unwrap() []error }:
			for _, item := range wrapped.Unwrap() {
				walk(item)
			}
		case interface{ Unwrap() error }:
			if inner := wrapped.Unwrap(); inner != nil {
				walk(inner)
			}
		}
	}
	walk(err)
	return found
}

// r1Reported returns every error that err joins, in order, of any type.
func r1Reported(err error) []error {
	if err == nil {
		return nil
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return []error{err}
	}
	var all []error
	for _, item := range joined.Unwrap() {
		all = append(all, r1Reported(item)...)
	}
	return all
}

func TestR1UnexportedAliasHolderRejectedAtValidation(t *testing.T) {
	for _, tc := range r1Rejected() {
		check := func(stage string, err error) {
			t.Helper()
			found := r1MissingNode(err)
			if len(found) != 1 || found[0].Level != tc.level || found[0].Path != tc.path {
				t.Errorf("%s %s: MissingNode diagnostics %+v in %v, want one at level %d, path %q", tc.name, stage, found, err, tc.level, tc.path)
				return
			}
			t.Logf("%s %s: MissingNode at level %d, path %q", tc.name, stage, found[0].Level, found[0].Path)
		}
		whole := func(stage string, err error) {
			t.Helper()
			if tc.whole == nil {
				return
			}
			reported := r1Reported(err)
			var diagnostic *heiry.Error
			if len(reported) == 1 {
				diagnostic, _ = reported[0].(*heiry.Error)
			}
			if diagnostic == nil || *diagnostic != *tc.whole {
				t.Errorf("%s %s: reported %d errors %v, want only %+v", tc.name, stage, len(reported), reported, *tc.whole)
				return
			}
			t.Logf("%s %s: reports only %+v", tc.name, stage, *tc.whole)
		}
		err := tc.validate()
		check("Validate", err)
		whole("Validate", err)
		// Eager preparation reports it before any object exists.
		err = tc.prepare()
		check("Prepare", err)
		whole("Prepare", err)
		// Init prepares lazily and reports the same diagnostic.
		holder, _, err := tc.object()
		check("Init", err)
		if holder.Scope() != (heiry.Scope{}) {
			t.Errorf("%s: the rejected holder received a scope", tc.name)
		}
		// A failed preparation is not cached, so each Init on one definition
		// prepares again and reports the same diagnostic.
		first, second := tc.initTwice()
		check("first Init of one definition", first)
		check("second Init of one definition", second)
		if a, b := r1MissingNode(first), r1MissingNode(second); len(a) == 1 && len(b) == 1 && *a[0] != *b[0] {
			t.Errorf("%s: the two Init calls of one definition report %+v and %+v", tc.name, *a[0], *b[0])
		}
	}
}

func TestR1ExportedAndDirectHoldersUnaffected(t *testing.T) {
	for _, tc := range r1Accepted() {
		if err := tc.validate(); err != nil {
			t.Fatalf("%s Validate: %v", tc.name, err)
		}
		if err := tc.prepare(); err != nil {
			t.Fatalf("%s Prepare: %v", tc.name, err)
		}
		holder, provider, err := tc.object()
		if err != nil {
			t.Fatalf("%s Init: %v", tc.name, err)
		}
		if holder.Scope() == (heiry.Scope{}) || holder.Virtual[r1Read]() != provider || holder.Virtual[r1Read]().Read() != 1 {
			t.Fatalf("%s: the holder does not dispatch to its provider", tc.name)
		}
	}
}
