package heiry

import (
	"testing"
)

// These fixed types force generated registrations through declaration,
// component slot validation, reference factories, binding, and typed lookup.
type m8OracleRoot struct {
	Node
	Left, Right m8OracleProvider
}
type m8OracleMiddle struct {
	m8OracleRoot
	Node
	Left, Right m8OracleProvider
}
type m8OracleLeaf struct {
	m8OracleMiddle
	Node
	Left, Right m8OracleProvider
}
type m8OracleContract interface {
	stage3Read
	stage3Add
	stage3Walk
}

func m8RegisterOracleLevel(l *LevelHandle[m8OracleLeaf], level int, masks [3]uint8, choices [3][3]uint8) [2]bool {
	var groups [2][]OperationToken
	for operation, capability := range []OperationToken{Of[stage3Read](), Of[stage3Add](), Of[stage3Walk]()} {
		if masks[operation]&(1<<level) != 0 {
			groups[choices[operation][level]&1] = append(groups[choices[operation][level]&1], capability)
		}
	}
	var used [2]bool
	for slot, path := range [2]string{"Left", "Right"} {
		if len(groups[slot]) != 0 {
			l.BindGroup((*m8OracleProvider)(nil), path, groups[slot]...)
			used[slot] = true
		}
	}
	return used
}

func m8OracleDefinition(masks [3]uint8, choices [3][3]uint8) (*Definition[m8OracleLeaf], [3][2]bool) {
	d := Define[m8OracleLeaf]()
	var used [3][2]bool
	used[0] = m8RegisterOracleLevel(d.Level[m8OracleRoot, m8OracleContract, m8Empty](""), 0, masks, choices)
	used[1] = m8RegisterOracleLevel(d.Level[m8OracleMiddle, m8OracleContract, m8Empty]("m8OracleRoot"), 1, masks, choices)
	used[2] = m8RegisterOracleLevel(d.Level[m8OracleLeaf, m8OracleContract, m8Empty]("m8OracleMiddle"), 2, masks, choices)
	return d, used
}

func m8OracleObject() (*m8OracleLeaf, [3][2]*m8OracleProvider, [3]*Node) {
	x := &m8OracleLeaf{}
	providers := [3][2]*m8OracleProvider{{&x.m8OracleRoot.Left, &x.m8OracleRoot.Right}, {&x.m8OracleMiddle.Left, &x.m8OracleMiddle.Right}, {&x.Left, &x.Right}}
	nodes := [3]*Node{&x.m8OracleRoot.Node, &x.m8OracleMiddle.Node, &x.Node}
	for level := range providers {
		for slot := range providers[level] {
			providers[level][slot].level, providers[level][slot].slot = level, slot
		}
	}
	return x, providers, nodes
}

func m8OracleDefinitionRefs(d *Definition[m8OracleLeaf], x *m8OracleLeaf, used [3][2]bool) []ProviderRef[m8OracleLeaf] {
	refs := []ProviderRef[m8OracleLeaf]{d.NodeProvider(&x.m8OracleRoot), d.NodeProvider(&x.m8OracleMiddle), d.NodeProvider(x)}
	if used[0][0] {
		refs = append(refs, d.ComponentProvider[m8OracleRoot]("Left", &x.m8OracleRoot.Left))
	}
	if used[0][1] {
		refs = append(refs, d.ComponentProvider[m8OracleRoot]("Right", &x.m8OracleRoot.Right))
	}
	if used[1][0] {
		refs = append(refs, d.ComponentProvider[m8OracleMiddle]("Left", &x.m8OracleMiddle.Left))
	}
	if used[1][1] {
		refs = append(refs, d.ComponentProvider[m8OracleMiddle]("Right", &x.m8OracleMiddle.Right))
	}
	if used[2][0] {
		refs = append(refs, d.ComponentProvider[m8OracleLeaf]("Left", &x.Left))
	}
	if used[2][1] {
		refs = append(refs, d.ComponentProvider[m8OracleLeaf]("Right", &x.Right))
	}
	return refs
}

func m8OraclePlanRefs(p *Plan[m8OracleLeaf], x *m8OracleLeaf, used [3][2]bool) []ProviderRef[m8OracleLeaf] {
	refs := []ProviderRef[m8OracleLeaf]{p.NodeProvider(&x.m8OracleRoot), p.NodeProvider(&x.m8OracleMiddle), p.NodeProvider(x)}
	if used[0][0] {
		refs = append(refs, p.ComponentProvider[m8OracleRoot]("Left", &x.m8OracleRoot.Left))
	}
	if used[0][1] {
		refs = append(refs, p.ComponentProvider[m8OracleRoot]("Right", &x.m8OracleRoot.Right))
	}
	if used[1][0] {
		refs = append(refs, p.ComponentProvider[m8OracleMiddle]("Left", &x.m8OracleMiddle.Left))
	}
	if used[1][1] {
		refs = append(refs, p.ComponentProvider[m8OracleMiddle]("Right", &x.m8OracleMiddle.Right))
	}
	if used[2][0] {
		refs = append(refs, p.ComponentProvider[m8OracleLeaf]("Left", &x.Left))
	}
	if used[2][1] {
		refs = append(refs, p.ComponentProvider[m8OracleLeaf]("Right", &x.Right))
	}
	return refs
}

