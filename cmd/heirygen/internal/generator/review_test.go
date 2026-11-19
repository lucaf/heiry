package generator

import (
	"strings"
	"testing"
)

// These are regressions for the owner's review of 2026-11-19 and for the
// re-audit of the guards that followed it.

// Finding 1. A wrapper depends on both of the leaf's contracts: it forwards
// the public one and must not carry the names of the private one. Each must
// therefore be listed by an operations line, because only listed contracts
// get the equality assertion that makes a later change a compile error.
func TestWrapperNeedsBothContractsListed(t *testing.T) {
	// Only the public contract is listed; the private operation keeps a
	// handwritten interface.
	app := strings.Replace(fullApp, "operations Public Private suffix=Op\n", "operations Public suffix=Op\n", 1)
	dir := tempModule(t, map[string]string{
		"main.go": app, "probe.go": fullProbe,
		"sealop.go": "package main\n\ntype SealOp interface{ Seal(reason string) error }\n",
	})
	exit, message := gen(dir)
	if exit != 1 || !strings.Contains(message, "wrapper view depends on Private, which no operations line of this package lists") {
		t.Fatalf("exit %d:\n%s", exit, message)
	}

	// Listed, with the handwritten interface kept as the operation's
	// identity, the private contract is guarded: a private operation added
	// later does not compile, and neither does a wrapper method of its name.
	edit(t, dir, "main.go", "operations Public suffix=Op\n", "operations Public Private suffix=Op\n\texisting Seal SealOp\n")
	mustGen(t, dir)
	mustBuild(t, dir)
	edit(t, dir, "main.go", "\tSeal(reason string) error\n}", "\tSeal(reason string) error\n\tWipe()\n}")
	write(t, dir, "extra.go", "package main\n\nfunc (v *view) Wipe() {}\n")
	buildFails(t, dir, "does not implement Private (missing method Wipe)")
}

// Finding 2. The freshness assertion must not depend on the handwritten
// declaration staying a constant, nor on the predeclared meaning of an
// identifier that handwritten code can redeclare.
func TestFreshnessAssertionCannotBeDisarmed(t *testing.T) {
	t.Run("the spec constant becomes a variable", func(t *testing.T) {
		dir := shapeModule(t, constCarrier)
		mustGen(t, dir)
		mustBuild(t, dir)
		edit(t, dir, "main.go", "const shapeSpec = `", "var shapeSpec = `")
		// Unedited text, and already not a constant: the build stops.
		buildFails(t, dir, "is not constant")
		edit(t, dir, "main.go", "level mid\n", "level mid\n\tnode Namer\n")
		output := buildFails(t, dir, "is not constant")
		if !strings.Contains(output, mainOutput) {
			t.Fatalf("the failure is not reported in the generated file:\n%s", output)
		}
		// The generator refuses it as well.
		if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "marks a specification only in the doc comment of a string constant") {
			t.Fatalf("exit %d:\n%s", exit, message)
		}
	})

	t.Run("the package redeclares false", func(t *testing.T) {
		dir := shapeModule(t, constCarrier)
		mustGen(t, dir)
		write(t, dir, "other.go", "package main\n\nconst false = 1 == 1\n")
		// Fresh output still compiles.
		mustBuild(t, dir)
		edit(t, dir, "main.go", "level mid\n", "level mid\n\tnode Namer\n")
		output := buildFails(t, dir, "duplicate key")
		reportedIn(t, output, mainOutput, "duplicate key")
		// Regeneration works for such a package, and the result is fresh.
		mustGen(t, dir)
		if got := mustRun(t, dir); got != "mid" {
			t.Fatalf("regenerated output = %q", got)
		}
	})

	// The other guards use nil. Redeclaring it cannot disarm them: either
	// the generated file stops compiling, or the assertions still compare
	// the same types. Here it stops compiling.
	t.Run("the package redeclares nil", func(t *testing.T) {
		dir := fullModule(t)
		write(t, dir, "other.go", "package main\n\nvar nil Public\n")
		output := buildFails(t, dir, "cannot convert nil")
		if !strings.Contains(output, mainOutput) {
			t.Fatalf("the failure is not reported in the generated file:\n%s", output)
		}
	})
}

// spellingApp writes one method several times with the same Go signature
// and different spelling: other parameter names, and other names for the
// same type. Overlapping embedded methods and a later contract that restates
// an earlier method are both valid Go.
const spellingApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type text = string

