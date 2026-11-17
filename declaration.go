package heiry

import (
	"errors"
	"go/token"
	"reflect"
	"strings"
)

// Definition describes an ordered hierarchy without holding an application
// object. It is mutable during declaration and does not represent a Plan.
type Definition[Leaf any] struct {
	levels   []*declaredLevel
	recorded []error
	plan     *Plan[Leaf]
	lineage  *definitionLineage
}

type declaredLevel struct {
	typ, public, private reflect.Type
	parentField          string
	bindings             []declaredBinding
	index                int
}

type declaredBinding struct {
	contractBinding
	providerPath string
}

// LevelHandle identifies one stable level in its definition.
type LevelHandle[Leaf any] struct {
	definition *Definition[Leaf]
	level      *declaredLevel
	rejected   bool
}

// Define begins one receiver-free hierarchy declaration.
func Define[Leaf any]() *Definition[Leaf] {
	return &Definition[Leaf]{lineage: &definitionLineage{marker: 1}}
}

// Level appends a level, with its direct anonymous parent field name.
// The root uses an empty parent field.
func (d *Definition[Leaf]) Level[Node, Public, Private any](parentField string) *LevelHandle[Leaf] {
	if d.plan != nil {
		d.recorded = append(d.recorded, &Error{Code: InvalidDeclaration, Target: reflect.TypeFor[Leaf](), Level: len(d.levels), Path: parentField})
		return &LevelHandle[Leaf]{definition: d, rejected: true}
	}
	level := &declaredLevel{
		typ: reflect.TypeFor[Node](), public: reflect.TypeFor[Public](),
		private: reflect.TypeFor[Private](), parentField: parentField, index: len(d.levels),
	}
	d.levels = append(d.levels, level)
	return &LevelHandle[Leaf]{definition: d, level: level}
}

// End returns the definition while preserving this handle's level identity.
func (l *LevelHandle[Leaf]) End() *Definition[Leaf] { return l.definition }

// Bind records an explicit typed operation for this level. The witness value
// is reduced to type metadata and is never retained.
func (l *LevelHandle[Leaf]) Bind[Operation any](provider Operation, providerPath string) *LevelHandle[Leaf] {
	if l.rejected {
		return l
	}
	if l.definition.plan != nil {
		l.definition.recorded = append(l.definition.recorded, &Error{Code: InvalidDeclaration, Target: reflect.TypeFor[Leaf](), Level: l.level.index, Path: providerPath, Capability: reflect.TypeFor[Operation]()})
		return l
	}
	capability := reflect.TypeFor[Operation]()
	if capability.Kind() != reflect.Interface || capability.NumMethod() != 1 || capability.Method(0).PkgPath != "" {
		l.definition.recorded = append(l.definition.recorded, &Error{
			Code: InvalidCapability, Target: reflect.TypeFor[Leaf](), Level: l.level.index,
			Capability: capability, Path: providerPath,
		})
		return l
	}
	value := reflect.ValueOf(provider)
	if !value.IsValid() || value.Kind() != reflect.Pointer || !value.IsNil() || !validProviderPath(providerPath) {
		l.definition.recorded = append(l.definition.recorded, &Error{
			Code: InvalidProvider, Target: reflect.TypeFor[Leaf](), Level: l.level.index,
			Capability: capability, Path: providerPath, Expected: reflect.PointerTo(l.level.typ), Actual: reflect.TypeOf(provider),
		})
		return l
	}
	l.level.bindings = append(l.level.bindings, declaredBinding{
		contractBinding: contractBinding{capability: capability, provider: reflect.TypeOf(provider)},
		providerPath:    providerPath,
	})
	return l
}

func validProviderPath(path string) bool {
	if path == "." {
		return true
	}
	for _, part := range strings.Split(path, ".") {
		if !token.IsIdentifier(part) {
			return false
		}
	}
	return true
}

// Validate checks the current declaration without freezing or preparing it.
func (d *Definition[Leaf]) Validate() error {
	_, err := d.validatedSnapshot()
	return d.withRecorded(err)
}

func (d *Definition[Leaf]) validatedSnapshot() (*validatedNodes, error) {
	target := reflect.TypeFor[Leaf]()
	if len(d.levels) == 0 {
		return nil, &Error{Code: InvalidHierarchy, Target: target}
	}
	last := d.levels[len(d.levels)-1].typ
	if last != target {
		return nil, &Error{Code: InvalidHierarchy, Target: target, Level: len(d.levels) - 1, Expected: target, Actual: last}
	}
	levels := make([]nodeLevel, len(d.levels))
	var localErrors []error
	for index, level := range d.levels {
		levels[index] = nodeLevel{typ: level.typ, parentField: level.parentField, contracts: contractLevel{public: level.public, private: level.private}}
		for _, binding := range level.bindings {
			if binding.providerPath != "." {
				localErrors = append(localErrors, &Error{Code: InvalidProvider, Target: target, Level: index, Path: binding.providerPath, Capability: binding.capability})
				continue
			}
			levels[index].contracts.bindings = append(levels[index].contracts.bindings, binding.contractBinding)
		}
	}
	validated, err := validateNodeDefinition(levels)
	localErrors = append(localErrors, err)
	if joined := errors.Join(localErrors...); joined != nil {
		return nil, joined
	}
	return validated, nil
}

func (d *Definition[Leaf]) withRecorded(validation error) error {
	if len(d.recorded) == 0 {
		return validation
	}
	all := append([]error(nil), d.recorded...)
	if validation != nil {
		all = append(all, validation)
	}
	return errors.Join(all...)
}
