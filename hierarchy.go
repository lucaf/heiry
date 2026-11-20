package heiry

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
)

// nodeLevel is internal definition metadata; it retains no receivers.
type nodeLevel struct {
	typ         reflect.Type
	parentField string
	// inferParent means parentField is resolved by validation from the
	// preceding level's node type. It stays empty when that is impossible.
	inferParent bool
	contracts   contractLevel
}

type validatedNodes struct {
	levels       []nodeLevel
	parentFields []int
	parentValid  []bool
	nodeFields   []int
	slots        [][]providerSlot
	slotIndex    []map[string]int
	known        map[reflect.Type]struct{}
}

// providerSlot is a receiver-free description of one node or owned component.
type providerSlot struct {
	path      string
	typ       reflect.Type
	fields    []int // direct fields relative to the owning node
	nodeField int
}

// validateNodeDefinition checks a value-embedded chain and its provider slots without
// allocating or invoking any application object.
func validateNodeDefinition(levels []nodeLevel) (*validatedNodes, error) {
	if len(levels) == 0 {
		return nil, &Error{Code: InvalidHierarchy}
	}
	v := &validatedNodes{
		levels: append([]nodeLevel(nil), levels...), parentFields: make([]int, len(levels)),
		nodeFields: make([]int, len(levels)), parentValid: make([]bool, len(levels)), slots: make([][]providerSlot, len(levels)),
		slotIndex: make([]map[string]int, len(levels)),
	}
	target := levels[len(levels)-1].typ
	for index := range v.levels {
		v.levels[index].contracts.bindings = append([]contractBinding(nil), levels[index].contracts.bindings...)
	}
	// Resolve inferred parent edges first, so every later diagnostic can use
	// the declared field names of the levels above it.
	for level := 1; level < len(levels); level++ {
		entry, parent := levels[level], levels[level-1].typ
		if !entry.inferParent || parent == nil || entry.typ == nil || entry.typ.Kind() != reflect.Struct {
			continue
		}
		candidates, name := 0, ""
		for index := 0; index < entry.typ.NumField(); index++ {
			if field := entry.typ.Field(index); field.Anonymous && field.Type == parent {
				candidates, name = candidates+1, field.Name
			}
		}
		if candidates == 1 {
			v.levels[level].parentField = name
		}
	}
	seen := make(map[reflect.Type]bool)
	var structuralErrors []error
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
					return nil, &Error{Code: MissingNode, Level: level, Target: target, Path: v.nodePath(level, "Node"), Expected: reflect.TypeFor[Node]()}
				}
				nodeIndex = index
			}
			if level > 0 && field.Name == v.levels[level].parentField && field.Anonymous && field.Type == levels[level-1].typ {
				parentIndex = index
			}
		}
		if nodeIndex == -1 {
			return nil, &Error{Code: MissingNode, Level: level, Target: target, Path: v.nodePath(level, "Node"), Expected: reflect.TypeFor[Node]()}
		}
		// The binder reaches a holder through reflection on its field,
		// which cannot use a field with an unexported name, so a holder
		// embedded under an unexported alias counts as missing; the path
		// names that field.
		if holder := typ.Field(nodeIndex); !holder.IsExported() {
			return nil, &Error{Code: MissingNode, Level: level, Target: target, Path: v.nodePath(level, holder.Name), Expected: reflect.TypeFor[Node]()}
		}
		v.nodeFields[level] = nodeIndex
		if level == 0 {
			if entry.parentField != "" {
				return nil, &Error{Code: InvalidHierarchy, Level: level, Target: target, Path: entry.parentField}
			}
		} else {
			if parentIndex == -1 {
				structuralErrors = append(structuralErrors, &Error{Code: InvalidHierarchy, Level: level, Target: target, Path: v.edgePath(level), Expected: levels[level-1].typ})
				continue
			}
			v.parentFields[level] = parentIndex
			v.parentValid[level] = true
		}
	}
	contracts := make([]contractLevel, len(levels))
	providerErrors := structuralErrors
	for level, entry := range levels {
		contracts[level] = contractLevel{public: entry.contracts.public, private: entry.contracts.private}
		v.slots[level] = []providerSlot{{path: ".", typ: entry.typ, nodeField: v.nodeFields[level]}}
		v.slotIndex[level] = map[string]int{".": 0}
		failedSlots := make(map[string]bool)
		for _, binding := range entry.contracts.bindings {
			path := binding.providerPath
			if path == "" { // internal single-node fixtures predate path metadata
				path = "."
			}
			index, exists := v.slotIndex[level][path]
			if !exists {
				if failedSlots[path] {
					binding.providerPath = path
					binding.invalidProvider = true
					contracts[level].bindings = append(contracts[level].bindings, binding)
					continue
				}
				slot, err := v.resolveComponentSlot(level, path)
				if err != nil {
					providerErrors = append(providerErrors, err)
					failedSlots[path] = true
					binding.providerPath = path
					binding.invalidProvider = true
					contracts[level].bindings = append(contracts[level].bindings, binding)
					continue
				}
				index = len(v.slots[level])
				v.slots[level] = append(v.slots[level], slot)
				v.slotIndex[level][path] = index
			}
			slot := v.slots[level][index]
			if binding.provider != reflect.PointerTo(slot.typ) {
				providerErrors = append(providerErrors, &Error{Code: InvalidProvider, Level: level, Target: target, Path: v.fullProviderPath(level, path), Capability: binding.capability, Expected: reflect.PointerTo(slot.typ), Actual: binding.provider})
				binding.providerPath = path
				binding.invalidProvider = true
				contracts[level].bindings = append(contracts[level].bindings, binding)
				continue
			}
			binding.providerPath = path
			contracts[level].bindings = append(contracts[level].bindings, binding)
		}
	}
	result, err := validateContracts(contracts, len(contracts)-1)
	if err != nil {
		providerErrors = append(providerErrors, v.withContext(err))
	}
	if len(providerErrors) != 0 {
		return v, errors.Join(providerErrors...)
	}
	v.known = result.known
	return v, nil
}

