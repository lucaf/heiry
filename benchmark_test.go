package heiry

import (
	"fmt"
	"reflect"
	"testing"
)

type m7BenchRead interface{ Read() int }
type m7BenchExtra1 interface{ Extra1() int }
type m7BenchExtra2 interface{ Extra2() int }
type m7BenchExtra3 interface{ Extra3() int }
type m7BenchExtra4 interface{ Extra4() int }

type m7BenchProvider struct{ value int }
type m7BenchOtherProvider struct{ value int }

// These native bodies perform the same public-to-private and parent calls as
// the heiry fixture, using ordinary Go interface values for each dispatch.
type m7NativeRoot struct{ complete stage4Private }

func (r *m7NativeRoot) Read() string { return "A/" + r.complete.Secret() }
func (*m7NativeRoot) Secret() string { return "A.secret" }

type m7NativeLeaf struct{ m7NativeRoot }

func (l *m7NativeLeaf) Read() string {
	var parent stage4Public = &l.m7NativeRoot
	return "B/" + parent.Read()
}
func (l *m7NativeLeaf) Secret() string {
	var parent stage4Private = &l.m7NativeRoot
	return "B/" + parent.Secret()
}

type m7NativeSparseRoot struct{}

func (*m7NativeSparseRoot) RootOnly() string { return "A.root" }
func (*m7NativeSparseRoot) Other() string    { return "A.other" }

type m7NativeSparseLeaf struct{ root m7NativeSparseRoot }

func (l *m7NativeSparseLeaf) Other() string {
	var parentOther m7Other = &l.root
	var parentRoot m7RootOnly = &l.root
	return "C/" + parentOther.Other() + "/" + parentRoot.RootOnly()
}

func (p *m7BenchProvider) Read() int      { return p.value }
func (p *m7BenchProvider) Extra1() int    { return p.value }
func (p *m7BenchProvider) Extra2() int    { return p.value }
func (p *m7BenchProvider) Extra3() int    { return p.value }
func (p *m7BenchProvider) Extra4() int    { return p.value }
func (p *m7BenchOtherProvider) Read() int { return p.value }

var m7BenchmarkResult int
var m7BenchmarkPlan *Plan[stage3C]
var m7BenchmarkLeaf *stage3C
var m7BenchmarkDefinition *Definition[stage3C]
var m8BenchmarkLeaf *m8TwinLeaf

func m7BenchmarkDispatch(depth, methods int, layout string) (m7BenchRead, Scope, Scope) {
	capabilities := []reflect.Type{
		reflect.TypeFor[m7BenchRead](), reflect.TypeFor[m7BenchExtra1](),
		reflect.TypeFor[m7BenchExtra2](), reflect.TypeFor[m7BenchExtra3](), reflect.TypeFor[m7BenchExtra4](),
	}
	known := make(map[reflect.Type]struct{}, methods)
	for _, capability := range capabilities[:methods] {
		known[capability] = struct{}{}
	}
	levels := make([]map[reflect.Type]any, depth)
	metadata := make([]nodeLevel, depth)
	var direct m7BenchRead
	for level := range levels {
		levels[level] = make(map[reflect.Type]any)
		provider := &m7BenchProvider{value: level + 1}
		registerRead := level == 0 || layout == "dense" || layout == "sparse" && level%3 == 0
		for index, capability := range capabilities[:methods] {
			if index == 0 && !registerRead || index > 0 && level != 0 {
				continue
			}
			levels[level][capability] = provider
			metadata[level].contracts.bindings = append(metadata[level].contracts.bindings, contractBinding{capability: capability})
			if index == 0 {
				direct = provider
			}
		}
	}
	plan := (&Plan[stage3C]{validated: &validatedNodes{levels: metadata, known: known}, lineage: &definitionLineage{marker: 1}}).MustPrecompute()
	dynamic := &dispatch{known: known, levels: levels}
	fast := &dispatch{known: known, levels: levels, owners: plan.owners}
	return direct, Scope{dispatch: dynamic, level: depth - 1}, Scope{dispatch: fast, level: depth - 1}
}

