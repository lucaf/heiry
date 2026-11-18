package heiry

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

type m8Read interface{ Read() int }
type m8Write interface{ Write(int) }
type m8Empty interface{}

type m8Reader struct {
	Node
	value int
}

func (r *m8Reader) Read() int  { return r.value }
func (r *m8Reader) Count() int { return r.value + 1 }
func (r *m8Reader) Audit() int { return r.value + 2 }

type m8Writer struct {
	Node
	value int
}

func (w *m8Writer) Write(value int) { w.value = value }

type m8Leaf struct {
	Node
	reader m8Reader
	writer m8Writer
}

type m8Public interface {
	m8Read
	m8Write
}

func TestM8ComponentSlotsAndEmbeddedState(t *testing.T) {
	d := Define[m8Leaf]()
	d.Level[m8Leaf, m8Public, m8Empty]("").
		Bind[m8Read]((*m8Reader)(nil), "reader").
		BindGroup((*m8Writer)(nil), "writer", Of[m8Write]()).End()
	plan, err := d.Prepare()
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	leaf := &m8Leaf{}
	if err := plan.Init(leaf,
		plan.NodeProvider(leaf),
		plan.ComponentProvider[m8Leaf]("reader", &leaf.reader),
		plan.ComponentProvider[m8Leaf]("writer", &leaf.writer)); err != nil {
		t.Fatalf("Init: %v", err)
	}
	Method[m8Write](leaf.Node.Scope()).Write(17)
	if got := Method[m8Read](leaf.Node.Scope()).Read(); got != 0 {
		t.Fatalf("Read = %d", got)
	}
	if leaf.writer.value != 17 || leaf.reader.Node.Scope().level != 0 || leaf.writer.Node.Scope().level != 0 {
		t.Fatalf("component state or scopes: %+v", leaf)
	}
}

type m8Count interface{ Count() int }
type m8Audit interface{ Audit() int }
type m8TwinPublic interface {
	m8Read
	m8Count
	m8Audit
}
type m8TwinLeaf struct {
	Node
	First  m8Reader
	Nested struct{ Second m8Reader }
}

func m8TwinDefinition() *Definition[m8TwinLeaf] {
	d := Define[m8TwinLeaf]()
	d.Level[m8TwinLeaf, m8TwinPublic, m8Empty]("").
		BindGroup((*m8Reader)(nil), "First", Of[m8Read](), Of[m8Count]()).
		Bind[m8Audit]((*m8Reader)(nil), "Nested.Second").End()
	return d
}

func TestM8SameTypeSlotsPlanReuseAndModes(t *testing.T) {
	d := m8TwinDefinition()
	first := &m8TwinLeaf{}
	first.First.value, first.Nested.Second.value = 10, 20
	// References may precede preparation, and their argument order is irrelevant.
	firstRefs := []ProviderRef[m8TwinLeaf]{
		d.ComponentProvider[m8TwinLeaf]("Nested.Second", &first.Nested.Second),
		d.NodeProvider(first),
		d.ComponentProvider[m8TwinLeaf]("First", &first.First),
	}
	dynamic, err := d.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	fast := dynamic.MustPrecompute()
	if err := fast.Init(first, firstRefs...); err != nil {
		t.Fatalf("precomputed Init: %v", err)
	}
	foreign := &m8TwinLeaf{}
	firstRefs[0] = fast.ComponentProvider[m8TwinLeaf]("Nested.Second", &foreign.Nested.Second)
	second := &m8TwinLeaf{}
	second.First.value, second.Nested.Second.value = 30, 40
	if err := dynamic.Init(second,
		dynamic.NodeProvider(second),
		dynamic.ComponentProvider[m8TwinLeaf]("First", &second.First),
		dynamic.ComponentProvider[m8TwinLeaf]("Nested.Second", &second.Nested.Second)); err != nil {
		t.Fatalf("dynamic Init: %v", err)
	}
	for _, tc := range []struct {
		leaf               *m8TwinLeaf
		read, count, audit int
	}{{first, 10, 11, 22}, {second, 30, 31, 42}} {
		if got := Method[m8Read](tc.leaf.Node.Scope()); got != &tc.leaf.First || got.Read() != tc.read {
			t.Fatalf("Read provider=%p value=%d", got, got.Read())
		}
		if got := Method[m8Count](tc.leaf.Node.Scope()); got != &tc.leaf.First || got.Count() != tc.count {
			t.Fatalf("Count provider=%p value=%d", got, got.Count())
		}
		if got := Method[m8Audit](tc.leaf.Node.Scope()); got != &tc.leaf.Nested.Second || got.Audit() != tc.audit {
			t.Fatalf("Audit provider=%p value=%d", got, got.Audit())
		}
		for _, holder := range []*Node{&tc.leaf.Node, &tc.leaf.First.Node, &tc.leaf.Nested.Second.Node} {
			if holder.Scope().level != 0 || holder.Scope().dispatch != tc.leaf.Node.Scope().dispatch {
				t.Fatal("component did not share its owning level's binding")
			}
		}
	}
	first.First.value = 99
	if Method[m8Read](first.Node.Scope()).Read() != 99 || Method[m8Read](second.Node.Scope()).Read() != 30 {
		t.Fatal("independent instances share state")
	}
	if m5InstanceBearing(reflect.ValueOf(d), 0) || m5InstanceBearing(reflect.ValueOf(dynamic), 0) || m5InstanceBearing(reflect.ValueOf(fast), 0) {
		t.Fatal("component Plan retained an instance")
	}
}

