package heiry

import (
	"errors"
	"go/token"
	"reflect"
	"strings"
	"sync"
)

// Definition describes an ordered hierarchy without holding an application
// object. It is mutable during declaration and does not represent a Plan.
type Definition[Leaf any] struct {
	mu       sync.Mutex
	levels   []*declaredLevel
	recorded []error
	plan     *Plan[Leaf]
	lineage  *definitionLineage
}

type declaredLevel struct {
	// A nil public and private pair means the level inherits the preceding
	// level's effective contracts.
	typ, public, private reflect.Type
	parentField          string
	// inferParent asks validation to find the single direct anonymous value
	// field of the preceding level's node type instead of using parentField.
	inferParent bool
	bindings    []declaredBinding
	index       int
}

type declaredBinding struct {
	contractBinding
	providerPath string
}

// The causes of the sticky InvalidDeclaration errors: a declaration call
// after preparation has frozen the definition, an extension with no level
// before it, and a group with no operation.
var (
	errPrepared     = errors.New("the Definition was already prepared and no longer accepts changes")
	errNoLevel      = errors.New("the Definition has no level to extend: declare the root with Level")
	errNoOperations = errors.New("the group names no operation: pass at least one Of token")
)

// OperationToken identifies one explicitly selected capability without a receiver.
type OperationToken struct{ capability reflect.Type }

// Of creates a receiver-free capability token for grouped registration.
func Of[T any]() OperationToken { return OperationToken{capability: reflect.TypeFor[T]()} }

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
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.plan != nil {
		d.recorded = append(d.recorded, &Error{Code: InvalidDeclaration, Target: reflect.TypeFor[Leaf](), Level: len(d.levels), Path: parentField, Cause: errPrepared})
		return &LevelHandle[Leaf]{definition: d, rejected: true}
	}
	level := &declaredLevel{
		typ: reflect.TypeFor[Node](), public: reflect.TypeFor[Public](),
		private: reflect.TypeFor[Private](), parentField: parentField, index: len(d.levels),
	}
	d.levels = append(d.levels, level)
	return &LevelHandle[Leaf]{definition: d, level: level}
}

// Extend appends a level that inherits the preceding level's effective
// public and private contracts. Its parent is the single direct anonymous
// value field whose type is the preceding level's node type.
func (d *Definition[Leaf]) Extend[Node any]() *LevelHandle[Leaf] {
	return d.extend(reflect.TypeFor[Node](), nil, nil)
}

// ExtendWith appends a level with the given contracts. Its parent is inferred
// as for Extend.
func (d *Definition[Leaf]) ExtendWith[Node, Public, Private any]() *LevelHandle[Leaf] {
	return d.extend(reflect.TypeFor[Node](), reflect.TypeFor[Public](), reflect.TypeFor[Private]())
}

// extend requires a preceding level. Without one, or after preparation, it
// records a sticky diagnostic, adds nothing, and returns an inert handle.
func (d *Definition[Leaf]) extend(node, public, private reflect.Type) *LevelHandle[Leaf] {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.plan != nil || len(d.levels) == 0 {
		refused := &Error{Code: InvalidDeclaration, Target: reflect.TypeFor[Leaf](), Level: len(d.levels), Actual: node, Cause: errNoLevel}
		if d.plan != nil {
			refused.Cause = errPrepared
		}
		d.recorded = append(d.recorded, refused)
		return &LevelHandle[Leaf]{definition: d, rejected: true}
	}
	level := &declaredLevel{typ: node, public: public, private: private, inferParent: true, index: len(d.levels)}
	d.levels = append(d.levels, level)
	return &LevelHandle[Leaf]{definition: d, level: level}
}

// End returns the definition while preserving this handle's level identity.
func (l *LevelHandle[Leaf]) End() *Definition[Leaf] { return l.definition }

// The methods below forward to the handle's definition, so a declaration
// chain can continue from a level handle and End is optional. A level added
// through any handle is appended to the end of the definition's chain.

// Level appends a level to the handle's definition. See Definition.Level.
func (l *LevelHandle[Leaf]) Level[Node, Public, Private any](parentField string) *LevelHandle[Leaf] {
	return l.definition.Level[Node, Public, Private](parentField)
}

