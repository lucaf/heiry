package heiry

import (
	"fmt"
	"testing"
)

// Fixtures for comparing dynamic and precomputed resolution across hierarchy
// shapes through the public declaration, preparation, and binding path.
//
// The root declares every operation body and each later level promotes them,
// so any level can be registered for any operation and every mode runs the
// same body. Registering a promoting provider is the S4 trust boundary; these
// bodies make no parent calls.

type pbEmpty interface{}

type pbOp0 interface{ Op0() int }
type pbOp1 interface{ Op1() int }
type pbOp2 interface{ Op2() int }
type pbOp3 interface{ Op3() int }
type pbOp4 interface{ Op4() int }
type pbOp5 interface{ Op5() int }
type pbOp6 interface{ Op6() int }
type pbOp7 interface{ Op7() int }
type pbOp8 interface{ Op8() int }
type pbOp9 interface{ Op9() int }
type pbOp10 interface{ Op10() int }
type pbOp11 interface{ Op11() int }
type pbOp12 interface{ Op12() int }
type pbOp13 interface{ Op13() int }
type pbOp14 interface{ Op14() int }
type pbOp15 interface{ Op15() int }

type pbPublic4 interface {
	pbOp0
	pbOp1
	pbOp2
	pbOp3
}

type pbPublic16 interface {
	pbPublic4
	pbOp4
	pbOp5
	pbOp6
	pbOp7
	pbOp8
	pbOp9
	pbOp10
	pbOp11
	pbOp12
	pbOp13
	pbOp14
	pbOp15
}

type pbPart struct {
	Node
	value int
}

func (p *pbPart) Op0() int { return p.value }

type pbL0 struct {
	Node
	Part  pbPart
	value int
}

func (l *pbL0) Op0() int  { return l.value }
func (l *pbL0) Op1() int  { return l.value }
func (l *pbL0) Op2() int  { return l.value }
func (l *pbL0) Op3() int  { return l.value }
func (l *pbL0) Op4() int  { return l.value }
func (l *pbL0) Op5() int  { return l.value }
func (l *pbL0) Op6() int  { return l.value }
func (l *pbL0) Op7() int  { return l.value }
func (l *pbL0) Op8() int  { return l.value }
func (l *pbL0) Op9() int  { return l.value }
func (l *pbL0) Op10() int { return l.value }
func (l *pbL0) Op11() int { return l.value }
func (l *pbL0) Op12() int { return l.value }
func (l *pbL0) Op13() int { return l.value }
func (l *pbL0) Op14() int { return l.value }
func (l *pbL0) Op15() int { return l.value }

type pbL1 struct {
	pbL0
	Node
	Part pbPart
}
type pbL2 struct {
	pbL1
	Node
	Part pbPart
}
type pbL3 struct {
	pbL2
	Node
	Part pbPart
}
type pbL4 struct {
	pbL3
	Node
	Part pbPart
}
type pbL5 struct {
	pbL4
	Node
	Part pbPart
}
type pbL6 struct {
	pbL5
	Node
	Part pbPart
}
type pbL7 struct {
	pbL6
	Node
	Part pbPart
}
type pbL8 struct {
	pbL7
	Node
	Part pbPart
}
type pbL9 struct {
	pbL8
	Node
	Part pbPart
}

var (
	pbSink     pbOp0
	pbPlanSink any
	pbLeafSink any
)

func pbTokens() []OperationToken {
	return []OperationToken{
		Of[pbOp0](), Of[pbOp1](), Of[pbOp2](), Of[pbOp3](), Of[pbOp4](), Of[pbOp5](), Of[pbOp6](), Of[pbOp7](),
		Of[pbOp8](), Of[pbOp9](), Of[pbOp10](), Of[pbOp11](), Of[pbOp12](), Of[pbOp13](), Of[pbOp14](), Of[pbOp15](),
	}
}

type pbLevelSpec[Leaf any] struct {
	extend func(*Definition[Leaf]) *LevelHandle[Leaf]
	bind   func(*LevelHandle[Leaf], []OperationToken)
}

