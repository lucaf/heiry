package heiry

import (
	"errors"
	"reflect"
	"slices"
	"testing"
)

func TestM6GroupedAndIndividualBindingsSharePreparedDispatch(t *testing.T) {
	d := Define[m4C]()
	d.Level[m4A, m4APublic, m4APrivate]("").
		BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Secret]()).End().
		Level[m4B, m4BPublic, m4BPrivate]("m4A").
		BindGroup((*m4B)(nil), ".", Of[m4Extra](), Of[m4Transform]()).End().
		Level[m4C, m4CPublic, m4CPrivate]("m4B").
		Bind[m4Finish]((*m4C)(nil), ".")
	plan := d.MustPrepare()
	leaf := &m4C{}
	if err := plan.Init(leaf,
		plan.NodeProvider(&leaf.m4A),
		plan.NodeProvider(&leaf.m4B),
		plan.NodeProvider(leaf)); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := Method[m4Read](leaf.Node.Scope()); got != &leaf.m4A {
		t.Fatalf("Read provider = %p, want %p", got, &leaf.m4A)
	}
	if got := Method[m4Transform](leaf.Node.Scope()); got != &leaf.m4B {
		t.Fatalf("Transform provider = %p, want %p", got, &leaf.m4B)
	}
	if got := Method[m4Finish](leaf.Node.Scope()); got != leaf {
		t.Fatalf("Finish provider = %p, want %p", got, leaf)
	}
}

func m6MissingNames(err error) []string {
	var names []string
	var visit func(error)
	visit = func(item error) {
		if joined, ok := item.(interface{ Unwrap() []error }); ok {
			for _, child := range joined.Unwrap() {
				visit(child)
			}
			return
		}
		if diagnostic, ok := item.(*Error); ok && diagnostic.Code == MissingMethods {
			names = append(names, diagnostic.Name)
		}
	}
	visit(err)
	return names
}

func TestM6MalformedGroupAddsNothingAndRetainsDiagnostics(t *testing.T) {
	for _, tt := range []struct {
		name     string
		provider *m4A
		path     string
		ops      []OperationToken
		code     Code
	}{
		{"invalid type token", nil, ".", []OperationToken{Of[m4Read](), Of[int]()}, InvalidCapability},
		{"zero token", nil, ".", []OperationToken{Of[m4Read](), {}}, InvalidCapability},
		{"empty capability", nil, ".", []OperationToken{Of[m4Read](), Of[contractEmpty]()}, InvalidCapability},
		{"aggregate capability", nil, ".", []OperationToken{Of[m4Read](), Of[contractReadAndWrite]()}, InvalidCapability},
		{"unexported capability", nil, ".", []OperationToken{Of[m4Read](), Of[contractBadMethod]()}, InvalidCapability},
		{"empty group", nil, ".", nil, InvalidDeclaration},
		{"live witness", &m4A{}, ".", []OperationToken{Of[m4Read]()}, InvalidProvider},
		{"malformed path", nil, "child..field", []OperationToken{Of[m4Read]()}, InvalidProvider},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := Define[m4A]()
			level := d.Level[m4A, m4APublic, m4APrivate]("")
			if got := level.BindGroup(tt.provider, tt.path, tt.ops...); got != level || len(level.level.bindings) != 0 {
				t.Fatal("malformed group changed binding metadata or handle")
			}
			err := d.Validate()
			if !errors.Is(err, &Error{Code: tt.code}) || !slices.Equal(m6MissingNames(err), []string{"Read", "Secret"}) {
				t.Fatalf("malformed group diagnostics = %v", err)
			}
			level.Bind[m4Read]((*m4A)(nil), ".").Bind[m4Secret]((*m4A)(nil), ".")
			if err := d.Validate(); !errors.Is(err, &Error{Code: tt.code}) || len(m6MissingNames(err)) != 0 {
				t.Fatalf("sticky diagnostic after valid bindings = %v", err)
			}
		})
	}
}

func TestM6MultipleMalformedTokensReportInArgumentOrder(t *testing.T) {
	d := Define[m4A]()
	level := d.Level[m4A, m4APublic, m4APrivate]("")
	level.BindGroup((*m4A)(nil), ".", Of[int](), Of[m4Read](), OperationToken{})
	if len(level.level.bindings) != 0 || len(d.recorded) != 2 {
		t.Fatalf("malformed group bindings = %d, diagnostics = %d", len(level.level.bindings), len(d.recorded))
	}
	for index, want := range []reflect.Type{reflect.TypeFor[int](), nil} {
		var diagnostic *Error
		if !errors.As(d.recorded[index], &diagnostic) || diagnostic.Code != InvalidCapability || diagnostic.Capability != want || diagnostic.Level != 0 || diagnostic.Target != reflect.TypeFor[m4A]() {
			t.Fatalf("token %d diagnostic = %+v", index, diagnostic)
		}
	}
}

