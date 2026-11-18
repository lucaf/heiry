package heiry

import (
	"errors"
	"testing"
)

func e1Recover(call func()) (diagnostic *Error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if err, ok := recovered.(error); ok && errors.As(err, &diagnostic) {
				return
			}
			panic(recovered)
		}
	}()
	call()
	return nil
}

func TestNodeDispatchNilAndUninitializedHolders(t *testing.T) {
	capability := m7OracleCapabilities[0]
	var absent *Node
	for name, call := range map[string]func(){
		"nil Virtual":           func() { absent.Virtual[stage3Read]() },
		"nil Parent":            func() { absent.Parent[stage3Read]() },
		"uninitialized Virtual": func() { new(Node).Virtual[stage3Read]() },
		"uninitialized Parent":  func() { new(Node).Parent[stage3Read]() },
	} {
		diagnostic := e1Recover(call)
		if diagnostic == nil || diagnostic.Code != Uninitialized || diagnostic.Capability != capability {
			t.Fatalf("%s: diagnostic = %+v", name, diagnostic)
		}
	}
}

// e1InvokeHolder mirrors m7InvokeOracle through the holder methods.
func e1InvokeHolder(holder *Node, operation int, parent bool) (provider any, diagnostic *Error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if err, ok := recovered.(error); ok && errors.As(err, &diagnostic) {
				return
			}
			panic(recovered)
		}
	}()
	switch operation {
	case 0:
		if parent {
			return holder.Parent[stage3Read](), nil
		}
		return holder.Virtual[stage3Read](), nil
	case 1:
		if parent {
			return holder.Parent[stage3Add](), nil
		}
		return holder.Virtual[stage3Add](), nil
	default:
		if parent {
			return holder.Parent[stage3Walk](), nil
		}
		return holder.Virtual[stage3Walk](), nil
	}
}

func e1SameDiagnostic(a, b *Error) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Code == b.Code && a.Capability == b.Capability && a.Level == b.Level
}

type (
	e1Read  interface{ Read() int }
	e1Empty interface{}
)

type e1Root struct {
	Node
	value int
}

func (r *e1Root) Read() int { return r.value }

// E1Dispatch is exported because the binder reaches a holder through its
// field, and an unexported embedded field cannot be extracted.
type E1Dispatch = Node

// e1AliasLeaf embeds its own holder under an alias name, so its promoted
// Node selector names the ancestor's holder.
type e1AliasLeaf struct {
	e1Root
	E1Dispatch
	value int
}

func (l *e1AliasLeaf) Read() int { return l.value + l.E1Dispatch.Parent[e1Read]().Read() }

func TestNodeDispatchAliasNamedHolderAboveAncestorNode(t *testing.T) {
	d := Define[e1AliasLeaf]()
	d.Level[e1Root, e1Read, e1Empty]("").Bind[e1Read]((*e1Root)(nil), ".")
	d.Level[e1AliasLeaf, e1Read, e1Empty]("e1Root").Bind[e1Read]((*e1AliasLeaf)(nil), ".")
	dynamic := d.MustPrepare()
	for name, plan := range map[string]*Plan[e1AliasLeaf]{"dynamic": dynamic, "precomputed": dynamic.MustPrecompute()} {
		x := &e1AliasLeaf{e1Root: e1Root{value: 2}, value: 3}
		if err := plan.Init(x, plan.NodeProvider(&x.e1Root), plan.NodeProvider(x)); err != nil {
			t.Fatalf("%s Init: %v", name, err)
		}
		if x.E1Dispatch.Scope().level != 1 || x.Node.Scope().level != 0 {
			t.Fatalf("%s holder levels: alias=%d promoted Node=%d", name, x.E1Dispatch.Scope().level, x.Node.Scope().level)
		}
		if x.E1Dispatch.Virtual[e1Read]() != e1Read(x) || x.E1Dispatch.Parent[e1Read]() != e1Read(&x.e1Root) {
			t.Fatalf("%s: the alias-named holder did not resolve from the leaf level", name)
		}
		if got := x.E1Dispatch.Virtual[e1Read]().Read(); got != 5 {
			t.Fatalf("%s Read = %d, want 5", name, got)
		}
		// The promoted Node selector is the ancestor's holder: from the root
		// level there is no parent implementation.
		diagnostic := e1Recover(func() { x.Node.Parent[e1Read]() })
		if diagnostic == nil || diagnostic.Code != NoParentImplementation || diagnostic.Level != 0 {
			t.Fatalf("%s ancestor holder parent call: %+v", name, diagnostic)
		}
	}
}

