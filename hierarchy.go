package heiry

import (
	"errors"
	"reflect"
	"strings"
)

// nodeLevel is internal definition metadata; it retains no receivers.
type nodeLevel struct {
	typ         reflect.Type
	parentField string
	contracts   contractLevel
}

type validatedNodes struct {
	levels       []nodeLevel
	parentFields []int
	nodeFields   []int
}

// validateNodeDefinition checks a single-provider value-embedded chain without
// allocating or invoking any application object.
func validateNodeDefinition(levels []nodeLevel) (*validatedNodes, error) {
	if len(levels) == 0 {
		return nil, &Error{Code: InvalidHierarchy}
	}
	v := &validatedNodes{levels: append([]nodeLevel(nil), levels...), parentFields: make([]int, len(levels)), nodeFields: make([]int, len(levels))}
	target := levels[len(levels)-1].typ
	for index := range v.levels {
		v.levels[index].contracts.bindings = append([]contractBinding(nil), levels[index].contracts.bindings...)
	}
	seen := make(map[reflect.Type]bool)
	for level, entry := range levels {
		typ := entry.typ
		if typ == nil || typ.Kind() != reflect.Struct || typ.Name() == "" || seen[typ] {
			return nil, &Error{Code: InvalidHierarchy, Level: level, Target: target, Path: v.providerPath(level), Actual: typ}
		}
		seen[typ] = true
		nodeIndex := -1
		parentIndex := -1
		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			if field.Type == reflect.TypeFor[Node]() && field.Anonymous {
				if nodeIndex != -1 {
					return nil, &Error{Code: MissingNode, Level: level, Target: target, Path: v.nodePath(level), Expected: reflect.TypeFor[Node]()}
				}
				nodeIndex = index
			}
			if level > 0 && field.Name == entry.parentField && field.Anonymous && field.Type == levels[level-1].typ {
				parentIndex = index
			}
		}
		if nodeIndex == -1 {
			return nil, &Error{Code: MissingNode, Level: level, Target: target, Path: v.nodePath(level), Expected: reflect.TypeFor[Node]()}
		}
		v.nodeFields[level] = nodeIndex
		if level == 0 {
			if entry.parentField != "" {
				return nil, &Error{Code: InvalidHierarchy, Level: level, Target: target, Path: entry.parentField}
			}
		} else {
			if parentIndex == -1 {
				return nil, &Error{Code: InvalidHierarchy, Level: level, Target: target, Path: v.edgePath(level), Expected: levels[level-1].typ}
			}
			v.parentFields[level] = parentIndex
		}
	}
	contracts := make([]contractLevel, len(levels))
	var providerErrors []error
	for level, entry := range levels {
		contracts[level] = contractLevel{public: entry.contracts.public, private: entry.contracts.private}
		for _, binding := range entry.contracts.bindings {
			if binding.provider != reflect.PointerTo(entry.typ) {
				providerErrors = append(providerErrors, &Error{Code: InvalidProvider, Level: level, Target: target, Path: v.providerPath(level), Capability: binding.capability, Expected: reflect.PointerTo(entry.typ), Actual: binding.provider})
				continue
			}
			contracts[level].bindings = append(contracts[level].bindings, binding)
		}
	}
	if _, err := validateContracts(contracts, len(contracts)-1); err != nil {
		providerErrors = append(providerErrors, v.withContext(err))
	}
	if len(providerErrors) != 0 {
		return nil, errors.Join(providerErrors...)
	}
	return v, nil
}

func (v *validatedNodes) withContext(err error) error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		items := joined.Unwrap()
		annotated := make([]error, len(items))
		for index, item := range items {
			annotated[index] = v.withContext(item)
		}
		return errors.Join(annotated...)
	}
	if diagnostic, ok := err.(*Error); ok {
		copy := *diagnostic
		copy.Target = v.levels[len(v.levels)-1].typ
		if copy.Level >= 0 && copy.Level < len(v.levels) {
			copy.Path = v.providerPath(copy.Level)
		}
		return &copy
	}
	return err
}

// bindValidatedNodes checks every address and holder before installing scopes.
func bindValidatedNodes(v *validatedNodes, leaf any, providers []any) error {
	if v == nil || len(v.levels) == 0 {
		return &Error{Code: BindingMismatch}
	}
	target := v.levels[len(v.levels)-1].typ
	root := reflect.ValueOf(leaf)
	if !root.IsValid() || root.Kind() != reflect.Pointer || root.IsNil() || root.Type() != reflect.PointerTo(target) {
		return &Error{Code: InvalidProvider, Target: target}
	}
	if len(providers) != len(v.levels) {
		return &Error{Code: BindingMismatch, Target: target}
	}
	values := make([]reflect.Value, len(v.levels))
	values[len(values)-1] = root.Elem()
	for level := len(values) - 1; level > 0; level-- {
		values[level-1] = values[level].Field(v.parentFields[level])
	}
	bindings := make([]nodeBinding, len(values))
	for level, expected := range values {
		provider := reflect.ValueOf(providers[level])
		if !provider.IsValid() || provider.Kind() != reflect.Pointer || provider.IsNil() || provider.Type() != reflect.PointerTo(v.levels[level].typ) {
			return &Error{Code: InvalidProvider, Target: target, Level: level, Path: v.providerPath(level), Expected: reflect.PointerTo(v.levels[level].typ), Actual: reflect.TypeOf(providers[level])}
		}
		if !expected.IsValid() || !expected.CanAddr() || provider.Pointer() != expected.Addr().Pointer() {
			return &Error{Code: ProviderOwnership, Target: target, Level: level, Path: v.providerPath(level), Expected: reflect.PointerTo(v.levels[level].typ), Actual: provider.Type()}
		}
		holder := provider.Elem().Field(v.nodeFields[level])
		if !holder.IsValid() || !holder.CanAddr() || !holder.Addr().CanInterface() {
			return &Error{Code: MissingNode, Target: target, Level: level}
		}
		node, ok := holder.Addr().Interface().(*Node)
		if !ok {
			return &Error{Code: MissingNode, Target: target, Level: level}
		}
		if node.Scope().dispatch != nil {
			return &Error{Code: AlreadyInitialized, Target: target, Level: level}
		}
		bindings[level] = nodeBinding{node: node, provider: providers[level]}
		for _, binding := range v.levels[level].contracts.bindings {
			bindings[level].capabilities = append(bindings[level].capabilities, binding.capability)
		}
	}
	bindNodeLevels(bindings)
	return nil
}

func (v *validatedNodes) providerPath(level int) string {
	var parts []string
	for index := len(v.levels) - 1; index > level; index-- {
		parts = append(parts, v.levels[index].parentField)
	}
	return strings.Join(parts, ".")
}

func (v *validatedNodes) edgePath(level int) string {
	return strings.Join(nonempty(v.providerPath(level), v.levels[level].parentField), ".")
}

func (v *validatedNodes) nodePath(level int) string {
	return strings.Join(nonempty(v.providerPath(level), "Node"), ".")
}

func nonempty(parts ...string) []string {
	var result []string
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
