package heiry

import (
	"errors"
	"reflect"
	"testing"
)

type (
	e2Write  interface{ Write(value int) }
	e2Public interface {
		e1Read
		e2Write
	}
)

type e2Leaf struct {
	e1Root
	Node
	value int
}

func (l *e2Leaf) Read() int       { return l.value + l.Node.Parent[e1Read]().Read() }
func (l *e2Leaf) Write(value int) { l.value = value }

// e2SameDiagnostics compares two errors by every structured field of every
// joined diagnostic, in order.
func e2SameDiagnostics(a, b error) bool {
	left, right := m8Diagnostics(a), m8Diagnostics(b)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if *left[index] != *right[index] {
			return false
		}
	}
	return true
}

func e2DeclaredLevels[Leaf any](d *Definition[Leaf]) []declaredLevel {
	levels := make([]declaredLevel, len(d.levels))
	for index, level := range d.levels {
		levels[index] = *level
	}
	return levels
}

func TestBindNodeFormsMatchBindWithNodePath(t *testing.T) {
	explicit := Define[e2Leaf]()
	explicit.Level[e1Root, e1Read, e1Empty]("").Bind[e1Read]((*e1Root)(nil), ".")
	explicit.Level[e2Leaf, e2Public, e1Empty]("e1Root").BindGroup((*e2Leaf)(nil), ".", Of[e1Read](), Of[e2Write]())

	concise := Define[e2Leaf]()
	concise.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
	concise.Level[e2Leaf, e2Public, e1Empty]("e1Root").BindNodeGroup((*e2Leaf)(nil), Of[e1Read](), Of[e2Write]())

	if !reflect.DeepEqual(e2DeclaredLevels(explicit), e2DeclaredLevels(concise)) {
		t.Fatal("concise node registration recorded different declaration metadata")
	}
	for name, d := range map[string]*Definition[e2Leaf]{"explicit": explicit, "concise": concise} {
		x := &e2Leaf{e1Root: e1Root{value: 2}, value: 3}
		if err := d.Init(x, d.NodeProvider(&x.e1Root), d.NodeProvider(x)); err != nil {
			t.Fatalf("%s Init: %v", name, err)
		}
		x.Node.Virtual[e2Write]().Write(10)
		if got := x.Node.Virtual[e1Read]().Read(); got != 12 {
			t.Fatalf("%s Read = %d, want 12", name, got)
		}
	}
}

