package heiry

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func m5Stage3Definition() *Definition[stage3C] {
	definition := Define[stage3C]()
	definition.Level[stage3A, stage5Public, contractEmpty]("").
		Bind[stage3Read]((*stage3A)(nil), ".").
		Bind[stage3Add]((*stage3A)(nil), ".").
		Bind[stage3Walk]((*stage3A)(nil), ".").End().
		Level[stage3B, stage5Public, contractEmpty]("stage3A").
		Bind[stage3Read]((*stage3B)(nil), ".").
		Bind[stage3Add]((*stage3B)(nil), ".").
		Bind[stage3Walk]((*stage3B)(nil), ".").End().
		Level[stage3C, stage5Public, contractEmpty]("stage3B").
		Bind[stage3Read]((*stage3C)(nil), ".").
		Bind[stage3Add]((*stage3C)(nil), ".").
		Bind[stage3Walk]((*stage3C)(nil), ".")
	return definition
}

func TestM5PrepareRetriesInvalidDefinitionAndReusesPlan(t *testing.T) {
	definition := Define[m4A]()
	level := definition.Level[m4A, m4APublic, m4APrivate]("")
	if plan, err := definition.Prepare(); plan != nil || !errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("invalid preparation = (%v, %v)", plan, err)
	}
	level.Bind[m4Read]((*m4A)(nil), ".").Bind[m4Secret]((*m4A)(nil), ".")
	plan, err := definition.Prepare()
	if err != nil || plan == nil {
		t.Fatalf("corrected preparation = (%v, %v)", plan, err)
	}
	again, err := definition.Prepare()
	if err != nil || again != plan {
		t.Fatalf("prepared Plan was not reused: (%v, %v)", again, err)
	}
}

func TestM5PreparationFreezesSavedAndRejectedLevelHandles(t *testing.T) {
	definition := Define[m4A]()
	level := definition.Level[m4A, m4APublic, m4APrivate]("")
	level.Bind[m4Read]((*m4A)(nil), ".").Bind[m4Secret]((*m4A)(nil), ".")
	plan, err := definition.Prepare()
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	before := len(level.level.bindings)
	level.Bind[m4Read]((*m4A)(nil), ".")
	rejected := definition.Level[m4A, m4APublic, m4APrivate]("")
	if rejected.End() != definition {
		t.Fatal("rejected level did not return its definition")
	}
	rejected.Bind[m4Read]((*m4A)(nil), ".")
	if len(definition.levels) != 1 || len(level.level.bindings) != before || definition.plan != plan {
		t.Fatal("post-preparation edit changed the prepared declaration")
	}
	if err := definition.Validate(); !errors.Is(err, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("rejected edit validation = %v", err)
	}
	var diagnostic *Error
	if err := definition.Validate(); !errors.As(err, &diagnostic) || diagnostic.Capability != reflect.TypeFor[m4Read]() || diagnostic.Path != "." {
		t.Fatalf("rejected binding context = %+v", diagnostic)
	}
	if later, err := definition.Prepare(); later != nil || !errors.Is(err, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("rejected edit preparation = (%v, %v)", later, err)
	}
}

func TestM5PlanInitializesIndependentEmbeddedObjects(t *testing.T) {
	plan := m5Stage3Definition().MustPrepare()
	first, second := &stage3C{}, &stage3C{}
	first.stage3A.value, first.stage3B.value, first.value = 1, 2, 3
	second.stage3A.value, second.stage3B.value, second.value = 10, 20, 30
	for _, leaf := range []*stage3C{first, second} {
		if err := plan.Init(leaf,
			plan.NodeProvider(&leaf.stage3A),
			plan.NodeProvider(&leaf.stage3B),
			plan.NodeProvider(leaf)); err != nil {
			t.Fatalf("initialize %p: %v", leaf, err)
		}
		for level, want := range []any{&leaf.stage3A, &leaf.stage3B, leaf} {
			if got := leaf.Node.Scope().dispatch.levels[level][reflect.TypeFor[stage3Add]()]; got != want {
				t.Fatalf("level %d provider = %p, want %p", level, got, want)
			}
		}
	}
	if first.Node.Scope().dispatch == second.Node.Scope().dispatch {
		t.Fatal("independent objects share dispatch")
	}
	if got := Method[stage3Add](first.Node.Scope()).Add(4); got != 18 {
		t.Fatalf("first Add = %d", got)
	}
	if first.stage3A.value != 5 || first.stage3B.value != 6 || first.value != 7 {
		t.Fatal("ancestor mutations did not reach first leaf")
	}
	if got := Method[stage3Read](second.Node.Scope()).Read(); got != 60 {
		t.Fatalf("second Read = %d", got)
	}
	if second.stage3A.value != 10 || second.stage3B.value != 20 || second.value != 30 {
		t.Fatal("first object changed second object")
	}
}