func TestM8ComponentReferenceErrorsAreTypedAndInstanceLocal(t *testing.T) {
	d := m8TwinDefinition()
	leaf := &m8TwinLeaf{}
	badPath := d.ComponentProvider[m8TwinLeaf]("Missing", &leaf.First)
	malformed := d.ComponentProvider[m8TwinLeaf]("Nested..Second", &leaf.Nested.Second)
	if err := d.Validate(); err != nil {
		t.Fatalf("reference creation changed declaration: %v", err)
	}
	plan := d.MustPrepare()
	var nilPlan *Plan[m8TwinLeaf]
	var nilDefinition *Definition[m8TwinLeaf]
	foreign := &m8TwinLeaf{}
	tests := []struct {
		name string
		refs []ProviderRef[m8TwinLeaf]
		code Code
		path string
	}{
		{"missing node", []ProviderRef[m8TwinLeaf]{plan.ComponentProvider[m8TwinLeaf]("First", &leaf.First), plan.ComponentProvider[m8TwinLeaf]("Nested.Second", &leaf.Nested.Second)}, BindingMismatch, ""},
		{"unknown slot", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), badPath}, BindingMismatch, "Missing"},
		{"malformed path", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), malformed}, InvalidProvider, "Nested..Second"},
		{"reserved node path", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), plan.ComponentProvider[m8TwinLeaf](".", &leaf.First)}, InvalidProvider, ""},
		{"nil pointer", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), plan.ComponentProvider[m8TwinLeaf]("First", (*m8Reader)(nil))}, InvalidProvider, "First"},
		{"wrong type", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), plan.ComponentProvider[m8TwinLeaf]("First", &m8Writer{})}, InvalidProvider, "First"},
		{"unknown owner", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), plan.ComponentProvider[m8Reader]("First", &leaf.First)}, InvalidProvider, "First"},
		{"foreign pointer", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), plan.ComponentProvider[m8TwinLeaf]("First", &foreign.First), plan.ComponentProvider[m8TwinLeaf]("Nested.Second", &leaf.Nested.Second)}, ProviderOwnership, "First"},
		{"duplicate slot", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), plan.ComponentProvider[m8TwinLeaf]("First", &leaf.First), plan.ComponentProvider[m8TwinLeaf]("First", &leaf.First)}, BindingMismatch, "First"},
		{"missing slot", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), plan.ComponentProvider[m8TwinLeaf]("First", &leaf.First)}, BindingMismatch, "Nested.Second"},
		{"unusable Plan factory", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), nilPlan.ComponentProvider[m8TwinLeaf]("First", &leaf.First)}, InvalidPlan, "First"},
		{"unprepared Plan factory", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), (&Plan[m8TwinLeaf]{}).ComponentProvider[m8TwinLeaf]("First", &leaf.First)}, InvalidPlan, "First"},
		{"unusable Definition factory", []ProviderRef[m8TwinLeaf]{plan.NodeProvider(leaf), nilDefinition.ComponentProvider[m8TwinLeaf]("First", &leaf.First)}, InvalidDeclaration, "First"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := plan.Init(leaf, tc.refs...)
			var diagnostic *Error
			if !errors.As(err, &diagnostic) || diagnostic.Code != tc.code || diagnostic.Path != tc.path || diagnostic.Target != reflect.TypeFor[m8TwinLeaf]() {
				t.Fatalf("diagnostic = %+v, want %s at %q", diagnostic, tc.code, tc.path)
			}
			if tc.name == "unknown owner" && diagnostic.Level != -1 {
				t.Fatalf("unknown owner invented level %d", diagnostic.Level)
			}
			for _, holder := range []*Node{&leaf.Node, &leaf.First.Node, &leaf.Nested.Second.Node} {
				if holder.Scope().dispatch != nil {
					t.Fatal("failed Init wrote a scope")
				}
			}
		})
	}
}

type m8BadStorage struct {
	Node
	Direct   m8Reader
	Ptr      *m8Reader
	Any      any
	List     []m8Reader
	Map      map[string]m8Reader
	Nested   struct{ Good m8Reader }
	Promoted m8PromotedReader
}

type m8PromotedReader struct{ m8Reader }

type m8Wrapper struct{ Good m8Reader }
type m8PromotedPath struct {
	Node
	m8Wrapper
}

func TestM8PromotedSelectorShortcutIsRejected(t *testing.T) {
	d := Define[m8PromotedPath]()
	d.Level[m8PromotedPath, m8Read, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "Good")
	err := d.Validate()
	if !errors.Is(err, &Error{Code: InvalidProvider}) || !errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("promoted selector path = %v", err)
	}
	valid := Define[m8PromotedPath]()
	valid.Level[m8PromotedPath, m8Read, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "m8Wrapper.Good")
	if err := valid.Validate(); err != nil {
		t.Fatalf("declared direct field path: %v", err)
	}
}

type m8BlankStorage struct {
	Node
	_ m8Reader
}

func TestM8BlankFieldIsNotAProviderPath(t *testing.T) {
	d := Define[m8BlankStorage]()
	d.Level[m8BlankStorage, m8Read, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "_")
	err := d.Validate()
	if !errors.Is(err, &Error{Code: InvalidProvider}) || !errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("blank-field path = %v", err)
	}
}

