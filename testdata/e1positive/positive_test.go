package e1positive

import (
	"errors"
	"testing"

	"github.com/lucaf/heiry"
)

type (
	read   interface{ Read() int }
	write  interface{ Write(value int) }
	secret interface{ Secret() int }

	rootPublic interface{ read }
	leafPublic interface {
		read
		write
	}
	private interface{ secret }
)

type root struct {
	heiry.Node
	value int
}

func (r *root) Read() int   { return r.value + r.Node.Virtual[secret]().Secret() }
func (r *root) Secret() int { return 1 }

type middle struct {
	root
	heiry.Node
}

type leaf struct {
	middle
	heiry.Node
	value int
}

func (l *leaf) Read() int       { return l.value + l.Node.Parent[read]().Read() }
func (l *leaf) Write(value int) { l.value = value }
func (l *leaf) Secret() int     { return 10 * l.Node.Parent[secret]().Secret() }

type view struct{ state *leaf }

func (v *view) Read() int       { return v.state.Node.Virtual[read]().Read() }
func (v *view) Write(value int) { v.state.Node.Virtual[write]().Write(value) }

// hierarchy is a level handle: the chain needs no End, and the handle
// prepares and initializes through its definition.
var hierarchy = heiry.Define[leaf]().
	Level[root, rootPublic, private]("").
	BindNodeGroup((*root)(nil), heiry.Of[read](), heiry.Of[secret]()).
	Extend[middle]().
	ExtendWith[leaf, leafPublic, private]().
	BindNode[read]((*leaf)(nil)).
	BindNode[write]((*leaf)(nil)).
	BindNode[secret]((*leaf)(nil))

func newLeaf(rootValue, leafValue int) *leaf {
	return &leaf{middle: middle{root: root{value: rootValue}}, value: leafValue}
}

func TestExternalConciseDeclarationAndHolderDispatch(t *testing.T) {
	dynamic, err := hierarchy.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	for name, plan := range map[string]*heiry.Plan[leaf]{"dynamic": dynamic, "precomputed": dynamic.MustPrecompute()} {
		x := newLeaf(2, 3)
		// References come from the handle and from the Plan; both share one lineage.
		err := plan.Init(x, hierarchy.NodeProvider(&x.middle.root), plan.NodeProvider(&x.middle), hierarchy.NodeProvider(x))
		if err != nil {
			t.Fatalf("%s Init: %v", name, err)
		}
		var public leafPublic = &view{state: x}
		public.Write(5)
		// leaf 5 + root (2 + virtual Secret: 10 * root Secret 1)
		if got := public.Read(); got != 17 {
			t.Fatalf("%s Read = %d, want 17", name, got)
		}
		if _, exposed := any(public).(secret); exposed {
			t.Fatalf("%s: the public view exposes the private operation", name)
		}
		// The inherited middle level registers nothing; its holder is at level 1.
		if x.middle.Node.Parent[read]() != read(&x.middle.root) || x.middle.Node.Virtual[read]() != read(x) {
			t.Fatalf("%s: the inherited level's holder resolves incorrectly", name)
		}
	}
	y := newLeaf(1, 1)
	if err := hierarchy.Init(y, hierarchy.NodeProvider(&y.middle.root), hierarchy.NodeProvider(&y.middle), hierarchy.NodeProvider(y)); err != nil {
		t.Fatalf("Init through the handle: %v", err)
	}
	other := newLeaf(1, 1)
	err = hierarchy.Init(other, hierarchy.NodeProvider(&y.middle.root), hierarchy.NodeProvider(&other.middle), hierarchy.NodeProvider(other))
	if !errors.Is(err, &heiry.Error{Code: heiry.ProviderOwnership}) || other.Node.Scope() != (heiry.Scope{}) {
		t.Fatalf("foreign node through the handle: %v", err)
	}
}

func TestExternalNilAndUninitializedHolders(t *testing.T) {
	for name, call := range map[string]func(){
		"nil holder":           func() { (*heiry.Node)(nil).Virtual[read]() },
		"uninitialized holder": func() { newLeaf(0, 0).Node.Parent[read]() },
	} {
		func() {
			defer func() {
				var diagnostic *heiry.Error
				recovered, _ := recover().(error)
				if !errors.As(recovered, &diagnostic) || diagnostic.Code != heiry.Uninitialized {
					t.Fatalf("%s: recovered %v", name, recovered)
				}
			}()
			call()
		}()
	}
}
