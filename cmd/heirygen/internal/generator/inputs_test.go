package generator

import (
	"regexp"
	"strings"
	"testing"
)

// These cases complete the freshness matrix for inputs other than the spec
// constant, and for generated files that differ from what the generator
// would write. Each records which boundary notices: compilation, check mode,
// a test, or nothing.

// visibilityTest is the kind of test an application can keep: it checks each
// private operation on the wrapper individually.
const visibilityTest = `package main

import "testing"

func TestPrivateOperationsAreNotExposed(t *testing.T) {
	var consumer any = &view{state: &node{}}
	if _, ok := consumer.(interface{ Audit() string }); ok {
		t.Error("the wrapper exposes Audit")
	}
	if _, ok := consumer.(interface{ Seal(reason string) error }); ok {
		t.Error("the wrapper exposes Seal")
	}
}
`

const moveAuditOld = "\tBase\n\tAudit() string\n}\n\ntype Private interface {\n"
const moveAuditNew = "\tBase\n}\n\ntype Private interface {\n\tAudit() string\n"

// Contract edits that do not change the method set compile, because the
// generated code is still correct for them. Check mode reports the ones
// that change the expected text.
func TestCosmeticContractChanges(t *testing.T) {
	for _, tc := range []struct {
		name, old, new string
		check          int
	}{
		{"rename a parameter", "\tSeal(reason string) error\n}", "\tSeal(why string) error\n}", 1},
		{"reorder embedding and method", "\tBase\n\tAudit() string\n", "\tAudit() string\n\tBase\n", 1},
		{"add a comment", "\tSeal(reason string) error\n}", "\t// Seal closes the record.\n\tSeal(reason string) error\n}", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := fullModule(t)
			edit(t, dir, "main.go", tc.old, tc.new)
			mustBuild(t, dir)
			if got := mustRun(t, dir); !strings.Contains(got, "exposes Read: true Audit: true Seal: false") {
				t.Fatalf("output = %q", got)
			}
			if exit, _ := gen("-check", dir); exit != tc.check {
				t.Fatalf("check exited %d, want %d", exit, tc.check)
			}
		})
	}
}

// Aliases are followed by the compiler, so retargeting one without
// regenerating is a compile error wherever the generated code depends on
// what the alias means.
func TestAliasChanges(t *testing.T) {
	t.Run("a contract alias", func(t *testing.T) {
		dir := tempModule(t, map[string]string{
			"main.go":  strings.Replace(fullApp, "type Public interface {\n\tBase\n\tAudit() string\n}", "type Wide interface {\n\tBase\n\tAudit() string\n}\n\ntype Public = Wide", 1),
			"probe.go": fullProbe,
			"other.go": "package main\n\ntype Narrow interface{ Base }\n",
		})
		mustGen(t, dir)
		mustBuild(t, dir)
		edit(t, dir, "main.go", "type Public = Wide", "type Public = Narrow")
		output := buildFails(t, dir, "(missing method Audit)")
		reportedIn(t, output, mainOutput, "Narrow does not implement interface{ReadOp; AuditOp} (missing method Audit)")
	})

	t.Run("the alias under which a parent is embedded", func(t *testing.T) {
		dir := staleModule(t, constCarrier, replace("level mid parent=base\n", "level mid parent=alt\n")(staleSpec))
		write(t, dir, "other.go", "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype elsewhere struct {\n\theiry.Node\n\tlabel         string\n\tfirst, second part\n}\n")
		mustGen(t, dir)
		mustBuild(t, dir)
		edit(t, dir, "main.go", "type alt = base", "type alt = elsewhere")
		output := buildFails(t, dir, "as *base value in argument to shape.NodeProvider[base]")
		reportedIn(t, output, mainOutput, "as *base value in argument to shape.NodeProvider[base]")
	})
}

// A generated file follows its source file's build constraint as of the
// last generation. Changing the constraint without regenerating leaves one
// of the two files out of some builds, and those builds fail.
func TestBuildConstraintChanges(t *testing.T) {
	t.Run("the source file gains a constraint", func(t *testing.T) {
		dir := fullModule(t)
		write(t, dir, "main.go", "//go:build special\n\n"+read(t, dir, "main.go"))
		// Without the tag only the generated file is built.
		buildFails(t, dir, "undefined: node")
		// With the tag both are built and the program is correct.
		if output, err := goTool(t, dir, nil, "vet", "-tags", "special", "."); err != nil {
			t.Fatalf("go vet -tags special:\n%s", output)
		}
		if exit, _ := gen("-check", dir); exit != 1 {
			t.Fatalf("check exited %d, want 1", exit)
		}
	})

	t.Run("the source file loses its constraint", func(t *testing.T) {
		dir := tempModule(t, map[string]string{"main.go": "//go:build special\n\n" + fullApp, "probe.go": fullProbe, "stub.go": "//go:build !special\n\npackage main\n\nfunc main() {}\n"})
		mustGen(t, dir)
		mustBuild(t, dir)
		write(t, dir, "main.go", fullApp)
		write(t, dir, "stub.go", "package main\n")
		// Without the tag only the handwritten file is built.
		buildFails(t, dir, "undefined: recordsInit")
		if exit, _ := gen("-check", dir); exit != 1 {
			t.Fatalf("check exited %d, want 1", exit)
		}
	})

	// The generator does not evaluate constraints, so it cannot choose
	// between two declarations of one contract. It says so.
	t.Run("a contract declared once per platform", func(t *testing.T) {
		app := strings.Replace(fullApp, "type Private interface {\n\tSeal(reason string) error\n}\n", "", 1)
		dir := tempModule(t, map[string]string{
			"main.go":            app,
			"probe.go":           fullProbe,
			"private_linux.go":   "package main\n\ntype Private interface {\n\tSeal(reason string) error\n}\n",
			"private_windows.go": "package main\n\ntype Private interface {\n\tSeal(reason string) error\n}\n",
		})
		exit, message := gen(dir)
		if exit != 1 || !strings.Contains(message, "contract Private is declared in private_linux.go and in private_windows.go") {
			t.Fatalf("exit %d:\n%s", exit, message)
		}
	})
}