func BenchmarkResolution(b *testing.B) {
	for _, depth := range []int{1, 3, 10} {
		for _, layout := range []string{"root", "sparse", "dense"} {
			for _, methods := range []int{1, 5} {
				name := fmt.Sprintf("depth%d/%s/methods%d", depth, layout, methods)
				direct, dynamic, fast := m7BenchmarkDispatch(depth, methods, layout)
				b.Run(name+"/interface", func(b *testing.B) {
					var sum int
					for i := 0; i < b.N; i++ {
						sum += direct.Read()
					}
					m7BenchmarkResult = sum
				})
				b.Run(name+"/dynamic", func(b *testing.B) {
					var sum int
					for i := 0; i < b.N; i++ {
						sum += Method[m7BenchRead](dynamic).Read()
					}
					m7BenchmarkResult = sum
				})
				b.Run(name+"/precomputed", func(b *testing.B) {
					var sum int
					for i := 0; i < b.N; i++ {
						sum += Method[m7BenchRead](fast).Read()
					}
					m7BenchmarkResult = sum
				})
			}
		}
	}
}

func BenchmarkMixedReceivers(b *testing.B) {
	var direct [4]m7BenchRead
	var dynamic, fast [4]Scope
	for i := range direct {
		direct[i], dynamic[i], fast[i] = m7BenchmarkDispatch(10, 1, "sparse")
		if i&1 != 0 {
			other := &m7BenchOtherProvider{value: i + 10}
			capability := reflect.TypeFor[m7BenchRead]()
			dynamic[i].dispatch.levels[9][capability] = other
			direct[i] = other
		}
	}
	for _, mode := range []string{"interface", "dynamic", "precomputed"} {
		b.Run(mode, func(b *testing.B) {
			var sum int
			for i := 0; i < b.N; i++ {
				slot := i & 3
				switch mode {
				case "interface":
					sum += direct[slot].Read()
				case "dynamic":
					sum += Method[m7BenchRead](dynamic[slot]).Read()
				default:
					sum += Method[m7BenchRead](fast[slot]).Read()
				}
			}
			m7BenchmarkResult = sum
		})
	}
}

func BenchmarkPublicPrivateDelegation(b *testing.B) {
	native := &m7NativeLeaf{}
	native.m7NativeRoot.complete = native
	var nativePublic stage4Public = native
	b.Run("interface", func(b *testing.B) {
		b.ResetTimer()
		var value string
		for i := 0; i < b.N; i++ {
			value = nativePublic.Read()
		}
		if value != "B/A/B/A.secret" {
			b.Fatalf("native result = %q", value)
		}
	})
	for _, mode := range []string{"dynamic", "precomputed"} {
		plan := m5PrivateDefinition().MustPrepare()
		if mode == "precomputed" {
			plan = plan.MustPrecompute()
		}
		leaf := &stage4B{}
		plan.MustInit(leaf, plan.NodeProvider(&leaf.stage4A), plan.NodeProvider(leaf))
		view := &stage4Object{leaf: leaf}
		b.Run(mode, func(b *testing.B) {
			b.ResetTimer()
			var value string
			for i := 0; i < b.N; i++ {
				value = view.Read()
			}
			if value != "B/A/B/A.secret" {
				b.Fatalf("dispatch result = %q", value)
			}
		})
	}
}

func BenchmarkParentOtherOperation(b *testing.B) {
	var nativeOther m7Other = &m7NativeSparseLeaf{}
	b.Run("interface", func(b *testing.B) {
		b.ResetTimer()
		var value string
		for i := 0; i < b.N; i++ {
			value = nativeOther.Other()
		}
		if value != "C/A.other/A.root" {
			b.Fatalf("native result = %q", value)
		}
	})
	for _, mode := range []string{"dynamic", "precomputed"} {
		plan := m7SparseDefinition().MustPrepare()
		if mode == "precomputed" {
			plan = plan.MustPrecompute()
		}
		leaf := &m7SparseC{}
		plan.MustInit(leaf, plan.NodeProvider(&leaf.m7SparseA), plan.NodeProvider(&leaf.m7SparseB), plan.NodeProvider(leaf))
		b.Run(mode, func(b *testing.B) {
			b.ResetTimer()
			var value string
			for i := 0; i < b.N; i++ {
				value = Method[m7Other](leaf.Node.Scope()).Other()
			}
			if value != "C/A.other/A.root" {
				b.Fatalf("dispatch result = %q", value)
			}
		})
	}
}

