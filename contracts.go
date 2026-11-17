package heiry

import (
	"errors"
	"reflect"
	"sort"
)

// These declarations are internal definition metadata, not a public schema API.
type contractBinding struct {
	capability reflect.Type
	provider   reflect.Type
}

type contractLevel struct {
	public, private reflect.Type
	bindings        []contractBinding
}

type contractResult struct {
	known map[reflect.Type]struct{}
}

type contractOperation struct {
	method   reflect.Method
	category string
}

func validateContracts(levels []contractLevel, target int) (*contractResult, error) {
	if len(levels) == 0 || target < 0 || target >= len(levels) {
		return nil, &Error{Code: InvalidContract, Level: target}
	}
	result := &contractResult{known: make(map[reflect.Type]struct{})}
	var previous map[string]contractOperation
	canonical := make(map[string]reflect.Type)
	bound := make(map[string]bool)
	var diagnostics []error
	fail := func(err error) (*contractResult, error) {
		if len(diagnostics) == 0 {
			return nil, err
		}
		return nil, errors.Join(append(diagnostics, err)...)
	}
	var effectivePublic, effectivePrivate reflect.Type
	for level := 0; level <= target; level++ {
		entry := levels[level]
		if level == 0 && (entry.public == nil || entry.private == nil) {
			return fail(&Error{Code: InvalidContract, Level: level})
		}
		if (entry.public == nil) != (entry.private == nil) {
			return fail(&Error{Code: InvalidContract, Level: level})
		}
		if entry.public != nil {
			effectivePublic, effectivePrivate = entry.public, entry.private
		}
		operations := make(map[string]contractOperation)
		for _, category := range []struct {
			name   string
			typeOf reflect.Type
		}{{"public", effectivePublic}, {"private", effectivePrivate}} {
			if category.typeOf == nil || category.typeOf.Kind() != reflect.Interface {
				return fail(&Error{Code: InvalidContract, Category: category.name, Level: level})
			}
			for i := 0; i < category.typeOf.NumMethod(); i++ {
				method := category.typeOf.Method(i)
				if method.PkgPath != "" {
					return fail(&Error{Code: InvalidContract, Name: method.Name, Category: category.name, Level: level})
				}
				if _, exists := operations[method.Name]; exists {
					return fail(&Error{Code: ContractCollision, Name: method.Name, Level: level})
				}
				operations[method.Name] = contractOperation{method: method, category: category.name}
			}
		}
		previousNames := make([]string, 0, len(previous))
		for name := range previous {
			previousNames = append(previousNames, name)
		}
		sort.Strings(previousNames)
		for _, name := range previousNames {
			old := previous[name]
			current, ok := operations[name]
			if !ok || current.category != old.category || current.method.Type != old.method.Type {
				return fail(&Error{Code: ContractExtension, Name: name, Category: old.category, Level: level, Expected: old.method.Type, Actual: current.method.Type})
			}
		}
		seen := make(map[string]bool)
		for _, binding := range entry.bindings {
			capability := binding.capability
			if capability == nil || capability.Kind() != reflect.Interface || capability.NumMethod() != 1 || capability.Method(0).PkgPath != "" {
				diagnostics = append(diagnostics, &Error{Code: InvalidCapability, Capability: capability, Level: level})
				continue
			}
			method := capability.Method(0)
			operation, ok := operations[method.Name]
			if !ok {
				diagnostics = append(diagnostics, &Error{Code: UndeclaredOperation, Name: method.Name, Capability: capability, Level: level})
				continue
			}
			if method.Type != operation.method.Type {
				diagnostics = append(diagnostics, &Error{Code: SignatureMismatch, Name: method.Name, Capability: capability, Level: level, Expected: operation.method.Type, Actual: method.Type})
				continue
			}
			if prior, exists := canonical[method.Name]; exists && prior != capability {
				diagnostics = append(diagnostics, &Error{Code: CapabilityIdentity, Name: method.Name, Capability: capability, Level: level, Expected: prior, Actual: capability})
				continue
			}
			if seen[method.Name] {
				diagnostics = append(diagnostics, &Error{Code: DuplicateBinding, Name: method.Name, Capability: capability, Level: level})
				continue
			}
			if binding.provider == nil || binding.provider.Kind() != reflect.Pointer || !binding.provider.Implements(capability) {
				diagnostics = append(diagnostics, &Error{Code: SignatureMismatch, Name: method.Name, Capability: capability, Level: level, Expected: capability, Actual: binding.provider})
				continue
			}
			seen[method.Name], bound[method.Name] = true, true
			canonical[method.Name] = capability
			result.known[capability] = struct{}{}
		}
		previous = operations
	}
	var missing []string
	for name := range previous {
		if !bound[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		diagnostics = append(diagnostics, &Error{Code: MissingMethods, Name: name, Category: previous[name].category, Capability: canonical[name], Level: target, Expected: previous[name].method.Type})
	}
	if len(diagnostics) != 0 {
		return nil, errors.Join(diagnostics...)
	}
	return result, nil
}