// A new handwritten package-level name either collides with a generated
// name, which the compiler reports, or only changes which local names the
// generator would pick, which is harmless and visible to check mode.
func TestNewPackageLevelIdentifiers(t *testing.T) {
	dir := fullModule(t)
	write(t, dir, "other.go", "package main\n\ntype AuditOp struct{}\n")
	buildFails(t, dir, "AuditOp redeclared in this block")

	dir = fullModule(t)
	write(t, dir, "other.go", "package main\n\ntype leaf struct{}\n\nvar _ leaf\n")
	mustBuild(t, dir)
	if exit, _ := gen("-check", dir); exit != 1 {
		t.Fatalf("check exited %d, want 1", exit)
	}
}

// A generated file that someone edited keeps compiling when the edit is
// well typed. Removing every guard is such an edit: afterwards a stale
// wrapper compiles and exposes an operation that became private. Check mode
// reports the edit, and a test that checks each private operation reports
// the exposure. The compiler reports neither.
func TestManuallyEditedGeneratedFile(t *testing.T) {
	dir := fullModule(t)
	write(t, dir, "visibility_test.go", visibilityTest)
	// Remove every guard function, with the comment above it. What remains
	// is well typed.
	guards := regexp.MustCompile(`(?ms)^(//[^\n]*\n)+func _\(\) \{\n.*?^\}\n`)
	edited := guards.ReplaceAllString(read(t, dir, mainOutput), "")
	if strings.Contains(edited, "func _()") {
		t.Fatalf("guards left in the edited file:\n%s", edited)
	}
	write(t, dir, mainOutput, edited)
	mustBuild(t, dir)
	if exit, _ := gen("-check", dir); exit != 1 {
		t.Fatalf("check on an edited file exited %d, want 1", exit)
	}

	edit(t, dir, "main.go", moveAuditOld, moveAuditNew)
	mustBuild(t, dir)
	if got := mustRun(t, dir); !strings.Contains(got, "exposes Read: true Audit: true Seal: false") {
		t.Fatalf("output = %q", got)
	}
	output, err := goTool(t, dir, nil, "test", ".")
	if err == nil || !strings.Contains(output, "the wrapper exposes Audit") || strings.Contains(output, "the wrapper exposes Seal") {
		t.Fatalf("go test: %v\n%s", err, output)
	}
}

// What the compiler checks independently of the generator, shown by making
// the generated file wrong in the way a generator defect would.
func TestCompilerCheckedPropertiesOfGeneratedCode(t *testing.T) {
	for _, tc := range []struct{ name, old, new, want string }{
		{
			name: "an operation interface with a miscopied signature",
			old:  "AuditOp interface{ Audit() string }", new: "AuditOp interface{ Audit() int }",
			want: "(wrong type for method Audit)",
		},
		{
			name: "a registration for a provider without the method",
			old:  "Bind[SealOp]((*node)(nil), \".\")", new: "Bind[SealOp]((*view)(nil), \".\")",
			want: "*view does not implement SealOp (missing method Seal)",
		},
		{
			name: "a forwarder that resolves another operation",
			old:  "Virtual[AuditOp]().Audit()", new: "Virtual[ReadOp]().Audit()",
			want: "Audit undefined",
		},
		{
			name: "a reference of the wrong provider type",
			old:  "records.NodeProvider[node](leaf)", new: "records.NodeProvider[view](leaf)",
			want: "cannot use leaf (variable of type *node) as *view value",
		},
		{
			name: "a wrapper without one public forwarder",
			old:  "func (w *view) Audit() string {\n\treturn w.state.Node.Virtual[AuditOp]().Audit()\n}\n", new: "",
			want: "*view does not implement Public (missing method Audit)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := fullModule(t)
			edit(t, dir, mainOutput, tc.old, tc.new)
			buildFails(t, dir, tc.want)
		})
	}

	// What it does not check: whether the registrations are the ones the
	// spec lists. Dropping one that an ancestor also supplies compiles and
	// validates; only its absence from the expected text is detectable.
	dir := staleModule(t, constCarrier, replace("level mid parent=base\n", "level mid parent=base\n\tnode Namer\n")(staleSpec))
	mustGen(t, dir)
	if got := mustRun(t, dir); got != "mid" {
		t.Fatalf("output = %q", got)
	}
	edit(t, dir, mainOutput, "\tBind[Namer]((*mid)(nil), \".\").\n", "")
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "base one" {
		t.Fatalf("output without the registration = %q", got)
	}
	if exit, _ := gen("-check", dir); exit != 1 {
		t.Fatalf("check exited %d, want 1", exit)
	}
}

// Output written by another generator version carries the same marker and
// differs in its body. It compiles as long as the library accepts the calls
// it makes. Only check mode reports it.
func TestOutputOfAnotherGeneratorVersion(t *testing.T) {
	dir := fullModule(t)
	edit(t, dir, mainOutput, "// The declarations below were generated from", "// These declarations were generated from")
	mustBuild(t, dir)
	if exit, message := gen("-check", dir); exit != 1 || !strings.Contains(message, "differs from the expected output") {
		t.Fatalf("check exited %d:\n%s", exit, message)
	}
}