func pbLevelSpecs[Leaf any]() []pbLevelSpec[Leaf] {
	type def = *Definition[Leaf]
	type handle = *LevelHandle[Leaf]
	return []pbLevelSpec[Leaf]{
		{nil, func(h handle, ops []OperationToken) { h.BindNodeGroup((*pbL0)(nil), ops...) }},
		{func(d def) handle { return d.Extend[pbL1]() }, func(h handle, ops []OperationToken) { h.BindNodeGroup((*pbL1)(nil), ops...) }},
		{func(d def) handle { return d.Extend[pbL2]() }, func(h handle, ops []OperationToken) { h.BindNodeGroup((*pbL2)(nil), ops...) }},
		{func(d def) handle { return d.Extend[pbL3]() }, func(h handle, ops []OperationToken) { h.BindNodeGroup((*pbL3)(nil), ops...) }},
		{func(d def) handle { return d.Extend[pbL4]() }, func(h handle, ops []OperationToken) { h.BindNodeGroup((*pbL4)(nil), ops...) }},
		{func(d def) handle { return d.Extend[pbL5]() }, func(h handle, ops []OperationToken) { h.BindNodeGroup((*pbL5)(nil), ops...) }},
		{func(d def) handle { return d.Extend[pbL6]() }, func(h handle, ops []OperationToken) { h.BindNodeGroup((*pbL6)(nil), ops...) }},
		{func(d def) handle { return d.Extend[pbL7]() }, func(h handle, ops []OperationToken) { h.BindNodeGroup((*pbL7)(nil), ops...) }},
		{func(d def) handle { return d.Extend[pbL8]() }, func(h handle, ops []OperationToken) { h.BindNodeGroup((*pbL8)(nil), ops...) }},
		{func(d def) handle { return d.Extend[pbL9]() }, func(h handle, ops []OperationToken) { h.BindNodeGroup((*pbL9)(nil), ops...) }},
	}
}

func pbRootLevel[Leaf any](d *Definition[Leaf], ops int) *LevelHandle[Leaf] {
	switch ops {
	case 1:
		return d.Level[pbL0, pbOp0, pbEmpty]("")
	case 4:
		return d.Level[pbL0, pbPublic4, pbEmpty]("")
	default:
		return d.Level[pbL0, pbPublic16, pbEmpty]("")
	}
}

// pbRegisters reports whether a level's node registers an operation. The root
// registers everything. Operation 0 is the measured one:
//
//	root   only the root registers anything; skipped levels are empty
//	busy   skipped levels register every other operation
//	mid    busy, plus operation 0 again halfway up
//	mixed  busy, plus operation 0 on a component of level 1
//	dense  every level registers every operation
func pbRegisters(layout string, depth, level, op int) bool {
	if level == 0 {
		return true
	}
	if op != 0 {
		return layout != "root"
	}
	switch layout {
	case "dense":
		return true
	case "mid":
		return level == depth/2
	}
	return false
}

func pbDefinition[Leaf any](depth, ops int, layout string) *Definition[Leaf] {
	d := Define[Leaf]()
	tokens := pbTokens()
	for level, spec := range pbLevelSpecs[Leaf]()[:depth] {
		var h *LevelHandle[Leaf]
		if level == 0 {
			h = pbRootLevel(d, ops)
		} else {
			h = spec.extend(d)
		}
		var selected []OperationToken
		for op := 0; op < ops; op++ {
			if pbRegisters(layout, depth, level, op) {
				selected = append(selected, tokens[op])
			}
		}
		if len(selected) != 0 {
			spec.bind(h, selected)
		}
		if layout == "mixed" && level == 1 {
			h.Bind[pbOp0]((*pbPart)(nil), "Part")
		}
	}
	return d
}