func m8CheckIntegratedComponentOracle(t *testing.T, masks [3]uint8, choices [3][3]uint8) {
	t.Helper()
	for operation := range masks {
		masks[operation] &= 7
		if masks[operation] == 0 {
			masks[operation] = 1
		}
	}
	d, used := m8OracleDefinition(masks, choices)
	dynamicObject, dynamicProviders, dynamicNodes := m8OracleObject()
	// References created before preparation must resolve against the eventual Plan.
	dynamicRefs := m8OracleDefinitionRefs(d, dynamicObject, used)
	dynamic, err := d.Prepare()
	if err != nil {
		t.Fatalf("generated declaration masks=%v choices=%v: %v", masks, choices, err)
	}
	fast := dynamic.MustPrecompute()
	fastObject, fastProviders, fastNodes := m8OracleObject()
	for _, mode := range []struct {
		name      string
		plan      *Plan[m8OracleLeaf]
		object    *m8OracleLeaf
		providers [3][2]*m8OracleProvider
		nodes     [3]*Node
		refs      []ProviderRef[m8OracleLeaf]
	}{
		{"dynamic", dynamic, dynamicObject, dynamicProviders, dynamicNodes, dynamicRefs},
		{"precomputed", fast, fastObject, fastProviders, fastNodes, m8OraclePlanRefs(fast, fastObject, used)},
	} {
		if err := mode.plan.Init(mode.object, mode.refs...); err != nil {
			t.Fatalf("%s Init: %v", mode.name, err)
		}
		for level := 0; level < 3; level++ {
			if mode.nodes[level].Scope().level != level {
				t.Fatalf("%s level %d node scope", mode.name, level)
			}
			for slot := 0; slot < 2; slot++ {
				if used[level][slot] && (mode.providers[level][slot].Node.Scope().level != level || mode.providers[level][slot].Node.Scope().dispatch != mode.nodes[level].Scope().dispatch) {
					t.Fatalf("%s level %d slot %d scope", mode.name, level, slot)
				}
			}
		}
		for operation, mask := range masks {
			for start := 0; start < 3; start++ {
				scopes := []Scope{mode.nodes[start].Scope()}
				holders := []*Node{mode.nodes[start]}
				for slot := 0; slot < 2; slot++ {
					if used[start][slot] {
						scopes = append(scopes, mode.providers[start][slot].Node.Scope())
						holders = append(holders, &mode.providers[start][slot].Node)
					}
				}
				for index, scope := range scopes {
					for _, parent := range []bool{false, true} {
						position := 3
						if parent {
							position = start
						}
						wantLevel := m7BackwardOracle(uint16(mask), 3, position)
						provider, diagnostic := m7InvokeOracle(scope, operation, parent)
						// The holder methods must agree with the functions for the same holder.
						holderProvider, holderDiagnostic := e1InvokeHolder(holders[index], operation, parent)
						if holderProvider != provider || !e1SameDiagnostic(holderDiagnostic, diagnostic) {
							t.Fatalf("%s masks=%v operation=%d start=%d parent=%t: holder gave %v %+v, function gave %v %+v", mode.name, masks, operation, start, parent, holderProvider, holderDiagnostic, provider, diagnostic)
						}
						if wantLevel < 0 {
							if provider != nil || diagnostic == nil || diagnostic.Code != NoParentImplementation || diagnostic.Capability != m7OracleCapabilities[operation] || diagnostic.Level != start {
								t.Fatalf("%s masks=%v operation=%d start=%d parent=%t: provider=%v diagnostic=%+v", mode.name, masks, operation, start, parent, provider, diagnostic)
							}
							continue
						}
						want := mode.providers[wantLevel][choices[operation][wantLevel]&1]
						if provider != want || diagnostic != nil {
							t.Fatalf("%s masks=%v operation=%d start=%d parent=%t: provider=%v, want %p, diagnostic=%+v", mode.name, masks, operation, start, parent, provider, want, diagnostic)
						}
					}
				}
			}
		}
	}
}

// Each base-three digit selects absent, Left, or Right at one level. Zero is
// the excluded all-absent assignment; absent levels keep a canonical choice.
func m8DecodeOracleAssignment(assignment int) (uint8, [3]uint8) {
	var mask uint8
	var choices [3]uint8
	for level := range choices {
		state := assignment % 3
		assignment /= 3
		if state != 0 {
			mask |= 1 << level
			choices[level] = uint8(state - 1)
		}
	}
	return mask, choices
}

func TestM8IntegratedComponentOracle(t *testing.T) {
	const perOperation = 3*3*3 - 1
	const expected = perOperation * perOperation * perOperation
	type layout struct {
		masks   [3]uint8
		choices [3][3]uint8
	}
	seen := make(map[layout]struct{}, expected)
	count := 0
	for first := 1; first <= perOperation; first++ {
		for second := 1; second <= perOperation; second++ {
			for third := 1; third <= perOperation; third++ {
				var current layout
				for operation, assignment := range [3]int{first, second, third} {
					current.masks[operation], current.choices[operation] = m8DecodeOracleAssignment(assignment)
				}
				if _, duplicate := seen[current]; duplicate {
					t.Fatalf("duplicate component layout: %+v", current)
				}
				seen[current] = struct{}{}
				m8CheckIntegratedComponentOracle(t, current.masks, current.choices)
				count++
			}
		}
	}
	if count != expected || len(seen) != expected {
		t.Fatalf("visited %d layouts (%d distinct), want %d", count, len(seen), expected)
	}
}
