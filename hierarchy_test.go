package heiry

import (
	"errors"
	"reflect"
	"testing"
)

type stage5Public interface {
	Read() int
	Add(int) int
	Walk(int) string
}

type stage5PointerChild struct {
	*stage3A
	Node
}
type stage5NamedChild struct {
	parent stage3A
	Node
}
type stage5NoNode struct{ stage3A }
type stage5PointerNode struct {
	stage3A
	*Node
}
type stage5Unrelated struct{ Node }

func stage5Definition() []nodeLevel {
	capabilities := []contractBinding{
		{capability: reflect.TypeFor[stage3Read](), provider: reflect.TypeFor[*stage3A]()},
		{capability: reflect.TypeFor[stage3Add](), provider: reflect.TypeFor[*stage3A]()},
		{capability: reflect.TypeFor[stage3Walk](), provider: reflect.TypeFor[*stage3A]()},
	}
	return []nodeLevel{
		{typ: reflect.TypeFor[stage3A](), contracts: contractLevel{public: reflect.TypeFor[stage5Public](), private: reflect.TypeFor[contractEmpty](), bindings: capabilities}},
		{typ: reflect.TypeFor[stage3B](), parentField: "stage3A", contracts: contractLevel{bindings: []contractBinding{
			{capability: reflect.TypeFor[stage3Read](), provider: reflect.TypeFor[*stage3B]()},
			{capability: reflect.TypeFor[stage3Add](), provider: reflect.TypeFor[*stage3B]()},
			{capability: reflect.TypeFor[stage3Walk](), provider: reflect.TypeFor[*stage3B]()},
		}}},
		{typ: reflect.TypeFor[stage3C](), parentField: "stage3B", contracts: contractLevel{bindings: []contractBinding{
			{capability: reflect.TypeFor[stage3Read](), provider: reflect.TypeFor[*stage3C]()},
			{capability: reflect.TypeFor[stage3Add](), provider: reflect.TypeFor[*stage3C]()},
			{capability: reflect.TypeFor[stage3Walk](), provider: reflect.TypeFor[*stage3C]()},
		}}},
	}
}

func TestStage5DefinitionValidationAndActualBindings(t *testing.T) {
	validated, err := validateNodeDefinition(stage5Definition())
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	leaf := &stage3C{}
	leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
	if err := bindValidatedNodes(validated, leaf, []any{&leaf.stage3A, &leaf.stage3B, leaf}); err != nil {
		t.Fatalf("binding: %v", err)
	}
	if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != 6 {
		t.Fatalf("Read = %d", got)
	}
	if got := Method[stage3Add](leaf.Node.Scope()).Add(1); got != 9 || leaf.stage3A.value != 2 {
		t.Fatalf("embedded mutation = %d, root value = %d", got, leaf.stage3A.value)
	}
}

func TestStage5InvalidDefinitionWithoutObject(t *testing.T) {
	tests := []struct {
		name   string
		levels []nodeLevel
		code   Code
	}{
		{"empty", nil, InvalidHierarchy},
		{"repeated type", []nodeLevel{{typ: reflect.TypeFor[stage3A]()}, {typ: reflect.TypeFor[stage3A](), parentField: "stage3A"}}, InvalidHierarchy},
		{"nonnamed type", []nodeLevel{{typ: reflect.StructOf([]reflect.StructField{{Name: "Node", Type: reflect.TypeFor[Node](), Anonymous: true}})}}, InvalidHierarchy},
		{"nonstruct type", []nodeLevel{{typ: reflect.TypeFor[int]()}}, InvalidHierarchy},
		{"pointer parent", []nodeLevel{{typ: reflect.TypeFor[stage3A]()}, {typ: reflect.TypeFor[stage5PointerChild](), parentField: "stage3A"}}, InvalidHierarchy},
		{"wrong parent path", []nodeLevel{{typ: reflect.TypeFor[stage3A]()}, {typ: reflect.TypeFor[stage3B](), parentField: "missing"}}, InvalidHierarchy},
		{"missing intermediate", []nodeLevel{{typ: reflect.TypeFor[stage3A]()}, {typ: reflect.TypeFor[stage3C](), parentField: "stage3B"}}, InvalidHierarchy},
		{"named parent", []nodeLevel{{typ: reflect.TypeFor[stage3A]()}, {typ: reflect.TypeFor[stage5NamedChild](), parentField: "parent"}}, InvalidHierarchy},
		{"missing holder", []nodeLevel{{typ: reflect.TypeFor[stage3A]()}, {typ: reflect.TypeFor[stage5NoNode](), parentField: "stage3A"}}, MissingNode},
		{"pointer holder", []nodeLevel{{typ: reflect.TypeFor[stage3A]()}, {typ: reflect.TypeFor[stage5PointerNode](), parentField: "stage3A"}}, MissingNode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateNodeDefinition(tt.levels)
			if !errors.Is(err, &Error{Code: tt.code}) {
				t.Fatalf("error = %v, want %s", err, tt.code)
			}
		})
	}
}

