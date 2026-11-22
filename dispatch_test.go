package heiry

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type stage1Calculate interface {
	Calculate(int, int) (int, error)
}

type stage1Report interface {
	Report(string, ...int) (string, int, error)
}

type stage1Unknown interface {
	Unknown()
}

var stage1ApplicationError = errors.New("application failure")

type stage1A struct{ scope Scope }

func (a *stage1A) Calculate(left, right int) (int, error) {
	if left < 0 {
		return 0, stage1ApplicationError
	}
	return left + right, nil
}

func (a *stage1A) Report(label string, values ...int) (string, int, error) {
	sum := 0
	for _, value := range values {
		sum += value
	}
	return fmt.Sprintf("%s:%d", label, sum), len(values), nil
}

type stage1B struct{ scope Scope }

func (b *stage1B) Calculate(left, right int) (int, error) {
	value, err := ParentMethod[stage1Calculate](b.scope).Calculate(left, right)
	if err != nil {
		return 0, err
	}
	return value * 3, nil
}

func (b *stage1B) Report(label string, values ...int) (string, int, error) {
	report, count, err := ParentMethod[stage1Report](b.scope).Report(label, values...)
	return "B/" + report, count, err
}

func stage1Fixture() (*dispatch, *stage1A, *stage1B) {
	a, b := new(stage1A), new(stage1B)
	d := &dispatch{
		known: map[reflect.Type]struct{}{
			reflect.TypeFor[stage1Calculate](): {},
			reflect.TypeFor[stage1Report]():    {},
		},
		levels: []map[reflect.Type]any{
			{
				reflect.TypeFor[stage1Calculate](): a,
				reflect.TypeFor[stage1Report]():    a,
			},
			{
				reflect.TypeFor[stage1Calculate](): b,
				reflect.TypeFor[stage1Report]():    b,
			},
		},
	}
	a.scope = Scope{dispatch: d, level: 0}
	b.scope = Scope{dispatch: d, level: 1}
	return d, a, b
}

func TestStage1TypedDynamicDispatch(t *testing.T) {
	d, a, b := stage1Fixture()
	leaf := Scope{dispatch: d, level: 1}

	calculator := Method[stage1Calculate](leaf)
	if calculator != b {
		t.Fatalf("virtual Calculate selected %T, want B", calculator)
	}
	if got, err := calculator.Calculate(2, 5); got != 21 || err != nil {
		t.Fatalf("Calculate = (%d, %v), want (21, nil)", got, err)
	}
	if got, err := calculator.Calculate(-1, 5); got != 0 || !errors.Is(err, stage1ApplicationError) {
		t.Fatalf("application error = (%d, %v)", got, err)
	}
	if got, err := a.Calculate(2, 5); got != 7 || err != nil {
		t.Fatalf("direct A Calculate = (%d, %v)", got, err)
	}

	reporter := Method[stage1Report](leaf)
	if reporter != b {
		t.Fatalf("virtual Report selected %T, want B", reporter)
	}
	if got, count, err := reporter.Report("items", 2, 3, 4); got != "B/items:9" || count != 3 || err != nil {
		t.Fatalf("variadic Report = (%q, %d, %v)", got, count, err)
	}
}

func TestStage1MissingParentAndUnknownCapability(t *testing.T) {
	_, a, _ := stage1Fixture()
	assertDispatchPanic(t, NoParentImplementation, reflect.TypeFor[stage1Calculate](), 0, func() {
		ParentMethod[stage1Calculate](a.scope)
	})
	assertDispatchPanic(t, UnknownCapability, reflect.TypeFor[stage1Unknown](), 0, func() {
		Method[stage1Unknown](a.scope)
	})
}

func assertDispatchPanic(t *testing.T, code Code, capability reflect.Type, level int, call func()) {
	t.Helper()
	defer func() {
		value := recover()
		if value == nil {
			t.Fatalf("expected panic with code %s", code)
		}
		err, ok := value.(*Error)
		if !ok {
			t.Fatalf("panic = %T, want *Error", value)
		}
		if err.Code != code || !errors.Is(err, &Error{Code: code}) {
			t.Fatalf("panic = %v, want code %s", err, code)
		}
		if err.Capability != capability || err.Level != level {
			t.Fatalf("panic context = (%v, %d), want (%v, %d)", err.Capability, err.Level, capability, level)
		}
		var typed *Error
		if !errors.As(err, &typed) || typed != err {
			t.Fatalf("errors.As did not preserve diagnostic: %v", err)
		}
	}()
	call()
}

// nodeBinding is one node provider at a level, as input to bindNodeLevels.
// Tests supply pointers to the leaf's actual embedded subobjects.
type nodeBinding struct {
	node         *Node
	provider     any
	capabilities []reflect.Type
}

// bindNodeLevels creates one object's provider table and installs a fixed
// scope in each embedded node. It trusts the caller's pointers and performs
// none of the validation or ownership checks of Plan.Init.
func bindNodeLevels(bindings []nodeBinding) *dispatch {
	d := &dispatch{
		known:  make(map[reflect.Type]struct{}),
		levels: make([]map[reflect.Type]any, len(bindings)),
	}
	for level, binding := range bindings {
		methods := make(map[reflect.Type]any, len(binding.capabilities))
		for _, capability := range binding.capabilities {
			d.known[capability] = struct{}{}
			methods[capability] = binding.provider
		}
		d.levels[level] = methods
	}
	for level, binding := range bindings {
		binding.node.scope = Scope{dispatch: d, level: level}
	}
	return d
}
