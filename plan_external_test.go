package heiry_test

import (
	"testing"

	"github.com/lucaf/heiry"
)

type m5Read interface{ Read() int }
type m5Empty interface{}

type m5Root struct {
	heiry.Node
	value int
}

func (r *m5Root) Read() int { return r.value }

type m5Leaf struct {
	m5Root
	heiry.Node
	value int
}

func (l *m5Leaf) Read() int {
	return l.value + heiry.ParentMethod[m5Read](l.Node.Scope()).Read()
}

type m5Public interface{ Read() int }

type m5Wrapper struct{ state *m5Leaf }

func (w *m5Wrapper) Read() int {
	return heiry.Method[m5Read](w.state.Node.Scope()).Read()
}

func m5PreparedPlan() (*heiry.Plan[m5Leaf], error) {
	definition := heiry.Define[m5Leaf]()
	definition.Level[m5Root, m5Read, m5Empty]("").
		Bind[m5Read]((*m5Root)(nil), ".").End().
		Level[m5Leaf, m5Read, m5Empty]("m5Root").
		Bind[m5Read]((*m5Leaf)(nil), ".")
	return definition.Prepare()
}

func m5NativeConstructor(root, leaf int) *m5Leaf {
	return &m5Leaf{m5Root: m5Root{value: root}, value: leaf}
}

func TestM5ExternalPlanWithoutDeclarationHandles(t *testing.T) {
	plan, err := m5PreparedPlan()
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	for _, tt := range []struct{ root, leaf, want int }{{1, 2, 3}, {10, 20, 30}} {
		object := m5NativeConstructor(tt.root, tt.leaf)
		if err := plan.Init(object,
			plan.NodeProvider(&object.m5Root),
			plan.NodeProvider(object)); err != nil {
			t.Fatalf("initialize: %v", err)
		}
		var public m5Public = &m5Wrapper{state: object}
		if got := public.Read(); got != tt.want {
			t.Fatalf("Read = %d, want %d", got, tt.want)
		}
	}
}
