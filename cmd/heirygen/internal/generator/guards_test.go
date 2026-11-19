package generator

import (
	"strings"
	"testing"
)

// guardApp keeps every operation interface handwritten, so the program
// builds whichever file holds the operations line.
const guardApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Reader interface{ Read() string }
	Sealer interface{ Seal(reason string) error }
)

type (
	Public  interface{ Read() string }
	Private interface{ Seal(reason string) error }
)

type node struct{ heiry.Node }

func (n *node) Read() string             { return "read" }
func (n *node) Seal(reason string) error { return nil }

SPEC

func main() {
	n := &node{}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println((&view{state: n}).Read())
}
`

const operationsSpec = "//heiry:spec\nconst operationsSpec = `\noperations Public Private suffix=Op\n\texisting Read Reader\n\texisting Seal Sealer\n`"

const recordsSpec = "//heiry:spec\nconst recordsSpec = `\nhierarchy records node\nlevel node public=Public private=Private\n\tnode Reader Sealer\nwrapper view\n`"

// Renaming a private method together with its existing interface keeps the
// two equal, so an equality assertion between them still holds. The guard
// must compare with what the contract was when the file was generated, so a
// wrapper method that now exposes the renamed operation cannot compile.
func TestLockstepRenameOfAnExistingPrivateOperation(t *testing.T) {
	app := strings.Replace(guardApp, "SPEC", operationsSpec+"\n\n"+recordsSpec, 1)
	dir := tempModule(t, map[string]string{"main.go": app})
	mustGen(t, dir)
	mustBuild(t, dir)
	for _, rename := range [][2]string{
		{"Sealer interface{ Seal(reason string) error }", "Sealer interface{ Close(reason string) error }"},
		{"Private interface{ Seal(reason string) error }", "Private interface{ Close(reason string) error }"},
		{"func (n *node) Seal(reason string) error { return nil }", "func (n *node) Close(reason string) error { return nil }"},
	} {
		edit(t, dir, "main.go", rename[0], rename[1])
	}
	write(t, dir, "exposed.go", "package main\n\nfunc (v *view) Close(reason string) error { return v.state.Node.Virtual[Sealer]().Close(reason) }\n")
	output := buildFails(t, dir, "missing method Seal")
	if !strings.Contains(output, mainOutput) {
		t.Fatalf("the failure is not reported in the generated file:\n%s", output)
	}
}

// The operations line may be in another file than the wrapper: a test file,
// or a file with a build constraint. Every build that compiles the wrapper
// must still check both of the leaf's contracts, including a private
// contract that had no operation when the file was generated.
func TestEveryBuildOfAWrapperChecksBothContracts(t *testing.T) {
	empty := strings.NewReplacer(
		"Private interface{ Seal(reason string) error }", "Private interface{}",
		"func (n *node) Seal(reason string) error { return nil }", "",
		"\tnode Reader Sealer\n", "\tnode Reader\n",
	)
	// Each edit adds an operation to one of the leaf's contracts, implements
	// it, and adds a wrapper method that exposes or forwards it, so that only
	// the wrapper's contract guards can notice.
	type change struct {
		contract, from, to, methods, diagnostic string
	}
	privateChange := change{"private", "Private interface{ Seal(reason string) error }", "Private interface {\n\t\tSeal(reason string) error\n\t\tWipe()\n\t}",
		"func (n *node) Wipe() {}\n\nfunc (v *view) Wipe() { v.state.Wipe() }\n", "does not implement Private (missing method Wipe)"}
	publicChange := change{"public", "Public  interface{ Read() string }", "Public interface {\n\t\tRead() string\n\t\tName() string\n\t}",
		"func (n *node) Name() string { return \"\" }\n\nfunc (v *view) Name() string { return v.state.Name() }\n", "interface{Read() string} does not implement Public (missing method Name)"}
	for _, tc := range []struct {
		name, opsFile, opsHeader string
		empty                    bool
		builds                   [][]string // go command arguments that must pass before the edit, and fail after it
	}{
		{"operations in a test file", "ops_test.go", "", false, [][]string{{"vet", "."}, {"test", "."}}},
		{"operations in a tagged file", "ops.go", "//go:build special\n\n", false, [][]string{{"vet", "."}, {"vet", "-tags", "special", "."}}},
		{"initially empty private contract, operations in a test file", "ops_test.go", "", true, [][]string{{"vet", "."}, {"test", "."}}},
		{"initially empty private contract, operations in a tagged file", "ops.go", "//go:build special\n\n", true, [][]string{{"vet", "."}, {"vet", "-tags", "special", "."}}},
	} {
		for _, c := range []change{privateChange, publicChange} {
			t.Run(tc.name+", "+c.contract+" contract", func(t *testing.T) {
				app := strings.Replace(guardApp, "SPEC", recordsSpec, 1)
				ops := tc.opsHeader + "package main\n\n" + operationsSpec + "\n"
				if tc.empty {
					app = empty.Replace(app)
					ops = strings.Replace(ops, "\texisting Seal Sealer\n", "", 1)
					if c.contract == "private" {
						c.from, c.to = "Private interface{}", "Private interface {\n\t\tWipe()\n\t}"
					}
				}
				dir := tempModule(t, map[string]string{"main.go": app, tc.opsFile: ops})
				mustGen(t, dir)
				for _, args := range tc.builds {
					if output, err := goTool(t, dir, nil, args...); err != nil {
						t.Fatalf("control: go %v:\n%s", args, output)
					}
				}
				edit(t, dir, "main.go", c.from, c.to)
				write(t, dir, "exposed.go", "package main\n\n"+c.methods)
				output := buildFails(t, dir, c.diagnostic)
				reportedIn(t, output, mainOutput, c.diagnostic)
				// Every build that compiles the wrapper checks it, including
				// those that also compile the operations line's file.
				for _, args := range tc.builds {
					output, err := goTool(t, dir, nil, args...)
					if err == nil {
						t.Fatalf("go %v succeeded after the %s contract changed", args, c.contract)
					}
					reportedIn(t, output, mainOutput, c.diagnostic)
				}
			})
		}
	}
}

// An existing interface that gains a method in step with its contract keeps
// the contract equality true. Its own guard ties it to the one method it was
// named for. This hierarchy has no wrapper, so no wrapper guard helps.
func TestExistingInterfaceStillIdentifiesItsMethod(t *testing.T) {
	app := strings.Replace(guardApp, "SPEC", operationsSpec+"\n\n//heiry:spec\nconst recordsSpec = `\nhierarchy records node\nlevel node public=Public private=Private\n\tnode Reader Sealer\n`", 1)
	app = strings.Replace(app, "\tfmt.Println((&view{state: n}).Read())\n", "\tfmt.Println(n.Node.Virtual[Reader]().Read())\n", 1)
	dir := tempModule(t, map[string]string{"main.go": app})
	mustGen(t, dir)
	if got := mustRun(t, dir); got != "read" {
		t.Fatalf("output = %q", got)
	}
	edit(t, dir, "main.go", "Sealer interface{ Seal(reason string) error }", "Sealer interface {\n\t\tSeal(reason string) error\n\t\tWipe()\n\t}")
	edit(t, dir, "main.go", "Private interface{ Seal(reason string) error }", "Private interface {\n\t\tSeal(reason string) error\n\t\tWipe()\n\t}")
	// The provider implements the grown interface, so only the guard of the
	// existing interface can fail.
	write(t, dir, "wipe.go", "package main\n\nfunc (n *node) Wipe() {}\n")
	output := buildFails(t, dir, "does not implement Sealer (missing method Wipe)")
	reportedIn(t, output, mainOutput, "interface{Seal(reason string) error} does not implement Sealer (missing method Wipe)")
}

// swapApp registers two handwritten private operation interfaces, which an
// operations line in another file names with existing lines.
const swapApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Reader interface{ Read() string }
	Sealer interface{ Seal() error }
	Wiper  interface{ Wipe() error }
)

type (
	Public  interface{ Read() string }
	Private interface {
		Seal() error
		Wipe() error
	}
)

type node struct{ heiry.Node }

func (n *node) Read() string { return "read" }
func (n *node) Seal() error  { return nil }
func (n *node) Wipe() error  { return nil }

//heiry:spec
const recordsSpec = ` + "`" + `
hierarchy records node
level node public=Public private=Private
	node Reader Sealer Wiper
` + "`" + `

func main() {
	n := &node{}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(n.Node.Virtual[Reader]().Read())
}
`