// Extend appends a level to the handle's definition. See Definition.Extend.
func (l *LevelHandle[Leaf]) Extend[Node any]() *LevelHandle[Leaf] {
	return l.definition.Extend[Node]()
}

// ExtendWith appends a level to the handle's definition. See Definition.ExtendWith.
func (l *LevelHandle[Leaf]) ExtendWith[Node, Public, Private any]() *LevelHandle[Leaf] {
	return l.definition.ExtendWith[Node, Public, Private]()
}

// Validate checks the handle's definition. See Definition.Validate.
func (l *LevelHandle[Leaf]) Validate() error { return l.definition.Validate() }

// Prepare prepares the handle's definition. See Definition.Prepare.
func (l *LevelHandle[Leaf]) Prepare() (*Plan[Leaf], error) { return l.definition.Prepare() }

// MustPrepare panics with the error returned by Prepare.
func (l *LevelHandle[Leaf]) MustPrepare() *Plan[Leaf] { return l.definition.MustPrepare() }

// Init binds one leaf through the handle's definition. See Definition.Init.
func (l *LevelHandle[Leaf]) Init(leaf *Leaf, refs ...ProviderRef[Leaf]) error {
	return l.definition.Init(leaf, refs...)
}

// MustInit panics with the error returned by Init and otherwise returns leaf.
func (l *LevelHandle[Leaf]) MustInit(leaf *Leaf, refs ...ProviderRef[Leaf]) *Leaf {
	return l.definition.MustInit(leaf, refs...)
}

// NodeProvider creates a node reference for the handle's definition. See
// Definition.NodeProvider.
func (l *LevelHandle[Leaf]) NodeProvider[Node any](ptr *Node) ProviderRef[Leaf] {
	return l.definition.NodeProvider(ptr)
}

// ComponentProvider creates a component reference for the handle's
// definition. See Definition.ComponentProvider.
func (l *LevelHandle[Leaf]) ComponentProvider[Node, Provider any](providerPath string, ptr *Provider) ProviderRef[Leaf] {
	return l.definition.ComponentProvider[Node](providerPath, ptr)
}

// Bind records an explicit typed operation for this level. The witness value
// is reduced to type metadata and is never retained.
func (l *LevelHandle[Leaf]) Bind[Operation any](provider Operation, providerPath string) *LevelHandle[Leaf] {
	if l.rejected {
		return l
	}
	l.definition.mu.Lock()
	defer l.definition.mu.Unlock()
	if l.definition.plan != nil {
		l.definition.recorded = append(l.definition.recorded, &Error{Code: InvalidDeclaration, Target: reflect.TypeFor[Leaf](), Level: l.level.index, Path: providerPath, Capability: reflect.TypeFor[Operation](), Cause: errPrepared})
		return l
	}
	capability := reflect.TypeFor[Operation]()
	if !validCapability(capability) {
		l.definition.recorded = append(l.definition.recorded, &Error{
			Code: InvalidCapability, Target: reflect.TypeFor[Leaf](), Level: l.level.index,
			Capability: capability, Path: providerPath,
		})
		return l
	}
	value := reflect.ValueOf(provider)
	if !value.IsValid() || value.Kind() != reflect.Pointer || !value.IsNil() || !validProviderPath(providerPath) {
		var expected reflect.Type
		if providerPath == "." {
			expected = reflect.PointerTo(l.level.typ)
		}
		l.definition.recorded = append(l.definition.recorded, &Error{
			Code: InvalidProvider, Target: reflect.TypeFor[Leaf](), Level: l.level.index,
			Capability: capability, Path: providerPath, Expected: expected, Actual: reflect.TypeOf(provider),
		})
		return l
	}
	l.level.bindings = append(l.level.bindings, declaredBinding{
		contractBinding: contractBinding{capability: capability, provider: reflect.TypeOf(provider)},
		providerPath:    providerPath,
	})
	return l
}

