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

// dispatch holds the capabilities and actual providers of one object.
// Later lifecycle work constructs it from a validated hierarchy definition.
type dispatch struct {
	known  map[reflect.Type]struct{}
	levels []map[reflect.Type]any
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