func TestBindNodeFormsKeepMalformedCallBehavior(t *testing.T) {
	type declare func(root, leaf *LevelHandle[e2Leaf])
	cases := []struct {
		name              string
		explicit, concise declare
		code              Code
	}{
		{"nonnil individual witness",
			func(_, leaf *LevelHandle[e2Leaf]) { leaf.Bind[e1Read](&e2Leaf{}, ".") },
			func(_, leaf *LevelHandle[e2Leaf]) { leaf.BindNode[e1Read](&e2Leaf{}) }, InvalidProvider},
		{"aggregate capability",
			func(_, leaf *LevelHandle[e2Leaf]) { leaf.Bind[e2Public]((*e2Leaf)(nil), ".") },
			func(_, leaf *LevelHandle[e2Leaf]) { leaf.BindNode[e2Public]((*e2Leaf)(nil)) }, InvalidCapability},
		{"nonnil group witness",
			func(_, leaf *LevelHandle[e2Leaf]) { leaf.BindGroup(&e2Leaf{}, ".", Of[e1Read]()) },
			func(_, leaf *LevelHandle[e2Leaf]) { leaf.BindNodeGroup(&e2Leaf{}, Of[e1Read]()) }, InvalidProvider},
		{"empty group",
			func(_, leaf *LevelHandle[e2Leaf]) { leaf.BindGroup((*e2Leaf)(nil), ".") },
			func(_, leaf *LevelHandle[e2Leaf]) { leaf.BindNodeGroup((*e2Leaf)(nil)) }, InvalidDeclaration},
		{"invalid token",
			func(_, leaf *LevelHandle[e2Leaf]) {
				leaf.BindGroup((*e2Leaf)(nil), ".", Of[e1Read](), OperationToken{})
			},
			func(_, leaf *LevelHandle[e2Leaf]) { leaf.BindNodeGroup((*e2Leaf)(nil), Of[e1Read](), OperationToken{}) }, InvalidCapability},
		{"provider of another level",
			func(root, _ *LevelHandle[e2Leaf]) { root.BindGroup((*e2Leaf)(nil), ".", Of[e1Read]()) },
			func(root, _ *LevelHandle[e2Leaf]) { root.BindNodeGroup((*e2Leaf)(nil), Of[e1Read]()) }, InvalidProvider},
		{"duplicate operation",
			func(_, leaf *LevelHandle[e2Leaf]) {
				leaf.Bind[e1Read]((*e2Leaf)(nil), ".").Bind[e1Read]((*e2Leaf)(nil), ".").Bind[e2Write]((*e2Leaf)(nil), ".")
			},
			func(_, leaf *LevelHandle[e2Leaf]) {
				leaf.BindNode[e1Read]((*e2Leaf)(nil)).BindNode[e1Read]((*e2Leaf)(nil)).BindNode[e2Write]((*e2Leaf)(nil))
			}, DuplicateBinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var results [2]error
			var levels [2][]declaredLevel
			for index, apply := range []declare{tc.explicit, tc.concise} {
				d := Define[e2Leaf]()
				root := d.Level[e1Root, e1Read, e1Empty]("")
				root.Bind[e1Read]((*e1Root)(nil), ".")
				leaf := d.Level[e2Leaf, e2Public, e1Empty]("e1Root")
				apply(root, leaf)
				results[index], levels[index] = d.Validate(), e2DeclaredLevels(d)
			}
			if !errors.Is(results[1], &Error{Code: tc.code}) {
				t.Fatalf("concise form: missing %s in %v", tc.code, results[1])
			}
			if !e2SameDiagnostics(results[0], results[1]) {
				t.Fatalf("diagnostics differ:\nexplicit: %v\nconcise:  %v", results[0], results[1])
			}
			if !reflect.DeepEqual(levels[0], levels[1]) {
				t.Fatal("declaration metadata differs after the malformed call")
			}
		})
	}
}

func TestBindNodeFormsRespectFreezingAndInertHandles(t *testing.T) {
	d := Define[e2Leaf]()
	root := d.Level[e1Root, e1Read, e1Empty]("")
	root.BindNode[e1Read]((*e1Root)(nil))
	leaf := d.Level[e2Leaf, e2Public, e1Empty]("e1Root")
	leaf.BindNodeGroup((*e2Leaf)(nil), Of[e1Read](), Of[e2Write]())
	plan := d.MustPrepare()
	before := e2DeclaredLevels(d)

	leaf.BindNode[e1Read]((*e2Leaf)(nil))
	leaf.BindNodeGroup((*e2Leaf)(nil), Of[e2Write]())
	if len(d.recorded) != 2 || !reflect.DeepEqual(before, e2DeclaredLevels(d)) || d.plan != plan {
		t.Fatalf("frozen edits: recorded=%d", len(d.recorded))
	}
	for _, recorded := range m8Diagnostics(d.Validate()) {
		if recorded.Code != InvalidDeclaration || recorded.Level != 1 {
			t.Fatalf("frozen edit diagnostic: %+v", recorded)
		}
	}
	inert := d.Level[e2Leaf, e2Public, e1Empty]("e1Root")
	recorded := len(d.recorded)
	inert.BindNode[e1Read]((*e2Leaf)(nil)).BindNodeGroup((*e2Leaf)(nil), Of[e2Write]())
	if !inert.rejected || len(d.recorded) != recorded {
		t.Fatal("an inert handle recorded or registered something")
	}
	x := &e2Leaf{e1Root: e1Root{value: 2}, value: 3}
	if err := plan.Init(x, plan.NodeProvider(&x.e1Root), plan.NodeProvider(x)); err != nil {
		t.Fatalf("retained Plan: %v", err)
	}
}

type e3Mid struct {
	e1Root
	Node
}

type e3Leaf struct {
	e3Mid
	Node
	value int
}

func (l *e3Leaf) Read() int       { return l.value + l.Node.Parent[e1Read]().Read() }
func (l *e3Leaf) Write(value int) { l.value = value }