const swapOperations = "//heiry:spec\nconst operationsSpec = `\noperations Public Private suffix=Op\n\texisting Read Reader\n\texisting Seal Sealer\n\texisting Wipe Wiper\n`"

// Swapping the methods of two existing interfaces keeps every contract and
// the specification text as they were, and the provider still implements
// both. The registrations now name each interface for the other method. The
// build that compiles the registrations must check the existing interfaces
// they use, wherever the operations line is.
func TestRegisteredExistingInterfacesAreCheckedWhereTheyAreUsed(t *testing.T) {
	for _, tc := range []struct {
		name, opsFile, opsHeader string
		controls                 [][]string
	}{
		{"operations in a test file", "ops_test.go", "", [][]string{{"vet", "."}, {"test", "."}}},
		{"operations in a tagged file", "ops.go", "//go:build special\n\n", [][]string{{"vet", "."}, {"vet", "-tags", "special", "."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempModule(t, map[string]string{"main.go": swapApp, tc.opsFile: tc.opsHeader + "package main\n\n" + swapOperations + "\n"})
			mustGen(t, dir)
			for _, args := range tc.controls {
				if output, err := goTool(t, dir, nil, args...); err != nil {
					t.Fatalf("control: go %v:\n%s", args, output)
				}
			}
			if got := mustRun(t, dir); got != "read" {
				t.Fatalf("output = %q", got)
			}
			edit(t, dir, "main.go", "\tSealer interface{ Seal() error }\n\tWiper  interface{ Wipe() error }\n", "\tSealer interface{ Wipe() error }\n\tWiper  interface{ Seal() error }\n")
			output := buildFails(t, dir, "Sealer does not implement interface{Seal() error} (missing method Seal)")
			reportedIn(t, output, mainOutput, "Sealer does not implement interface{Seal() error} (missing method Seal)")
		})
	}
}

// reportedIn requires a line of the build output that names the file and
// contains the diagnostic.
func reportedIn(t *testing.T, output, file, diagnostic string) {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, file) && strings.Contains(line, diagnostic) {
			return
		}
	}
	t.Fatalf("no line reports %q in %s:\n%s", diagnostic, file, output)
}

