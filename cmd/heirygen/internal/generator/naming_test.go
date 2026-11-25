package generator

import (
	"strings"
	"testing"
)

// The application below uses, as its own names, every identifier a generated
// file would naturally pick for a parameter, a local, the package import, and
// the freshness constant.
const awkwardApp = `package main

import (
	"fmt"

	rt "github.com/lucaf/heiry"
)

type plan interface{ Name() string }

type (
	ref0  interface{ plan }
	heiry interface{}
)

type root struct{ rt.Node }

func (r *root) Name() string { return "root" }

type leaf struct {
	root
	rt.Node
}

func (l *leaf) Name() string { return "leaf > " + l.Node.Parent[plan]().Name() }

//heiry:spec
const generatedFrom = ` + "`" + `
hierarchy tree leaf
level root public=ref0 private=heiry
	node plan
level leaf
	node plan
` + "`" + `

func main() {
	l := &leaf{}
	if err := treeInit(l); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(l.Node.Virtual[plan]().Name())
	fresh := &leaf{}
	if err := treeInitPlan(tree.MustPrepare().MustPrecompute(), fresh); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(treeInit(nil) != nil)
}
`

// Generated parameters, locals, the import name, and the freshness constant
// must not capture application identifiers.
func TestGeneratedNamesDoNotCaptureApplicationIdentifiers(t *testing.T) {
	dir := tempModule(t, map[string]string{"main.go": awkwardApp})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "leaf > root\ntrue" {
		t.Fatalf("output = %q", got)
	}
	// The freshness assertion still compares against the spec constant, not
	// against a local that shadows it.
	edit(t, dir, "main.go", "level leaf\n\tnode plan\n", "level leaf\n")
	buildFails(t, dir, "duplicate key false in map literal")
}

// A generated package-level name that the application already declares is
// reported by the generator, with both names, before anything is written.
func TestGeneratedNameCollisionsAreReported(t *testing.T) {
	for _, name := range []string{"shape", "shapeRefs", "shapeInit", "shapeInitPlan"} {
		dir := shapeModule(t, constCarrier)
		write(t, dir, "other.go", "package main\n\nvar "+name+"Unrelated, "+name+" = 1, 2\n")
		exit, message := gen(dir)
		if exit != 1 || !strings.Contains(message, name) || !strings.Contains(message, "other.go") {
			t.Fatalf("%s: exit %d:\n%s", name, exit, message)
		}
	}

	// Two specs may not claim one hierarchy name.
	dir := shapeModule(t, constCarrier)
	write(t, dir, "second.go", "package main\n\n//heiry:spec\nconst secondSpec = `"+shapeSpecText+"`\n")
	exit, message := gen(dir)
	if exit != 1 || !strings.Contains(message, "shape") || !strings.Contains(message, "second.go") {
		t.Fatalf("duplicate hierarchy name: exit %d:\n%s", exit, message)
	}
}