func TestM8DefinitionRejectsForbiddenPathsAndMissingDirectHolder(t *testing.T) {
	for _, tc := range []struct {
		path     string
		provider m8Read
		code     Code
	}{
		{"Ptr", (*m8Reader)(nil), InvalidProvider},
		{"Any", (*m8Reader)(nil), InvalidProvider},
		{"List", (*m8Reader)(nil), InvalidProvider},
		{"Map", (*m8Reader)(nil), InvalidProvider},
		{"Nested", (*m8Reader)(nil), MissingNode},
		{"Promoted", (*m8PromotedReader)(nil), MissingNode},
		{"Good", (*m8Reader)(nil), InvalidProvider}, // no promoted field shortcut
	} {
		t.Run(tc.path, func(t *testing.T) {
			d := Define[m8BadStorage]()
			d.Level[m8BadStorage, m8Read, m8Empty]("").Bind[m8Read](tc.provider, tc.path)
			err := d.Validate()
			if !errors.Is(err, &Error{Code: tc.code}) || !errors.Is(err, &Error{Code: MissingMethods}) {
				t.Fatalf("validation = %v, want %s and missing Read", err, tc.code)
			}
		})
	}
	d := Define[m8BadStorage]()
	d.Level[m8BadStorage, m8Read, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "Nested.Good")
	if err := d.Validate(); err != nil {
		t.Fatalf("direct nested value path: %v", err)
	}
}

type m8HolderRoot struct {
	Node
	Bad m8PromotedReader
}
type m8HolderMiddle struct {
	m8HolderRoot
	Node
}
type m8HolderLeaf struct {
	m8HolderMiddle
	Node
}

func TestM8MissingComponentHolderHasLeafRelativePath(t *testing.T) {
	d := Define[m8HolderLeaf]()
	d.Level[m8HolderRoot, m8Read, m8Empty]("").Bind[m8Read]((*m8PromotedReader)(nil), "Bad").End().
		Level[m8HolderMiddle, m8Read, m8Empty]("m8HolderRoot").End().
		Level[m8HolderLeaf, m8Read, m8Empty]("m8HolderMiddle")
	err := d.Validate()
	var found bool
	for _, diagnostic := range m8Diagnostics(err) {
		if diagnostic.Code == MissingNode {
			found = diagnostic.Path == "m8HolderMiddle.m8HolderRoot.Bad.Node" && diagnostic.Level == 0 && diagnostic.Target == reflect.TypeFor[m8HolderLeaf]()
		}
	}
	if !found || !errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("component holder and missing method diagnostics = %v", err)
	}
}

type m8Parent struct {
	Node
	Part m8Reader
}
type m8Child struct {
	m8Parent
	Node
	Own m8Reader
}

func TestM8ParentSubtreeIsNotChildComponentOwnership(t *testing.T) {
	d := Define[m8Child]()
	d.Level[m8Parent, m8Read, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "Part").End().
		Level[m8Child, m8Read, m8Empty]("m8Parent").Bind[m8Read]((*m8Reader)(nil), "m8Parent.Part")
	err := d.Validate()
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != ProviderOwnership || diagnostic.Path != "m8Parent.Part" || diagnostic.Level != 1 {
		t.Fatalf("parent-subtree diagnostic = %+v", diagnostic)
	}
}

func TestM8InvalidParentEdgeDoesNotInventComponentPrefix(t *testing.T) {
	d := Define[m8HierarchyLeaf]()
	d.Level[m8HierarchyRoot, m8Read, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "Missing").End().
		Level[m8HierarchyMiddle, m8Read, m8Empty]("wrongParent").End().
		Level[m8HierarchyLeaf, m8Read, m8Empty]("m8HierarchyMiddle")
	err := d.Validate()
	if !errors.Is(err, &Error{Code: InvalidHierarchy}) || !errors.Is(err, &Error{Code: InvalidProvider}) {
		t.Fatalf("structural and component errors = %v", err)
	}
	for _, diagnostic := range m8Diagnostics(err) {
		if diagnostic.Code == InvalidProvider && diagnostic.Path != "Missing" {
			t.Fatalf("invalid edge invented path %q", diagnostic.Path)
		}
	}
}

func TestM8StickyComponentDiagnosticUsesEstablishedLeafPath(t *testing.T) {
	d := Define[m8HierarchyLeaf]()
	root := d.Level[m8HierarchyRoot, m8Read, m8Empty]("")
	root.Bind[m8Read]((*m8Reader)(nil), "First")
	root.Bind[m8Read]((*m8Reader)(nil), "Bad..Path") // malformed call, no binding
	d.Level[m8HierarchyMiddle, m8Read, m8Empty]("m8HierarchyRoot").End().
		Level[m8HierarchyLeaf, m8Read, m8Empty]("m8HierarchyMiddle")
	err := d.Validate()
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != InvalidProvider || diagnostic.Path != "m8HierarchyMiddle.m8HierarchyRoot.Bad..Path" {
		t.Fatalf("sticky component path = %+v", diagnostic)
	}
}

func m8Diagnostics(err error) []*Error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var all []*Error
		for _, item := range joined.Unwrap() {
			all = append(all, m8Diagnostics(item)...)
		}
		return all
	}
	var diagnostic *Error
	if errors.As(err, &diagnostic) {
		return []*Error{diagnostic}
	}
	return nil
}