func BenchmarkLifecycle(b *testing.B) {
	b.Run("builder", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m7BenchmarkDefinition = m5Stage3Definition()
		}
	})
	b.Run("validation", func(b *testing.B) {
		d := m5Stage3Definition()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := d.Validate(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("preparation", func(b *testing.B) {
		b.ResetTimer()
		for done := 0; done < b.N; {
			count := min(256, b.N-done)
			b.StopTimer()
			definitions := make([]*Definition[stage3C], count)
			for index := range definitions {
				definitions[index] = m5Stage3Definition()
			}
			b.StartTimer()
			for _, d := range definitions {
				m7BenchmarkPlan = d.MustPrepare()
			}
			done += count
		}
	})
	b.Run("lazy first use", func(b *testing.B) {
		b.ResetTimer()
		for done := 0; done < b.N; {
			count := min(256, b.N-done)
			b.StopTimer()
			definitions := make([]*Definition[stage3C], count)
			leaves := make([]*stage3C, count)
			refs := make([][3]ProviderRef[stage3C], count)
			for index := range definitions {
				d := m5Stage3Definition()
				leaf := &stage3C{}
				definitions[index], leaves[index] = d, leaf
				refs[index] = [3]ProviderRef[stage3C]{d.NodeProvider(&leaf.stage3A), d.NodeProvider(&leaf.stage3B), d.NodeProvider(leaf)}
			}
			b.StartTimer()
			for index, d := range definitions {
				if err := d.Init(leaves[index], refs[index][:]...); err != nil {
					b.Fatal(err)
				}
				m7BenchmarkLeaf = leaves[index]
			}
			done += count
		}
	})
	b.Run("warm preparation", func(b *testing.B) {
		d := m5Stage3Definition()
		d.MustPrepare()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m7BenchmarkPlan = d.MustPrepare()
		}
	})
	b.Run("conversion", func(b *testing.B) {
		plan := m5Stage3Definition().MustPrepare()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m7BenchmarkPlan = plan.MustPrecompute()
		}
	})
	for _, mode := range []string{"dynamic", "precomputed"} {
		b.Run("per-object Init/"+mode, func(b *testing.B) {
			plan := m5Stage3Definition().MustPrepare()
			if mode == "precomputed" {
				plan = plan.MustPrecompute()
			}
			b.ResetTimer()
			for done := 0; done < b.N; {
				count := min(256, b.N-done)
				b.StopTimer()
				leaves := make([]*stage3C, count)
				refs := make([][3]ProviderRef[stage3C], count)
				for index := range leaves {
					leaf := &stage3C{}
					leaves[index] = leaf
					refs[index] = [3]ProviderRef[stage3C]{plan.NodeProvider(&leaf.stage3A), plan.NodeProvider(&leaf.stage3B), plan.NodeProvider(leaf)}
				}
				b.StartTimer()
				for index, leaf := range leaves {
					if err := plan.Init(leaf, refs[index][:]...); err != nil {
						b.Fatal(err)
					}
					m7BenchmarkLeaf = leaf
				}
				done += count
			}
		})
	}
}

func BenchmarkM8ComponentDispatch(b *testing.B) {
	definition := m8TwinDefinition()
	dynamic := definition.MustPrepare()
	fast := dynamic.MustPrecompute()
	for _, tc := range []struct {
		name string
		plan *Plan[m8TwinLeaf]
	}{{"dynamic", dynamic}, {"precomputed", fast}} {
		leaf := &m8TwinLeaf{}
		leaf.First.value, leaf.Nested.Second.value = 1, 2
		tc.plan.MustInit(leaf, tc.plan.NodeProvider(leaf),
			tc.plan.ComponentProvider[m8TwinLeaf]("First", &leaf.First),
			tc.plan.ComponentProvider[m8TwinLeaf]("Nested.Second", &leaf.Nested.Second))
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			var sum int
			for i := 0; i < b.N; i++ {
				sum += Method[m8Read](leaf.Node.Scope()).Read()
				sum += Method[m8Audit](leaf.Node.Scope()).Audit()
			}
			m7BenchmarkResult = sum
		})
	}
	var first m8Read = &m8Reader{value: 1}
	var second m8Audit = &m8Reader{value: 2}
	b.Run("interface", func(b *testing.B) {
		b.ResetTimer()
		var sum int
		for i := 0; i < b.N; i++ {
			sum += first.Read()
			sum += second.Audit()
		}
		m7BenchmarkResult = sum
	})
}

