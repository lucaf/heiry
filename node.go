package heiry

import "reflect"

// Node is the scope holder embedded directly in each inheritance-level
// receiver. An initialized receiver must not be copied.
type Node struct {
	noCopy noCopy
	scope  Scope
}

// Scope returns this receiver's fixed dispatch position.
func (n *Node) Scope() Scope { return n.scope }

// Virtual returns the most-derived provider of T in the object this holder
// belongs to. It is equivalent to Method[T](n.Scope()). A nil holder panics
// with a typed Uninitialized error.
func (n *Node) Virtual[T any]() T {
	if n == nil {
		panic(&Error{Code: Uninitialized, Capability: reflect.TypeFor[T]()})
	}
	return Method[T](n.scope)
}

// Parent returns the closest provider of T below this holder's level. It is
// equivalent to ParentMethod[T](n.Scope()). A nil holder panics with a typed
// Uninitialized error.
func (n *Node) Parent[T any]() T {
	if n == nil {
		panic(&Error{Code: Uninitialized, Capability: reflect.TypeFor[T]()})
	}
	return ParentMethod[T](n.scope)
}

// noCopy makes ordinary value copies of initialized nodes visible to go vet.
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