func TestM6GroupDetachesCallerTokenSlice(t *testing.T) {
	d := Define[m4A]()
	level := d.Level[m4A, m4APublic, m4APrivate]("")
	tokens := []OperationToken{Of[m4Read](), Of[m4Secret]()}
	level.BindGroup((*m4A)(nil), ".", tokens...)
	tokens[0], tokens[1] = Of[int](), OperationToken{}
	if err := d.Validate(); err != nil {
		t.Fatalf("caller changed accepted group: %v", err)
	}
	if len(level.level.bindings) != 2 || level.level.bindings[0].capability != reflect.TypeFor[m4Read]() {
		t.Fatal("accepted group retained mutable tokens")
	}
}

type m6Partial struct{ Node }

func (*m6Partial) Read() int { return 1 }

func TestM6WellFormedGroupReportsPerOperationFailures(t *testing.T) {
	d := Define[m6Partial]()
	level := d.Level[m6Partial, m4APublic, m4APrivate]("")
	level.BindGroup((*m6Partial)(nil), ".", Of[m4Read](), Of[m4Secret]())
	if len(level.level.bindings) != 2 {
		t.Fatal("well-formed group did not expand into two registrations")
	}
	err := d.Validate()
	if !errors.Is(err, &Error{Code: SignatureMismatch}) || !slices.Equal(m6MissingNames(err), []string{"Secret"}) {
		t.Fatalf("mixed provider capability diagnostics = %v", err)
	}
	if plan, err := d.Prepare(); plan != nil || !errors.Is(err, &Error{Code: SignatureMismatch}) {
		t.Fatalf("invalid group prepared Plan: (%v, %v)", plan, err)
	}
}

func TestM6GroupedSemanticErrorsPreserveSafeCoverage(t *testing.T) {
	for _, tt := range []struct {
		name    string
		bind    func(*LevelHandle[m4A])
		code    Code
		missing []string
	}{
		{"unsupported component path", func(l *LevelHandle[m4A]) {
			l.BindGroup((*m4A)(nil), "component", Of[m4Read](), Of[m4Secret]())
		}, InvalidProvider, []string{"Read", "Secret"}},
		{"undeclared operation", func(l *LevelHandle[m4A]) {
			l.BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Extra](), Of[m4Secret]())
		}, UndeclaredOperation, nil},
		{"distinct capability identity", func(l *LevelHandle[m4A]) {
			l.BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Secret](), Of[contractReadOther]())
		}, CapabilityIdentity, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := Define[m4A]()
			tt.bind(d.Level[m4A, m4APublic, m4APrivate](""))
			err := d.Validate()
			if !errors.Is(err, &Error{Code: tt.code}) || !slices.Equal(m6MissingNames(err), tt.missing) {
				t.Fatalf("group semantic diagnostics = %v", err)
			}
		})
	}
}

func TestM6DuplicateOperationsWithinAndAcrossStyles(t *testing.T) {
	for _, tt := range []struct {
		name     string
		register func(*LevelHandle[m4A])
	}{
		{"within group", func(l *LevelHandle[m4A]) { l.BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Read](), Of[m4Secret]()) }},
		{"individual then group", func(l *LevelHandle[m4A]) {
			l.Bind[m4Read]((*m4A)(nil), ".").BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Secret]())
		}},
		{"group then individual", func(l *LevelHandle[m4A]) {
			l.BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Secret]()).Bind[m4Read]((*m4A)(nil), ".")
		}},
		{"across groups", func(l *LevelHandle[m4A]) {
			l.BindGroup((*m4A)(nil), ".", Of[m4Read]()).BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Secret]())
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := Define[m4A]()
			tt.register(d.Level[m4A, m4APublic, m4APrivate](""))
			if err := d.Validate(); !errors.Is(err, &Error{Code: DuplicateBinding}) || len(m6MissingNames(err)) != 0 {
				t.Fatalf("duplicate diagnostics = %v", err)
			}
			if plan, err := d.Prepare(); plan != nil || !errors.Is(err, &Error{Code: DuplicateBinding}) {
				t.Fatalf("duplicate group prepared Plan: (%v, %v)", plan, err)
			}
		})
	}
}

func TestM6EarlierGroupDiagnosticSurvivesLaterContractFailure(t *testing.T) {
	d := Define[m4B]()
	d.Level[m4A, m4APublic, m4APrivate]("").
		BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Read](), Of[m4Secret]()).End().
		Level[m4B, contractEmpty, m4APrivate]("m4A")
	err := d.Validate()
	if !errors.Is(err, &Error{Code: DuplicateBinding}) || !errors.Is(err, &Error{Code: ContractExtension}) {
		t.Fatalf("earlier group and later contract diagnostics = %v", err)
	}
}