func BenchmarkM8ComponentLifecycle(b *testing.B) {
	b.Run("builder", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m8BenchmarkDefinition = m8TwinDefinition()
		}
	})
	b.Run("validation", func(b *testing.B) {
		d := m8TwinDefinition()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := d.Validate(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("preparation", func(b *testing.B) {
		b.ResetTimer()
		for done := 0; done < b.N; {
			count := min(256, b.N-done)
			b.StopTimer()
			definitions := make([]*Definition[m8TwinLeaf], count)
			for index := range definitions {
				definitions[index] = m8TwinDefinition()
			}
			b.StartTimer()
			for _, d := range definitions {
				m8BenchmarkPlan = d.MustPrepare()
			}
			done += count
		}
	})
	b.Run("conversion", func(b *testing.B) {
		plan := m8TwinDefinition().MustPrepare()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m8BenchmarkPlan = plan.MustPrecompute()
		}
	})
	b.Run("lazy first use", func(b *testing.B) {
		b.ResetTimer()
		for done := 0; done < b.N; {
			count := min(256, b.N-done)
			b.StopTimer()
			definitions := make([]*Definition[m8TwinLeaf], count)
			leaves := make([]*m8TwinLeaf, count)
			refs := make([][3]ProviderRef[m8TwinLeaf], count)
			for index := range definitions {
				d := m8TwinDefinition()
				leaf := &m8TwinLeaf{}
				definitions[index], leaves[index] = d, leaf
				refs[index] = [3]ProviderRef[m8TwinLeaf]{d.NodeProvider(leaf), d.ComponentProvider[m8TwinLeaf]("First", &leaf.First), d.ComponentProvider[m8TwinLeaf]("Nested.Second", &leaf.Nested.Second)}
			}
			b.StartTimer()
			for index, d := range definitions {
				if err := d.Init(leaves[index], refs[index][:]...); err != nil {
					b.Fatal(err)
				}
				m8BenchmarkLeaf = leaves[index]
			}
			done += count
		}
	})
	b.Run("warm preparation", func(b *testing.B) {
		d := m8TwinDefinition()
		d.MustPrepare()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m8BenchmarkPlan = d.MustPrepare()
		}
	})
	for _, mode := range []string{"dynamic", "precomputed"} {
		b.Run("per-object Init/"+mode, func(b *testing.B) {
			plan := m8TwinDefinition().MustPrepare()
			if mode == "precomputed" {
				plan = plan.MustPrecompute()
			}
			b.ResetTimer()
			for done := 0; done < b.N; {
				count := min(256, b.N-done)
				b.StopTimer()
				leaves := make([]*m8TwinLeaf, count)
				refs := make([][3]ProviderRef[m8TwinLeaf], count)
				for index := range leaves {
					leaf := &m8TwinLeaf{}
					leaves[index] = leaf
					refs[index] = [3]ProviderRef[m8TwinLeaf]{
						plan.NodeProvider(leaf),
						plan.ComponentProvider[m8TwinLeaf]("First", &leaf.First),
						plan.ComponentProvider[m8TwinLeaf]("Nested.Second", &leaf.Nested.Second),
					}
				}
				b.StartTimer()
				for index, leaf := range leaves {
					if err := plan.Init(leaf, refs[index][:]...); err != nil {
						b.Fatal(err)
					}
					m8BenchmarkLeaf = leaf
				}
				done += count
			}
		})
	}
}

var m8BenchmarkPlan *Plan[m8TwinLeaf]
var m8BenchmarkDefinition *Definition[m8TwinLeaf]

type m8BenchScore interface{ Score() int }
type m8BenchPublic interface {
	m8Read
	m8Count
	m8Audit
	m8BenchScore
}
type m8BenchComponent struct {
	Node
	value int
}

func (p *m8BenchComponent) Read() int  { return p.value }
func (p *m8BenchComponent) Count() int { return p.value }
func (p *m8BenchComponent) Audit() int { return p.value }
func (p *m8BenchComponent) Score() int { return p.value }