func TestM8DuplicatesAcrossProvidersAndStyles(t *testing.T) {
	public := Define[m8TwinLeaf]()
	public.Level[m8TwinLeaf, m8TwinPublic, m8Empty]("").
		BindGroup((*m8Reader)(nil), "First", Of[m8Read](), Of[m8Count]()).
		Bind[m8Read]((*m8Reader)(nil), "Nested.Second")
	err := public.Validate()
	if !errors.Is(err, &Error{Code: DuplicateBinding}) || !errors.Is(err, &Error{Code: MissingMethods}) {
		t.Fatalf("duplicate and missing Audit = %v", err)
	}
}

type m8HierarchyRoot struct {
	Node
	First m8Reader
}
type m8HierarchyMiddle struct {
	m8HierarchyRoot
	Node
	Second m8Reader
}
type m8HierarchyLeaf struct {
	m8HierarchyMiddle
	Node
}

func m8HierarchyDefinition() *Definition[m8HierarchyLeaf] {
	d := Define[m8HierarchyLeaf]()
	d.Level[m8HierarchyRoot, m8Read, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "First").End().
		Level[m8HierarchyMiddle, m8Read, m8Empty]("m8HierarchyRoot").Bind[m8Read]((*m8Reader)(nil), "Second").End().
		Level[m8HierarchyLeaf, m8Read, m8Empty]("m8HierarchyMiddle").End()
	return d
}

func m8HierarchyRefs(p *Plan[m8HierarchyLeaf], x *m8HierarchyLeaf) []ProviderRef[m8HierarchyLeaf] {
	return []ProviderRef[m8HierarchyLeaf]{
		p.NodeProvider(&x.m8HierarchyRoot),
		p.ComponentProvider[m8HierarchyRoot]("First", &x.m8HierarchyRoot.First),
		p.NodeProvider(&x.m8HierarchyMiddle),
		p.ComponentProvider[m8HierarchyMiddle]("Second", &x.m8HierarchyMiddle.Second),
		p.NodeProvider(x),
	}
}

func TestM8LaterHolderFailureIsAtomicAndPathsAreLeafRelative(t *testing.T) {
	plan := m8HierarchyDefinition().MustPrepare()
	leaf := &m8HierarchyLeaf{}
	foreign := &m8Reader{}
	refs := m8HierarchyRefs(plan, leaf)
	refs[3] = plan.ComponentProvider[m8HierarchyMiddle]("Second", foreign)
	err := plan.Init(leaf, refs...)
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != ProviderOwnership || diagnostic.Path != "m8HierarchyMiddle.Second" || diagnostic.Level != 1 {
		t.Fatalf("foreign later provider = %+v", diagnostic)
	}
	for _, holder := range []*Node{&leaf.m8HierarchyRoot.Node, &leaf.m8HierarchyRoot.First.Node, &leaf.m8HierarchyMiddle.Node, &leaf.m8HierarchyMiddle.Second.Node, &leaf.Node} {
		if holder.Scope().dispatch != nil {
			t.Fatal("later provider failure changed a prior holder")
		}
	}
	if err := plan.Init(leaf, m8HierarchyRefs(plan, leaf)...); err != nil {
		t.Fatalf("Init after failure: %v", err)
	}
	if got := Method[m8Read](leaf.Node.Scope()); got != &leaf.m8HierarchyMiddle.Second {
		t.Fatalf("virtual provider = %v", got)
	}
	if got := ParentMethod[m8Read](leaf.m8HierarchyMiddle.Second.Node.Scope()); got != &leaf.m8HierarchyRoot.First {
		t.Fatalf("parent provider = %v", got)
	}
	if leaf.m8HierarchyRoot.First.Node.Scope().level != 0 || leaf.m8HierarchyMiddle.Second.Node.Scope().level != 1 {
		t.Fatal("component scopes have wrong owning levels")
	}
	other := &m8HierarchyLeaf{}
	otherRefs := m8HierarchyRefs(plan, other)
	otherRefs[3] = plan.ComponentProvider[m8HierarchyMiddle]("Unknown", &other.m8HierarchyMiddle.Second)
	err = plan.Init(other, otherRefs...)
	if !errors.As(err, &diagnostic) || diagnostic.Code != BindingMismatch || diagnostic.Path != "m8HierarchyMiddle.Unknown" {
		t.Fatalf("undeclared component diagnostic = %+v", diagnostic)
	}
	rootForeign := m8HierarchyRefs(plan, other)
	rootForeign[1] = plan.ComponentProvider[m8HierarchyRoot]("First", foreign)
	err = plan.Init(other, rootForeign...)
	if !errors.As(err, &diagnostic) || diagnostic.Code != ProviderOwnership || diagnostic.Path != "m8HierarchyMiddle.m8HierarchyRoot.First" || diagnostic.Level != 0 {
		t.Fatalf("root component path = %+v", diagnostic)
	}
}

