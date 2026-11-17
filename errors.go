package heiry

import (
	"fmt"
	"reflect"
)

// Code identifies a framework error independently of its message.
type Code string

const (
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
	Cause      error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Capability == nil {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: capability %s at level %d", e.Code, e.Capability, e.Level)
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