func e3Explicit() *Definition[e3Leaf] {
	d := Define[e3Leaf]()
	d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
	d.Level[e3Mid, e1Read, e1Empty]("e1Root")
	d.Level[e3Leaf, e2Public, e1Empty]("e3Mid").BindNode[e1Read]((*e3Leaf)(nil)).BindNode[e2Write]((*e3Leaf)(nil))
	return d
}

func e3Concise() *Definition[e3Leaf] {
	d := Define[e3Leaf]()
	d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
	d.Extend[e3Mid]()
	d.ExtendWith[e3Leaf, e2Public, e1Empty]().BindNode[e1Read]((*e3Leaf)(nil)).BindNode[e2Write]((*e3Leaf)(nil))
	return d
}

func TestExtendFormsMatchExplicitLevels(t *testing.T) {
	explicit, concise := e3Explicit().MustPrepare(), e3Concise().MustPrepare()
	for level := range explicit.validated.levels {
		a, b := explicit.validated, concise.validated
		if a.levels[level].parentField != b.levels[level].parentField || a.parentFields[level] != b.parentFields[level] ||
			a.nodeFields[level] != b.nodeFields[level] || a.providerPath(level) != b.providerPath(level) || a.parentValid[level] != b.parentValid[level] {
			t.Fatalf("level %d: inferred parent metadata differs from the explicit declaration", level)
		}
	}
	if !reflect.DeepEqual(explicit.validated.known, concise.validated.known) || !reflect.DeepEqual(explicit.validated.slots, concise.validated.slots) {
		t.Fatal("capabilities or slots differ")
	}
	for name, base := range map[string]*Plan[e3Leaf]{"explicit": explicit, "concise": concise} {
		for mode, plan := range map[string]*Plan[e3Leaf]{"dynamic": base, "precomputed": base.MustPrecompute()} {
			x := &e3Leaf{e3Mid: e3Mid{e1Root: e1Root{value: 2}}, value: 3}
			if err := plan.Init(x, plan.NodeProvider(&x.e3Mid.e1Root), plan.NodeProvider(&x.e3Mid), plan.NodeProvider(x)); err != nil {
				t.Fatalf("%s %s Init: %v", name, mode, err)
			}
			x.Node.Virtual[e2Write]().Write(10)
			if got := x.e3Mid.Node.Virtual[e1Read]().Read(); got != 12 {
				t.Fatalf("%s %s Read = %d, want 12", name, mode, got)
			}
			if x.Node.Parent[e1Read]() != e1Read(&x.e3Mid.e1Root) {
				t.Fatalf("%s %s: parent call did not pass over the inherited level", name, mode)
			}
		}
	}
}

type e3NoHolderMid struct{ e1Root }

type e3OverNoHolder struct {
	e3NoHolderMid
	Node
}

func (l *e3OverNoHolder) Read() int { return 0 }