// pbChain describes one concrete leaf type of the fixture chain.
type pbChain[Leaf any] struct {
	depth int
	refs  func(p *Plan[Leaf], leaf *Leaf, part bool) []ProviderRef[Leaf]
	nodes func(leaf *Leaf) []any
	part  func(leaf *Leaf) any // the level-1 component, where the chain has one
	scope func(leaf *Leaf) Scope
}

var pbChain1 = pbChain[pbL0]{
	depth: 1,
	refs: func(p *Plan[pbL0], leaf *pbL0, _ bool) []ProviderRef[pbL0] {
		return []ProviderRef[pbL0]{p.NodeProvider(leaf)}
	},
	nodes: func(leaf *pbL0) []any { return []any{leaf} },
	scope: func(leaf *pbL0) Scope { return leaf.Node.Scope() },
}

var pbChain3 = pbChain[pbL2]{
	depth: 3,
	refs: func(p *Plan[pbL2], leaf *pbL2, part bool) []ProviderRef[pbL2] {
		refs := []ProviderRef[pbL2]{p.NodeProvider(&leaf.pbL0), p.NodeProvider(&leaf.pbL1), p.NodeProvider(leaf)}
		if part {
			refs = append(refs, p.ComponentProvider[pbL1]("Part", &leaf.pbL1.Part))
		}
		return refs
	},
	nodes: func(leaf *pbL2) []any { return []any{&leaf.pbL0, &leaf.pbL1, leaf} },
	part:  func(leaf *pbL2) any { return &leaf.pbL1.Part },
	scope: func(leaf *pbL2) Scope { return leaf.Node.Scope() },
}

var pbChain10 = pbChain[pbL9]{
	depth: 10,
	refs: func(p *Plan[pbL9], leaf *pbL9, _ bool) []ProviderRef[pbL9] {
		return []ProviderRef[pbL9]{
			p.NodeProvider(&leaf.pbL0), p.NodeProvider(&leaf.pbL1), p.NodeProvider(&leaf.pbL2), p.NodeProvider(&leaf.pbL3),
			p.NodeProvider(&leaf.pbL4), p.NodeProvider(&leaf.pbL5), p.NodeProvider(&leaf.pbL6), p.NodeProvider(&leaf.pbL7),
			p.NodeProvider(&leaf.pbL8), p.NodeProvider(leaf),
		}
	},
	nodes: func(leaf *pbL9) []any {
		return []any{&leaf.pbL0, &leaf.pbL1, &leaf.pbL2, &leaf.pbL3, &leaf.pbL4, &leaf.pbL5, &leaf.pbL6, &leaf.pbL7, &leaf.pbL8, leaf}
	},
	scope: func(leaf *pbL9) Scope { return leaf.Node.Scope() },
}

// pbExpected searches the declared layout backward. It is independent of both
// resolvers and guards each measurement against selecting another provider.
func pbExpected[Leaf any](chain pbChain[Leaf], leaf *Leaf, layout string, start int) any {
	for level := start; level >= 0; level-- {
		if layout == "mixed" && level == 1 {
			return chain.part(leaf)
		}
		if pbRegisters(layout, chain.depth, level, 0) {
			return chain.nodes(leaf)[level]
		}
	}
	return nil
}

type pbShape struct {
	ops    int
	layout string
	parent bool // also measure a parent lookup from the leaf
}

// pbLookup measures resolution alone: the selected provider is kept, not called.
func pbLookup[Leaf any](b *testing.B, chain pbChain[Leaf], shapes []pbShape) {
	for _, shape := range shapes {
		dynamic := pbDefinition[Leaf](chain.depth, shape.ops, shape.layout).MustPrepare()
		for _, mode := range []struct {
			name string
			plan *Plan[Leaf]
		}{{"dynamic", dynamic}, {"precomputed", dynamic.MustPrecompute()}} {
			leaf := new(Leaf)
			mode.plan.MustInit(leaf, chain.refs(mode.plan, leaf, shape.layout == "mixed")...)
			scope := chain.scope(leaf)
			calls := []string{"virtual"}
			if shape.parent {
				calls = append(calls, "parent")
			}
			for _, call := range calls {
				start := chain.depth - 1
				if call == "parent" {
					start--
				}
				want := pbExpected(chain, leaf, shape.layout, start)
				name := fmt.Sprintf("depth%d/ops%d/%s/%s/%s", chain.depth, shape.ops, shape.layout, call, mode.name)
				b.Run(name, func(b *testing.B) {
					var got pbOp0
					if call == "parent" {
						for i := 0; i < b.N; i++ {
							got = ParentMethod[pbOp0](scope)
						}
					} else {
						for i := 0; i < b.N; i++ {
							got = Method[pbOp0](scope)
						}
					}
					if want == nil || any(got) != want {
						b.Fatalf("selected provider %p, want %p", got, want)
					}
					pbSink = got
				})
			}
		}
	}
}