type m8BenchLeaf struct {
	Node
	One, Two, Three, Four m8BenchComponent
}

func m8BenchDefinition(count int) *Definition[m8BenchLeaf] {
	d := Define[m8BenchLeaf]()
	l := d.Level[m8BenchLeaf, m8BenchPublic, m8Empty]("")
	switch count {
	case 1:
		l.BindGroup((*m8BenchComponent)(nil), "One", Of[m8Read](), Of[m8Count](), Of[m8Audit](), Of[m8BenchScore]())
	case 2:
		l.BindGroup((*m8BenchComponent)(nil), "One", Of[m8Read](), Of[m8Count]())
		l.BindGroup((*m8BenchComponent)(nil), "Two", Of[m8Audit](), Of[m8BenchScore]())
	case 4:
		l.Bind[m8Read]((*m8BenchComponent)(nil), "One")
		l.Bind[m8Count]((*m8BenchComponent)(nil), "Two")
		l.Bind[m8Audit]((*m8BenchComponent)(nil), "Three")
		l.Bind[m8BenchScore]((*m8BenchComponent)(nil), "Four")
	}
	return d
}

func m8BenchRefs(p *Plan[m8BenchLeaf], leaf *m8BenchLeaf, count int) []ProviderRef[m8BenchLeaf] {
	refs := []ProviderRef[m8BenchLeaf]{p.NodeProvider(leaf), p.ComponentProvider[m8BenchLeaf]("One", &leaf.One)}
	if count >= 2 {
		refs = append(refs, p.ComponentProvider[m8BenchLeaf]("Two", &leaf.Two))
	}
	if count == 4 {
		refs = append(refs, p.ComponentProvider[m8BenchLeaf]("Three", &leaf.Three), p.ComponentProvider[m8BenchLeaf]("Four", &leaf.Four))
	}
	return refs
}

func BenchmarkM8ComponentCount(b *testing.B) {
	for _, count := range []int{1, 2, 4} {
		name := fmt.Sprintf("components%d", count)
		dynamic := m8BenchDefinition(count).MustPrepare()
		fast := dynamic.MustPrecompute()
		for _, mode := range []struct {
			name string
			plan *Plan[m8BenchLeaf]
		}{{"dynamic", dynamic}, {"precomputed", fast}} {
			leaf := &m8BenchLeaf{}
			leaf.One.value, leaf.Two.value, leaf.Three.value, leaf.Four.value = 1, 2, 3, 4
			mode.plan.MustInit(leaf, m8BenchRefs(mode.plan, leaf, count)...)
			b.Run(name+"/"+mode.name, func(b *testing.B) {
				b.ResetTimer()
				var sum int
				for i := 0; i < b.N; i++ {
					sum += Method[m8Read](leaf.Node.Scope()).Read()
					sum += Method[m8BenchScore](leaf.Node.Scope()).Score()
				}
				m7BenchmarkResult = sum
			})
		}
		leaf := &m8BenchLeaf{}
		leaf.One.value, leaf.Two.value, leaf.Three.value, leaf.Four.value = 1, 2, 3, 4
		var first m8Read = &leaf.One
		var last m8BenchScore = &leaf.One
		if count == 2 {
			last = &leaf.Two
		} else if count == 4 {
			last = &leaf.Four
		}
		b.Run(name+"/interface", func(b *testing.B) {
			b.ResetTimer()
			var sum int
			for i := 0; i < b.N; i++ {
				sum += first.Read()
				sum += last.Score()
			}
			m7BenchmarkResult = sum
		})
	}
}

type m8NativeRootStep struct{ complete m8Step }

func (s *m8NativeRootStep) Step(n int) int {
	if n == 0 {
		return 1
	}
	return 1 + s.complete.Step(n-1)
}

type m8NativeRootSecret struct{}

func (*m8NativeRootSecret) Secret() int { return 3 }

type m8NativeLeafSecret struct{ parent m8Secret }

func (s *m8NativeLeafSecret) Secret() int { return 1 + s.parent.Secret() }

type m8NativeLeafStep struct {
	parent  m8Step
	private m8Secret
}

func (s *m8NativeLeafStep) Step(n int) int {
	if n == 0 {
		return s.private.Secret()
	}
	return 1 + s.parent.Step(n-1)
}