// Invalid configurations must report the same structured diagnostics whether
// a level is declared explicitly or through an extension call.
func TestExtendFormsPreserveValidationDiagnostics(t *testing.T) {
	pairs := []struct {
		name              string
		explicit, concise func() error
		code              Code
	}{
		{"missing coverage",
			func() error {
				d := Define[e3Leaf]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.Level[e3Mid, e1Read, e1Empty]("e1Root")
				d.Level[e3Leaf, e2Public, e1Empty]("e3Mid")
				return d.Validate()
			},
			func() error {
				d := Define[e3Leaf]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.Extend[e3Mid]()
				d.ExtendWith[e3Leaf, e2Public, e1Empty]()
				return d.Validate()
			}, MissingMethods},
		{"narrowed contract",
			func() error {
				d := Define[e3Leaf]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.Level[e3Mid, e1Empty, e1Empty]("e1Root")
				d.Level[e3Leaf, e1Read, e1Empty]("e3Mid")
				return d.Validate()
			},
			func() error {
				d := Define[e3Leaf]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.ExtendWith[e3Mid, e1Empty, e1Empty]()
				d.ExtendWith[e3Leaf, e1Read, e1Empty]()
				return d.Validate()
			}, ContractExtension},
		{"public and private collision",
			func() error {
				d := Define[e3Leaf]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.Level[e3Mid, e1Read, e1Read]("e1Root")
				d.Level[e3Leaf, e1Read, e1Read]("e3Mid")
				return d.Validate()
			},
			func() error {
				d := Define[e3Leaf]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.ExtendWith[e3Mid, e1Read, e1Read]()
				d.Extend[e3Leaf]()
				return d.Validate()
			}, ContractCollision},
		{"operation outside the level's contracts",
			func() error {
				d := Define[e3Leaf]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.Level[e3Mid, e1Read, e1Empty]("e1Root")
				d.Level[e3Leaf, e1Read, e1Empty]("e3Mid").BindNode[e2Write]((*e3Leaf)(nil))
				return d.Validate()
			},
			func() error {
				d := Define[e3Leaf]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.Extend[e3Mid]()
				d.Extend[e3Leaf]().BindNode[e2Write]((*e3Leaf)(nil))
				return d.Validate()
			}, UndeclaredOperation},
		{"last level is not the leaf",
			func() error {
				d := Define[e3Leaf]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.Level[e3Mid, e1Read, e1Empty]("e1Root")
				return d.Validate()
			},
			func() error {
				d := Define[e3Leaf]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.Extend[e3Mid]()
				return d.Validate()
			}, InvalidHierarchy},
		{"level without its own holder",
			func() error {
				d := Define[e3OverNoHolder]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.Level[e3NoHolderMid, e1Read, e1Empty]("e1Root")
				d.Level[e3OverNoHolder, e1Read, e1Empty]("e3NoHolderMid")
				return d.Validate()
			},
			func() error {
				d := Define[e3OverNoHolder]()
				d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
				d.Extend[e3NoHolderMid]()
				d.Extend[e3OverNoHolder]()
				return d.Validate()
			}, MissingNode},
	}
	for _, tc := range pairs {
		t.Run(tc.name, func(t *testing.T) {
			explicit, concise := tc.explicit(), tc.concise()
			if !errors.Is(concise, &Error{Code: tc.code}) {
				t.Fatalf("concise form: missing %s in %v", tc.code, concise)
			}
			if !e2SameDiagnostics(explicit, concise) {
				t.Fatalf("diagnostics differ:\nexplicit: %+v\nconcise:  %+v", m8Diagnostics(explicit), m8Diagnostics(concise))
			}
		})
	}
}

type E3Alias = e1Root

type e3AliasChild struct {
	E3Alias
	Node
}

func (c *e3AliasChild) Read() int { return 1 + c.Node.Parent[e1Read]().Read() }

// e3Twice embeds the parent type twice under different names, so the parent
// edge cannot be inferred.
type e3Twice struct {
	e1Root
	E3Alias
	Node
}

func (c *e3Twice) Read() int { return 1 + c.Node.Parent[e1Read]().Read() }

type e3PointerParent struct {
	*e1Root
	Node
}

func (c *e3PointerParent) Read() int { return 0 }

type e3NamedParent struct {
	Base e1Root
	Node
}

func (c *e3NamedParent) Read() int { return 0 }

func TestExtendInfersAliasNamedParent(t *testing.T) {
	d := Define[e3AliasChild]()
	d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
	d.Extend[e3AliasChild]().BindNode[e1Read]((*e3AliasChild)(nil))
	plan, err := d.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	if plan.validated.levels[1].parentField != "E3Alias" || plan.validated.providerPath(0) != "E3Alias" {
		t.Fatalf("recorded parent field = %q", plan.validated.levels[1].parentField)
	}
	x := &e3AliasChild{E3Alias: e1Root{value: 4}}
	if err := plan.Init(x, plan.NodeProvider(&x.E3Alias), plan.NodeProvider(x)); err != nil {
		t.Fatal(err)
	}
	if got := x.Node.Virtual[e1Read]().Read(); got != 5 {
		t.Fatalf("Read = %d, want 5", got)
	}
	// A later diagnostic uses the declared alias name.
	err = plan.Init(x, plan.NodeProvider(&x.E3Alias), plan.NodeProvider(x))
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != AlreadyInitialized || diagnostic.Path != "E3Alias" {
		t.Fatalf("repeated Init diagnostic = %+v", diagnostic)
	}
}