func TestM8PreviouslyInitializedLaterComponentPreservesAllScopes(t *testing.T) {
	leaf := &m8HierarchyLeaf{}
	standalone := Define[m8Reader]()
	standalone.Level[m8Reader, m8Read, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), ".")
	standalone.MustPrepare().MustInit(&leaf.m8HierarchyMiddle.Second,
		standalone.NodeProvider(&leaf.m8HierarchyMiddle.Second))
	prior := leaf.m8HierarchyMiddle.Second.Node.Scope().dispatch
	plan := m8HierarchyDefinition().MustPrepare()
	err := plan.Init(leaf, m8HierarchyRefs(plan, leaf)...)
	var diagnostic *Error
	if !errors.As(err, &diagnostic) || diagnostic.Code != AlreadyInitialized || diagnostic.Path != "m8HierarchyMiddle.Second" || diagnostic.Level != 1 {
		t.Fatalf("later holder diagnostic = %+v", diagnostic)
	}
	if leaf.m8HierarchyMiddle.Second.Node.Scope().dispatch != prior {
		t.Fatal("prior component scope was replaced")
	}
	for _, holder := range []*Node{&leaf.m8HierarchyRoot.Node, &leaf.m8HierarchyRoot.First.Node, &leaf.m8HierarchyMiddle.Node, &leaf.Node} {
		if holder.Scope().dispatch != nil {
			t.Fatal("failed Init changed another holder")
		}
	}
}

func TestM8CrossLevelProviderReuseIsRejected(t *testing.T) {
	plan := m8HierarchyDefinition().MustPrepare()
	leaf := &m8HierarchyLeaf{}
	refs := m8HierarchyRefs(plan, leaf)
	refs[3] = plan.ComponentProvider[m8HierarchyMiddle]("Second", &leaf.m8HierarchyRoot.First)
	err := plan.Init(leaf, refs...)
	if !errors.Is(err, &Error{Code: ProviderOwnership}) {
		t.Fatalf("cross-level provider reuse = %v", err)
	}
	for _, holder := range []*Node{&leaf.m8HierarchyRoot.Node, &leaf.m8HierarchyRoot.First.Node, &leaf.m8HierarchyMiddle.Node, &leaf.m8HierarchyMiddle.Second.Node, &leaf.Node} {
		if holder.Scope().dispatch != nil {
			t.Fatal("cross-level reuse changed a scope")
		}
	}
}

type m8SidewaysCaller struct{ Node }

func (c *m8SidewaysCaller) Count() int {
	return ParentMethod[m8Read](c.Node.Scope()).Read()
}

type m8SidewaysLeaf struct {
	Node
	Reader m8Reader
	Caller m8SidewaysCaller
}

type m8SidewaysPublic interface {
	m8Read
	m8Count
}

func TestM8ParentCallDoesNotSearchSiblingComponent(t *testing.T) {
	d := Define[m8SidewaysLeaf]()
	d.Level[m8SidewaysLeaf, m8SidewaysPublic, m8Empty]("").
		Bind[m8Read]((*m8Reader)(nil), "Reader").
		Bind[m8Count]((*m8SidewaysCaller)(nil), "Caller")
	dynamic := d.MustPrepare()
	for _, plan := range []*Plan[m8SidewaysLeaf]{dynamic, dynamic.MustPrecompute()} {
		leaf := &m8SidewaysLeaf{}
		plan.MustInit(leaf, plan.NodeProvider(leaf),
			plan.ComponentProvider[m8SidewaysLeaf]("Reader", &leaf.Reader),
			plan.ComponentProvider[m8SidewaysLeaf]("Caller", &leaf.Caller))
		func() {
			defer func() {
				var diagnostic *Error
				if !errors.As(asError(recover()), &diagnostic) || diagnostic.Code != NoParentImplementation || diagnostic.Level != 0 || diagnostic.Capability != reflect.TypeFor[m8Read]() {
					t.Fatalf("sideways parent result = %+v, precomputed=%t", diagnostic, plan.owners != nil)
				}
			}()
			Method[m8Count](leaf.Node.Scope()).Count()
		}()
	}
}

func asError(value any) error {
	err, _ := value.(error)
	return err
}

type m8MixedLeaf struct {
	Node
	Reader m8Reader
	value  int
}

func (m *m8MixedLeaf) Write(value int) { m.value = value }

func TestM8NodeAndComponentAtSameLevel(t *testing.T) {
	d := Define[m8MixedLeaf]()
	d.Level[m8MixedLeaf, m8Public, m8Empty]("").
		Bind[m8Read]((*m8Reader)(nil), "Reader").
		Bind[m8Write]((*m8MixedLeaf)(nil), ".")
	plan := d.MustPrepare()
	leaf := &m8MixedLeaf{}
	plan.MustInit(leaf, plan.ComponentProvider[m8MixedLeaf]("Reader", &leaf.Reader), plan.NodeProvider(leaf))
	Method[m8Write](leaf.Node.Scope()).Write(23)
	if leaf.value != 23 || Method[m8Read](leaf.Node.Scope()) != &leaf.Reader {
		t.Fatal("mixed node/component dispatch or state")
	}
}

type m8ReaderAlias = m8Reader
type m8AliasLeaf struct {
	Node
	m8ReaderAlias
}

func TestM8AnonymousAliasUsesDeclaredFieldName(t *testing.T) {
	if got := reflect.TypeFor[m8AliasLeaf]().Field(1).Name; got != "m8ReaderAlias" {
		t.Fatalf("declared alias field name = %q", got)
	}
	d := Define[m8AliasLeaf]()
	d.Level[m8AliasLeaf, m8Read, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "m8ReaderAlias")
	plan := d.MustPrepare()
	leaf := &m8AliasLeaf{}
	plan.MustInit(leaf, plan.NodeProvider(leaf), plan.ComponentProvider[m8AliasLeaf]("m8ReaderAlias", &leaf.m8ReaderAlias))
	if Method[m8Read](leaf.Node.Scope()) != &leaf.m8ReaderAlias {
		t.Fatal("alias-named field did not select its provider")
	}
	wrong := Define[m8AliasLeaf]()
	wrong.Level[m8AliasLeaf, m8Read, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "m8Reader")
	if err := wrong.Validate(); !errors.Is(err, &Error{Code: InvalidProvider}) {
		t.Fatalf("underlying type name was accepted as a field shortcut: %v", err)
	}
}

