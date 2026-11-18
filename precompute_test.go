package heiry

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestM7PrecomputePreservesDynamicSourceAndEmbeddedProviders(t *testing.T) {
	d := m5Stage3Definition()
	dynamic := d.MustPrepare()
	existing := &stage3C{}
	existing.stage3A.value, existing.stage3B.value, existing.value = 1, 2, 3
	dynamic.MustInit(existing, dynamic.NodeProvider(&existing.stage3A), dynamic.NodeProvider(&existing.stage3B), dynamic.NodeProvider(existing))
	fast, err := dynamic.Precompute()
	if err != nil || fast == nil || fast == dynamic {
		t.Fatalf("Precompute = (%p, %v)", fast, err)
	}
	if again, err := fast.Precompute(); err != nil || again != fast {
		t.Fatalf("repeated Precompute = (%p, %v)", again, err)
	}
	if dynamic.owners != nil || fast.owners == nil {
		t.Fatal("conversion changed dynamic source or omitted resolution table")
	}
	if existing.Node.Scope().dispatch.owners != nil || Method[stage3Read](existing.Node.Scope()).Read() != 6 {
		t.Fatal("conversion changed an already initialized dynamic object")
	}
	if m5InstanceBearing(reflect.ValueOf(fast), 0) {
		t.Fatal("converted Plan retained instance-bearing metadata")
	}
	for _, plan := range []*Plan[stage3C]{dynamic, fast} {
		leaf := &stage3C{}
		leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
		if err := plan.Init(leaf, d.NodeProvider(&leaf.stage3A), d.NodeProvider(&leaf.stage3B), d.NodeProvider(leaf)); err != nil {
			t.Fatalf("Init: %v", err)
		}
		if got := Method[stage3Add](leaf.Node.Scope()).Add(4); got != 18 {
			t.Fatalf("Add = %d", got)
		}
		if got := m7BoundProvider(leaf.Node.Scope().dispatch, 0, reflect.TypeFor[stage3Add]()); got != &leaf.stage3A {
			t.Fatalf("root provider = %p, want %p", got, &leaf.stage3A)
		}
	}
}

// m7BoundProvider reads the provider one object holds for a capability at a
// level, from the level map of a dynamic object or the slot of a precomputed one.
func m7BoundProvider(d *dispatch, level int, capability reflect.Type) any {
	if d.owners == nil {
		return d.levels[level][capability]
	}
	return d.providers[d.owners[capability].slot[level]]
}

func TestM7PrecomputedPlanStillChecksEachInstance(t *testing.T) {
	plan := m5Stage3Definition().MustPrepare().MustPrecompute()
	bad, foreign := &stage3C{}, &stage3C{}
	if err := plan.Init(bad, plan.NodeProvider(&foreign.stage3A), plan.NodeProvider(&bad.stage3B), plan.NodeProvider(bad)); !errors.Is(err, &Error{Code: ProviderOwnership}) {
		t.Fatalf("foreign provider = %v", err)
	}
	for _, node := range []*Node{&bad.stage3A.Node, &bad.stage3B.Node, &bad.Node} {
		if node.Scope().dispatch != nil {
			t.Fatal("failed Init changed a scope")
		}
	}
	if err := plan.Init(bad, plan.NodeProvider(&bad.stage3A), plan.NodeProvider(&bad.stage3B), plan.NodeProvider(bad)); err != nil {
		t.Fatalf("valid Init after failure: %v", err)
	}
	if err := plan.Init(bad, plan.NodeProvider(&bad.stage3A), plan.NodeProvider(&bad.stage3B), plan.NodeProvider(bad)); !errors.Is(err, &Error{Code: AlreadyInitialized}) {
		t.Fatalf("repeat Init = %v", err)
	}
	good := &stage3C{}
	if err := plan.Init(good, plan.NodeProvider(&good.stage3A), plan.NodeProvider(&good.stage3B), plan.NodeProvider(good)); err != nil {
		t.Fatalf("Plan damaged by failed object: %v", err)
	}
}

