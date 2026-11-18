package heiry_test

import (
	"testing"

	"github.com/lucaf/heiry"
)

type m7Read interface{ Read() int }
type m7Secret interface{ Secret() int }
type m7Public interface{ Read() int }
type m7Private interface{ Secret() int }

type m7Root struct {
	heiry.Node
	value int
}

func (r *m7Root) Read() int   { return r.value }
func (r *m7Root) Secret() int { return r.value * 2 }

type m7Leaf struct {
	m7Root
	heiry.Node
	value int
}

func (l *m7Leaf) Read() int {
	return l.value + heiry.ParentMethod[m7Read](l.Node.Scope()).Read() + heiry.Method[m7Secret](l.Node.Scope()).Secret()
}

type m7PublicView struct{ state *m7Leaf }

func (v *m7PublicView) Read() int { return heiry.Method[m7Read](v.state.Node.Scope()).Read() }

func m7ExternalDefinition() *heiry.Definition[m7Leaf] {
	d := heiry.Define[m7Leaf]()
	d.Level[m7Root, m7Public, m7Private]("").
		BindGroup((*m7Root)(nil), ".", heiry.Of[m7Read](), heiry.Of[m7Secret]()).End().
		Level[m7Leaf, m7Public, m7Private]("m7Root").
		Bind[m7Read]((*m7Leaf)(nil), ".")
	return d
}

func TestM7ExternalChainedAndSeparatePrecomputedLifecycle(t *testing.T) {
	d := m7ExternalDefinition()
	chained := &m7Leaf{m7Root: m7Root{value: 2}, value: 3}
	if got := d.MustPrepare().MustPrecompute().MustInit(chained,
		d.NodeProvider(&chained.m7Root), d.NodeProvider(chained)); got != chained {
		t.Fatalf("chained MustInit returned %p, want %p", got, chained)
	}
	separateDefinition := m7ExternalDefinition()
	dynamic, err := separateDefinition.Prepare()
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	fast, err := dynamic.Precompute()
	if err != nil {
		t.Fatalf("Precompute: %v", err)
	}
	separate := &m7Leaf{m7Root: m7Root{value: 2}, value: 3}
	if err := fast.Init(separate, fast.NodeProvider(&separate.m7Root), fast.NodeProvider(separate)); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for _, leaf := range []*m7Leaf{chained, separate} {
		var public m7Public = &m7PublicView{state: leaf}
		if got := public.Read(); got != 9 {
			t.Fatalf("public Read = %d", got)
		}
		if _, exposed := any(public).(m7Private); exposed {
			t.Fatal("public wrapper exposes private method")
		}
	}
}
