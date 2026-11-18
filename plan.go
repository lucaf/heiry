package heiry

import "reflect"

type definitionLineage struct{ marker byte }

// Plan is a prepared, immutable, receiver-free dynamic hierarchy.
type Plan[Leaf any] struct {
	validated *validatedNodes
	lineage   *definitionLineage
	nodes     map[reflect.Type]int
	owners    map[reflect.Type]ownerRow
}

// Precompute returns an immutable resolution variant without changing p.
func (p *Plan[Leaf]) Precompute() (*Plan[Leaf], error) {
	if p == nil || p.validated == nil || p.lineage == nil {
		return nil, &Error{Code: InvalidPlan, Target: reflect.TypeFor[Leaf]()}
	}
	if p.owners != nil {
		return p, nil
	}
	depth := len(p.validated.levels)
	count := len(p.validated.known)
	// Two backing arrays hold every row. Each row is capped at its own
	// segment and nothing writes to them after this function returns.
	owner := make([]int, count*(depth+1))
	slot := make([]int, count*depth)
	for index := range slot {
		slot[index] = -1
	}
	owners := make(map[reflect.Type]ownerRow, count)
	for capability := range p.validated.known {
		index := len(owners)
		owners[capability] = ownerRow{
			owner: owner[index*(depth+1) : (index+1)*(depth+1) : (index+1)*(depth+1)],
			slot:  slot[index*depth : (index+1)*depth : (index+1)*depth],
		}
	}
	// Number the registrations in level and declaration order. Init fills
	// each object's provider slice by the same numbers.
	next := 0
	for level, entry := range p.validated.levels {
		for _, binding := range entry.contracts.bindings {
			if row, known := owners[binding.capability]; known {
				row.slot[level] = next
			}
			next++
		}
	}
	// One forward pass per capability records the previous owner before
	// taking in each level's own registration.
	for _, row := range owners {
		current := -1
		for level := 0; level < depth; level++ {
			row.owner[level] = current
			if row.slot[level] >= 0 {
				current = level
			}
		}
		row.owner[depth] = current
	}
	return &Plan[Leaf]{validated: p.validated, lineage: p.lineage, nodes: p.nodes, owners: owners}, nil
}

// MustPrecompute panics with the error returned by Precompute.
func (p *Plan[Leaf]) MustPrecompute() *Plan[Leaf] {
	converted, err := p.Precompute()
	if err != nil {
		panic(err)
	}
	return converted
}

// ProviderRef carries one instance pointer and a provider-slot identity.
// A Plan or Definition never retains a ProviderRef.
type ProviderRef[Leaf any] struct {
	lineage  *definitionLineage
	nodeType reflect.Type
	path     string
	provider any
	invalid  Code
}