func TestM8ConcurrentLazyPreparationAndIndependentInstances(t *testing.T) {
	d := m8TwinDefinition()
	const count = 16
	plans := make([]*Plan[m8TwinLeaf], count)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for index := range plans {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			leaf := &m8TwinLeaf{}
			leaf.First.value, leaf.Nested.Second.value = index+1, index+10
			if err := d.Init(leaf,
				d.NodeProvider(leaf),
				d.ComponentProvider[m8TwinLeaf]("First", &leaf.First),
				d.ComponentProvider[m8TwinLeaf]("Nested.Second", &leaf.Nested.Second)); err != nil {
				t.Errorf("lazy component Init: %v", err)
				return
			}
			if got := Method[m8Read](leaf.Node.Scope()).Read(); got != index+1 {
				t.Errorf("Read = %d", got)
			}
			plan, err := d.Prepare()
			if err != nil {
				t.Errorf("warm Prepare: %v", err)
				return
			}
			plans[index] = plan
		}()
	}
	close(start)
	workers.Wait()
	for index, plan := range plans {
		if plan == nil || plan != plans[0] {
			t.Fatalf("plan %d = %p, first = %p", index, plan, plans[0])
		}
	}
	dynamicLeaf := &m8TwinLeaf{}
	dynamicLeaf.First.value = 12
	plans[0].MustInit(dynamicLeaf, plans[0].NodeProvider(dynamicLeaf),
		plans[0].ComponentProvider[m8TwinLeaf]("First", &dynamicLeaf.First),
		plans[0].ComponentProvider[m8TwinLeaf]("Nested.Second", &dynamicLeaf.Nested.Second))
	var conversions sync.WaitGroup
	for range count {
		conversions.Go(func() {
			fast := plans[0].MustPrecompute()
			if got := Method[m8Read](dynamicLeaf.Node.Scope()).Read(); got != 12 {
				t.Errorf("concurrent source Read = %d", got)
			}
			leaf := &m8TwinLeaf{}
			leaf.First.value = 5
			if err := fast.Init(leaf, fast.NodeProvider(leaf), fast.ComponentProvider[m8TwinLeaf]("First", &leaf.First), fast.ComponentProvider[m8TwinLeaf]("Nested.Second", &leaf.Nested.Second)); err != nil {
				t.Errorf("concurrent converted Init: %v", err)
			} else if got := Method[m8Read](leaf.Node.Scope()).Read(); got != 5 {
				t.Errorf("converted Read = %d", got)
			}
		})
	}
	for range 1000 {
		if got := Method[m8Read](dynamicLeaf.Node.Scope()).Read(); got != 12 {
			t.Fatalf("source Plan changed during conversion: Read=%d", got)
		}
	}
	conversions.Wait()
}

func TestM8SeparateStatementsMatchFluentConfiguration(t *testing.T) {
	fluent := m8TwinDefinition().MustPrepare()
	d := Define[m8TwinLeaf]()
	level := d.Level[m8TwinLeaf, m8TwinPublic, m8Empty]("")
	level.BindGroup((*m8Reader)(nil), "First", Of[m8Read](), Of[m8Count]())
	level.Bind[m8Audit]((*m8Reader)(nil), "Nested.Second")
	separate, err := d.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	individualDefinition := Define[m8TwinLeaf]()
	individualDefinition.Level[m8TwinLeaf, m8TwinPublic, m8Empty]("").
		Bind[m8Read]((*m8Reader)(nil), "First").
		Bind[m8Count]((*m8Reader)(nil), "First").
		Bind[m8Audit]((*m8Reader)(nil), "Nested.Second")
	individual := individualDefinition.MustPrepare()
	for _, plan := range []*Plan[m8TwinLeaf]{fluent, separate, individual} {
		leaf := &m8TwinLeaf{}
		leaf.First.value, leaf.Nested.Second.value = 7, 11
		plan.MustInit(leaf, plan.NodeProvider(leaf), plan.ComponentProvider[m8TwinLeaf]("First", &leaf.First), plan.ComponentProvider[m8TwinLeaf]("Nested.Second", &leaf.Nested.Second))
		if Method[m8Read](leaf.Node.Scope()).Read() != 7 || Method[m8Count](leaf.Node.Scope()).Count() != 8 || Method[m8Audit](leaf.Node.Scope()).Audit() != 13 {
			t.Fatal("fluent and separate component behavior differs")
		}
	}
}

type m8Step interface{ Step(int) int }
type m8Secret interface{ Secret() int }
type m8StepPublic interface{ m8Step }
type m8StepPrivate interface{ m8Secret }

type m8RootStepper struct{ Node }

func (s *m8RootStepper) Step(n int) int {
	if n == 0 {
		return 1
	}
	return 1 + Method[m8Step](s.Node.Scope()).Step(n-1)
}

type m8RootSecret struct{ Node }

func (*m8RootSecret) Secret() int { return 3 }