func TestExtendRejectsMissingOrAmbiguousParent(t *testing.T) {
	check := func(name string, err error, expected reflect.Type, target reflect.Type) {
		t.Helper()
		var found *Error
		for _, diagnostic := range m8Diagnostics(err) {
			if diagnostic.Code == InvalidHierarchy {
				found = diagnostic
			}
		}
		if found == nil || found.Level != 1 || found.Expected != expected || found.Target != target || found.Path != "" {
			t.Fatalf("%s: diagnostic = %+v in %v", name, found, err)
		}
	}
	root := reflect.TypeFor[e1Root]()

	twice := Define[e3Twice]()
	twice.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
	twice.Extend[e3Twice]().BindNode[e1Read]((*e3Twice)(nil))
	check("ambiguous", twice.Validate(), root, reflect.TypeFor[e3Twice]())
	if _, err := twice.Prepare(); err == nil {
		t.Fatal("an ambiguous parent prepared a Plan")
	}

	pointer := Define[e3PointerParent]()
	pointer.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
	pointer.ExtendWith[e3PointerParent, e1Read, e1Empty]().BindNode[e1Read]((*e3PointerParent)(nil))
	check("pointer parent", pointer.Validate(), root, reflect.TypeFor[e3PointerParent]())

	named := Define[e3NamedParent]()
	named.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
	named.Extend[e3NamedParent]().BindNode[e1Read]((*e3NamedParent)(nil))
	check("named parent", named.Validate(), root, reflect.TypeFor[e3NamedParent]())

	// The explicit form selects one of the two candidates.
	explicit := Define[e3Twice]()
	explicit.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
	explicit.Level[e3Twice, e1Read, e1Empty]("E3Alias").BindNode[e1Read]((*e3Twice)(nil))
	x := &e3Twice{e1Root: e1Root{value: 100}, E3Alias: e1Root{value: 4}}
	if err := explicit.Init(x, explicit.NodeProvider(&x.E3Alias), explicit.NodeProvider(x)); err != nil {
		t.Fatalf("explicit selection: %v", err)
	}
	if got := x.Node.Virtual[e1Read]().Read(); got != 5 {
		t.Fatalf("explicitly selected parent Read = %d, want 5", got)
	}
}

func TestExtendFormsRequireAPrecedingLevel(t *testing.T) {
	for name, first := range map[string]func(*Definition[e1Root]) *LevelHandle[e1Root]{
		"Extend":     func(d *Definition[e1Root]) *LevelHandle[e1Root] { return d.Extend[e1Root]() },
		"ExtendWith": func(d *Definition[e1Root]) *LevelHandle[e1Root] { return d.ExtendWith[e1Root, e1Read, e1Empty]() },
	} {
		d := Define[e1Root]()
		handle := first(d)
		if !handle.rejected || len(d.levels) != 0 || len(d.recorded) != 1 {
			t.Fatalf("%s first: rejected=%t levels=%d recorded=%d", name, handle.rejected, len(d.levels), len(d.recorded))
		}
		handle.BindNode[e1Read]((*e1Root)(nil))
		if len(d.recorded) != 1 {
			t.Fatalf("%s: the inert handle recorded another diagnostic", name)
		}
		var diagnostic *Error
		if err := d.Validate(); !errors.As(err, &diagnostic) || diagnostic.Code != InvalidDeclaration || diagnostic.Level != 0 ||
			diagnostic.Target != reflect.TypeFor[e1Root]() || diagnostic.Actual != reflect.TypeFor[e1Root]() {
			t.Fatalf("%s first diagnostic = %+v", name, diagnostic)
		}
		// The error is sticky even after a valid root is declared.
		d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
		if _, err := d.Prepare(); !errors.Is(err, &Error{Code: InvalidDeclaration}) {
			t.Fatalf("%s: sticky error lost: %v", name, err)
		}
	}
}

func TestExtendFormsAreRejectedAfterPreparation(t *testing.T) {
	d := e3Concise()
	plan := d.MustPrepare()
	before := e2DeclaredLevels(d)
	for name, handle := range map[string]*LevelHandle[e3Leaf]{
		"Extend": d.Extend[e3Leaf](), "ExtendWith": d.ExtendWith[e3Leaf, e2Public, e1Empty](),
	} {
		if !handle.rejected {
			t.Fatalf("%s after preparation returned a live handle", name)
		}
	}
	if len(d.recorded) != 2 || !reflect.DeepEqual(before, e2DeclaredLevels(d)) || d.plan != plan {
		t.Fatalf("frozen extension calls changed the definition: recorded=%d", len(d.recorded))
	}
	for _, diagnostic := range m8Diagnostics(d.Validate()) {
		if diagnostic.Code != InvalidDeclaration || diagnostic.Level != 3 {
			t.Fatalf("frozen extension diagnostic = %+v", diagnostic)
		}
	}
}

