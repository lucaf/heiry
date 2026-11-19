package generator

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sharedHierarchy registers fmt.Stringer, which an operations line of this
// file names as an existing interface when OPERATIONS is replaced by one.
const sharedHierarchy = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Public  interface{ String() string }
	Private interface{}
)

type node struct{ heiry.Node }

func (n *node) String() string { return "node" }

//heiry:spec
const recordsSpec = ` + "`" + `
import fmt "fmt"
OPERATIONS
hierarchy records node
level node public=Public private=Private
	node fmt.Stringer
` + "`" + `

func main() {
	n := &node{}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(n.Node.Virtual[fmt.Stringer]().String())
}
`

// labeledOps and titledOps name fmt.Stringer for the same method with the
// same valid signature, spelled differently: another import name for fmt and
// a named result.
const labeledOps = "package main\n\ntype Labeled interface{ String() string }\n\n//heiry:spec\nconst labeledSpec = `\nimport fmt \"fmt\"\noperations Labeled\n\texisting String fmt.Stringer\n`\n"

const titledOps = "package main\n\ntype Titled interface{ String() (text string) }\n\n//heiry:spec\nconst titledSpec = `\nimport std \"fmt\"\noperations Titled\n\texisting String std.Stringer\n`\n"

const sharedRepeat = "Each existing interface the registrations or forwarders of records use"

// outputsOf returns the generated files of dir.
func outputsOf(t *testing.T, dir string) map[string]string {
	t.Helper()
	outputs := make(map[string]string)
	for name, content := range snapshot(t, dir) {
		if strings.Contains(name, ".heirygen") {
			outputs[name] = content
		}
	}
	return outputs
}

// generateRepeatedly runs generation and check mode many times. Every run
// must produce the same bytes and leave check mode clean, whatever order the
// generator's maps are walked in.
func generateRepeatedly(t *testing.T, dir string) map[string]string {
	t.Helper()
	mustGen(t, dir)
	first := outputsOf(t, dir)
	for run := 2; run <= 40; run++ {
		if exit, message := gen("-check", dir); exit != 0 {
			t.Fatalf("check before run %d: exit %d:\n%s", run, exit, message)
		}
		mustGen(t, dir)
		if got := outputsOf(t, dir); !maps.Equal(got, first) {
			for name := range first {
				if got[name] != first[name] {
					t.Fatalf("run %d changed %s:\n--- first\n%s\n--- now\n%s", run, name, first[name], got[name])
				}
			}
			t.Fatalf("run %d changed the set of outputs", run)
		}
	}
	return first
}

