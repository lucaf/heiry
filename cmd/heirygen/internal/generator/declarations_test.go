package generator

import (
	"maps"
	"strings"
	"testing"
)

// stringerHierarchy registers fmt.Stringer and forwards String through the
// wrapper. PUBLIC is replaced by the operations line of the public contract.
const stringerHierarchy = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Public  interface{ String() string }
	Private interface{}
	Labeled interface{ String() string }
)

type node struct{ heiry.Node }

func (n *node) String() string { return "node" }

//heiry:spec
const recordsSpec = ` + "`" + `
import fmt "fmt"
PUBLIC
operations Labeled
	existing String fmt.Stringer
hierarchy records node
level node public=Public private=Private
	node fmt.Stringer
wrapper view
` + "`" + `

func main() {
	n := &node{}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println((&view{state: n}).String())
}
`

// stringerDeclaration names fmt.Stringer for METHOD in a file that sorts
// before the hierarchy's.
const stringerDeclaration = "package main\n\ntype Named interface{ METHOD() string }\n\n//heiry:spec\nconst namedSpec = `\nimport fmt \"fmt\"\noperations Named\n\texisting METHOD fmt.Stringer\n`\n"

// One interface identifies one operation in the whole package, whatever file
// or build names it. Two lines that name fmt.Stringer for String and for
// Name contradict each other. Placed in a test or tagged file that sorts
// first, the wrong line used to decide which method the dispatch check
// expected, so a wrapper whose forwarder resolves an unregistered interface
// was accepted.
func TestContradictoryExistingLinesAreRefused(t *testing.T) {
	wrong := strings.ReplaceAll(stringerDeclaration, "METHOD", "Name")
	main := strings.Replace(stringerHierarchy, "PUBLIC", "operations Public Private suffix=Op", 1)
	for _, tc := range []struct{ name, file, header string }{
		{"in a test file", "aaa_test.go", ""},
		{"in a tagged file", "aaa.go", "//go:build legacy\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempModule(t, map[string]string{tc.file: tc.header + wrong, "main.go": main})
			before := snapshot(t, dir)
			exit, message := gen(dir)
			if exit == 0 {
				run, _ := goTool(t, dir, nil, "run", ".")
				t.Fatalf("the contradictory lines were accepted; the ordinary build then ran:\n%s", run)
			}
			t.Logf("error: %s", message)
			for _, want := range []string{tc.file + ":", "main.go:", "existing Name fmt.Stringer", "existing String fmt.Stringer", "method Name", "method String"} {
				if !strings.Contains(message, want) {
					t.Fatalf("the error does not say %q:\n%s", want, message)
				}
			}
			if after := snapshot(t, dir); !maps.Equal(after, before) {
				t.Fatal("the refused run changed the directory")
			}
		})
	}

	// The same layout with every line naming fmt.Stringer for String is
	// valid reuse of that interface.
	t.Run("valid shared declarations", func(t *testing.T) {
		valid := strings.ReplaceAll(stringerDeclaration, "METHOD", "String")
		dir := tempModule(t, map[string]string{
			"aaa.go":  "//go:build legacy\n\n" + valid,
			"main.go": strings.Replace(stringerHierarchy, "PUBLIC", "operations Public Private\n\texisting String fmt.Stringer", 1),
		})
		mustGen(t, dir)
		for _, tags := range []string{"", "legacy"} {
			if output, err := goTool(t, dir, nil, "vet", "-tags", tags, "."); err != nil {
				t.Fatalf("go vet -tags %q:\n%s", tags, output)
			}
		}
		if got := mustRun(t, dir); got != "node" {
			t.Fatalf("output = %q", got)
		}
	})
}

// auditDeclaration names Sealer for Seal with a signature that uses a name
// declared only in its own file, which a test file is.
const auditDeclaration = "package main\n\ntype status = error\n\ntype Audited interface{ Seal() status }\n\n//heiry:spec\nconst auditSpec = `\noperations Audited\n\texisting Seal Sealer\n`\n"

// maintainedDeclaration names Sealer and Wiper with signatures every build
// can use.
const maintainedDeclaration = "package main\n\ntype Maintained interface {\n\tSeal() error\n\tWipe() error\n}\n\n//heiry:spec\nconst maintainedSpec = `\noperations Maintained\n\texisting Seal Sealer\n\texisting Wipe Wiper\n`\n"