type (
	Reader  interface{ Read(x string) string }
	Scanner interface {
		Read(y text) string
	}
	Public interface {
		Reader
		Scanner
		Tag(
			label string,
			values ...any,
		) (out string)
	}
	Private     interface{ Note(text string, extra any) }
	LeafPrivate interface {
		Note(t string, more interface{})
		Limit() (w int)
	}
)

type thing struct {
	heiry.Node
	notes int
}

func (t *thing) Read(x string) string { return "read " + x }
func (t *thing) Tag(label string, values ...any) string {
	t.Node.Virtual[NoteOp]().Note(label, len(values))
	return fmt.Sprint(label, values, t.notes, t.Node.Virtual[LimitOp]().Limit())
}
func (t *thing) Note(string, any) { t.notes++ }
func (t *thing) Limit() int       { return 7 }

//heiry:spec
const thingSpec = ` + "`" + `
operations Public Private LeafPrivate suffix=Op
hierarchy things thing
level thing public=Public private=LeafPrivate
	node ReadOp TagOp NoteOp LimitOp
wrapper view
` + "`" + `

func main() {
	t := &thing{}
	if err := thingsInit(t); err != nil {
		fmt.Println("error:", err)
		return
	}
	var public Public = &view{state: t}
	fmt.Println(public.Read("a"), public.Tag("b", 1, 2))
}
`

// Finding 3, first part. The generator must not reject valid contracts
// because two spellings of one signature differ. It compares only what
// cannot be equal, the number of parameters and results; the compiler
// decides everything else through the equality assertion.
func TestSignaturesThatDifferOnlyInSpelling(t *testing.T) {
	dir := tempModule(t, map[string]string{"main.go": spellingApp})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "read a b[1 2] 1 7" {
		t.Fatalf("output = %q", got)
	}

	// Really different types with the same number of parameters are left to
	// the compiler, which rejects the contract itself or the assertion.
	dir = tempModule(t, map[string]string{"main.go": strings.Replace(spellingApp, "Note(t string, more interface{})", "Note(t string, more int)", 1)})
	mustGen(t, dir)
	buildFails(t, dir, "(wrong type for method Note)")
}

// Finding 3, second part. A forwarder's receiver, parameters, and body must
// not collide with a contract's named results, whatever they are called.
func TestForwardersWithNamedResults(t *testing.T) {
	app := strings.Replace(forwardApp, "\tReset()\n}", "\tReset()\n\tCount() (w int)\n\tLast() (opLast string, heiry error)\n}", 1)
	app = strings.Replace(app, "func (s *sink) Reset()", "func (s *sink) Count() int             { return s.resets }\nfunc (s *sink) Last() (string, error) { return \"last\", nil }\nfunc (s *sink) Reset()", 1)
	app = strings.Replace(app, "\tnode opWrite opLookup opPick opReset\n", "\tnode opWrite opLookup opPick opReset opCount opLast\n", 1)
	app = strings.Replace(app, "\tfmt.Println(s.resets)\n", "\tfmt.Println(s.resets)\n\tfmt.Println(public.Count())\n\tfmt.Println(public.Last())\n", 1)
	dir := tempModule(t, map[string]string{"main.go": app})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "a+b+c\n6 <nil>\nxxx true\np7\n1\n1\nlast <nil>" {
		t.Fatalf("output = %q", got)
	}
}

// A signature is copied into the generated file, which does not have the
// contract file's dot imports, so a name a dot import provides would be
// undefined there. The generator reports it instead of writing code that
// cannot compile. Names the package declares, or that are predeclared, are
// fine in a file with a dot import.
func TestSignatureUsingADotImport(t *testing.T) {
	app := strings.Replace(fullApp, "import (\n\t\"fmt\"\n", "import (\n\t\"fmt\"\n\t. \"strings\"\n", 1)
	dot := strings.Replace(app, "\tAudit() string\n}", "\tAudit() string\n\tRender(b *Builder)\n}", 1)
	dot = strings.Replace(dot, "\tnode ReadOp AuditOp SealOp\n", "\tnode ReadOp AuditOp SealOp RenderOp\n", 1)
	dir := sourceDir(t, map[string]string{"main.go": dot, "probe.go": fullProbe})
	exit, message := gen(dir)
	if exit != 1 || !strings.Contains(message, "Render uses Builder, which main.go may get from its dot import") {
		t.Fatalf("exit %d:\n%s", exit, message)
	}

	// The same file with only local and predeclared names in its signatures.
	local := strings.Replace(app, "var _ = Join", "", 1) + "\nvar _ = Join\n"
	dir = tempModule(t, map[string]string{"main.go": local, "probe.go": fullProbe})
	mustGen(t, dir)
	mustBuild(t, dir)
}