func TestStage5FailedBindingPreservesAllScopes(t *testing.T) {
	validated, err := validateNodeDefinition(stage5Definition())
	if err != nil {
		t.Fatal(err)
	}
	leaf := &stage3C{}
	foreign := &stage3B{}
	if err := bindValidatedNodes(validated, leaf, []any{&leaf.stage3A, foreign, leaf}); !errors.Is(err, &Error{Code: ProviderOwnership}) {
		t.Fatalf("foreign provider error = %v", err)
	}
	for _, node := range []*Node{&leaf.stage3A.Node, &leaf.stage3B.Node, &leaf.Node} {
		if node.Scope().dispatch != nil {
			t.Fatal("failed binding wrote a scope")
		}
	}
	if err := bindValidatedNodes(validated, leaf, []any{&leaf.stage3A, &leaf.stage3B, leaf}); err != nil {
		t.Fatalf("binding after failure: %v", err)
	}
	before := leaf.Node.Scope().dispatch
	if err := bindValidatedNodes(validated, leaf, []any{&leaf.stage3A, &leaf.stage3B, leaf}); !errors.Is(err, &Error{Code: AlreadyInitialized}) {
		t.Fatalf("repeat binding error = %v", err)
	}
	if leaf.Node.Scope().dispatch != before {
		t.Fatal("repeat binding replaced scope")
	}
}

func TestStage5InvalidInstanceInputsAreTypedAndAtomic(t *testing.T) {
	validated, err := validateNodeDefinition(stage5Definition())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		leaf      any
		providers func(*stage3C) []any
		code      Code
	}{
		{"nil target", (*stage3C)(nil), func(c *stage3C) []any { return []any{&c.stage3A, &c.stage3B, c} }, InvalidProvider},
		{"nonpointer target", 42, func(c *stage3C) []any { return []any{&c.stage3A, &c.stage3B, c} }, InvalidProvider},
		{"nil provider", nil, func(c *stage3C) []any { return []any{&c.stage3A, (*stage3B)(nil), c} }, InvalidProvider},
		{"wrong provider type", nil, func(c *stage3C) []any { return []any{&c.stage3A, &stage5Unrelated{}, c} }, InvalidProvider},
		{"provider value", nil, func(c *stage3C) []any { return []any{&c.stage3A, 42, c} }, InvalidProvider},
		{"missing provider", nil, func(c *stage3C) []any { return []any{&c.stage3A, c} }, BindingMismatch},
		{"foreign leaf", nil, func(c *stage3C) []any { return []any{&c.stage3A, &c.stage3B, &stage3C{}} }, ProviderOwnership},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leaf := &stage3C{}
			target := tt.leaf
			if target == nil {
				target = leaf
			}
			err := bindValidatedNodes(validated, target, tt.providers(leaf))
			if !errors.Is(err, &Error{Code: tt.code}) {
				t.Fatalf("error = %v, want %s", err, tt.code)
			}
			if tt.name == "wrong provider type" {
				var typed *Error
				if !errors.As(err, &typed) || typed.Actual != reflect.TypeFor[*stage5Unrelated]() || typed.Level != 1 {
					t.Fatalf("wrong-provider context = %+v", typed)
				}
			}
			for _, node := range []*Node{&leaf.stage3A.Node, &leaf.stage3B.Node, &leaf.Node} {
				if node.Scope().dispatch != nil {
					t.Fatal("failed instance check wrote scope")
				}
			}
		})
	}
}