// e1Tree has an application field named Parent. The promoted spelling does
// not compile for it (see testdata/e1negativeshadow); the holder form does.
type e1Tree struct {
	Node
	Parent *e1Tree
	value  int
}

func (n *e1Tree) Read() int { return n.value }

// e1Shadow declares an application method with the library method's name and
// shape, which silently replaces the promoted spelling.
type e1Shadow struct {
	e1Root
	Node
	value int
}

func (s *e1Shadow) Read() int { return s.value }

func (s *e1Shadow) Parent[T any]() T {
	var zero T
	return zero
}

func TestNodeDispatchExplicitHolderFormSurvivesApplicationNames(t *testing.T) {
	tree := &e1Tree{value: 7}
	treeDefinition := Define[e1Tree]()
	treeDefinition.Level[e1Tree, e1Read, e1Empty]("").Bind[e1Read]((*e1Tree)(nil), ".")
	if err := treeDefinition.Init(tree, treeDefinition.NodeProvider(tree)); err != nil {
		t.Fatal(err)
	}
	if tree.Node.Virtual[e1Read]() != e1Read(tree) {
		t.Fatal("holder form failed beside an application Parent field")
	}
	if diagnostic := e1Recover(func() { tree.Node.Parent[e1Read]() }); diagnostic == nil || diagnostic.Code != NoParentImplementation {
		t.Fatalf("root parent call through the holder: %+v", diagnostic)
	}

	shadow := &e1Shadow{e1Root: e1Root{value: 2}, value: 3}
	shadowDefinition := Define[e1Shadow]()
	shadowDefinition.Level[e1Root, e1Read, e1Empty]("").Bind[e1Read]((*e1Root)(nil), ".")
	shadowDefinition.Level[e1Shadow, e1Read, e1Empty]("e1Root").Bind[e1Read]((*e1Shadow)(nil), ".")
	if err := shadowDefinition.Init(shadow, shadowDefinition.NodeProvider(&shadow.e1Root), shadowDefinition.NodeProvider(shadow)); err != nil {
		t.Fatal(err)
	}
	// The promoted spelling compiles and reaches the application's method.
	if shadow.Parent[e1Read]() != nil {
		t.Fatal("application Parent method was expected to shadow the promoted one")
	}
	// The holder form still reaches the library.
	if shadow.Node.Parent[e1Read]() != e1Read(&shadow.e1Root) {
		t.Fatal("holder form did not reach the library resolver")
	}
}

var e1Sink any

// BenchmarkNodeDispatchForms compares the holder methods with the existing
// functions on the same holder, for a virtual and a parent lookup.
func BenchmarkNodeDispatchForms(b *testing.B) {
	d := Define[e1AliasLeaf]()
	d.Level[e1Root, e1Read, e1Empty]("").BindNode[e1Read]((*e1Root)(nil))
	d.Level[e1AliasLeaf, e1Read, e1Empty]("e1Root").BindNode[e1Read]((*e1AliasLeaf)(nil))
	dynamic := d.MustPrepare()
	for _, mode := range []struct {
		name string
		plan *Plan[e1AliasLeaf]
	}{{"dynamic", dynamic}, {"precomputed", dynamic.MustPrecompute()}} {
		x := &e1AliasLeaf{}
		if err := mode.plan.Init(x, mode.plan.NodeProvider(&x.e1Root), mode.plan.NodeProvider(x)); err != nil {
			b.Fatal(err)
		}
		holder := &x.E1Dispatch
		for _, tc := range []struct {
			name string
			call func() e1Read
		}{
			{"virtual/function", func() e1Read { return Method[e1Read](holder.Scope()) }},
			{"virtual/holder", func() e1Read { return holder.Virtual[e1Read]() }},
			{"parent/function", func() e1Read { return ParentMethod[e1Read](holder.Scope()) }},
			{"parent/holder", func() e1Read { return holder.Parent[e1Read]() }},
		} {
			b.Run(mode.name+"/"+tc.name, func(b *testing.B) {
				b.ReportAllocs()
				var selected e1Read
				for b.Loop() {
					selected = tc.call()
				}
				e1Sink = selected
			})
		}
	}
}