// BindGroup records several explicit operations for one typed-nil provider.
// It stores only type metadata and never retains the caller's token slice.
func (l *LevelHandle[Leaf]) BindGroup[Provider any](provider *Provider, providerPath string, operations ...OperationToken) *LevelHandle[Leaf] {
	if l.rejected {
		return l
	}
	l.definition.mu.Lock()
	defer l.definition.mu.Unlock()
	target := reflect.TypeFor[Leaf]()
	level := l.level.index
	if l.definition.plan != nil {
		l.definition.recorded = append(l.definition.recorded, &Error{Code: InvalidDeclaration, Target: target, Level: level, Path: providerPath, Cause: errPrepared})
		return l
	}
	var invalid []error
	if len(operations) == 0 {
		invalid = append(invalid, &Error{Code: InvalidDeclaration, Target: target, Level: level, Path: providerPath, Cause: errNoOperations})
	}
	providerType := reflect.TypeOf(provider)
	if provider != nil || !validProviderPath(providerPath) {
		var expected reflect.Type
		if providerPath == "." {
			expected = reflect.PointerTo(l.level.typ)
		}
		invalid = append(invalid, &Error{Code: InvalidProvider, Target: target, Level: level, Path: providerPath, Expected: expected, Actual: providerType})
	}
	for _, operation := range operations {
		capability := operation.capability
		if !validCapability(capability) {
			invalid = append(invalid, &Error{Code: InvalidCapability, Target: target, Level: level, Path: providerPath, Capability: capability})
		}
	}
	if len(invalid) != 0 {
		l.definition.recorded = append(l.definition.recorded, invalid...)
		return l
	}
	for _, operation := range operations {
		l.level.bindings = append(l.level.bindings, declaredBinding{
			contractBinding: contractBinding{capability: operation.capability, provider: providerType},
			providerPath:    providerPath,
		})
	}
	return l
}

// BindNode records an explicit typed operation supplied by this level's own
// node. It is Bind with the provider path ".".
func (l *LevelHandle[Leaf]) BindNode[Operation any](witness Operation) *LevelHandle[Leaf] {
	return l.Bind[Operation](witness, ".")
}

// BindNodeGroup records several explicit operations supplied by this level's
// own node. It is BindGroup with the provider path ".".
func (l *LevelHandle[Leaf]) BindNodeGroup[Provider any](witness *Provider, operations ...OperationToken) *LevelHandle[Leaf] {
	return l.BindGroup(witness, ".", operations...)
}

func validCapability(capability reflect.Type) bool {
	return capability != nil && capability.Kind() == reflect.Interface && capability.NumMethod() == 1 && capability.Method(0).PkgPath == ""
}

func validProviderPath(path string) bool {
	if path == "." {
		return true
	}
	for _, part := range strings.Split(path, ".") {
		if part == "_" || !token.IsIdentifier(part) {
			return false
		}
	}
	return true
}

// Validate checks the current declaration without freezing or preparing it.
func (d *Definition[Leaf]) Validate() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	validated, err := d.validatedSnapshot()
	return d.withRecorded(err, validated)
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
		levels[index] = nodeLevel{typ: level.typ, parentField: level.parentField, inferParent: level.inferParent, contracts: contractLevel{public: level.public, private: level.private}}
		for _, binding := range level.bindings {
			entry := binding.contractBinding
			entry.providerPath = binding.providerPath
			levels[index].contracts.bindings = append(levels[index].contracts.bindings, entry)
		}
	}
	validated, err := validateNodeDefinition(levels)
	localErrors = append(localErrors, err)
	if joined := errors.Join(localErrors...); joined != nil {
		return validated, joined
	}
	return validated, nil
}

func (d *Definition[Leaf]) withRecorded(validation error, validated *validatedNodes) error {
	if len(d.recorded) == 0 {
		return validation
	}
	all := make([]error, 0, len(d.recorded)+1)
	for _, recorded := range d.recorded {
		if diagnostic, ok := recorded.(*Error); ok && validated != nil &&
			(diagnostic.Code == InvalidProvider || diagnostic.Code == InvalidCapability || diagnostic.Code == InvalidDeclaration) &&
			diagnostic.Level >= 0 && diagnostic.Level < len(validated.levels) && diagnostic.Path != "" {
			copy := *diagnostic
			copy.Path = validated.fullProviderPath(copy.Level, copy.Path)
			if copy.Path == "" && diagnostic.Path == "." {
				copy.Path = "."
			}
			all = append(all, &copy)
		} else {
			all = append(all, recorded)
		}
	}
	if validation != nil {
		all = append(all, validation)
	}
	return errors.Join(all...)
}