func TestM6FrozenAndRejectedHandlesPreservePreparedPlan(t *testing.T) {
	d := Define[m4A]()
	level := d.Level[m4A, m4APublic, m4APrivate]("")
	level.BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Secret]())
	plan := d.MustPrepare()
	count := len(level.level.bindings)
	level.BindGroup((*m4A)(nil), ".", Of[m4Read]())
	if !errors.Is(d.Validate(), &Error{Code: InvalidDeclaration}) || len(level.level.bindings) != count {
		t.Fatal("frozen handle edited prepared metadata")
	}
	rejected := d.Level[m4A, m4APublic, m4APrivate]("")
	priorErrors := len(d.recorded)
	if rejected.BindGroup((*m4A)(nil), ".", Of[int]()).End() != d || len(d.recorded) != priorErrors {
		t.Fatal("rejected handle added metadata or a new diagnostic")
	}
	if later, err := d.Prepare(); later != nil || !errors.Is(err, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("definition after frozen edit = (%v, %v)", later, err)
	}
	leaf := &m4A{}
	if err := plan.Init(leaf, plan.NodeProvider(leaf)); err != nil {
		t.Fatalf("retained Plan after frozen edit: %v", err)
	}
}

func TestM6ChainSplitAndIndividualRegistrationEquivalence(t *testing.T) {
	chained := Define[m4C]()
	chained.Level[m4A, m4APublic, m4APrivate]("").
		BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Secret]()).End().
		Level[m4B, m4BPublic, m4BPrivate]("m4A").
		Bind[m4Extra]((*m4B)(nil), ".").
		BindGroup((*m4B)(nil), ".", Of[m4Transform]()).End().
		Level[m4C, m4CPublic, m4CPrivate]("m4B").
		Bind[m4Finish]((*m4C)(nil), ".")

	split := Define[m4C]()
	a := split.Level[m4A, m4APublic, m4APrivate]("")
	b := split.Level[m4B, m4BPublic, m4BPrivate]("m4A")
	c := split.Level[m4C, m4CPublic, m4CPrivate]("m4B")
	a.BindGroup((*m4A)(nil), ".", Of[m4Read](), Of[m4Secret]())
	b.Bind[m4Extra]((*m4B)(nil), ".")
	b.BindGroup((*m4B)(nil), ".", Of[m4Transform]())
	c.Bind[m4Finish]((*m4C)(nil), ".")

	individual := Define[m4C]()
	individual.Level[m4A, m4APublic, m4APrivate]("").
		Bind[m4Read]((*m4A)(nil), ".").Bind[m4Secret]((*m4A)(nil), ".").End().
		Level[m4B, m4BPublic, m4BPrivate]("m4A").
		Bind[m4Extra]((*m4B)(nil), ".").Bind[m4Transform]((*m4B)(nil), ".").End().
		Level[m4C, m4CPublic, m4CPrivate]("m4B").
		Bind[m4Finish]((*m4C)(nil), ".")

	for _, d := range []*Definition[m4C]{chained, split, individual} {
		if err := d.Validate(); err != nil {
			t.Fatalf("equivalent definition: %v", err)
		}
		if !reflect.DeepEqual(d.levels, chained.levels) {
			t.Fatal("chain, split, and individual registration produced different metadata")
		}
		plan := d.MustPrepare()
		leaf := &m4C{}
		if err := plan.Init(leaf,
			plan.NodeProvider(&leaf.m4A), plan.NodeProvider(&leaf.m4B), plan.NodeProvider(leaf)); err != nil {
			t.Fatalf("Init: %v", err)
		}
		if got := Method[m4Extra](leaf.Node.Scope()).Extra(2); got != 3 {
			t.Fatalf("Extra = %d", got)
		}
		if got := Method[m4Transform](leaf.Node.Scope()).Transform("x"); got != "B/x" {
			t.Fatalf("Transform = %q", got)
		}
	}
}

func m6Stage3Definition() *Definition[stage3C] {
	d := Define[stage3C]()
	d.Level[stage3A, stage5Public, contractEmpty]("").
		BindGroup((*stage3A)(nil), ".", Of[stage3Read](), Of[stage3Add](), Of[stage3Walk]()).End().
		Level[stage3B, stage5Public, contractEmpty]("stage3A").
		BindGroup((*stage3B)(nil), ".", Of[stage3Read](), Of[stage3Add](), Of[stage3Walk]()).End().
		Level[stage3C, stage5Public, contractEmpty]("stage3B").
		Bind[stage3Read]((*stage3C)(nil), ".").
		BindGroup((*stage3C)(nil), ".", Of[stage3Add](), Of[stage3Walk]())
	return d
}