func TestM5LazyFirstUseAndExplicitPlanReuse(t *testing.T) {
	definition := m5Stage3Definition()
	first := &stage3C{}
	first.stage3A.value, first.stage3B.value, first.value = 1, 2, 3
	if err := definition.Init(first,
		definition.NodeProvider(&first.stage3A),
		definition.NodeProvider(&first.stage3B),
		definition.NodeProvider(first)); err != nil {
		t.Fatalf("lazy first use: %v", err)
	}
	plan, err := definition.Prepare()
	if err != nil || plan == nil {
		t.Fatalf("prepared Plan after first use = (%v, %v)", plan, err)
	}
	second := &stage3C{}
	second.stage3A.value, second.stage3B.value, second.value = 10, 20, 30
	if got := plan.MustInit(second,
		plan.NodeProvider(&second.stage3A),
		plan.NodeProvider(&second.stage3B),
		plan.NodeProvider(second)); got != second {
		t.Fatalf("MustInit returned %p, want %p", got, second)
	}
	if got := Method[stage3Read](first.Node.Scope()).Read(); got != 6 {
		t.Fatalf("first Read = %d", got)
	}
	if got := Method[stage3Read](second.Node.Scope()).Read(); got != 60 {
		t.Fatalf("second Read = %d", got)
	}
}

func TestM5EagerAndLazyPreparationHaveEquivalentBehavior(t *testing.T) {
	newLeaf := func() *stage3C {
		leaf := &stage3C{}
		leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
		return leaf
	}
	eagerDefinition := m5Stage3Definition()
	eagerPlan := eagerDefinition.MustPrepare()
	eager := newLeaf()
	if err := eagerPlan.Init(eager,
		eagerPlan.NodeProvider(&eager.stage3A),
		eagerPlan.NodeProvider(&eager.stage3B),
		eagerPlan.NodeProvider(eager)); err != nil {
		t.Fatalf("eager Init: %v", err)
	}
	lazyDefinition := m5Stage3Definition()
	lazy := newLeaf()
	if err := lazyDefinition.Init(lazy,
		lazyDefinition.NodeProvider(&lazy.stage3A),
		lazyDefinition.NodeProvider(&lazy.stage3B),
		lazyDefinition.NodeProvider(lazy)); err != nil {
		t.Fatalf("lazy Init: %v", err)
	}
	for _, leaf := range []*stage3C{eager, lazy} {
		if got := Method[stage3Add](leaf.Node.Scope()).Add(4); got != 18 {
			t.Fatalf("Add = %d", got)
		}
		if leaf.stage3A.value != 5 || leaf.stage3B.value != 6 || leaf.value != 7 {
			t.Fatalf("state = (%d, %d, %d)", leaf.stage3A.value, leaf.stage3B.value, leaf.value)
		}
	}
}

func m5PrivateDefinition() *Definition[stage4B] {
	d := Define[stage4B]()
	d.Level[stage4A, stage4Public, stage4Private]("").
		Bind[stage4Public]((*stage4A)(nil), ".").
		Bind[stage4Private]((*stage4A)(nil), ".").End().
		Level[stage4B, stage4Public, stage4Private]("stage4A").
		Bind[stage4Public]((*stage4B)(nil), ".").
		Bind[stage4Private]((*stage4B)(nil), ".")
	return d
}

