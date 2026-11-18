package heiry

import (
	"errors"
	"reflect"
	"testing"
)

var m7OracleCapabilities = []reflect.Type{
	reflect.TypeFor[stage3Read](),
	reflect.TypeFor[stage3Add](),
	reflect.TypeFor[stage3Walk](),
}

type m7OracleProvider struct{ level int }

func (p *m7OracleProvider) Read() int       { return p.level }
func (p *m7OracleProvider) Add(n int) int   { return p.level + n }
func (p *m7OracleProvider) Walk(int) string { return "oracle" }

func m7InvokeOracle(scope Scope, operation int, parent bool) (provider any, diagnostic *Error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if err, ok := recovered.(error); ok && errors.As(err, &diagnostic) {
				return
			}
			panic(recovered)
		}
	}()
	switch operation {
	case 0:
		if parent {
			return ParentMethod[stage3Read](scope), nil
		}
		return Method[stage3Read](scope), nil
	case 1:
		if parent {
			return ParentMethod[stage3Add](scope), nil
		}
		return Method[stage3Add](scope), nil
	default:
		if parent {
			return ParentMethod[stage3Walk](scope), nil
		}
		return Method[stage3Walk](scope), nil
	}
}

func m7OraclePlan(depth int, masks []uint16) *Plan[stage3C] {
	levels := make([]nodeLevel, depth)
	known := make(map[reflect.Type]struct{}, len(masks))
	for operation, mask := range masks {
		capability := m7OracleCapabilities[operation]
		known[capability] = struct{}{}
		for level := range levels {
			if mask&(1<<level) != 0 {
				levels[level].contracts.bindings = append(levels[level].contracts.bindings, contractBinding{capability: capability})
			}
		}
	}
	return &Plan[stage3C]{validated: &validatedNodes{levels: levels, known: known}, lineage: &definitionLineage{marker: 1}}
}

// m7TableDispatch attaches synthetic level rows to a converted Plan's table:
// each registered provider goes to the slot the table names for its level.
// It decides no destination; expected results come from the backward oracle.
func m7TableDispatch[Leaf any](fast *Plan[Leaf], levels []map[reflect.Type]any) *dispatch {
	registrations := 0
	for _, methods := range levels {
		registrations += len(methods)
	}
	d := &dispatch{known: fast.validated.known, owners: fast.owners, providers: make([]any, registrations)}
	for level, methods := range levels {
		for capability, provider := range methods {
			d.providers[fast.owners[capability].slot[level]] = provider
		}
	}
	return d
}

// The oracle searches the supplied bitset backward; it does not use the
// conversion algorithm or read its table.
func m7BackwardOracle(mask uint16, depth, position int) int {
	start := position - 1
	if position == depth {
		start = depth - 1
	}
	for level := start; level >= 0; level-- {
		if mask&(1<<level) != 0 {
			return level
		}
	}
	return -1
}

func m7CheckOracle(t *testing.T, depth int, masks []uint16) {
	t.Helper()
	fast := m7OraclePlan(depth, masks).MustPrecompute()
	providers := make([]*m7OracleProvider, depth)
	levels := make([]map[reflect.Type]any, depth)
	for level := range levels {
		providers[level] = &m7OracleProvider{level: level}
		levels[level] = make(map[reflect.Type]any)
		for operation, mask := range masks {
			if mask&(1<<level) != 0 {
				levels[level][m7OracleCapabilities[operation]] = providers[level]
			}
		}
	}
	dynamicDispatch := &dispatch{known: fast.validated.known, levels: levels}
	precomputedDispatch := m7TableDispatch(fast, levels)
	for operation, mask := range masks {
		capability := m7OracleCapabilities[operation]
		for position := 0; position <= depth; position++ {
			want := m7BackwardOracle(mask, depth, position)
			if got := fast.owners[capability].owner[position]; got != want {
				t.Fatalf("depth=%d masks=%v operation=%d position=%d: owner=%d, want %d", depth, masks, operation, position, got, want)
			}
			// Every parent start is distinct; the last table slot is virtual.
			// Virtual lookup must also agree from every executing level.
			starts := []int{position}
			if position == depth {
				starts = starts[:0]
				for level := range levels {
					starts = append(starts, level)
				}
			}
			for _, start := range starts {
				parent := position != depth
				for _, mode := range []struct {
					name string
					d    *dispatch
				}{{"dynamic", dynamicDispatch}, {"precomputed", precomputedDispatch}} {
					provider, diagnostic := m7InvokeOracle(Scope{dispatch: mode.d, level: start}, operation, parent)
					if want < 0 {
						if provider != nil || diagnostic == nil || diagnostic.Code != NoParentImplementation || diagnostic.Capability != capability || diagnostic.Level != start {
							t.Fatalf("%s depth=%d masks=%v operation=%d parent start=%d: provider=%v diagnostic=%+v", mode.name, depth, masks, operation, start, provider, diagnostic)
						}
					} else if diagnostic != nil || provider != providers[want] {
						t.Fatalf("%s depth=%d masks=%v operation=%d parent=%t start=%d: provider=%v diagnostic=%+v, want %p", mode.name, depth, masks, operation, parent, start, provider, diagnostic, providers[want])
					}
				}
			}
		}
	}
}

func TestM7ExhaustiveSmallResolutionLayouts(t *testing.T) {
	for depth := 1; depth <= 5; depth++ {
		limit := uint16(1 << depth)
		for first := uint16(1); first < limit; first++ {
			m7CheckOracle(t, depth, []uint16{first})
			if depth <= 4 {
				for second := uint16(1); second < limit; second++ {
					m7CheckOracle(t, depth, []uint16{first, second})
				}
			}
		}
	}
}