type e3OverTwice struct {
	e3Twice
	Node
}

// Below an edge that could not be inferred, diagnostics keep the supplied
// path instead of a guessed leaf-relative prefix.
func TestExtendUnresolvedEdgeDoesNotInventPrefixes(t *testing.T) {
	d := Define[e3OverTwice]()
	d.Level[e1Root, e1Read, e1Empty]("").Bind[e1Read]((*e1Root)(nil), "missing")
	d.Extend[e3Twice]().BindNode[e1Read]((*e3Twice)(nil))
	d.Extend[e3OverTwice]()
	var edge, provider *Error
	for _, diagnostic := range m8Diagnostics(d.Validate()) {
		switch diagnostic.Code {
		case InvalidHierarchy:
			edge = diagnostic
		case InvalidProvider:
			provider = diagnostic
		}
	}
	if edge == nil || edge.Level != 1 || edge.Path != "e3Twice" || edge.Expected != reflect.TypeFor[e1Root]() {
		t.Fatalf("unresolved edge diagnostic = %+v", edge)
	}
	if provider == nil || provider.Level != 0 || provider.Path != "missing" {
		t.Fatalf("root provider diagnostic = %+v", provider)
	}
}

func e4Object() *e3Leaf { return &e3Leaf{e3Mid: e3Mid{e1Root: e1Root{value: 2}}, value: 3} }

func TestHandleForwardingMakesEndOptional(t *testing.T) {
	withEnd := Define[e3Leaf]().
		Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil)).End().
		Extend[e3Mid]().End().
		ExtendWith[e3Leaf, e2Public, e1Empty]().BindNode[e1Read]((*e3Leaf)(nil)).BindNode[e2Write]((*e3Leaf)(nil)).End()
	last := Define[e3Leaf]().
		Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil)).
		Extend[e3Mid]().
		ExtendWith[e3Leaf, e2Public, e1Empty]().BindNode[e1Read]((*e3Leaf)(nil)).BindNode[e2Write]((*e3Leaf)(nil))
	if !reflect.DeepEqual(e2DeclaredLevels(withEnd), e2DeclaredLevels(last.End())) {
		t.Fatal("a chain without End recorded different levels")
	}
	if err := last.Validate(); err != nil {
		t.Fatal(err)
	}
	plan, err := last.Prepare()
	if err != nil || plan != last.MustPrepare() || plan != last.End().MustPrepare() {
		t.Fatalf("handle preparation is not the definition's: %v", err)
	}
	// References and initialization through the handle use the same definition.
	x := e4Object()
	if err := last.Init(x, last.NodeProvider(&x.e3Mid.e1Root), last.NodeProvider(&x.e3Mid), last.End().NodeProvider(x)); err != nil {
		t.Fatalf("Init through a handle: %v", err)
	}
	y := e4Object()
	if last.MustInit(y, plan.NodeProvider(&y.e3Mid.e1Root), last.NodeProvider(&y.e3Mid), last.NodeProvider(y)) != y {
		t.Fatal("MustInit through a handle returned another leaf")
	}
	if x.Node.Virtual[e1Read]().Read() != 5 || y.Node.Virtual[e1Read]().Read() != 5 {
		t.Fatal("objects initialized through a handle dispatch incorrectly")
	}
	// A reference from another definition is still rejected by lineage.
	z := e4Object()
	other := e3Concise()
	err = last.Init(z, other.NodeProvider(&z.e3Mid.e1Root), last.NodeProvider(&z.e3Mid), last.NodeProvider(z))
	if !errors.Is(err, &Error{Code: BindingMismatch}) || z.Node.Scope().dispatch != nil {
		t.Fatalf("foreign-lineage reference through a handle: %v", err)
	}
}

type e4Part struct {
	Node
	value int
}

func (p *e4Part) Read() int { return p.value }

type e4Owner struct {
	Node
	Part e4Part
}