func TestM5EagerAndLazyPrivateDispatchAndVisibility(t *testing.T) {
	for _, schedule := range []string{"eager", "lazy"} {
		t.Run(schedule, func(t *testing.T) {
			d := m5PrivateDefinition()
			leaf := &stage4B{}
			var err error
			if schedule == "eager" {
				p := d.MustPrepare()
				err = p.Init(leaf, p.NodeProvider(&leaf.stage4A), p.NodeProvider(leaf))
			} else {
				err = d.Init(leaf, d.NodeProvider(&leaf.stage4A), d.NodeProvider(leaf))
			}
			if err != nil {
				t.Fatalf("Init: %v", err)
			}
			wrapper := &stage4Object{leaf: leaf}
			if got := wrapper.Read(); got != "B/A/B/A.secret" {
				t.Fatalf("public/private dispatch = %q", got)
			}
			if _, private := any(wrapper).(stage4Private); private || reflect.TypeOf(wrapper).NumMethod() != 1 {
				t.Fatal("public wrapper exposed private operation")
			}
		})
	}
}

type m5ErrorOperation interface{ Calculate(int) (int, error) }
type m5ErrorRoot struct{ Node }

func (*m5ErrorRoot) Calculate(n int) (int, error) {
	if n < 0 {
		return 0, stage1ApplicationError
	}
	return n + 1, nil
}

type m5ErrorLeaf struct {
	m5ErrorRoot
	Node
}

func (l *m5ErrorLeaf) Calculate(n int) (int, error) {
	value, err := ParentMethod[m5ErrorOperation](l.Node.Scope()).Calculate(n)
	if err != nil {
		return 0, err
	}
	return value * 2, nil
}

func m5ErrorDefinition() *Definition[m5ErrorLeaf] {
	d := Define[m5ErrorLeaf]()
	d.Level[m5ErrorRoot, m5ErrorOperation, contractEmpty]("").
		Bind[m5ErrorOperation]((*m5ErrorRoot)(nil), ".").End().
		Level[m5ErrorLeaf, m5ErrorOperation, contractEmpty]("m5ErrorRoot").
		Bind[m5ErrorOperation]((*m5ErrorLeaf)(nil), ".")
	return d
}

func TestM5EagerAndLazyApplicationErrors(t *testing.T) {
	for _, schedule := range []string{"eager", "lazy"} {
		t.Run(schedule, func(t *testing.T) {
			d := m5ErrorDefinition()
			leaf := &m5ErrorLeaf{}
			var err error
			if schedule == "eager" {
				p := d.MustPrepare()
				err = p.Init(leaf, p.NodeProvider(&leaf.m5ErrorRoot), p.NodeProvider(leaf))
			} else {
				err = d.Init(leaf, d.NodeProvider(&leaf.m5ErrorRoot), d.NodeProvider(leaf))
			}
			if err != nil {
				t.Fatalf("Init: %v", err)
			}
			operation := Method[m5ErrorOperation](leaf.Node.Scope())
			if got, err := operation.Calculate(2); got != 6 || err != nil {
				t.Fatalf("successful Calculate = (%d, %v)", got, err)
			}
			if got, err := operation.Calculate(-1); got != 0 || !errors.Is(err, stage1ApplicationError) {
				t.Fatalf("application error = (%d, %v)", got, err)
			}
		})
	}
}