type m8LeafSecret struct{ Node }

func (s *m8LeafSecret) Secret() int {
	return 1 + ParentMethod[m8Secret](s.Node.Scope()).Secret()
}

type m8StepRoot struct {
	Node
	Stepper m8RootStepper
	Hidden  m8RootSecret
}
type m8StepLeaf struct {
	m8StepRoot
	Node
	Hidden m8LeafSecret
}

func (l *m8StepLeaf) Step(n int) int {
	if n == 0 {
		return Method[m8Secret](l.Node.Scope()).Secret()
	}
	return 1 + ParentMethod[m8Step](l.Node.Scope()).Step(n-1)
}

func m8StepDefinition() *Definition[m8StepLeaf] {
	d := Define[m8StepLeaf]()
	d.Level[m8StepRoot, m8StepPublic, m8StepPrivate]("").
		Bind[m8Step]((*m8RootStepper)(nil), "Stepper").
		Bind[m8Secret]((*m8RootSecret)(nil), "Hidden").End().
		Level[m8StepLeaf, m8StepPublic, m8StepPrivate]("m8StepRoot").
		Bind[m8Step]((*m8StepLeaf)(nil), ".").
		Bind[m8Secret]((*m8LeafSecret)(nil), "Hidden").End()
	return d
}

func TestM8ComponentRecursionPrivateAndParentScopes(t *testing.T) {
	dynamic := m8StepDefinition().MustPrepare()
	for _, plan := range []*Plan[m8StepLeaf]{dynamic, dynamic.MustPrecompute()} {
		leaf := &m8StepLeaf{}
		plan.MustInit(leaf,
			plan.NodeProvider(&leaf.m8StepRoot),
			plan.ComponentProvider[m8StepRoot]("Stepper", &leaf.m8StepRoot.Stepper),
			plan.ComponentProvider[m8StepRoot]("Hidden", &leaf.m8StepRoot.Hidden),
			plan.NodeProvider(leaf),
			plan.ComponentProvider[m8StepLeaf]("Hidden", &leaf.Hidden))
		if got := Method[m8Step](leaf.Node.Scope()).Step(4); got != 8 {
			t.Fatalf("recursive Step = %d, precomputed=%t", got, plan.owners != nil)
		}
		if got := Method[m8Secret](leaf.Node.Scope()); got != &leaf.Hidden || got.Secret() != 4 {
			t.Fatalf("private provider=%v, precomputed=%t", got, plan.owners != nil)
		}
		if got := ParentMethod[m8Secret](leaf.Hidden.Node.Scope()); got != &leaf.m8StepRoot.Hidden {
			t.Fatalf("private parent provider=%v", got)
		}
		if leaf.m8StepRoot.Stepper.Node.Scope().level != 0 || leaf.Hidden.Node.Scope().level != 1 {
			t.Fatal("component scope lost its owning level")
		}
	}
}

func TestM8InvalidRegistrationRetainsValidCoverageAndMissingMethods(t *testing.T) {
	d := Define[m8TwinLeaf]()
	d.Level[m8TwinLeaf, m8TwinPublic, m8Empty]("").
		Bind[m8Read]((*m8Reader)(nil), "First").
		Bind[m8Count]((*m8Reader)(nil), "Missing").
		Bind[m8Audit]((*m8Reader)(nil), "Nested.Second")
	err := d.Validate()
	var invalid, missing bool
	for _, diagnostic := range m8Diagnostics(err) {
		if diagnostic.Code == InvalidProvider && diagnostic.Path == "Missing" {
			invalid = true
		}
		if diagnostic.Code == MissingMethods && diagnostic.Name == "Count" {
			missing = true
		}
		if diagnostic.Code == MissingMethods && diagnostic.Name != "Count" {
			t.Fatalf("valid registration lost coverage: %+v", diagnostic)
		}
	}
	if !invalid || !missing {
		t.Fatalf("invalid path and missing Count diagnostics = %v", err)
	}
}

type m8MissingOperation interface{ Missing() int }
type m8SignaturePublic interface {
	m8Read
	m8MissingOperation
}

func TestM8GroupedComponentSignatureDiagnosticRetainsMissingMethod(t *testing.T) {
	d := Define[m8TwinLeaf]()
	d.Level[m8TwinLeaf, m8SignaturePublic, m8Empty]("").
		BindGroup((*m8Reader)(nil), "First", Of[m8Read](), Of[m8MissingOperation]())
	err := d.Validate()
	var signature, missing bool
	for _, diagnostic := range m8Diagnostics(err) {
		if diagnostic.Code == SignatureMismatch && diagnostic.Name == "Missing" {
			signature = diagnostic.Target == reflect.TypeFor[m8TwinLeaf]() && diagnostic.Level == 0 && diagnostic.Path == "First" && diagnostic.Expected == reflect.TypeFor[m8MissingOperation]() && diagnostic.Actual == reflect.TypeFor[*m8Reader]()
		}
		if diagnostic.Code == MissingMethods && diagnostic.Name == "Missing" {
			missing = true
		}
		if diagnostic.Code == MissingMethods && diagnostic.Name == "Read" {
			t.Fatal("valid grouped Read lost coverage")
		}
	}
	if !signature || !missing {
		t.Fatalf("signature and missing diagnostics = %v", err)
	}
}

