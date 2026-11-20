package heiry

import (
	"reflect"
	"strconv"
	"strings"
)

// Code identifies a framework error independently of its message.
type Code string

const (
	InvalidDeclaration  Code = "InvalidDeclaration"
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
	InvalidPlan         Code = "InvalidPlan"
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

// Error returns one line: the code, then the context fields that are set.
// The text is not API; match errors by code and fields.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	var parts []string
	if e.Target != nil {
		parts = append(parts, "target "+e.Target.String())
	}
	if e.levelKnown() {
		level := "level " + strconv.Itoa(e.Level)
		if e.Level == -1 {
			level += " (unknown owner)"
		}
		parts = append(parts, level)
	}
	if e.Path != "" {
		parts = append(parts, "path "+strconv.Quote(e.Path))
	}
	// Category, name, and capability read as one phrase, such as
	// "public Read capability heiry.Reader".
	subject := nonempty(e.Category, e.Name)
	switch {
	case e.Name != "" || e.Capability != nil:
		subject = append(subject, "capability")
		if e.Capability != nil {
			subject = append(subject, e.Capability.String())
		}
	case e.Category != "":
		subject = append(subject, "contract")
	}
	if len(subject) != 0 {
		parts = append(parts, strings.Join(subject, " "))
	}
	if e.Expected != nil {
		parts = append(parts, "expected "+e.Expected.String())
	}
	if e.Actual != nil {
		parts = append(parts, "actual "+e.Actual.String())
	}
	if e.Cause != nil {
		// A joined cause has one line per error.
		lines := strings.FieldsFunc(e.Cause.Error(), func(r rune) bool { return r == '\n' || r == '\r' })
		parts = append(parts, "cause: "+strings.Join(lines, "; "))
	}
	if len(parts) == 0 {
		return string(e.Code)
	}
	return string(e.Code) + ": " + strings.Join(parts, ", ")
}

// levelKnown reports whether Level locates the error. An unset scope has
// no level. Level 0 is both the root level and the zero value, so it counts
// only when the error names something within the hierarchy, or carries a
// target and a code that always concerns one level; -1 is an owner outside
// the hierarchy.
func (e *Error) levelKnown() bool {
	switch {
	case e.Code == Uninitialized:
		return false
	case e.Level != 0:
		return true
	case e.Path != "" || e.Category != "" || e.Name != "" || e.Capability != nil || e.Expected != nil || e.Actual != nil:
		return true
	}
	switch e.Code {
	case InvalidDeclaration, InvalidHierarchy, InvalidProvider, BindingMismatch, InvalidPlan:
		// These codes also describe a whole target, with no level set.
		return false
	}
	return e.Target != nil
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
