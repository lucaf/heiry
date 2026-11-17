package heiry

import (
	"fmt"
	"reflect"
)

// Code identifies a framework error independently of its message.
type Code string

const (
	InvalidContract     Code = "InvalidContract"
	ContractExtension   Code = "ContractExtension"
	ContractCollision   Code = "ContractCollision"
	InvalidCapability   Code = "InvalidCapability"
	CapabilityIdentity  Code = "CapabilityIdentity"
	SignatureMismatch   Code = "SignatureMismatch"
	UndeclaredOperation Code = "UndeclaredOperation"
	DuplicateBinding    Code = "DuplicateBinding"
	InvalidHierarchy    Code = "InvalidHierarchy"
	InvalidProvider     Code = "InvalidProvider"
	MissingNode         Code = "MissingNode"
	ProviderOwnership   Code = "ProviderOwnership"
	BindingMismatch     Code = "BindingMismatch"
	AlreadyInitialized  Code = "AlreadyInitialized"
	// MissingMethods means a known capability has no registered implementation.
	MissingMethods Code = "MissingMethods"
	// Uninitialized means a resolution was attempted through an unset scope.
	Uninitialized Code = "Uninitialized"
	// UnknownCapability means the requested type is not in the target's contract.
	UnknownCapability Code = "UnknownCapability"
	// NoParentImplementation means no lower level implements a known capability.
	NoParentImplementation Code = "NoParentImplementation"
)

// Error is a typed framework diagnostic.
type Error struct {
	Code       Code
	Capability reflect.Type
	Level      int
	Name       string
	Category   string
	Target     reflect.Type
	Path       string
	Expected   reflect.Type
	Actual     reflect.Type
	Cause      error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s: %s %s capability %v at level %d (expected %v, actual %v)", e.Code, e.Category, e.Name, e.Capability, e.Level, e.Expected, e.Actual)
}

// Is matches errors by stable code, including a code-only target.
func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && e != nil && other != nil && e.Code == other.Code
}

// Unwrap exposes an underlying error when present.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