func BenchmarkM8ComponentDelegation(b *testing.B) {
	rootStep := &m8NativeRootStep{}
	rootSecret := &m8NativeRootSecret{}
	leafSecret := &m8NativeLeafSecret{parent: rootSecret}
	native := &m8NativeLeafStep{parent: rootStep, private: leafSecret}
	rootStep.complete = native
	var public m8Step = native
	if got := public.Step(4); got != 8 {
		b.Fatalf("native component delegation = %d", got)
	}
	b.Run("interface", func(b *testing.B) {
		b.ResetTimer()
		var result int
		for i := 0; i < b.N; i++ {
			result = public.Step(4)
		}
		m7BenchmarkResult = result
	})
	dynamic := m8StepDefinition().MustPrepare()
	for _, mode := range []struct {
		name string
		plan *Plan[m8StepLeaf]
	}{{"dynamic", dynamic}, {"precomputed", dynamic.MustPrecompute()}} {
		leaf := &m8StepLeaf{}
		mode.plan.MustInit(leaf,
			mode.plan.NodeProvider(&leaf.m8StepRoot),
			mode.plan.ComponentProvider[m8StepRoot]("Stepper", &leaf.m8StepRoot.Stepper),
			mode.plan.ComponentProvider[m8StepRoot]("Hidden", &leaf.m8StepRoot.Hidden),
			mode.plan.NodeProvider(leaf),
			mode.plan.ComponentProvider[m8StepLeaf]("Hidden", &leaf.Hidden))
		if got := Method[m8Step](leaf.Node.Scope()).Step(4); got != 8 {
			b.Fatalf("%s component delegation = %d", mode.name, got)
		}
		b.Run(mode.name, func(b *testing.B) {
			b.ResetTimer()
			var result int
			for i := 0; i < b.N; i++ {
				result = Method[m8Step](leaf.Node.Scope()).Step(4)
			}
			m7BenchmarkResult = result
		})
	}
}

func BenchmarkM8ComponentParentOtherOperation(b *testing.B) {
	root := &m8Reader{value: 5}
	var parent m8Read = root
	if got := parent.Read(); got != 5 {
		b.Fatalf("native parent operation = %d", got)
	}
	b.Run("interface", func(b *testing.B) {
		b.ResetTimer()
		var result int
		for i := 0; i < b.N; i++ {
			result = parent.Read()
		}
		m7BenchmarkResult = result
	})
	d := Define[m8ExtendedChild]()
	d.Level[m8ExtendedRoot, m8RootPublic, m8Empty]("").Bind[m8Read]((*m8Reader)(nil), "Reader").End().
		Level[m8ExtendedChild, m8ChildPublic, m8ChildPrivate]("m8ExtendedRoot").
		Bind[m8Write]((*m8Writer)(nil), "Writer").
		Bind[m8Secret]((*m8RootSecret)(nil), "Hidden")
	dynamic := d.MustPrepare()
	for _, mode := range []struct {
		name string
		plan *Plan[m8ExtendedChild]
	}{{"dynamic", dynamic}, {"precomputed", dynamic.MustPrecompute()}} {
		leaf := &m8ExtendedChild{}
		leaf.m8ExtendedRoot.Reader.value = 5
		mode.plan.MustInit(leaf,
			mode.plan.NodeProvider(&leaf.m8ExtendedRoot),
			mode.plan.ComponentProvider[m8ExtendedRoot]("Reader", &leaf.m8ExtendedRoot.Reader),
			mode.plan.NodeProvider(leaf),
			mode.plan.ComponentProvider[m8ExtendedChild]("Writer", &leaf.Writer),
			mode.plan.ComponentProvider[m8ExtendedChild]("Hidden", &leaf.Hidden))
		if got := ParentMethod[m8Read](leaf.Writer.Node.Scope()).Read(); got != 5 {
			b.Fatalf("%s component parent operation = %d", mode.name, got)
		}
		b.Run(mode.name, func(b *testing.B) {
			b.ResetTimer()
			var result int
			for i := 0; i < b.N; i++ {
				result = ParentMethod[m8Read](leaf.Writer.Node.Scope()).Read()
			}
			m7BenchmarkResult = result
		})
	}
}