func TestM7PrecomputedSameObjectConcurrentReadOnlyDispatch(t *testing.T) {
	plan := m5Stage3Definition().MustPrepare().MustPrecompute()
	leaf := &stage3C{}
	leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
	plan.MustInit(leaf, plan.NodeProvider(&leaf.stage3A), plan.NodeProvider(&leaf.stage3B), plan.NodeProvider(leaf))
	var workers sync.WaitGroup
	for range 12 {
		workers.Go(func() {
			for range 100 {
				if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != 6 {
					t.Errorf("Read = %d", got)
				}
				if got := ParentMethod[stage3Read](leaf.stage3B.Node.Scope()).Read(); got != 1 {
					t.Errorf("parent Read = %d", got)
				}
			}
		})
	}
	workers.Wait()
}

func TestM7OnePrecomputedPlanInitializesIndependentObjectsConcurrently(t *testing.T) {
	plan := m5Stage3Definition().MustPrepare().MustPrecompute()
	const count = 16
	leaves := make([]*stage3C, count)
	var workers sync.WaitGroup
	for index := range leaves {
		workers.Add(1)
		go func() {
			defer workers.Done()
			leaf := &stage3C{}
			leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
			if err := plan.Init(leaf, plan.NodeProvider(&leaf.stage3A), plan.NodeProvider(&leaf.stage3B), plan.NodeProvider(leaf)); err != nil {
				t.Errorf("Init: %v", err)
				return
			}
			if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != 6 {
				t.Errorf("Read = %d", got)
			}
			leaves[index] = leaf
		}()
	}
	workers.Wait()
	if leaves[0] == nil {
		t.Fatal("first object was not initialized")
	}
	for index, leaf := range leaves {
		if leaf == nil || leaf.Node.Scope().dispatch == nil || index > 0 && leaf.Node.Scope().dispatch == leaves[0].Node.Scope().dispatch {
			t.Fatalf("object %d has missing or shared binding", index)
		}
	}
}

func TestM7DynamicAndPrecomputedSemantics(t *testing.T) {
	for _, schedule := range []string{"eager", "first use"} {
		for _, mode := range []string{"dynamic", "precomputed"} {
			t.Run(schedule+"/"+mode, func(t *testing.T) {
				d := m5Stage3Definition()
				leaf := &stage3C{}
				leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
				if schedule == "first use" && mode == "dynamic" {
					if err := d.Init(leaf, d.NodeProvider(&leaf.stage3A), d.NodeProvider(&leaf.stage3B), d.NodeProvider(leaf)); err != nil {
						t.Fatalf("lazy Init: %v", err)
					}
				} else {
					var plan *Plan[stage3C]
					if schedule == "eager" {
						plan = d.MustPrepare()
					}
					if plan == nil {
						plan = d.MustPrepare()
					}
					if mode == "precomputed" {
						plan = plan.MustPrecompute()
					}
					if err := plan.Init(leaf, d.NodeProvider(&leaf.stage3A), d.NodeProvider(&leaf.stage3B), d.NodeProvider(leaf)); err != nil {
						t.Fatalf("Init: %v", err)
					}
				}
				if got := Method[stage3Walk](leaf.Node.Scope()).Walk(1); got != "C/B/C/B/A" {
					t.Fatalf("reentry = %q", got)
				}
				if got := ParentMethod[stage3Read](leaf.stage3B.Node.Scope()).Read(); got != 1 {
					t.Fatalf("parent Read = %d", got)
				}
				if got := Method[stage3Add](leaf.Node.Scope()).Add(4); got != 18 {
					t.Fatalf("Add = %d", got)
				}
				if leaf.stage3A.value != 5 || leaf.stage3B.value != 6 || leaf.value != 7 {
					t.Fatalf("state = (%d, %d, %d)", leaf.stage3A.value, leaf.stage3B.value, leaf.value)
				}
				assertDispatchPanic(t, NoParentImplementation, reflect.TypeFor[stage3Read](), 0, func() {
					ParentMethod[stage3Read](leaf.stage3A.Node.Scope())
				})
				assertDispatchPanic(t, UnknownCapability, reflect.TypeFor[stage1Unknown](), 2, func() {
					Method[stage1Unknown](leaf.Node.Scope())
				})
			})
		}
	}
	for _, mode := range []string{"dynamic", "precomputed"} {
		t.Run("private/"+mode, func(t *testing.T) {
			plan := m5PrivateDefinition().MustPrepare()
			if mode == "precomputed" {
				plan = plan.MustPrecompute()
			}
			leaf := &stage4B{}
			plan.MustInit(leaf, plan.NodeProvider(&leaf.stage4A), plan.NodeProvider(leaf))
			if got := (&stage4Object{leaf: leaf}).Read(); got != "B/A/B/A.secret" {
				t.Fatalf("public/private = %q", got)
			}
		})
		t.Run("application errors/"+mode, func(t *testing.T) {
			plan := m5ErrorDefinition().MustPrepare()
			if mode == "precomputed" {
				plan = plan.MustPrecompute()
			}
			leaf := &m5ErrorLeaf{}
			plan.MustInit(leaf, plan.NodeProvider(&leaf.m5ErrorRoot), plan.NodeProvider(leaf))
			if got, err := Method[m5ErrorOperation](leaf.Node.Scope()).Calculate(-1); got != 0 || !errors.Is(err, stage1ApplicationError) {
				t.Fatalf("application error = (%d, %v)", got, err)
			}
		})
	}
}