// resolveComponentSlot walks declared direct value fields only. The parent
// subtree is not a component owned by this level.
func (v *validatedNodes) resolveComponentSlot(level int, path string) (providerSlot, error) {
	context := func(code Code) error {
		return &Error{Code: code, Target: v.levels[len(v.levels)-1].typ, Level: level, Path: v.fullProviderPath(level, path)}
	}
	if path == "." || !validProviderPath(path) {
		return providerSlot{}, context(InvalidProvider)
	}
	current := v.levels[level].typ
	indices := make([]int, 0, strings.Count(path, ".")+1)
	for step, name := range strings.Split(path, ".") {
		if step == 0 && level > 0 && name == v.levels[level].parentField {
			return providerSlot{}, context(ProviderOwnership)
		}
		found := -1
		for field := 0; field < current.NumField(); field++ {
			if current.Field(field).Name == name {
				found = field
				break
			}
		}
		if found < 0 || current.Field(found).Type.Kind() != reflect.Struct {
			return providerSlot{}, context(InvalidProvider)
		}
		indices = append(indices, found)
		current = current.Field(found).Type
	}
	nodeField := -1
	for field := 0; field < current.NumField(); field++ {
		declared := current.Field(field)
		if declared.Type == reflect.TypeFor[Node]() && declared.Anonymous {
			if nodeField >= 0 {
				return providerSlot{}, &Error{Code: MissingNode, Target: v.levels[len(v.levels)-1].typ, Level: level, Path: v.fullProviderPath(level, path) + ".Node"}
			}
			nodeField = field
		}
	}
	if nodeField < 0 {
		return providerSlot{}, &Error{Code: MissingNode, Target: v.levels[len(v.levels)-1].typ, Level: level, Path: v.fullProviderPath(level, path) + ".Node"}
	}
	if holder := current.Field(nodeField); !holder.IsExported() { // as for level nodes
		return providerSlot{}, &Error{Code: MissingNode, Target: v.levels[len(v.levels)-1].typ, Level: level, Path: v.fullProviderPath(level, path) + "." + holder.Name}
	}
	return providerSlot{path: path, typ: current, fields: indices, nodeField: nodeField}, nil
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
			switch {
			case isRegistrationPathError(copy.Code) && copy.Path != "":
				copy.Path = v.fullProviderPath(copy.Level, copy.Path)
			case (copy.Code == ContractExtension || copy.Code == ContractCollision || copy.Code == InvalidContract) && copy.Path == "":
				copy.Path = v.fullProviderPath(copy.Level, "")
			}
		}
		return &copy
	}
	return err
}

func isRegistrationPathError(code Code) bool {
	switch code {
	case InvalidCapability, UndeclaredOperation, SignatureMismatch, CapabilityIdentity, DuplicateBinding:
		return true
	default:
		return false
	}
}

// bindValidatedNodes checks every address and holder before installing scopes.
func bindValidatedNodes(v *validatedNodes, leaf any, providers []any) error {
	return bindValidatedNodesWithOwners(v, leaf, providers, nil)
}

func bindValidatedNodesWithOwners(v *validatedNodes, leaf any, providers []any, owners map[reflect.Type]ownerRow) error {
	if v == nil || len(v.levels) == 0 {
		return &Error{Code: BindingMismatch}
	}
	if len(providers) != len(v.levels) {
		return &Error{Code: BindingMismatch, Target: v.levels[len(v.levels)-1].typ}
	}
	bySlot := make([]map[string]any, len(providers))
	for level, provider := range providers {
		bySlot[level] = map[string]any{".": provider}
	}
	return bindValidatedProviders(v, leaf, bySlot, owners)
}