func TestSpecErrors(t *testing.T) {
	for _, tc := range []struct{ name, spec, want string }{
		{"no hierarchy line", "level base public=Public private=Private\n", "level needs a preceding line: hierarchy"},
		{"empty spec", "// nothing\n", "a hierarchy, chain, or part line, an operations line, or both"},
		{"two hierarchies", "hierarchy a top\nhierarchy b top\n", "at most one hierarchy"},
		{"root without contracts", "hierarchy shape top\nlevel top\n", "root level names public= and private="},
		{"root with parent", "hierarchy shape top\nlevel top parent=x public=Public private=Private\n", "root level has no parent"},
		{"one contract", "hierarchy shape top\nlevel base public=Public private=Private\nlevel top public=Public\n", "given together"},
		{"unknown option", "hierarchy shape top\nlevel top public=Public private=Private final=yes\n", "unknown level option"},
		{"unknown keyword", "hierarchy shape top\nlevel top public=Public private=Private\nbind Namer\n", "unknown keyword"},
		{"leaf mismatch", "hierarchy shape top\nlevel base public=Public private=Private\n", "the last level is base but the leaf is top"},
		{"node before level", "hierarchy shape top\nnode Namer\n", "needs a level line above it"},
		{"empty node", "hierarchy shape top\nlevel top public=Public private=Private\nnode\n", "want: node"},
		{"component without operations", "hierarchy shape top\nlevel top public=Public private=Private\ncomponent part part\n", "want: component"},
		{"node path as component", "hierarchy shape top\nlevel top public=Public private=Private\ncomponent . top Namer\n", "not a component path"},
		{"blank path step", "hierarchy shape top\nlevel top public=Public private=Private\ncomponent a._ part Namer\n", "not a component path"},
		{"two provider types on one path", "hierarchy shape top\nlevel top public=Public private=Private\ncomponent a part Namer\ncomponent a other Namer\n", "provider types part and other"},
		{"not a type", "hierarchy shape top\nlevel top public=Public private=Private\nnode Namer)\n", "not a Go type expression"},
		{"undeclared package", "hierarchy shape top\nlevel top public=Public private=Private\nnode fmt.Stringer\n", `add a line: import fmt "<path>"`},
		{"unused import", "hierarchy shape top\nimport fmt \"fmt\"\nlevel top public=Public private=Private\nnode Namer\n", "import fmt is not used"},
		{"underivable parent", "hierarchy shape top\nlevel []int public=Public private=Private\nlevel top\n", "write parent=<Field>"},
	} {
		dir := tempModule(t, map[string]string{"main.go": "package main\n\n//heiry:spec\nconst shapeSpec = `\n" + tc.spec + "`\n"})
		exit, message := gen(dir)
		if exit != 1 || !strings.Contains(message, tc.want) {
			t.Errorf("%s: exit %d, want a message containing %q:\n%s", tc.name, exit, tc.want, message)
		}
	}

	// The marked constant must be one string literal.
	dir := tempModule(t, map[string]string{"main.go": "package main\n\n//heiry:spec\nconst shapeSpec = \"a\" + \"b\"\n"})
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "one string literal") {
		t.Errorf("concatenated constant: exit %d:\n%s", exit, message)
	}
}

// A name generated for one file must also stay clear of the names generated
// for the other files of the package. Here the application imports the
// library under another name, so a hierarchy may itself be called heiry, and
// two more hierarchies are named after a parameter and a local of the
// generated helpers.
func TestGeneratedNamesAcrossFiles(t *testing.T) {
	second := func(name string) string {
		return "package main\n\n//heiry:spec\nconst " + name + "Spec = `\nhierarchy " + name + " top\nlevel base public=Public private=Private\n\tnode Namer\nlevel mid\nlevel top\n`\n"
	}
	app := strings.Replace(shapeApp, "SPEC", constCarrier(shapeSpecText), 1)
	app = strings.ReplaceAll(strings.Replace(app, "\t\"github.com/lucaf/heiry\"", "\trt \"github.com/lucaf/heiry\"", 1), "heiry.Node", "rt.Node")
	dir := tempModule(t, map[string]string{"main.go": app, "second.go": second("heiry"), "third.go": second("leaf"), "fourth.go": second("ref0")})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "base" {
		t.Fatalf("output = %q", got)
	}

	// An import name chosen in a spec may not be a package-level name.
	dir = shapeModule(t, constCarrier)
	write(t, dir, "other.go", "package main\n\nvar text = 1\n")
	edit(t, dir, "main.go", "hierarchy shape top\n", "hierarchy shape top\nimport text \"fmt\"\n")
	edit(t, dir, "main.go", "\tnode Namer\n", "\tnode Namer text.Stringer\n")
	exit, message := gen(dir)
	if exit != 1 || !strings.Contains(message, "imports a package as text") || !strings.Contains(message, "other.go") {
		t.Fatalf("import name collision: exit %d:\n%s", exit, message)
	}
}