func TestM5EagerAndLazyInvalidDefinitionsAgree(t *testing.T) {
	makeInvalid := func() *Definition[m4A] {
		return Define[m4A]().Level[m4A, m4APublic, m4APrivate]("").End()
	}
	eager := makeInvalid()
	eagerPlan, eagerErr := eager.Prepare()
	if eagerPlan != nil || !errors.Is(eagerErr, &Error{Code: MissingMethods}) {
		t.Fatalf("eager invalid definition = (%v, %v)", eagerPlan, eagerErr)
	}
	lazy := makeInvalid()
	leaf := &m4A{}
	err := lazy.Init(leaf, lazy.NodeProvider(leaf))
	if !errors.Is(err, &Error{Code: MissingMethods}) || leaf.Node.Scope().dispatch != nil || lazy.plan != nil {
		t.Fatalf("lazy invalid definition = %v, scope = %+v, plan = %v", err, leaf.Node.Scope(), lazy.plan)
	}
	type fields struct {
		code     Code
		name     string
		category string
		target   reflect.Type
		expected reflect.Type
		level    int
		path     string
	}
	collect := func(err error) map[string]fields {
		result := make(map[string]fields)
		var visit func(error)
		visit = func(item error) {
			if joined, ok := item.(interface{ Unwrap() []error }); ok {
				for _, child := range joined.Unwrap() {
					visit(child)
				}
				return
			}
			var diagnostic *Error
			if errors.As(item, &diagnostic) {
				result[diagnostic.Name] = fields{
					code: diagnostic.Code, name: diagnostic.Name, category: diagnostic.Category,
					target: diagnostic.Target, expected: diagnostic.Expected,
					level: diagnostic.Level, path: diagnostic.Path,
				}
			}
		}
		visit(err)
		return result
	}
	want := map[string]fields{
		"Read":   {code: MissingMethods, name: "Read", category: "public", target: reflect.TypeFor[m4A](), expected: reflect.TypeFor[m4APublic]().Method(0).Type},
		"Secret": {code: MissingMethods, name: "Secret", category: "private", target: reflect.TypeFor[m4A](), expected: reflect.TypeFor[m4APrivate]().Method(0).Type},
	}
	if got := collect(eagerErr); !reflect.DeepEqual(got, want) {
		t.Fatalf("eager diagnostics = %+v, want %+v", got, want)
	}
	if got := collect(err); !reflect.DeepEqual(got, want) {
		t.Fatalf("lazy diagnostics = %+v, want %+v", got, want)
	}
}

func TestM5ProviderReferencesCanBeSuppliedOutOfOrder(t *testing.T) {
	plan := m5Stage3Definition().MustPrepare()
	leaf := &stage3C{}
	leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
	if err := plan.Init(leaf,
		plan.NodeProvider(leaf),
		plan.NodeProvider(&leaf.stage3A),
		plan.NodeProvider(&leaf.stage3B)); err != nil {
		t.Fatalf("shuffled Init: %v", err)
	}
	if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != 6 {
		t.Fatalf("Read = %d", got)
	}
}