func TestM7ConcurrentConversionAndReuse(t *testing.T) {
	source := m5Stage3Definition().MustPrepare()
	const count = 12
	var workers sync.WaitGroup
	for range count {
		workers.Go(func() {
			dynamicLeaf := &stage3C{}
			dynamicLeaf.stage3A.value, dynamicLeaf.stage3B.value, dynamicLeaf.value = 1, 2, 3
			if err := source.Init(dynamicLeaf, source.NodeProvider(&dynamicLeaf.stage3A), source.NodeProvider(&dynamicLeaf.stage3B), source.NodeProvider(dynamicLeaf)); err != nil {
				t.Errorf("dynamic Init during conversion: %v", err)
				return
			}
			fast, err := source.Precompute()
			if err != nil {
				t.Errorf("conversion: %v", err)
				return
			}
			leaf := &stage3C{}
			leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
			if err := fast.Init(leaf, source.NodeProvider(&leaf.stage3A), source.NodeProvider(&leaf.stage3B), source.NodeProvider(leaf)); err != nil {
				t.Errorf("Init: %v", err)
				return
			}
			if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != 6 {
				t.Errorf("Read = %d", got)
			}
			if got := Method[stage3Read](dynamicLeaf.Node.Scope()).Read(); got != 6 {
				t.Errorf("dynamic Read = %d", got)
			}
		})
	}
	workers.Wait()
	if source.owners != nil || m5InstanceBearing(reflect.ValueOf(source), 0) {
		t.Fatal("conversion changed source or captured an instance")
	}
}

type m7RootOnly interface{ RootOnly() string }
type m7Other interface{ Other() string }
type m7SparsePublic interface {
	RootOnly() string
	Other() string
}

type m7SparseA struct{ Node }

func (*m7SparseA) RootOnly() string { return "A.root" }
func (*m7SparseA) Other() string    { return "A.other" }

type m7SparseB struct {
	m7SparseA
	Node
}

type m7SparseC struct {
	m7SparseB
	Node
}

func (c *m7SparseC) Other() string {
	return "C/" + ParentMethod[m7Other](c.Node.Scope()).Other() + "/" + ParentMethod[m7RootOnly](c.Node.Scope()).RootOnly()
}

func m7SparseDefinition() *Definition[m7SparseC] {
	d := Define[m7SparseC]()
	d.Level[m7SparseA, m7SparsePublic, contractEmpty]("").
		Bind[m7RootOnly]((*m7SparseA)(nil), ".").
		Bind[m7Other]((*m7SparseA)(nil), ".").End().
		Level[m7SparseB, m7SparsePublic, contractEmpty]("m7SparseA").End().
		Level[m7SparseC, m7SparsePublic, contractEmpty]("m7SparseB").
		Bind[m7Other]((*m7SparseC)(nil), ".")
	return d
}

