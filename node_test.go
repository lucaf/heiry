package heiry

import (
	"reflect"
	"sync"
	"testing"
)

type stage3Read interface{ Read() int }
type stage3Add interface{ Add(int) int }
type stage3Walk interface{ Walk(int) string }

type stage3A struct {
	Node
	value int
}

func (a *stage3A) Read() int { return a.value }
func (a *stage3A) Add(n int) int {
	a.value += n
	return a.value
}
func (a *stage3A) Walk(int) string { return "A" }

type stage3B struct {
	stage3A
	Node
	value int
}

func (b *stage3B) Read() int {
	return b.value + ParentMethod[stage3Read](b.Node.Scope()).Read()
}
func (b *stage3B) Add(n int) int {
	b.value += n
	return b.value + ParentMethod[stage3Add](b.Node.Scope()).Add(n)
}
func (b *stage3B) Walk(n int) string {
	if n == 0 {
		return "B/" + ParentMethod[stage3Walk](b.Node.Scope()).Walk(0)
	}
	return "B/" + Method[stage3Walk](b.Node.Scope()).Walk(n-1)
}

type stage3C struct {
	stage3B
	Node
	value int
}

func (c *stage3C) Read() int {
	return c.value + ParentMethod[stage3Read](c.Node.Scope()).Read()
}
func (c *stage3C) Add(n int) int {
	c.value += n
	return c.value + ParentMethod[stage3Add](c.Node.Scope()).Add(n)
}
func (c *stage3C) Walk(n int) string {
	return "C/" + ParentMethod[stage3Walk](c.Node.Scope()).Walk(n)
}

func stage3Fixture(a, b, c int) *stage3C {
	leaf := &stage3C{}
	leaf.stage3A.value, leaf.stage3B.value, leaf.value = a, b, c
	bindNodeLevels([]nodeBinding{
		{node: &leaf.stage3A.Node, provider: &leaf.stage3A, capabilities: []reflect.Type{reflect.TypeFor[stage3Read](), reflect.TypeFor[stage3Add](), reflect.TypeFor[stage3Walk]()}},
		{node: &leaf.stage3B.Node, provider: &leaf.stage3B, capabilities: []reflect.Type{reflect.TypeFor[stage3Read](), reflect.TypeFor[stage3Add](), reflect.TypeFor[stage3Walk]()}},
		{node: &leaf.Node, provider: leaf, capabilities: []reflect.Type{reflect.TypeFor[stage3Read](), reflect.TypeFor[stage3Add](), reflect.TypeFor[stage3Walk]()}},
	})
	return leaf
}

func TestStage3EmbeddedNodeScopesAndState(t *testing.T) {
	var leaf stage3C
	leaf.stage3A.value = 1
	leaf.stage3B.value = 2
	leaf.value = 3
	if got := leaf.Node.Scope(); got.dispatch != nil {
		t.Fatalf("zero Node scope has dispatch %p", got.dispatch)
	}
	assertDispatchPanic(t, Uninitialized, reflect.TypeFor[stage3Read](), 0, func() {
		Method[stage3Read](leaf.Node.Scope())
	})
	assertDispatchPanic(t, Uninitialized, reflect.TypeFor[stage3Read](), 0, func() {
		ParentMethod[stage3Read](leaf.stage3B.Node.Scope())
	})
}

func TestStage3ActualEmbeddedProvidersAndMutation(t *testing.T) {
	leaf := stage3Fixture(1, 2, 3)
	d := leaf.Node.Scope().dispatch
	for level, want := range []any{&leaf.stage3A, &leaf.stage3B, leaf} {
		if got := d.levels[level][reflect.TypeFor[stage3Add]()]; got != want {
			t.Fatalf("level %d provider = %p, want %p", level, got, want)
		}
	}
	for level, node := range []*Node{&leaf.stage3A.Node, &leaf.stage3B.Node, &leaf.Node} {
		scope := node.Scope()
		if scope.level != level || scope.dispatch != d {
			t.Fatalf("level %d scope = %+v", level, scope)
		}
	}
	if got := Method[stage3Add](leaf.Node.Scope()).Add(4); got != 18 {
		t.Fatalf("Add = %d, want 18", got)
	}
	if leaf.stage3A.value != 5 || leaf.stage3B.value != 6 || leaf.value != 7 {
		t.Fatalf("embedded state = (%d, %d, %d)", leaf.stage3A.value, leaf.stage3B.value, leaf.value)
	}
}

func TestStage3IndependentObjectsAndReentry(t *testing.T) {
	first, second := stage3Fixture(1, 2, 3), stage3Fixture(10, 20, 30)
	if first.Node.Scope().dispatch == second.Node.Scope().dispatch {
		t.Fatal("independent leaves share dispatch")
	}
	if got := Method[stage3Walk](first.Node.Scope()).Walk(1); got != "C/B/C/B/A" {
		t.Fatalf("reentry trace = %q", got)
	}
	if got := Method[stage3Add](first.Node.Scope()).Add(1); got != 9 {
		t.Fatalf("first Add = %d", got)
	}
	if got := Method[stage3Read](second.Node.Scope()).Read(); got != 60 {
		t.Fatalf("second Read = %d, want 60", got)
	}
	if second.stage3A.value != 10 || second.stage3B.value != 20 || second.value != 30 {
		t.Fatal("first object changed second object's embedded state")
	}
	for level, want := range []any{&second.stage3A, &second.stage3B, second} {
		if got := second.Node.Scope().dispatch.levels[level][reflect.TypeFor[stage3Read]()]; got != want {
			t.Fatalf("second level %d provider = %p, want %p", level, got, want)
		}
	}
}

func TestStage3ConcurrentReadOnlyDispatch(t *testing.T) {
	leaf := stage3Fixture(1, 2, 3)
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			for range 100 {
				if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != 6 {
					t.Errorf("concurrent Read = %d, want 6", got)
				}
				if got := ParentMethod[stage3Read](leaf.stage3B.Node.Scope()).Read(); got != 1 {
					t.Errorf("concurrent parent Read = %d, want 1", got)
				}
			}
		})
	}
	workers.Wait()
}