// Two operations lines in different files may name the same existing
// interface. The guard the hierarchy needs is then in the output of the
// hierarchy's own file when that file's operations line names the interface,
// and otherwise repeated with one representative declaration, chosen in file
// and line order. Either way the outputs do not depend on map order.
func TestSharedExistingInterfaceIsDeterministic(t *testing.T) {
	sameFile := map[string]string{
		"labels.go": labeledOps,
		"main.go":   strings.Replace(sharedHierarchy, "OPERATIONS", "operations Public Private\n\texisting String fmt.Stringer", 1),
	}
	// The hierarchy's file has no operations line; both declarations are in
	// tagged files that the default build leaves out.
	thirdFile := map[string]string{
		"labels.go": "//go:build labels\n\n" + labeledOps,
		"main.go":   strings.Replace(sharedHierarchy, "OPERATIONS\n", "", 1),
		"titles.go": "//go:build titles\n\n" + titledOps,
	}

	t.Run("hierarchy file names the interface", func(t *testing.T) {
		outputs := generateRepeatedly(t, sourceDir(t, sameFile))
		main := outputs[mainOutput]
		if strings.Contains(main, sharedRepeat) {
			t.Fatalf("the hierarchy's own operations line already guards fmt.Stringer, but its output repeats the guard:\n%s", main)
		}
		for _, name := range []string{mainOutput, outputName("labels.go")} {
			if !strings.Contains(outputs[name], "var _ interface{ String() string } = fmt.Stringer(nil)") {
				t.Fatalf("%s does not guard fmt.Stringer:\n%s", name, outputs[name])
			}
		}
	})

	t.Run("declarations in two other files", func(t *testing.T) {
		outputs := generateRepeatedly(t, sourceDir(t, thirdFile))
		main := outputs[mainOutput]
		// labels.go precedes titles.go, so its spelling is the representative.
		want := sharedRepeat + " must\n// still have exactly the method it identifies. If an assignment below fails\n// to compile, it has changed since this file was generated: run go generate,\n// or correct its existing line.\nfunc _() {\n\tvar _ interface{ String() string } = fmt.Stringer(nil)\n\tvar _ fmt.Stringer = interface{ String() string }(nil)\n}\n"
		if !strings.Contains(main, want) {
			t.Fatalf("the repeated guard is not the one of labels.go:\n%s", main)
		}
		if !strings.Contains(outputs[outputName("titles.go")], "var _ interface{ String() (text string) } = std.Stringer(nil)") {
			t.Fatalf("titles.go keeps its own spelling in its own guard:\n%s", outputs[outputName("titles.go")])
		}
	})

	t.Run("builds", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			files map[string]string
			tags  []string
		}{
			{"hierarchy file names the interface", sameFile, []string{""}},
			{"declarations in two other files", thirdFile, []string{"", "labels", "titles", "labels,titles"}},
		} {
			dir := tempModule(t, tc.files)
			mustGen(t, dir)
			for _, tags := range tc.tags {
				if output, err := goTool(t, dir, nil, "vet", "-tags", tags, "."); err != nil {
					t.Fatalf("%s: go vet -tags %q:\n%s", tc.name, tags, output)
				}
			}
			if got := mustRun(t, dir); got != "node" {
				t.Fatalf("%s: output = %q", tc.name, got)
			}
		}
	})
}

// Two operations lines that name one existing interface for different
// methods contradict each other and are refused. The diagnostic must not
// depend on map order, so it is the same in every run.
func TestConflictingSharedExistingIsDeterministic(t *testing.T) {
	named := "package main\n\ntype Named interface{ Name() string }\n\n//heiry:spec\nconst namedSpec = `\nimport fmt \"fmt\"\noperations Named\n\texisting Name fmt.Stringer\n`\n"
	hierarchy := strings.Replace(sharedHierarchy, "OPERATIONS\n", "operations Public Private suffix=Op\n", 1)
	hierarchy = strings.Replace(hierarchy, "\tnode fmt.Stringer\n", "\tnode fmt.Stringer\nwrapper view\n", 1)
	dir := sourceDir(t, map[string]string{"labels.go": labeledOps, "main.go": hierarchy, "names.go": named})
	exit, first := gen(dir)
	const refusal = "existing Name fmt.Stringer names fmt.Stringer for method Name, but"
	if exit != 1 || !strings.Contains(first, refusal) || !strings.Contains(first, "labels.go:") {
		t.Fatalf("exit %d, want the refusal %q naming labels.go:\n%s", exit, refusal, first)
	}
	outputs := outputsOf(t, dir)
	if len(outputs) != 0 {
		t.Fatalf("the refused run wrote %v", outputs)
	}
	for run := 2; run <= 40; run++ {
		// Start each run from the handwritten files alone.
		for name := range outputsOf(t, dir) {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
		}
		got, message := gen(dir)
		if got != exit || message != first {
			t.Fatalf("run %d: exit %d %q, first run: exit %d %q", run, got, message, exit, first)
		}
		if now := outputsOf(t, dir); !maps.Equal(now, outputs) {
			t.Fatalf("run %d wrote other outputs than the first run", run)
		}
	}
	t.Logf("exit %d: %s", exit, first)
}