func m6Leaf(a, b, c int) *stage3C {
	leaf := &stage3C{}
	leaf.stage3A.value, leaf.stage3B.value, leaf.value = a, b, c
	return leaf
}

func TestM6GroupedDynamicEagerLazyAndPlanReuse(t *testing.T) {
	eager := m6Stage3Definition()
	plan := eager.MustPrepare()
	first, second := m6Leaf(1, 2, 3), m6Leaf(10, 20, 30)
	if err := plan.Init(first,
		plan.NodeProvider(&first.stage3A), plan.NodeProvider(&first.stage3B), plan.NodeProvider(first)); err != nil {
		t.Fatalf("eager first Init: %v", err)
	}
	if got := plan.MustInit(second,
		plan.NodeProvider(&second.stage3A), plan.NodeProvider(&second.stage3B), plan.NodeProvider(second)); got != second {
		t.Fatal("MustInit did not return the original leaf")
	}
	if got := Method[stage3Walk](first.Node.Scope()).Walk(1); got != "C/B/C/B/A" {
		t.Fatalf("eager reentry = %q", got)
	}
	if got := Method[stage3Add](first.Node.Scope()).Add(4); got != 18 || first.stage3A.value != 5 {
		t.Fatalf("eager Add = %d, root state = %d", got, first.stage3A.value)
	}
	if got := Method[stage3Read](second.Node.Scope()).Read(); got != 60 {
		t.Fatalf("independent second Read = %d", got)
	}

	lazy := m6Stage3Definition()
	third := m6Leaf(1, 2, 3)
	if err := lazy.Init(third,
		lazy.NodeProvider(&third.stage3A), lazy.NodeProvider(&third.stage3B), lazy.NodeProvider(third)); err != nil {
		t.Fatalf("lazy first Init: %v", err)
	}
	lazyPlan, err := lazy.Prepare()
	if err != nil || lazyPlan == nil {
		t.Fatalf("lazy prepared Plan = (%v, %v)", lazyPlan, err)
	}
	if got := Method[stage3Walk](third.Node.Scope()).Walk(1); got != "C/B/C/B/A" {
		t.Fatalf("lazy reentry = %q", got)
	}
	if got := Method[stage3Add](third.Node.Scope()).Add(4); got != 18 || third.stage3A.value != 5 {
		t.Fatalf("lazy Add = %d, root state = %d", got, third.stage3A.value)
	}
	fourth := m6Leaf(10, 20, 30)
	if err := lazyPlan.Init(fourth,
		lazyPlan.NodeProvider(&fourth.stage3A), lazyPlan.NodeProvider(&fourth.stage3B), lazyPlan.NodeProvider(fourth)); err != nil {
		t.Fatalf("lazy Plan reuse: %v", err)
	}
	if got := Method[stage3Read](fourth.Node.Scope()).Read(); got != 60 {
		t.Fatalf("lazy reused Read = %d", got)
	}
	if m5InstanceBearing(reflect.ValueOf(plan), 0) || m5InstanceBearing(reflect.ValueOf(lazyPlan), 0) {
		t.Fatal("grouped prepared metadata retained an instance")
	}
}

func TestM6GroupedErrorsAndMustOperationsPreserveScopes(t *testing.T) {
	d := Define[m4A]()
	d.Level[m4A, m4APublic, m4APrivate]("").BindGroup((*m4A)(nil), ".", Of[int]())
	leaf := &m4A{}
	if err := d.Init(leaf, d.NodeProvider(leaf)); !errors.Is(err, &Error{Code: InvalidCapability}) || leaf.Node.Scope().dispatch != nil {
		t.Fatalf("invalid lazy group Init = %v", err)
	}
	for _, call := range []func(){
		func() { d.MustPrepare() },
		func() { d.MustInit(leaf, d.NodeProvider(leaf)) },
	} {
		func() {
			defer func() {
				value := recover()
				err, ok := value.(error)
				if !ok || !errors.Is(err, &Error{Code: InvalidCapability}) {
					t.Errorf("Must panic = %v", value)
				}
			}()
			call()
		}()
	}
	plan := m6Stage3Definition().MustPrepare()
	other, foreign := m6Leaf(1, 2, 3), &stage3B{}
	if err := plan.Init(other,
		plan.NodeProvider(&other.stage3A), plan.NodeProvider(foreign), plan.NodeProvider(other)); !errors.Is(err, &Error{Code: ProviderOwnership}) {
		t.Fatalf("foreign grouped provider = %v", err)
	}
	for _, node := range []*Node{&other.stage3A.Node, &other.stage3B.Node, &other.Node} {
		if node.Scope().dispatch != nil {
			t.Fatal("failed grouped Plan Init installed a scope")
		}
	}
}
