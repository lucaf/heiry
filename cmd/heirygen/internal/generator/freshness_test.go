package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shapeApp is a three-level hierarchy in which base and mid both declare
// Name, but only base registers it. Registering it for mid as well changes
// the program's output from "base" to "mid" and nothing else: the ancestor
// keeps supplying the operation, so validation succeeds either way.
const shapeApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type Namer interface{ Name() string }

type (
	Public  interface{ Namer }
	Private interface{}
)

type base struct{ heiry.Node }

func (b *base) Name() string { return "base" }

type mid struct {
	base
	heiry.Node
}

func (m *mid) Name() string { return "mid" }

type top struct {
	mid
	heiry.Node
}

SPEC

func main() {
	t := &top{}
	if err := shapeInit(t); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(t.Node.Virtual[Namer]().Name())
}
`

const shapeSpecText = `
hierarchy shape top
level base public=Public private=Private
	node Namer
level mid
level top
`

// constCarrier holds the spec in a Go string constant, which the compiler
// sees. commentCarrier holds the same text in a comment, which it does not,
// and which the generator therefore refuses.
func constCarrier(spec string) string {
	return "//heiry:spec\nconst shapeSpec = `" + spec + "`"
}

func commentCarrier(spec string) string {
	lines := strings.Split(strings.Trim(spec, "\n"), "\n")
	return "//heiry:spec\n// " + strings.Join(lines, "\n// ")
}

func shapeModule(t *testing.T, carrier func(string) string) string {
	t.Helper()
	return tempModule(t, map[string]string{"main.go": strings.Replace(shapeApp, "SPEC", carrier(shapeSpecText), 1)})
}

// An edit to a spec constant that is not followed by generation must stop
// the build: the generated file asserts, at compile time, that the constant
// still has the text it was generated from.
func TestEditedSpecConstantFailsCompilation(t *testing.T) {
	dir := shapeModule(t, constCarrier)
	mustGen(t, dir)
	if got := mustRun(t, dir); got != "base" {
		t.Fatalf("fresh output = %q, want base", got)
	}

	edit(t, dir, "main.go", "level mid\n", "level mid\n\tnode Namer\n")
	output := buildFails(t, dir, "duplicate key false in map literal")
	if !strings.Contains(output, mainOutput) {
		t.Fatalf("the failure is not reported in the generated file:\n%s", output)
	}
	if testOutput, err := goTool(t, dir, nil, "test", "."); err == nil {
		t.Fatalf("go test passed on a stale spec:\n%s", testOutput)
	}

	mustGen(t, dir)
	if got := mustRun(t, dir); got != "mid" {
		t.Fatalf("regenerated output = %q, want mid", got)
	}
}

// Check mode compares without writing, and reports missing, different, and
// orphaned generated files.
func TestCheckMode(t *testing.T) {
	dir := shapeModule(t, constCarrier)
	output := filepath.Join(dir, mainOutput)

	if exit, _ := gen("-check", dir); exit != 1 {
		t.Fatalf("check with no generated file exited %d, want 1", exit)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("check mode wrote the generated file")
	}

	mustGen(t, dir)
	if exit, message := gen("-check", dir); exit != 0 {
		t.Fatalf("check on fresh output exited %d:\n%s", exit, message)
	}

	fresh := read(t, dir, mainOutput)
	edit(t, dir, "main.go", "level mid\n", "level mid\n\tnode Namer\n")
	exit, message := gen("-check", dir)
	if exit != 1 || !strings.Contains(message, mainOutput) {
		t.Fatalf("check on a stale spec exited %d:\n%s", exit, message)
	}
	if read(t, dir, mainOutput) != fresh {
		t.Fatal("check mode overwrote the generated file")
	}

	// A hand edit of the generated file leaves the spec constant unchanged,
	// so the compiler accepts it. Only the comparison detects it.
	mustGen(t, dir)
	edit(t, dir, mainOutput, "\tBind[Namer]((*mid)(nil), \".\").\n", "")
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "base" {
		t.Fatalf("hand-edited output = %q, want base", got)
	}
	if exit, _ := gen("-check", dir); exit != 1 {
		t.Fatalf("check on a hand-edited file exited %d, want 1", exit)
	}

	// A generated file whose source no longer holds a spec is an orphan.
	mustGen(t, dir)
	write(t, dir, outputName("gone.go"), marker("gone.go")+"\npackage main\n")
	exit, message = gen("-check", dir)
	if exit != 1 || !strings.Contains(message, outputName("gone.go")) {
		t.Fatalf("check with an orphan exited %d:\n%s", exit, message)
	}
	mustGen(t, dir)
	if _, err := os.Stat(filepath.Join(dir, outputName("gone.go"))); !os.IsNotExist(err) {
		t.Fatal("generation left an orphaned generated file")
	}
}

// Generation is a function of the spec text: repeated runs are byte-identical.
func TestDeterministicOutput(t *testing.T) {
	dir := shapeModule(t, constCarrier)
	mustGen(t, dir)
	first := read(t, dir, mainOutput)
	for range 3 {
		mustGen(t, dir)
		if read(t, dir, mainOutput) != first {
			t.Fatal("two runs produced different output")
		}
	}
}