// The guard repeated for a registration copies the signature of one
// declaration of its interface, and must compile in every build that
// compiles the registration. The hierarchy's own declaration is preferred;
// otherwise the first one whose names that build has. With none, generation
// says what to make available.
func TestRepeatedGuardUsesASignatureTheBuildCompiles(t *testing.T) {
	// aliasApp without its own operations line.
	noOwn := strings.Replace(aliasApp, "operations Public Private\n\texisting Read Reader\n\texisting Seal Sealer\n\texisting Wipe Wiper\n", "", 1)
	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"hierarchy's own declaration", map[string]string{"audit_test.go": auditDeclaration, "main.go": aliasApp}},
		{"declaration in a file every build compiles", map[string]string{"audit_test.go": auditDeclaration, "main.go": noOwn, "ops.go": maintainedDeclaration}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempModule(t, tc.files)
			mustGen(t, dir)
			if output, err := goTool(t, dir, nil, "build", "-o", "/dev/null", "."); err != nil {
				t.Fatalf("the ordinary build failed:\n%s", output)
			}
			mustBuild(t, dir)
			if got := mustRun(t, dir); got != "read <nil> <nil>" {
				t.Fatalf("output = %q", got)
			}
			edit(t, dir, "main.go", "\tSealing = Sealer\n\tWiping  = Wiper\n", "\tSealing = Wiper\n\tWiping  = Sealer\n")
			output := buildFails(t, dir, "missing method Seal")
			reportedIn(t, output, mainOutput, "does not implement interface{Seal() error} (missing method Seal)")
		})
	}

	t.Run("no declaration the build can use", func(t *testing.T) {
		wipeOnly := "package main\n\ntype Wiped interface{ Wipe() error }\n\n//heiry:spec\nconst wipedSpec = `\noperations Wiped\n\texisting Wipe Wiper\n`\n"
		dir := sourceDir(t, map[string]string{"audit_test.go": auditDeclaration, "main.go": noOwn, "wiped.go": wipeOnly})
		before := snapshot(t, dir)
		exit, message := gen(dir)
		if exit == 0 {
			t.Fatalf("generated a guard whose signature the ordinary build cannot compile:\n%s", read(t, dir, mainOutput))
		}
		t.Logf("error: %s", message)
		for _, want := range []string{"main.go:", "recordsSpec, line 4", "Sealing", "status", "audit_test.go"} {
			if !strings.Contains(message, want) {
				t.Fatalf("the error does not say %q:\n%s", want, message)
			}
		}
		if after := snapshot(t, dir); !maps.Equal(after, before) {
			t.Fatal("the refused run changed the directory")
		}
	})
}

// A name declared in another file is usable in an output only when every
// build that compiles the output compiles that file. The generator compares
// builds and does not solve constraints, so it accepts a file without
// conditions or with exactly the conditions of the receiving one.
func TestFileBuildComparison(t *testing.T) {
	for name, want := range map[string]string{
		"main.go": "", "linux.go": "", "a.b_linux.go": "", "x_unix.go": "", "my_types.go": "", "audit_test.go": "",
		"x_linux.go": "_linux", "y_linux_test.go": "_linux", "x_linux_amd64.go": "linux_amd64", "x_amd64.go": "_amd64", "a_b_linux.go": "b_linux",
	} {
		if got := platformOf(name); got != want {
			t.Errorf("platformOf(%s) = %q, want %q", name, got, want)
		}
	}
	plain := fileBuild{}
	tagged := fileBuild{constraint: "//go:build special"}
	linux := fileBuild{platform: "_linux"}
	for _, tc := range []struct {
		name     string
		file, to fileBuild
		want     bool
	}{
		{"a plain file, in any output", plain, tagged, true},
		{"a plain file, in a test output", plain, fileBuild{test: true, platform: "_linux"}, true},
		{"a test file, in a plain output", fileBuild{test: true}, plain, false},
		{"a test file, in a test output", fileBuild{test: true}, fileBuild{test: true}, true},
		{"a tagged file, in a plain output", tagged, plain, false},
		{"a tagged file, in an output of the same tag", tagged, tagged, true},
		{"a tagged file, in an output of another tag", tagged, fileBuild{constraint: "//go:build other"}, false},
		{"a linux file, in a plain output", linux, plain, false},
		{"a linux file, in a linux output", linux, linux, true},
		{"a linux file, in a linux output with a tag", linux, fileBuild{constraint: "//go:build special", platform: "_linux"}, false},
	} {
		if got := tc.file.compiledWith(tc.to); got != tc.want {
			t.Errorf("%s: %t, want %t", tc.name, got, tc.want)
		}
	}
}
