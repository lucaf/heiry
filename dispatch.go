// Package heiry provides typed virtual and parent method resolution for
// explicitly registered inheritance levels.
package heiry

import "reflect"

// Scope identifies one provider's fixed inheritance level in an object.
// The complete-object dispatch reference is installed during initialization.
type Scope struct {
	dispatch *dispatch
	level    int
}

// dispatch holds one object's receivers and the prepared capability set.
type dispatch struct {
	known  map[reflect.Type]struct{}
	levels []map[reflect.Type]any
}

// nodeBinding is an internal stage-3 input for one node provider at a level.
// Fixtures supply pointers to the leaf's actual embedded subobjects. Later
// validation will establish those identities from declared paths.
type nodeBinding struct {
	node         *Node
	provider     any
	capabilities []reflect.Type
}

// bindNodeLevels creates one object's provider table and installs a fixed
// scope in each embedded node. It is used only after the fixture has selected
// its real subobjects; public initialization and ownership checks come later.
func bindNodeLevels(bindings []nodeBinding) *dispatch {
	d := &dispatch{
		known:  make(map[reflect.Type]struct{}),
		levels: make([]map[reflect.Type]any, len(bindings)),
	}
	for level, binding := range bindings {
		methods := make(map[reflect.Type]any, len(binding.capabilities))
		for _, capability := range binding.capabilities {
			d.known[capability] = struct{}{}
			methods[capability] = binding.provider
		}
		d.levels[level] = methods
	}
	for level, binding := range bindings {
		binding.node.scope = Scope{dispatch: d, level: level}
	}
	return d
}

// Method returns the most-derived provider of T in the complete object.
func Method[T any](scope Scope) T {
	return resolve(scope, reflect.TypeFor[T](), false).(T)
}

// ParentMethod returns the closest provider of T below the caller's level.
func ParentMethod[T any](scope Scope) T {
	return resolve(scope, reflect.TypeFor[T](), true).(T)
}

func resolve(scope Scope, capability reflect.Type, parent bool) any {
	if scope.dispatch == nil {
		panic(&Error{Code: Uninitialized, Capability: capability})
	}
	d := scope.dispatch
	if _, ok := d.known[capability]; !ok {
		panic(&Error{Code: UnknownCapability, Capability: capability, Level: scope.level})
	}

	start := len(d.levels) - 1
	if parent {
		start = scope.level - 1
	}
	for level := start; level >= 0; level-- {
		if provider, ok := d.levels[level][capability]; ok {
			return provider
		}
	}
	code := MissingMethods
	if parent {
		code = NoParentImplementation
	}
	panic(&Error{Code: code, Capability: capability, Level: scope.level})
}
