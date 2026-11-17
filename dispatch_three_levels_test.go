package heiry

import (
	"reflect"
	"slices"
	"testing"
)

type stage2Process interface{ Process() string }
type stage2Validate interface{ Validate() string }
type stage2RootOnly interface{ RootOnly() string }
type stage2Step interface{ Step(int) string }

type stage2A struct {
	scope Scope
	trace *[]string
}

func (a *stage2A) Process() string {
	*a.trace = append(*a.trace, "A.Process")
	return "A/" + Method[stage2Validate](a.scope).Validate()
}

func (a *stage2A) Validate() string {
	*a.trace = append(*a.trace, "A.Validate")
	return "A"
}

func (a *stage2A) RootOnly() string { return "A only" }

type stage2B struct {
	scope Scope
	trace *[]string
}

func (b *stage2B) Validate() string {
	*b.trace = append(*b.trace, "B.Validate")
	return "B/" + ParentMethod[stage2Validate](b.scope).Validate()
}

func (b *stage2B) Step(depth int) string {
	*b.trace = append(*b.trace, "B.Step")
	if depth == 0 {
		return "B0"
	}
	return "B/" + Method[stage2Step](b.scope).Step(depth-1)
}

type stage2C struct {
	scope Scope
	trace *[]string
}

func (c *stage2C) Process() string {
	*c.trace = append(*c.trace, "C.Process")
	process := ParentMethod[stage2Process](c.scope).Process()
	validation := ParentMethod[stage2Validate](c.scope).Validate()
	return "C/" + process + "|" + validation
}

func (c *stage2C) Step(depth int) string {
	*c.trace = append(*c.trace, "C.Step")
	if depth == 0 {
		return "C0"
	}
	return "C/" + ParentMethod[stage2Step](c.scope).Step(depth-1)
}

func stage2Fixture() (*dispatch, *stage2A, *stage2B, *stage2C, *[]string) {
	trace := new([]string)
	a := &stage2A{trace: trace}
	b := &stage2B{trace: trace}
	c := &stage2C{trace: trace}
	d := &dispatch{
		known: map[reflect.Type]struct{}{
			reflect.TypeFor[stage2Process]():  {},
			reflect.TypeFor[stage2Validate](): {},
			reflect.TypeFor[stage2RootOnly](): {},
			reflect.TypeFor[stage2Step]():     {},
		},
		levels: []map[reflect.Type]any{
			{
				reflect.TypeFor[stage2Process]():  a,
				reflect.TypeFor[stage2Validate](): a,
				reflect.TypeFor[stage2RootOnly](): a,
			},
			{
				reflect.TypeFor[stage2Validate](): b,
				reflect.TypeFor[stage2Step]():     b,
			},
			{
				reflect.TypeFor[stage2Process](): c,
				reflect.TypeFor[stage2Step]():    c,
			},
		},
	}
	a.scope = Scope{dispatch: d, level: 0}
	b.scope = Scope{dispatch: d, level: 1}
	c.scope = Scope{dispatch: d, level: 2}
	return d, a, b, c, trace
}

func TestStage2SkippedLevelsAndNestedDifferentMethodCalls(t *testing.T) {
	d, a, b, c, trace := stage2Fixture()
	leaf := Scope{dispatch: d, level: 2}

	if got := Method[stage2Process](leaf).Process(); got != "C/A/B/A|B/A" {
		t.Fatalf("Process = %q", got)
	}
	want := []string{"C.Process", "A.Process", "B.Validate", "A.Validate", "B.Validate", "A.Validate"}
	if !slices.Equal(*trace, want) {
		t.Fatalf("trace = %v, want %v", *trace, want)
	}
	for level, scope := range []Scope{a.scope, b.scope, c.scope} {
		if got := Method[stage2Process](scope); got != c {
			t.Fatalf("virtual Process from level %d = %T, want C", level, got)
		}
	}
	if got := a.Process(); got != "A/B/A" {
		t.Fatalf("direct A.Process = %q", got)
	}
	if got := ParentMethod[stage2Process](c.scope); got != a {
		t.Fatalf("C.Process parent = %T, want A", got)
	}
	if got := ParentMethod[stage2Process](b.scope); got != a {
		t.Fatalf("B.Process parent = %T, want A", got)
	}
	if got := Method[stage2Validate](leaf); got != b {
		t.Fatalf("virtual Validate = %T, want B", got)
	}
	if got := ParentMethod[stage2Validate](c.scope); got != b {
		t.Fatalf("C.Validate parent = %T, want B", got)
	}
	if got := ParentMethod[stage2Validate](b.scope); got != a {
		t.Fatalf("B.Validate parent = %T, want A", got)
	}
	assertDispatchPanic(t, NoParentImplementation, reflect.TypeFor[stage2Validate](), 0, func() {
		ParentMethod[stage2Validate](a.scope)
	})
}

func TestStage2RootOnlyAndAbsentRootImplementation(t *testing.T) {
	d, a, b, c, _ := stage2Fixture()
	leaf := Scope{dispatch: d, level: 2}
	if got := Method[stage2RootOnly](leaf); got != a {
		t.Fatalf("root-only virtual = %T, want A", got)
	}
	if got := ParentMethod[stage2RootOnly](c.scope); got != a {
		t.Fatalf("root-only parent from C = %T, want A", got)
	}
	if got := ParentMethod[stage2RootOnly](b.scope); got != a {
		t.Fatalf("root-only parent from B = %T, want A", got)
	}
	assertDispatchPanic(t, NoParentImplementation, reflect.TypeFor[stage2RootOnly](), 0, func() {
		ParentMethod[stage2RootOnly](a.scope)
	})
	assertDispatchPanic(t, NoParentImplementation, reflect.TypeFor[stage2Step](), 1, func() {
		ParentMethod[stage2Step](b.scope)
	})
}

func TestStage2FiniteVirtualReentryAndParentDescent(t *testing.T) {
	d, _, b, c, trace := stage2Fixture()
	if got := Method[stage2Step](Scope{dispatch: d, level: 2}).Step(4); got != "C/B/C/B/C0" {
		t.Fatalf("Step = %q", got)
	}
	want := []string{"C.Step", "B.Step", "C.Step", "B.Step", "C.Step"}
	if !slices.Equal(*trace, want) {
		t.Fatalf("trace = %v, want %v", *trace, want)
	}
	if got := ParentMethod[stage2Step](c.scope); got != b {
		t.Fatalf("C.Step parent = %T, want B", got)
	}
}