func TestM7SparseResolutionAndNoDynamicFallback(t *testing.T) {
	for _, mode := range []string{"dynamic", "precomputed"} {
		plan := m7SparseDefinition().MustPrepare()
		if mode == "precomputed" {
			plan = plan.MustPrecompute()
		}
		leaf := &m7SparseC{}
		plan.MustInit(leaf, plan.NodeProvider(&leaf.m7SparseA), plan.NodeProvider(&leaf.m7SparseB), plan.NodeProvider(leaf))
		if got := Method[m7RootOnly](leaf.Node.Scope()); got != &leaf.m7SparseA {
			t.Fatalf("%s root-only provider = %p, want %p", mode, got, &leaf.m7SparseA)
		}
		if got := Method[m7Other](leaf.Node.Scope()).Other(); got != "C/A.other/A.root" {
			t.Fatalf("%s virtual Other = %q", mode, got)
		}
		if got := ParentMethod[m7Other](leaf.m7SparseB.Node.Scope()); got != &leaf.m7SparseA {
			t.Fatalf("%s skipped parent = %p, want %p", mode, got, &leaf.m7SparseA)
		}
		assertDispatchPanic(t, NoParentImplementation, reflect.TypeFor[m7RootOnly](), 0, func() {
			ParentMethod[m7RootOnly](leaf.m7SparseA.Node.Scope())
		})
		if mode == "precomputed" {
			if leaf.Node.Scope().dispatch.owners == nil {
				t.Fatal("precomputed object has no table")
			}
			if leaf.Node.Scope().dispatch.levels != nil {
				t.Fatal("precomputed object kept level maps that a lookup could scan")
			}
		} else if leaf.Node.Scope().dispatch.owners != nil {
			t.Fatal("dynamic object unexpectedly has a table")
		}
	}
	capability := reflect.TypeFor[m7Other]()
	a, c := &m7SparseA{}, &m7SparseC{}
	// The table contradicts what a scan of the same registrations would find.
	// The level maps stay populated so that a fallback scan would be visible.
	proof := &dispatch{
		known:     map[reflect.Type]struct{}{capability: {}},
		levels:    []map[reflect.Type]any{{capability: a}, {capability: c}},
		owners:    map[reflect.Type]ownerRow{capability: {owner: []int{-1, -1, 0}, slot: []int{0, 1}}},
		providers: []any{a, c},
	}
	if got := Method[m7Other](Scope{dispatch: proof, level: 1}); got != a {
		t.Fatalf("table owner = %p, want %p", got, a)
	}
	assertDispatchPanic(t, NoParentImplementation, capability, 1, func() {
		ParentMethod[m7Other](Scope{dispatch: proof, level: 1})
	})
}