func TestM8ComponentGroupTokensAreDetachedAndMalformedCallsAreAtomic(t *testing.T) {
	tokens := []OperationToken{Of[m8Read](), Of[m8Count]()}
	d := Define[m8TwinLeaf]()
	level := d.Level[m8TwinLeaf, m8TwinPublic, m8Empty]("")
	level.BindGroup((*m8Reader)(nil), "First", tokens...)
	tokens[0] = Of[m8Audit]()
	level.Bind[m8Audit]((*m8Reader)(nil), "Nested.Second")
	if err := d.Validate(); err != nil {
		t.Fatalf("mutated token slice changed component registrations: %v", err)
	}

	bad := Define[m8TwinLeaf]()
	bad.Level[m8TwinLeaf, m8TwinPublic, m8Empty]("").
		BindGroup((*m8Reader)(nil), "First", Of[m8Read](), OperationToken{})
	err := bad.Validate()
	if !errors.Is(err, &Error{Code: InvalidCapability}) {
		t.Fatalf("malformed group = %v", err)
	}
	want := map[string]bool{"Read": false, "Count": false, "Audit": false}
	for _, diagnostic := range m8Diagnostics(err) {
		if diagnostic.Code == MissingMethods {
			want[diagnostic.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("atomic malformed group counted %s as covered: %v", name, err)
		}
	}
}

func TestM8PreparedPlanIgnoresLaterDefinitionErrorForComponentReferences(t *testing.T) {
	d := m8TwinDefinition()
	plan := d.MustPrepare()
	d.Level[m8TwinLeaf, m8TwinPublic, m8Empty]("") // rejected frozen edit
	leaf := &m8TwinLeaf{}
	if err := plan.Init(leaf,
		d.NodeProvider(leaf),
		d.ComponentProvider[m8TwinLeaf]("First", &leaf.First),
		d.ComponentProvider[m8TwinLeaf]("Nested.Second", &leaf.Nested.Second)); err != nil {
		t.Fatalf("retained Plan rejected reference from its definition: %v", err)
	}
	if err := d.Init(&m8TwinLeaf{}); !errors.Is(err, &Error{Code: InvalidDeclaration}) {
		t.Fatalf("definition-based Init hid frozen-edit error: %v", err)
	}
}

type m8Fail interface{ Fail() (int, error) }
type m8FailProvider struct{ Node }

var m8ApplicationError = errors.New("component failure")

func (*m8FailProvider) Fail() (int, error) { return 7, m8ApplicationError }

type m8FailLeaf struct {
	Node
	Part m8FailProvider
}

func TestM8ComponentApplicationErrorsRemainOrdinary(t *testing.T) {
	d := Define[m8FailLeaf]()
	d.Level[m8FailLeaf, m8Fail, m8Empty]("").Bind[m8Fail]((*m8FailProvider)(nil), "Part")
	dynamic := d.MustPrepare()
	for _, plan := range []*Plan[m8FailLeaf]{dynamic, dynamic.MustPrecompute()} {
		leaf := &m8FailLeaf{}
		plan.MustInit(leaf, plan.NodeProvider(leaf), plan.ComponentProvider[m8FailLeaf]("Part", &leaf.Part))
		value, err := Method[m8Fail](leaf.Node.Scope()).Fail()
		if value != 7 || err != m8ApplicationError {
			t.Fatalf("component application result = (%d, %v)", value, err)
		}
	}
}

type m8RootPublic interface{ m8Read }
type m8ChildPublic interface {
	m8RootPublic
	m8Write
}
type m8ChildPrivate interface{ m8Secret }
type m8ExtendedRoot struct {
	Node
	Reader m8Reader
}
type m8ExtendedChild struct {
	m8ExtendedRoot
	Node
	Writer m8Writer
	Hidden m8RootSecret
}

func TestM8ComponentsWithIndependentContractExtensions(t *testing.T) {
	d := Define[m8ExtendedChild]()
	d.Level[m8ExtendedRoot, m8RootPublic, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "Reader").End().
		Level[m8ExtendedChild, m8ChildPublic, m8ChildPrivate]("m8ExtendedRoot").
		Bind[m8Write]((*m8Writer)(nil), "Writer").
		Bind[m8Secret]((*m8RootSecret)(nil), "Hidden")
	dynamic := d.MustPrepare()
	for _, plan := range []*Plan[m8ExtendedChild]{dynamic, dynamic.MustPrecompute()} {
		leaf := &m8ExtendedChild{}
		leaf.m8ExtendedRoot.Reader.value = 5
		plan.MustInit(leaf,
			plan.NodeProvider(&leaf.m8ExtendedRoot),
			plan.ComponentProvider[m8ExtendedRoot]("Reader", &leaf.m8ExtendedRoot.Reader),
			plan.NodeProvider(leaf),
			plan.ComponentProvider[m8ExtendedChild]("Writer", &leaf.Writer),
			plan.ComponentProvider[m8ExtendedChild]("Hidden", &leaf.Hidden))
		Method[m8Write](leaf.Node.Scope()).Write(9)
		if Method[m8Read](leaf.Node.Scope()).Read() != 5 || leaf.Writer.value != 9 || Method[m8Secret](leaf.Node.Scope()).Secret() != 3 {
			t.Fatal("independently extended public/private component contracts failed")
		}
		if ParentMethod[m8Read](leaf.Writer.Node.Scope()) != &leaf.m8ExtendedRoot.Reader {
			t.Fatal("component parent call to another operation selected wrong provider")
		}
	}
}