func bindValidatedProviders(v *validatedNodes, leaf any, providers []map[string]any, owners map[reflect.Type]ownerRow) error {
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
	type installation struct {
		holder *Node
		level  int
	}
	installations := make([]installation, 0)
	seenHolders := make(map[*Node]int)
	for level, slots := range v.slots {
		for _, slot := range slots {
			selected := providers[level][slot.path]
			provider := reflect.ValueOf(selected)
			path := v.fullProviderPath(level, slot.path)
			if !provider.IsValid() || provider.Kind() != reflect.Pointer || provider.IsNil() || provider.Type() != reflect.PointerTo(slot.typ) {
				return &Error{Code: InvalidProvider, Target: target, Level: level, Path: path, Expected: reflect.PointerTo(slot.typ), Actual: reflect.TypeOf(selected)}
			}
			expected := values[level]
			for _, field := range slot.fields {
				if !expected.IsValid() || expected.Kind() != reflect.Struct || field >= expected.NumField() {
					return &Error{Code: InvalidProvider, Target: target, Level: level, Path: path}
				}
				expected = expected.Field(field)
			}
			owned := expected.IsValid() && expected.CanAddr() && provider.Pointer() == expected.Addr().Pointer()
			runtime.KeepAlive(leaf)
			runtime.KeepAlive(selected)
			if !owned {
				return &Error{Code: ProviderOwnership, Target: target, Level: level, Path: path, Expected: reflect.PointerTo(slot.typ), Actual: provider.Type()}
			}
			holder := provider.Elem().Field(slot.nodeField)
			// The holder path names the holder field, which is Node unless it
			// is embedded through an alias.
			if !holder.IsValid() || !holder.CanAddr() || !holder.Addr().CanInterface() {
				return &Error{Code: MissingNode, Target: target, Level: level, Path: v.fullNodePath(level, slot.path, slot.typ.Field(slot.nodeField).Name)}
			}
			node, ok := holder.Addr().Interface().(*Node)
			if !ok {
				return &Error{Code: MissingNode, Target: target, Level: level, Path: v.fullNodePath(level, slot.path, slot.typ.Field(slot.nodeField).Name)}
			}
			if node.Scope().dispatch != nil {
				return &Error{Code: AlreadyInitialized, Target: target, Level: level, Path: path}
			}
			if prior, exists := seenHolders[node]; exists {
				// Defend against future slot layouts that could alias one holder across levels.
				if prior != level {
					return &Error{Code: ProviderOwnership, Target: target, Level: level, Path: path}
				}
				continue
			}
			seenHolders[node] = level
			installations = append(installations, installation{holder: node, level: level})
		}
	}
	d := &dispatch{known: v.known, owners: owners}
	if owners == nil {
		d.levels = make([]map[reflect.Type]any, len(values))
	} else {
		registrations := 0
		for _, entry := range v.levels {
			registrations += len(entry.contracts.bindings)
		}
		d.providers = make([]any, registrations)
	}
	next := 0
	for level, entry := range v.levels {
		var methods map[reflect.Type]any
		if owners == nil {
			methods = make(map[reflect.Type]any, len(entry.contracts.bindings))
			d.levels[level] = methods
		}
		for _, binding := range entry.contracts.bindings {
			path := binding.providerPath
			if path == "" {
				path = "."
			}
			if owners == nil {
				methods[binding.capability] = providers[level][path]
			} else {
				// Registrations are numbered in level and declaration
				// order, as Precompute numbered the table's slots.
				d.providers[next] = providers[level][path]
				next++
			}
		}
	}
	for _, site := range installations {
		site.holder.scope = Scope{dispatch: d, level: site.level}
	}
	return nil
}

func (v *validatedNodes) providerPath(level int) string {
	var parts []string
	for index := len(v.levels) - 1; index > level; index-- {
		if v.levels[index].inferParent && v.levels[index].parentField == "" {
			// An edge that could not be inferred has no name; do not invent a prefix.
			return ""
		}
		parts = append(parts, v.levels[index].parentField)
	}
	return strings.Join(parts, ".")
}

func (v *validatedNodes) edgePath(level int) string {
	return strings.Join(nonempty(v.providerPath(level), v.levels[level].parentField), ".")
}

// nodePath appends a level node's holder field name to its path.
func (v *validatedNodes) nodePath(level int, holder string) string {
	return strings.Join(nonempty(v.providerPath(level), holder), ".")
}

func (v *validatedNodes) fullProviderPath(level int, path string) string {
	for edge := len(v.levels) - 1; edge > level; edge-- {
		if !v.parentValid[edge] {
			return path
		}
	}
	if path == "." || path == "" {
		return v.providerPath(level)
	}
	return strings.Join(nonempty(v.providerPath(level), path), ".")
}

func (v *validatedNodes) fullNodePath(level int, path, holder string) string {
	return strings.Join(nonempty(v.fullProviderPath(level, path), holder), ".")
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