func TestM7DistinctDefinitionsAndModesKeepTheirOwnResolution(t *testing.T) {
	makeDefinition := func(override bool) *Definition[stage3C] {
		d := Define[stage3C]()
		d.Level[stage3A, stage3Read, contractEmpty]("").Bind[stage3Read]((*stage3A)(nil), ".").End().
			Level[stage3B, stage3Read, contractEmpty]("stage3A").End()
		leaf := d.Level[stage3C, stage3Read, contractEmpty]("stage3B")
		if override {
			leaf.Bind[stage3Read]((*stage3C)(nil), ".")
		}
		return d
	}
	for _, variation := range []struct {
		override bool
		want     int
	}{{false, 1}, {true, 4}} {
		d := makeDefinition(variation.override)
		for _, plan := range []*Plan[stage3C]{d.MustPrepare(), d.MustPrepare().MustPrecompute()} {
			leaf := &stage3C{}
			leaf.stage3A.value, leaf.stage3B.value, leaf.value = 1, 2, 3
			plan.MustInit(leaf, d.NodeProvider(&leaf.stage3A), d.NodeProvider(&leaf.stage3B), d.NodeProvider(leaf))
			if got := Method[stage3Read](leaf.Node.Scope()).Read(); got != variation.want {
				t.Fatalf("override=%t precomputed=%t: Read=%d, want %d", variation.override, plan.owners != nil, got, variation.want)
			}
		}
	}
	valid := makeDefinition(true).MustPrepare().MustPrecompute()
	invalid := Define[stage3C]()
	invalid.Level[stage3A, stage3Read, contractEmpty]("").Bind[stage3Read]((*stage3A)(nil), ".").End().
		Level[stage3B, stage3Read, contractEmpty]("wrongParent").End().
		Level[stage3C, stage3Read, contractEmpty]("stage3B")
	if result, err := invalid.Prepare(); result != nil || !errors.Is(err, &Error{Code: InvalidHierarchy}) {
		t.Fatalf("same-leaf invalid path = (%p, %v)", result, err)
	}
	leaf := &stage3C{}
	if err := valid.Init(leaf, invalid.NodeProvider(&leaf.stage3A), invalid.NodeProvider(&leaf.stage3B), invalid.NodeProvider(leaf)); !errors.Is(err, &Error{Code: BindingMismatch}) {
		t.Fatalf("foreign definition references = %v", err)
	}
	if leaf.Node.Scope().dispatch != nil {
		t.Fatal("foreign definition changed a scope")
	}
	for _, variation := range []struct {
		name  string
		build func() *Definition[stage3C]
		code  Code
	}{
		{"contract narrowing", func() *Definition[stage3C] {
			d := Define[stage3C]()
			d.Level[stage3A, stage5Public, contractEmpty]("").
				Bind[stage3Read]((*stage3A)(nil), ".").
				Bind[stage3Add]((*stage3A)(nil), ".").
				Bind[stage3Walk]((*stage3A)(nil), ".").End().
				Level[stage3B, stage3Read, contractEmpty]("stage3A").End().
				Level[stage3C, stage3Read, contractEmpty]("stage3B")
			return d
		}, ContractExtension},
		{"canonical identity", func() *Definition[stage3C] {
			d := Define[stage3C]()
			d.Level[stage3A, stage3Read, contractEmpty]("").Bind[stage3Read]((*stage3A)(nil), ".").End().
				Level[stage3B, stage3Read, contractEmpty]("stage3A").Bind[m4Read]((*stage3B)(nil), ".").End().
				Level[stage3C, stage3Read, contractEmpty]("stage3B")
			return d
		}, CapabilityIdentity},
		{"provider type", func() *Definition[stage3C] {
			d := Define[stage3C]()
			d.Level[stage3A, stage3Read, contractEmpty]("").Bind[stage3Read]((*stage3B)(nil), ".").End().
				Level[stage3B, stage3Read, contractEmpty]("stage3A").End().
				Level[stage3C, stage3Read, contractEmpty]("stage3B")
			return d
		}, InvalidProvider},
		{"provider path", func() *Definition[stage3C] {
			d := Define[stage3C]()
			d.Level[stage3A, stage3Read, contractEmpty]("").Bind[stage3Read]((*stage3A)(nil), "wrong").End().
				Level[stage3B, stage3Read, contractEmpty]("stage3A").End().
				Level[stage3C, stage3Read, contractEmpty]("stage3B")
			return d
		}, InvalidProvider},
	} {
		t.Run(variation.name, func(t *testing.T) {
			if result, err := variation.build().Prepare(); result != nil || !errors.Is(err, &Error{Code: variation.code}) {
				t.Fatalf("same-leaf invalid definition = (%p, %v), want %s", result, err, variation.code)
			}
		})
	}
}

func TestM7PrecomputeInvalidPlanAndMust(t *testing.T) {
	var plan *Plan[stage3C]
	if converted, err := plan.Precompute(); converted != nil || !errors.Is(err, &Error{Code: InvalidPlan}) {
		t.Fatalf("nil Precompute = (%p, %v)", converted, err)
	}
	zero := new(Plan[stage3C])
	if converted, err := zero.Precompute(); converted != nil || !errors.Is(err, &Error{Code: InvalidPlan}) {
		t.Fatalf("zero Precompute = (%p, %v)", converted, err)
	}
	defer func() {
		if recovered := recover(); !errors.Is(recovered.(error), &Error{Code: InvalidPlan}) {
			t.Errorf("MustPrecompute panic = %v", recovered)
		}
	}()
	plan.MustPrecompute()
}
