package generator

import (
	"strings"
	"testing"
)

// These probes demonstrate, on handwritten stand-ins, the compile-time
// mechanisms that generated operation interfaces and wrappers would rely on.
// No generator is involved: guards.go below is written by hand.

const mechanismContracts = `package main

type Base interface {
	Read(key string) (string, bool)
}

type Public interface {
	Base
	Audit() string
}

type Private interface {
	Seal(reason string) error
}
`

const mechanismOps = `package main

type (
	ReadOp  interface{ Read(key string) (string, bool) }
	AuditOp interface{ Audit() string }
	SealOp  interface{ Seal(reason string) error }
)

type node struct{}

func (n *node) Read(key string) (string, bool) { return key, true }
func (n *node) Audit() string                  { return "audit" }

type view struct{ state *node }

func (v *view) Read(key string) (string, bool) { return v.state.Read(key) }
func (v *view) Audit() string                  { return v.state.Audit() }

func main() {}
`

// satisfaction is the assertion handwritten wrappers use today. It proves
// that the wrapper has every public method and nothing about other methods.
const mechanismSatisfaction = `package main

var _ Public = (*view)(nil)
`

// equality asserts, in both directions, that each contract has exactly the
// methods of its operation interfaces, with identical signatures.
const mechanismEquality = `package main

func _() {
	var _ interface {
		ReadOp
		AuditOp
	} = Public(nil)
	var _ Public = interface {
		ReadOp
		AuditOp
	}(nil)
	var _ interface{ SealOp } = Private(nil)
	var _ Private = interface{ SealOp }(nil)
}
`

// absence asserts that the wrapper declares no method named after a private
// operation: if it did, the selector below would be ambiguous between the
// wrapper's method and the interface's, both one embedding deep.
const mechanismAbsence = `package main

func _() {
	type private interface{ Seal() }
	type probe struct {
		*view
		private
	}
	_ = probe.Seal
}
`

func mechanismModule(t *testing.T, guards ...string) string {
	t.Helper()
	files := map[string]string{"contracts.go": mechanismContracts, "ops.go": mechanismOps}
	for index, guard := range guards {
		files["guard"+string(rune('a'+index))+".go"] = guard
	}
	dir := tempModule(t, files)
	mustBuild(t, dir)
	return dir
}

// Two interface assignments, one in each direction, reject every change of a
// contract's method set: a moved, removed, or added method, a changed
// signature, and a change inside an embedded contract.
func TestContractEqualityAssertion(t *testing.T) {
	for _, tc := range []struct {
		name, old, new, want string
	}{
		{
			name: "move a public operation into the private contract",
			old:  "\tBase\n\tAudit() string\n}\n\ntype Private interface {\n",
			new:  "\tBase\n}\n\ntype Private interface {\n\tAudit() string\n",
			want: "(missing method Audit)",
		},
		{
			name: "remove a public operation",
			old:  "\tBase\n\tAudit() string\n", new: "\tBase\n",
			want: "(missing method Audit)",
		},
		{
			name: "add a private operation",
			old:  "\tSeal(reason string) error\n", new: "\tSeal(reason string) error\n\tWipe()\n",
			want: "does not implement Private (missing method Wipe)",
		},
		{
			name: "change an operation's signature",
			old:  "\tSeal(reason string) error\n", new: "\tSeal(reason string, force bool) error\n",
			want: "(wrong type for method Seal)",
		},
		{
			name: "change an embedded contract",
			old:  "\tRead(key string) (string, bool)\n", new: "\tRead(key string) (string, bool)\n\tScan() []string\n",
			want: "does not implement Public (missing method Scan)",
		},
		{
			name: "replace an embedded contract",
			old:  "\tBase\n\tAudit() string\n", new: "\tOther\n\tAudit() string\n",
			want: "(missing method Read)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := mechanismModule(t, mechanismEquality)
			write(t, dir, "other.go", "package main\n\ntype Other interface{ Fetch(key string) (string, bool) }\n")
			mustBuild(t, dir)
			edit(t, dir, "contracts.go", tc.old, tc.new)
			output := buildFails(t, dir, tc.want)
			if !strings.Contains(output, "guarda.go") {
				t.Fatalf("the failure is not reported in the guard:\n%s", output)
			}
		})
	}

	// A change that leaves the method set alone still compiles: parameter
	// names and method order are not part of it.
	dir := mechanismModule(t, mechanismEquality)
	edit(t, dir, "contracts.go", "\tSeal(reason string) error\n", "\tSeal(why string) error\n")
	mustBuild(t, dir)
}

const privateMethod = "package main\n\nfunc (v *view) Seal(reason string) error { return nil }\n"

// The satisfaction assertion alone accepts a wrapper that exposes a private
// operation. It is not a check of the wrapper's whole method set.
func TestSatisfactionAssertionAcceptsExtraMethods(t *testing.T) {
	dir := mechanismModule(t, mechanismSatisfaction)
	write(t, dir, "extra.go", privateMethod)
	mustBuild(t, dir)
}

// The absence assertion rejects a method on the wrapper that is named after
// a private operation, whatever its receiver and signature, and accepts
// other additional methods.
func TestPrivateNameAbsenceAssertion(t *testing.T) {
	for name, method := range map[string]string{
		"same signature, pointer receiver": privateMethod,
		"same signature, value receiver":   "package main\n\nfunc (v view) Seal(reason string) error { return nil }\n",
		"another signature":                "package main\n\nfunc (v *view) Seal() {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := mechanismModule(t, mechanismSatisfaction, mechanismAbsence)
			write(t, dir, "extra.go", method)
			output := buildFails(t, dir, "ambiguous selector probe.Seal")
			if !strings.Contains(output, "guardb.go") {
				t.Fatalf("the failure is not reported in the guard:\n%s", output)
			}
		})
	}

	t.Run("an additional method with another name is accepted", func(t *testing.T) {
		dir := mechanismModule(t, mechanismSatisfaction, mechanismAbsence)
		write(t, dir, "extra.go", "package main\n\nfunc (v *view) Count() int { return 0 }\n")
		mustBuild(t, dir)
	})

	// The assertion sees names, not behavior. A method with another name can
	// still call a private operation's implementation; nothing a compiler
	// checks can tell that from any other method.
	t.Run("a private operation behind another name is accepted", func(t *testing.T) {
		dir := mechanismModule(t, mechanismSatisfaction, mechanismAbsence)
		write(t, dir, "extra.go", "package main\n\nfunc (n *node) Seal(reason string) error { return nil }\n\nfunc (v *view) Close(reason string) error { return v.state.Seal(reason) }\n")
		mustBuild(t, dir)
	})

	// The assertion compares methods one embedding deep. A wrapper type that
	// itself embeds a type with the private method promotes it from deeper,
	// and the assertion does not see it. A generated wrapper struct has only
	// its one named field, so it cannot promote anything.
	t.Run("a promoted method is not seen", func(t *testing.T) {
		dir := mechanismModule(t, mechanismSatisfaction, mechanismAbsence)
		edit(t, dir, "ops.go", "type view struct{ state *node }", "type sealer struct{}\n\nfunc (sealer) Seal(reason string) error { return nil }\n\ntype view struct {\n\tstate *node\n\tsealer\n}")
		mustBuild(t, dir)
	})
}