func FuzzResolution(f *testing.F) {
	for _, seed := range [][]byte{
		{1, 1, 1},          // root only
		{4, 0x10, 0x10},    // leaf only
		{4, 0x15, 0x0a},    // alternating
		{4, 0x1f, 0x1f},    // dense
		{6, 0x20, 0x04, 0}, // missing parents
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		depth := int(data[0]%10) + 1
		limit := uint16((1 << depth) - 1)
		masks := make([]uint16, 3)
		for operation := range masks {
			var low, high byte
			if len(data) > 1+operation*2 {
				low = data[1+operation*2]
			}
			if len(data) > 2+operation*2 {
				high = data[2+operation*2]
			}
			masks[operation] = (uint16(low) | uint16(high)<<8) & limit
			if masks[operation] == 0 {
				masks[operation] = 1 << (operation % depth)
			}
		}
		m7CheckOracle(t, depth, masks)
		choices := make([][]uint8, len(masks))
		for operation := range choices {
			choices[operation] = make([]uint8, depth)
			for level := range choices[operation] {
				position := 7 + operation*depth + level
				if position < len(data) {
					choices[operation][level] = data[position] & 1
				} else {
					choices[operation][level] = uint8((operation + level) & 1)
				}
			}
		}
		m8CheckComponentOracle(t, depth, masks, choices)
		var integrationMasks [3]uint8
		var integrationChoices [3][3]uint8
		for operation := range integrationMasks {
			integrationMasks[operation] = uint8(masks[operation] & 7)
			for level := 0; level < 3; level++ {
				integrationChoices[operation][level] = choices[operation][level%depth]
			}
		}
		m8CheckIntegratedComponentOracle(t, integrationMasks, integrationChoices)
	})
}

type m8OracleProvider struct {
	Node
	level, slot int
}

func (p *m8OracleProvider) Read() int       { return p.level*2 + p.slot }
func (p *m8OracleProvider) Add(n int) int   { return p.level*2 + p.slot + n }
func (p *m8OracleProvider) Walk(int) string { return "component oracle" }

// This kernel oracle checks owner and component receiver selection from
// synthetic dispatch rows; the integration oracle exercises declaration and binding.
func m8CheckComponentOracle(t *testing.T, depth int, masks []uint16, choices [][]uint8) {
	t.Helper()
	fast := m7OraclePlan(depth, masks).MustPrecompute()
	providers := make([][2]*m8OracleProvider, depth)
	levels := make([]map[reflect.Type]any, depth)
	for level := range levels {
		providers[level] = [2]*m8OracleProvider{{level: level, slot: 0}, {level: level, slot: 1}}
		levels[level] = make(map[reflect.Type]any)
		for operation, mask := range masks {
			if mask&(1<<level) != 0 {
				levels[level][m7OracleCapabilities[operation]] = providers[level][choices[operation][level]]
			}
		}
	}
	for _, mode := range []struct {
		name string
		d    *dispatch
	}{{"dynamic", &dispatch{known: fast.validated.known, levels: levels}},
		{"precomputed", m7TableDispatch(fast, levels)}} {
		for operation, mask := range masks {
			capability := m7OracleCapabilities[operation]
			for position := 0; position <= depth; position++ {
				wantLevel := m7BackwardOracle(mask, depth, position)
				if mode.name == "precomputed" && fast.owners[capability].owner[position] != wantLevel {
					t.Fatalf("table owner=%d, want %d", fast.owners[capability].owner[position], wantLevel)
				}
				starts := []int{position}
				if position == depth {
					starts = starts[:0]
					for level := range levels {
						starts = append(starts, level)
					}
				}
				for _, start := range starts {
					provider, diagnostic := m7InvokeOracle(Scope{dispatch: mode.d, level: start}, operation, position != depth)
					if wantLevel < 0 {
						if provider != nil || diagnostic == nil || diagnostic.Code != NoParentImplementation || diagnostic.Capability != capability || diagnostic.Level != start {
							t.Fatalf("%s depth=%d masks=%v position=%d: provider=%v diagnostic=%+v", mode.name, depth, masks, position, provider, diagnostic)
						}
						continue
					}
					want := providers[wantLevel][choices[operation][wantLevel]]
					if diagnostic != nil || provider != want {
						t.Fatalf("%s depth=%d masks=%v position=%d: provider=%v diagnostic=%+v, want %p", mode.name, depth, masks, position, provider, diagnostic, want)
					}
				}
			}
		}
	}
}

func TestM8ExhaustiveSmallComponentLayouts(t *testing.T) {
	for depth := 1; depth <= 3; depth++ {
		limit := uint16(1 << depth)
		for first := uint16(1); first < limit; first++ {
			for second := uint16(1); second < limit; second++ {
				for firstSlots := uint16(0); firstSlots < limit; firstSlots++ {
					for secondSlots := uint16(0); secondSlots < limit; secondSlots++ {
						choices := [][]uint8{make([]uint8, depth), make([]uint8, depth)}
						for level := 0; level < depth; level++ {
							choices[0][level] = uint8((firstSlots >> level) & 1)
							choices[1][level] = uint8((secondSlots >> level) & 1)
						}
						m8CheckComponentOracle(t, depth, []uint16{first, second}, choices)
					}
				}
			}
		}
	}
}