func TestM5DistinctDefinitionsForOneLeafKeepDistinctDispatch(t *testing.T) {
	makeDefinition := func(leafOverride bool) *Definition[stage3C] {
		d := Define[stage3C]()
		d.Level[stage3A, stage3Read, contractEmpty]("").
			Bind[stage3Read]((*stage3A)(nil), ".").End().
			Level[stage3B, stage3Read, contractEmpty]("stage3A").End()
		leafLevel := d.Level[stage3C, stage3Read, contractEmpty]("stage3B")
		if leafOverride {
			leafLevel.Bind[stage3Read]((*stage3C)(nil), ".")
		}
		return d
	}
	for _, tt := range []struct {
		name     string
		override bool
		want     int
	}{{"root only", false, 1}, {"leaf override", true, 4}} {
		t.Run(tt.name, func(t *testing.T) {
			plan := makeDefinition(tt.override).MustPrepare()
			leaf := &stage3C{}
			leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
			if err := plan.Init(leaf,
				plan.NodeProvider(&leaf.stage3A),
				plan.NodeProvider(&leaf.stage3B),
				plan.NodeProvider(leaf)); err != nil {
				t.Fatalf("Init: %v", err)
			}
			if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != tt.want {
				t.Fatalf("Read = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestM5InstanceFailurePreservesAllScopesAndPreparedPlan(t *testing.T) {
	definition := m5Stage3Definition()
	plan := definition.MustPrepare()
	leaf, foreign := &stage3C{}, &stage3B{}
	err := plan.Init(leaf,
		plan.NodeProvider(&leaf.stage3A),
		plan.NodeProvider(foreign),
		plan.NodeProvider(leaf))
	if !errors.Is(err, &Error{Code: ProviderOwnership}) {
		t.Fatalf("foreign provider = %v", err)
	}
	for _, node := range []*Node{&leaf.stage3A.Node, &leaf.stage3B.Node, &leaf.Node} {
		if node.Scope().dispatch != nil {
			t.Fatal("failed initialization installed a scope")
		}
	}
	if definition.plan != plan {
		t.Fatal("instance failure discarded prepared Plan")
	}
	if err := plan.Init(leaf,
		plan.NodeProvider(&leaf.stage3A),
		plan.NodeProvider(&leaf.stage3B),
		plan.NodeProvider(leaf)); err != nil {
		t.Fatalf("valid retry after instance failure: %v", err)
	}
	original := leaf.Node.Scope().dispatch
	alias := leaf
	if err := plan.Init(alias,
		plan.NodeProvider(&alias.stage3A),
		plan.NodeProvider(&alias.stage3B),
		plan.NodeProvider(alias)); !errors.Is(err, &Error{Code: AlreadyInitialized}) {
		t.Fatalf("repeat initialization through alias = %v", err)
	}
	for _, node := range []*Node{&leaf.stage3A.Node, &leaf.stage3B.Node, &leaf.Node} {
		if node.Scope().dispatch != original {
			t.Fatal("repeat initialization changed an existing scope")
		}
	}
}

func TestM5LaterInitializedHolderReportsPathWithoutPartialWrites(t *testing.T) {
	leaf := &stage3C{}
	partial := Define[stage3B]().Level[stage3B, contractEmpty, contractEmpty]("").End().MustPrepare()
	if err := partial.Init(&leaf.stage3B, partial.NodeProvider(&leaf.stage3B)); err != nil {
		t.Fatalf("initialize B: %v", err)
	}
	previous := leaf.stage3B.Node.Scope()
	plan := m5Stage3Definition().MustPrepare()
	err := plan.Init(leaf,
		plan.NodeProvider(&leaf.stage3A),
		plan.NodeProvider(&leaf.stage3B),
		plan.NodeProvider(leaf))
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != AlreadyInitialized ||
		diagnostic.Target != reflect.TypeFor[stage3C]() || diagnostic.Level != 1 || diagnostic.Path != "stage3B" {
		t.Fatalf("later initialized holder diagnostic = %+v", diagnostic)
	}
	if leaf.stage3A.Node.Scope().dispatch != nil || leaf.Node.Scope().dispatch != nil || leaf.stage3B.Node.Scope() != previous {
		t.Fatal("failed initialization changed one or more scopes")
	}
}

func TestM5RejectedDefinitionEditDoesNotPoisonPlan(t *testing.T) {
	definition := m5Stage3Definition()
	plan := definition.MustPrepare()
	definition.Level[stage3C, stage5Public, contractEmpty]("stage3B")
	leaf := &stage3C{}
	if err := definition.Init(leaf,
		definition.NodeProvider(&leaf.stage3A),
		definition.NodeProvider(&leaf.stage3B),
		definition.NodeProvider(leaf)); !errors.Is(err, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("definition Init after frozen edit = %v", err)
	}
	if leaf.Node.Scope().dispatch != nil {
		t.Fatal("failed definition Init installed a scope")
	}
	if err := plan.Init(leaf,
		plan.NodeProvider(&leaf.stage3A),
		plan.NodeProvider(&leaf.stage3B),
		plan.NodeProvider(leaf)); err != nil {
		t.Fatalf("retained Plan after rejected edit: %v", err)
	}
}

func TestM5ProviderReferenceBoundaries(t *testing.T) {
	definition := m5Stage3Definition()
	leaf := &stage3C{}
	beforePreparation := []ProviderRef[stage3C]{
		definition.NodeProvider(&leaf.stage3A),
		definition.NodeProvider(&leaf.stage3B),
		definition.NodeProvider(leaf),
	}
	if definition.plan != nil {
		t.Fatal("reference creation prepared the definition")
	}
	plan := definition.MustPrepare()
	if err := plan.Init(leaf, beforePreparation...); err != nil {
		t.Fatalf("preparation-independent references: %v", err)
	}

	tests := []struct {
		name string
		refs []ProviderRef[stage3C]
		code Code
	}{
		{"zero reference", []ProviderRef[stage3C]{{}, plan.NodeProvider(&stage3B{}), plan.NodeProvider(&stage3C{})}, BindingMismatch},
		{"nil node pointer", []ProviderRef[stage3C]{plan.NodeProvider((*stage3A)(nil)), plan.NodeProvider(&stage3B{}), plan.NodeProvider(&stage3C{})}, InvalidProvider},
		{"wrong node type", []ProviderRef[stage3C]{plan.NodeProvider(&stage5Unrelated{}), plan.NodeProvider(&stage3B{}), plan.NodeProvider(&stage3C{})}, InvalidProvider},
		{"duplicate node slot", []ProviderRef[stage3C]{plan.NodeProvider(&stage3A{}), plan.NodeProvider(&stage3A{}), plan.NodeProvider(&stage3C{})}, BindingMismatch},
		{"other definition", []ProviderRef[stage3C]{m5Stage3Definition().NodeProvider(&stage3A{}), plan.NodeProvider(&stage3B{}), plan.NodeProvider(&stage3C{})}, BindingMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			other := &stage3C{}
			if err := plan.Init(other, tt.refs...); !errors.Is(err, &Error{Code: tt.code}) {
				t.Fatalf("invalid reference = %v, want %s", err, tt.code)
			}
			if other.Node.Scope().dispatch != nil {
				t.Fatal("invalid reference installed a scope")
			}
		})
	}
	var zeroPlan Plan[stage3C]
	if err := zeroPlan.Init(&stage3C{}); !errors.Is(err, &Error{Code: InvalidPlan}) {
		t.Fatalf("zero Plan Init = %v", err)
	}
	var nilPlan *Plan[stage3C]
	if err := nilPlan.Init(&stage3C{}); !errors.Is(err, &Error{Code: InvalidPlan}) {
		t.Fatalf("nil Plan Init = %v", err)
	}
	if err := plan.Init(&stage3C{}, nilPlan.NodeProvider(&stage3A{}), plan.NodeProvider(&stage3B{}), plan.NodeProvider(&stage3C{})); !errors.Is(err, &Error{Code: InvalidPlan}) {
		t.Fatalf("invalid Plan factory reference = %v", err)
	}
}

func TestM5MissingNodeReferenceReportsItsSlot(t *testing.T) {
	plan := m5Stage3Definition().MustPrepare()
	leaf := &stage3C{}
	err := plan.Init(leaf, plan.NodeProvider(&leaf.stage3A), plan.NodeProvider(leaf))
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != BindingMismatch || diagnostic.Level != 1 || diagnostic.Path != "stage3B" {
		t.Fatalf("missing B reference = %+v", diagnostic)
	}
	if leaf.Node.Scope().dispatch != nil {
		t.Fatal("missing reference installed a scope")
	}
}

func TestM5NilDefinitionPreparationReturnsTypedError(t *testing.T) {
	var definition *Definition[m4A]
	if plan, err := definition.Prepare(); plan != nil || !errors.Is(err, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("nil definition preparation = (%v, %v)", plan, err)
	}
}

func TestM5ReferenceSliceAndInstanceErrorsStayLocal(t *testing.T) {
	definition := m5Stage3Definition()
	plan := definition.MustPrepare()
	leaf := &stage3C{}
	leaf.stage3A.value = 7
	refs := []ProviderRef[stage3C]{
		plan.NodeProvider(&leaf.stage3A),
		plan.NodeProvider(&leaf.stage3B),
		plan.NodeProvider(leaf),
	}
	if err := plan.Init(leaf, refs...); err != nil {
		t.Fatalf("initialization: %v", err)
	}
	refs[0] = plan.NodeProvider(&stage3A{})
	if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != 7 {
		t.Fatalf("mutated reference slice changed dispatch: Read = %d", got)
	}
	bad := &stage3C{}
	if err := definition.Init(bad,
		definition.NodeProvider((*stage3A)(nil)),
		definition.NodeProvider(&bad.stage3B),
		definition.NodeProvider(bad)); !errors.Is(err, &Error{Code: InvalidProvider}) {
		t.Fatalf("bad instance = %v", err)
	}
	if again, err := definition.Prepare(); err != nil || again != plan {
		t.Fatalf("instance error changed definition preparation: (%v, %v)", again, err)
	}
}

func TestM5PreparedPlanSupportsIndependentConcurrentInitialization(t *testing.T) {
	plan := m5Stage3Definition().MustPrepare()
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			leaf := &stage3C{}
			leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
			if err := plan.Init(leaf,
				plan.NodeProvider(&leaf.stage3A),
				plan.NodeProvider(&leaf.stage3B),
				plan.NodeProvider(leaf)); err != nil {
				t.Errorf("independent Init: %v", err)
				return
			}
			if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != 6 {
				t.Errorf("Read = %d", got)
			}
		})
	}
	workers.Wait()
}

func TestM5MustLifecyclePanicsWithTypedErrors(t *testing.T) {
	assertCode := func(t *testing.T, code Code, call func()) {
		t.Helper()
		defer func() {
			recovered := recover()
			err, ok := recovered.(error)
			if !ok || !errors.Is(err, &Error{Code: code}) {
				t.Errorf("panic = %v, want %s", recovered, code)
			}
		}()
		call()
		t.Fatal("expected Must panic")
	}
	t.Run("prepare", func(t *testing.T) {
		definition := Define[m4A]().Level[m4A, m4APublic, m4APrivate]("").End()
		assertCode(t, MissingMethods, func() { definition.MustPrepare() })
	})
	t.Run("plan Init", func(t *testing.T) {
		var plan *Plan[stage3C]
		assertCode(t, InvalidPlan, func() { plan.MustInit(&stage3C{}) })
	})
	t.Run("definition Init", func(t *testing.T) {
		var definition *Definition[m4A]
		assertCode(t, InvalidDeclaration, func() { definition.MustInit(&m4A{}) })
	})
}

func TestM5PreparedMetadataContainsNoInstanceReferences(t *testing.T) {
	plan := m5Stage3Definition().MustPrepare()
	leaf := &stage3C{}
	if err := plan.Init(leaf,
		plan.NodeProvider(&leaf.stage3A),
		plan.NodeProvider(&leaf.stage3B),
		plan.NodeProvider(leaf)); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if m5InstanceBearing(reflect.ValueOf(plan), 0) {
		t.Fatal("prepared metadata retained an instance pointer, scope, binding, or closure")
	}
}

func m5InstanceBearing(value reflect.Value, depth int) bool {
	if !value.IsValid() {
		return false
	}
	if depth > 32 {
		return true
	}
	if value.Type() == reflect.TypeFor[reflect.Type]() {
		return false // Type values describe declarations, not application objects.
	}
	for _, forbidden := range []reflect.Type{
		reflect.TypeFor[*stage3A](), reflect.TypeFor[*stage3B](), reflect.TypeFor[*stage3C](),
		reflect.TypeFor[*Node](), reflect.TypeFor[Scope](), reflect.TypeFor[*dispatch](),
		reflect.TypeFor[ProviderRef[stage3C]](),
	} {
		if value.Type() == forbidden {
			return true
		}
	}
	switch value.Kind() {
	case reflect.Func:
		return !value.IsNil()
	case reflect.Interface, reflect.Pointer:
		return !value.IsNil() && m5InstanceBearing(value.Elem(), depth+1)
	case reflect.Struct:
		for field := range value.NumField() {
			if m5InstanceBearing(value.Field(field), depth+1) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		for index := range value.Len() {
			if m5InstanceBearing(value.Index(index), depth+1) {
				return true
			}
		}
	case reflect.Map:
		iterator := value.MapRange()
		for iterator.Next() {
			if m5InstanceBearing(iterator.Key(), depth+1) || m5InstanceBearing(iterator.Value(), depth+1) {
				return true
			}
		}
	}
	return false
}
