package heiry

import "reflect"

type definitionLineage struct{ marker byte }

// Plan is a prepared, immutable, receiver-free dynamic hierarchy.
type Plan[Leaf any] struct {
	validated *validatedNodes
	lineage   *definitionLineage
	nodes     map[reflect.Type]int
}

// ProviderRef carries one instance pointer and a node-slot identity.
// A Plan or Definition never retains a ProviderRef.
type ProviderRef[Leaf any] struct {
	lineage  *definitionLineage
	nodeType reflect.Type
	provider any
	invalid  Code
}

// Prepare validates one snapshot and returns its reusable dynamic Plan.
func (d *Definition[Leaf]) Prepare() (*Plan[Leaf], error) {
	if d == nil {
		return nil, &Error{Code: InvalidDeclaration, Target: reflect.TypeFor[Leaf]()}
	}
	if d.plan != nil {
		if len(d.recorded) != 0 {
			return nil, d.withRecorded(nil)
		}
		return d.plan, nil
	}
	validated, err := d.validatedSnapshot()
	if err = d.withRecorded(err); err != nil {
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
	if d.lineage == nil {
		d.lineage = &definitionLineage{marker: 1}
	}
	return ProviderRef[Leaf]{lineage: d.lineage, nodeType: reflect.TypeFor[Node](), provider: ptr}
}

// NodeProvider creates an instance reference using only immutable Plan data.
func (p *Plan[Leaf]) NodeProvider[Node any](ptr *Node) ProviderRef[Leaf] {
	if p == nil || p.validated == nil {
		return ProviderRef[Leaf]{invalid: InvalidPlan}
	}
	return ProviderRef[Leaf]{lineage: p.lineage, nodeType: reflect.TypeFor[Node](), provider: ptr}
}

// Init checks one leaf and every supplied node before installing any scopes.
func (p *Plan[Leaf]) Init(leaf *Leaf, refs ...ProviderRef[Leaf]) error {
	if p == nil || p.validated == nil || p.lineage == nil {
		return &Error{Code: InvalidPlan, Target: reflect.TypeFor[Leaf]()}
	}
	providers := make([]any, len(p.validated.levels))
	seen := make([]bool, len(providers))
	for _, ref := range refs {
		if ref.invalid != "" {
			return &Error{Code: ref.invalid, Target: reflect.TypeFor[Leaf](), Actual: reflect.TypeOf(ref.provider)}
		}
		if ref.lineage == nil || ref.lineage != p.lineage {
			return &Error{Code: BindingMismatch, Target: reflect.TypeFor[Leaf](), Actual: reflect.TypeOf(ref.provider)}
		}
		level, ok := p.nodes[ref.nodeType]
		if !ok {
			return &Error{Code: InvalidProvider, Target: reflect.TypeFor[Leaf](), Actual: reflect.TypeOf(ref.provider)}
		}
		if seen[level] {
			return &Error{Code: BindingMismatch, Target: reflect.TypeFor[Leaf](), Level: level, Path: p.validated.providerPath(level)}
		}
		seen[level] = true
		providers[level] = ref.provider
	}
	for level, present := range seen {
		if !present {
			return &Error{Code: BindingMismatch, Target: reflect.TypeFor[Leaf](), Level: level, Path: p.validated.providerPath(level), Expected: reflect.PointerTo(p.validated.levels[level].typ)}
		}
	}
	return bindValidatedNodes(p.validated, leaf, providers)
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
