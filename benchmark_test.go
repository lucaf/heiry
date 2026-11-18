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