// Prepare validates one snapshot and returns its reusable dynamic Plan.
func (d *Definition[Leaf]) Prepare() (*Plan[Leaf], error) {
	if d == nil {
		return nil, &Error{Code: InvalidDeclaration, Target: reflect.TypeFor[Leaf]()}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.plan != nil {
		if len(d.recorded) != 0 {
			return nil, d.withRecorded(nil, d.plan.validated)
		}
		return d.plan, nil
	}
	validated, err := d.validatedSnapshot()
	if err = d.withRecorded(err, validated); err != nil {
		return nil, err
	}
	if d.lineage == nil {
		d.lineage = &definitionLineage{marker: 1}
	}
	nodes := make(map[reflect.Type]int, len(validated.levels))
	for level, entry := range validated.levels {
		nodes[entry.typ] = level
	}
	d.plan = &Plan[Leaf]{validated: validated, lineage: d.lineage, nodes: nodes}
	return d.plan, nil
}

// MustPrepare panics with the error returned by Prepare.
func (d *Definition[Leaf]) MustPrepare() *Plan[Leaf] {
	plan, err := d.Prepare()
	if err != nil {
		panic(err)
	}
	return plan
}

// NodeProvider describes an instance node without preparing the definition.
func (d *Definition[Leaf]) NodeProvider[Node any](ptr *Node) ProviderRef[Leaf] {
	if d == nil {
		return ProviderRef[Leaf]{invalid: InvalidDeclaration}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lineage == nil {
		d.lineage = &definitionLineage{marker: 1}
	}
	return ProviderRef[Leaf]{lineage: d.lineage, nodeType: reflect.TypeFor[Node](), path: ".", provider: ptr}
}

// NodeProvider creates an instance reference using only immutable Plan data.
func (p *Plan[Leaf]) NodeProvider[Node any](ptr *Node) ProviderRef[Leaf] {
	if p == nil || p.validated == nil {
		return ProviderRef[Leaf]{invalid: InvalidPlan}
	}
	return ProviderRef[Leaf]{lineage: p.lineage, nodeType: reflect.TypeFor[Node](), path: ".", provider: ptr}
}

// ComponentProvider describes one owned component without preparing the definition.
func (d *Definition[Leaf]) ComponentProvider[Node, Provider any](providerPath string, ptr *Provider) ProviderRef[Leaf] {
	if d == nil {
		return ProviderRef[Leaf]{nodeType: reflect.TypeFor[Node](), path: providerPath, provider: ptr, invalid: InvalidDeclaration}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lineage == nil {
		d.lineage = &definitionLineage{marker: 1}
	}
	ref := ProviderRef[Leaf]{lineage: d.lineage, nodeType: reflect.TypeFor[Node](), path: providerPath, provider: ptr}
	if providerPath == "." || !validProviderPath(providerPath) {
		ref.invalid = InvalidProvider
	}
	return ref
}

// ComponentProvider describes one owned component using immutable Plan metadata.
func (p *Plan[Leaf]) ComponentProvider[Node, Provider any](providerPath string, ptr *Provider) ProviderRef[Leaf] {
	ref := ProviderRef[Leaf]{nodeType: reflect.TypeFor[Node](), path: providerPath, provider: ptr}
	if p == nil || p.validated == nil || p.lineage == nil {
		ref.invalid = InvalidPlan
		return ref
	}
	ref.lineage = p.lineage
	if providerPath == "." || !validProviderPath(providerPath) {
		ref.invalid = InvalidProvider
	}
	return ref
}

// Init checks one leaf and every required provider slot before installing scopes.
func (p *Plan[Leaf]) Init(leaf *Leaf, refs ...ProviderRef[Leaf]) error {
	if p == nil || p.validated == nil || p.lineage == nil {
		return &Error{Code: InvalidPlan, Target: reflect.TypeFor[Leaf]()}
	}
	providers := make([]map[string]any, len(p.validated.levels))
	for level := range providers {
		providers[level] = make(map[string]any, len(p.validated.slots[level]))
	}
	for _, ref := range refs {
		level, ownerKnown := p.nodes[ref.nodeType]
		if !ownerKnown {
			level = -1
		}
		path := ref.path
		if ownerKnown {
			path = p.validated.fullProviderPath(level, ref.path)
		}
		if ref.invalid != "" {
			return &Error{Code: ref.invalid, Target: reflect.TypeFor[Leaf](), Level: level, Path: path, Actual: reflect.TypeOf(ref.provider)}
		}
		if ref.lineage == nil || ref.lineage != p.lineage {
			return &Error{Code: BindingMismatch, Target: reflect.TypeFor[Leaf](), Level: level, Path: path, Actual: reflect.TypeOf(ref.provider)}
		}
		if !ownerKnown {
			return &Error{Code: InvalidProvider, Target: reflect.TypeFor[Leaf](), Level: -1, Path: ref.path, Actual: reflect.TypeOf(ref.provider)}
		}
		index, declared := p.validated.slotIndex[level][ref.path]
		if !declared {
			return &Error{Code: BindingMismatch, Target: reflect.TypeFor[Leaf](), Level: level, Path: path, Actual: reflect.TypeOf(ref.provider)}
		}
		slot := p.validated.slots[level][index]
		actual := reflect.ValueOf(ref.provider)
		if !actual.IsValid() || actual.Kind() != reflect.Pointer || actual.IsNil() || actual.Type() != reflect.PointerTo(slot.typ) {
			return &Error{Code: InvalidProvider, Target: reflect.TypeFor[Leaf](), Level: level, Path: path, Expected: reflect.PointerTo(slot.typ), Actual: reflect.TypeOf(ref.provider)}
		}
		if _, seen := providers[level][ref.path]; seen {
			return &Error{Code: BindingMismatch, Target: reflect.TypeFor[Leaf](), Level: level, Path: path}
		}
		providers[level][ref.path] = ref.provider
	}
	for level, slots := range p.validated.slots {
		for _, slot := range slots {
			if _, present := providers[level][slot.path]; !present {
				return &Error{Code: BindingMismatch, Target: reflect.TypeFor[Leaf](), Level: level, Path: p.validated.fullProviderPath(level, slot.path), Expected: reflect.PointerTo(slot.typ)}
			}
		}
	}
	return bindValidatedProviders(p.validated, leaf, providers, p.owners)
}

// MustInit panics with the error returned by Init and otherwise returns leaf.
func (p *Plan[Leaf]) MustInit(leaf *Leaf, refs ...ProviderRef[Leaf]) *Leaf {
	if err := p.Init(leaf, refs...); err != nil {
		panic(err)
	}
	return leaf
}

// Init prepares on first need, then binds one leaf through the prepared Plan.
func (d *Definition[Leaf]) Init(leaf *Leaf, refs ...ProviderRef[Leaf]) error {
	plan, err := d.Prepare()
	if err != nil {
		return err
	}
	return plan.Init(leaf, refs...)
}

// MustInit panics with the error returned by Init and otherwise returns leaf.
func (d *Definition[Leaf]) MustInit(leaf *Leaf, refs ...ProviderRef[Leaf]) *Leaf {
	if err := d.Init(leaf, refs...); err != nil {
		panic(err)
	}
	return leaf
}