// aliasApp names its existing interfaces on an operations line of the
// hierarchy's own file and registers two of them through local aliases.
const aliasApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Reader interface{ Read() string }
	Sealer interface{ Seal() error }
	Wiper  interface{ Wipe() error }
)

type (
	Sealing = Sealer
	Wiping  = Wiper
)

type (
	Public  interface{ Read() string }
	Private interface {
		Seal() error
		Wipe() error
	}
)

type node struct{ heiry.Node }

func (n *node) Read() string { return "read" }
func (n *node) Seal() error  { return nil }
func (n *node) Wipe() error  { return nil }

//heiry:spec
const recordsSpec = ` + "`" + `
operations Public Private
	existing Read Reader
	existing Seal Sealer
	existing Wipe Wiper
hierarchy records node
level node public=Public private=Private
	node Reader Sealing Wiping
` + "`" + `

func main() {
	n := &node{}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(n.Node.Virtual[Reader]().Read(), n.Node.Virtual[Sealer]().Seal(), n.Node.Virtual[Wiper]().Wipe())
}
`

// The guard of an operations line ties each existing interface, as that
// line spells it, to its method. A registration with the same spelling in
// the same file needs no other guard. One spelled through a local alias
// resolved to the same interface only when the file was generated: swapping
// the aliases changes neither the contracts, nor the interfaces, nor the
// specification, so its own guard must catch it.
func TestAliasRegistrationsAreGuardedInTheirOwnFile(t *testing.T) {
	dir := tempModule(t, map[string]string{"main.go": aliasApp})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "read <nil> <nil>" {
		t.Fatalf("output = %q", got)
	}
	generated := read(t, dir, mainOutput)
	// The registration of Reader has the spelling of its existing line, whose
	// guard covers it.
	if got := strings.Count(generated, "var _ interface{ Read() string } = Reader(nil)"); got != 1 {
		t.Fatalf("Reader is guarded %d times, want once:\n%s", got, generated)
	}
	// The aliases have other spellings and get guards of their own.
	for _, want := range []string{"var _ interface{ Seal() error } = Sealing(nil)", "var _ interface{ Wipe() error } = Wiping(nil)"} {
		if !strings.Contains(generated, want) {
			t.Fatalf("no %q:\n%s", want, generated)
		}
	}

	edit(t, dir, "main.go", "\tSealing = Sealer\n\tWiping  = Wiper\n", "\tSealing = Wiper\n\tWiping  = Sealer\n")
	output := buildFails(t, dir, "missing method Seal")
	t.Logf("after the swap:\n%s", output)
	reportedIn(t, output, mainOutput, "does not implement interface{Seal() error} (missing method Seal)")
}
