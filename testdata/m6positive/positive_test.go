package m6positive

import (
	"testing"

	"github.com/lucaf/heiry"
)

type read interface{ Read() int }
type secret interface{ Secret() int }
type public interface{ Read() int }
type private interface{ Secret() int }

type root struct {
	heiry.Node
	value int
}

func (r *root) Read() int   { return r.value }
func (r *root) Secret() int { return r.value * 2 }

type leaf struct {
	root
	heiry.Node
	value int
}

func (l *leaf) Read() int {
	return l.value + heiry.ParentMethod[read](l.Node.Scope()).Read() + heiry.Method[secret](l.Node.Scope()).Secret()
}

type wrapper struct{ state *leaf }

func (w *wrapper) Read() int { return heiry.Method[read](w.state.Node.Scope()).Read() }

func newLeaf(rootValue, leafValue int) *leaf {
	return &leaf{root: root{value: rootValue}, value: leafValue}
}

func TestGroupedPublicFluentLifecycle(t *testing.T) {
	d := heiry.Define[leaf]()
	plan := d.Level[root, public, private]("").
		BindGroup((*root)(nil), ".", heiry.Of[read](), heiry.Of[secret]()).End().
		Level[leaf, public, private]("root").
		Bind[read]((*leaf)(nil), ".").End().MustPrepare()
	object := newLeaf(2, 3)
	if err := plan.Init(object, plan.NodeProvider(&object.root), plan.NodeProvider(object)); err != nil {
		t.Fatalf("Init: %v", err)
	}
	var publicView public = &wrapper{state: object}
	if got := publicView.Read(); got != 9 {
		t.Fatalf("Read = %d, want 9", got)
	}
	if _, leaked := any(publicView).(private); leaked {
		t.Fatal("public wrapper exposes private method")
	}
}

func TestFullChainedLifecycleMatchesSeparateCalls(t *testing.T) {
	d := heiry.Define[leaf]()
	chainedObject := newLeaf(2, 3)
	returned := d.Level[root, public, private]("").
		BindGroup((*root)(nil), ".", heiry.Of[read](), heiry.Of[secret]()).End().
		Level[leaf, public, private]("root").
		Bind[read]((*leaf)(nil), ".").End().
		MustPrepare().MustInit(chainedObject,
		d.NodeProvider(&chainedObject.root),
		d.NodeProvider(chainedObject))
	if returned != chainedObject {
		t.Fatal("full chain returned a different leaf pointer")
	}

	separate := heiry.Define[leaf]()
	rootLevel := separate.Level[root, public, private]("")
	leafLevel := separate.Level[leaf, public, private]("root")
	rootLevel.BindGroup((*root)(nil), ".", heiry.Of[read](), heiry.Of[secret]())
	leafLevel.Bind[read]((*leaf)(nil), ".")
	plan, err := separate.Prepare()
	if err != nil {
		t.Fatalf("separate Prepare: %v", err)
	}
	separateObject := newLeaf(2, 3)
	if err := plan.Init(separateObject,
		plan.NodeProvider(&separateObject.root),
		plan.NodeProvider(separateObject)); err != nil {
		t.Fatalf("separate Init: %v", err)
	}
	chainedView := &wrapper{state: chainedObject}
	separateView := &wrapper{state: separateObject}
	if chainedView.Read() != 9 || separateView.Read() != 9 {
		t.Fatalf("initial dispatch differs: chain=%d separate=%d", chainedView.Read(), separateView.Read())
	}
	chainedObject.root.value += 5
	chainedObject.value += 7
	if got := chainedView.Read(); got != 31 || separateView.Read() != 9 {
		t.Fatalf("independent embedded state: chain=%d separate=%d", got, separateView.Read())
	}
	separateObject.root.value += 5
	separateObject.value += 7
	if got := separateView.Read(); got != chainedView.Read() || got != 31 {
		t.Fatalf("updated dispatch differs: chain=%d separate=%d", chainedView.Read(), got)
	}
}