func TestHandleForwardsComponentReferences(t *testing.T) {
	handle := Define[e4Owner]().Level[e4Owner, e1Read, e1Empty]("").Bind[e1Read]((*e4Part)(nil), "Part")
	x := &e4Owner{Part: e4Part{value: 9}}
	if err := handle.Init(x, handle.NodeProvider(x), handle.ComponentProvider[e4Owner]("Part", &x.Part)); err != nil {
		t.Fatal(err)
	}
	if got := x.Part.Node.Virtual[e1Read]().Read(); got != 9 {
		t.Fatalf("Read = %d, want 9", got)
	}
	y, foreign := &e4Owner{}, &e4Owner{}
	err := handle.Init(y, handle.NodeProvider(y), handle.ComponentProvider[e4Owner]("Part", &foreign.Part))
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != ProviderOwnership || diagnostic.Path != "Part" {
		t.Fatalf("foreign component through a handle: %+v", diagnostic)
	}
}

func TestSavedHandlesKeepTheirLevelAndAppendToTheChain(t *testing.T) {
	d := Define[e3Leaf]()
	root := d.Level[e1Root, e1Read, e1Empty]("")
	// Levels added through the oldest handle are appended, never inserted.
	mid := root.Extend[e3Mid]()
	leaf := root.ExtendWith[e3Leaf, e2Public, e1Empty]()
	for index, want := range []reflect.Type{reflect.TypeFor[e1Root](), reflect.TypeFor[e3Mid](), reflect.TypeFor[e3Leaf]()} {
		if d.levels[index].typ != want || d.levels[index].index != index {
			t.Fatalf("level %d is %v", index, d.levels[index].typ)
		}
	}
	// Registrations through saved handles still go to their own levels.
	leaf.BindNode[e2Write]((*e3Leaf)(nil))
	root.BindNode[e1Read]((*e1Root)(nil))
	leaf.BindNode[e1Read]((*e3Leaf)(nil))
	if len(d.levels[0].bindings) != 1 || len(d.levels[1].bindings) != 0 || len(d.levels[2].bindings) != 2 || mid.level != d.levels[1] {
		t.Fatal("a saved handle registered on another level")
	}
	explicit := Define[e3Leaf]()
	explicit.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
	explicit.Level[e3Mid, e1Read, e1Empty]("e1Root")
	explicit.Level[e3Leaf, e2Public, e1Empty]("e3Mid").BindNode[e2Write]((*e3Leaf)(nil)).BindNode[e1Read]((*e3Leaf)(nil))
	if err := mid.Validate(); err != nil || !e2SameDiagnostics(explicit.Validate(), mid.Validate()) {
		t.Fatalf("validation through a saved handle: %v", err)
	}
	// Level through a handle appends as well, and its explicit name is still checked.
	extra := Define[e3Leaf]()
	extra.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil)).
		Level[e3Mid, e1Read, e1Empty]("wrong").
		Extend[e3Leaf]()
	if err := extra.Validate(); !errors.Is(err, &Error{Code: InvalidHierarchy}) || len(extra.levels) != 3 {
		t.Fatalf("Level through a handle: %v", err)
	}
}

func TestHandleForwardingKeepsFreezingAndInertBehavior(t *testing.T) {
	d := e3Concise()
	saved := d.levels[2]
	handle := &LevelHandle[e3Leaf]{definition: d, level: saved}
	plan := handle.MustPrepare()
	inert := handle.Extend[e3Leaf]()
	if !inert.rejected || len(d.recorded) != 1 || len(d.levels) != 3 {
		t.Fatal("extension through a handle after freezing was not rejected")
	}
	// Lifecycle calls through the inert handle reach the same definition.
	if err := inert.Validate(); !errors.Is(err, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("Validate through an inert handle: %v", err)
	}
	if _, err := inert.Prepare(); !errors.Is(err, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("Prepare through an inert handle: %v", err)
	}
	if again := inert.Level[e3Leaf, e2Public, e1Empty]("e3Mid"); !again.rejected || len(d.recorded) != 2 {
		t.Fatal("Level through an inert handle was not forwarded to the frozen definition")
	}
	x := e4Object()
	if err := plan.Init(x, plan.NodeProvider(&x.e3Mid.e1Root), plan.NodeProvider(&x.e3Mid), inert.NodeProvider(x)); err != nil {
		t.Fatalf("retained Plan with a reference made through an inert handle: %v", err)
	}
}