func BenchmarkPrecomputeLookup(b *testing.B) {
	pbLookup(b, pbChain1, []pbShape{{ops: 1, layout: "dense"}, {ops: 4, layout: "dense"}, {ops: 16, layout: "dense"}})
	pbLookup(b, pbChain3, []pbShape{
		{ops: 4, layout: "root", parent: true},
		{ops: 4, layout: "busy"},
		{ops: 4, layout: "dense", parent: true},
		{ops: 4, layout: "mixed"},
		{ops: 16, layout: "busy"},
	})
	pbLookup(b, pbChain10, []pbShape{
		{ops: 4, layout: "root", parent: true},
		{ops: 4, layout: "busy", parent: true},
		{ops: 4, layout: "mid"},
		{ops: 4, layout: "dense", parent: true},
	})
}

// pbLifecycle separates definition-level preparation, conversion, and
// per-object initialization. Declaration, allocation, and reference creation
// are excluded from each timed region.
func pbLifecycle[Leaf any](b *testing.B, chain pbChain[Leaf], shape pbShape) {
	prefix := fmt.Sprintf("depth%d/ops%d/%s/", chain.depth, shape.ops, shape.layout)
	b.Run(prefix+"preparation", func(b *testing.B) {
		b.ResetTimer()
		for done := 0; done < b.N; {
			count := min(256, b.N-done)
			b.StopTimer()
			definitions := make([]*Definition[Leaf], count)
			for index := range definitions {
				definitions[index] = pbDefinition[Leaf](chain.depth, shape.ops, shape.layout)
			}
			b.StartTimer()
			for _, d := range definitions {
				pbPlanSink = d.MustPrepare()
			}
			done += count
		}
	})
	b.Run(prefix+"conversion", func(b *testing.B) {
		plan := pbDefinition[Leaf](chain.depth, shape.ops, shape.layout).MustPrepare()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pbPlanSink = plan.MustPrecompute()
		}
	})
	for _, mode := range []string{"dynamic", "precomputed"} {
		b.Run(prefix+"Init/"+mode, func(b *testing.B) {
			plan := pbDefinition[Leaf](chain.depth, shape.ops, shape.layout).MustPrepare()
			if mode == "precomputed" {
				plan = plan.MustPrecompute()
			}
			b.ResetTimer()
			for done := 0; done < b.N; {
				count := min(256, b.N-done)
				b.StopTimer()
				leaves := make([]*Leaf, count)
				refs := make([][]ProviderRef[Leaf], count)
				for index := range leaves {
					leaves[index] = new(Leaf)
					refs[index] = chain.refs(plan, leaves[index], shape.layout == "mixed")
				}
				b.StartTimer()
				for index, leaf := range leaves {
					if err := plan.Init(leaf, refs[index]...); err != nil {
						b.Fatal(err)
					}
					pbLeafSink = leaf
				}
				done += count
			}
		})
	}
}

func BenchmarkPrecomputeLifecycle(b *testing.B) {
	pbLifecycle(b, pbChain3, pbShape{ops: 4, layout: "dense"})
	pbLifecycle(b, pbChain3, pbShape{ops: 16, layout: "dense"})
	pbLifecycle(b, pbChain10, pbShape{ops: 4, layout: "busy"})
}