func TestStage5RestrictedParentReflection(t *testing.T) {
	leaf := &stage3C{}
	parent := reflect.ValueOf(leaf).Elem().FieldByName("stage3B")
	if !parent.IsValid() || !parent.CanAddr() || parent.CanInterface() || parent.CanSet() {
		t.Fatalf("parent reflection flags = (%t, %t, %t)", parent.CanAddr(), parent.CanInterface(), parent.CanSet())
	}
	assertReflectionPanic := func(name string, call func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s did not panic", name)
			}
		}()
		call()
	}
	assertReflectionPanic("Interface", func() { parent.Interface() })
	assertReflectionPanic("Addr.Interface", func() { parent.Addr().Interface() })
	assertReflectionPanic("Set", func() { parent.Set(parent) })
	validated, err := validateNodeDefinition(stage5Definition())
	if err != nil {
		t.Fatal(err)
	}
	if err := bindValidatedNodes(validated, leaf, []any{&leaf.stage3A, &leaf.stage3B, leaf}); err != nil {
		t.Fatalf("restricted parent binding: %v", err)
	}
}

func TestStage5ValidatedDefinitionIsDetachedFromInput(t *testing.T) {
	levels := stage5Definition()
	validated, err := validateNodeDefinition(levels)
	if err != nil {
		t.Fatal(err)
	}
	levels[0].contracts.bindings[0].capability = reflect.TypeFor[contractSecret]()
	if got := validated.levels[0].contracts.bindings[0].capability; got != reflect.TypeFor[stage3Read]() {
		t.Fatalf("validated capability changed to %v", got)
	}
}

func TestStage5ContractDiagnosticContext(t *testing.T) {
	levels := stage5Definition()
	levels[1].contracts.bindings[0].capability = reflect.TypeFor[contractReadVariadic]()
	_, err := validateNodeDefinition(levels)
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != SignatureMismatch || typed.Target != reflect.TypeFor[stage3C]() || typed.Path != "stage3B" || typed.Expected != reflect.TypeFor[stage5Public]().Method(1).Type {
		t.Fatalf("signature context = %+v", typed)
	}

	levels = stage5Definition()
	for index := range levels {
		levels[index].contracts.bindings = nil
	}
	_, err = validateNodeDefinition(levels)
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("missing-method error = %T, want joined diagnostics", err)
	}
	for _, item := range joined.Unwrap() {
		var missing *Error
		if !errors.As(item, &missing) || missing.Code != MissingMethods || missing.Target != reflect.TypeFor[stage3C]() || missing.Expected == nil {
			t.Fatalf("individual missing-method context = %+v", missing)
		}
	}
}

func TestStage5StructuralDiagnosticContext(t *testing.T) {
	levels := stage5Definition()
	levels[1].parentField = "missing"
	_, err := validateNodeDefinition(levels)
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != InvalidHierarchy || typed.Target != reflect.TypeFor[stage3C]() || typed.Path != "stage3B.missing" || typed.Expected != reflect.TypeFor[stage3A]() {
		t.Fatalf("parent-edge context = %+v", typed)
	}
	levels = stage5Definition()
	levels[1].contracts.bindings[0].provider = reflect.TypeFor[*stage3A]()
	_, err = validateNodeDefinition(levels)
	if !errors.As(err, &typed) || typed.Code != InvalidProvider || typed.Target != reflect.TypeFor[stage3C]() || typed.Path != "stage3B" || typed.Expected != reflect.TypeFor[*stage3B]() || typed.Actual != reflect.TypeFor[*stage3A]() {
		t.Fatalf("registration-provider context = %+v", typed)
	}
}
