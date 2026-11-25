package generator

import (
	"strings"
	"testing"
)

// staleApp has every piece the stale-output experiments change: an override
// that an ancestor also supplies, two same-type components, two embedded
// copies of the parent type, a wider private contract, and a further level.
const staleApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Namer interface{ Name() string }
	Sizer interface{ Size() int }
)

type (
	Public      interface{ Namer }
	Private     interface{}
	WidePrivate interface{ Sizer }
)

type part struct {
	heiry.Node
	label string
}

func (p *part) Name() string { return p.label }

type base struct {
	heiry.Node
	label         string
	first, second part
}

func (b *base) Name() string { return "base " + b.label }

type alt = base

type mid struct {
	base
	alt
	heiry.Node
}

func (m *mid) Name() string { return "mid" }

type top struct {
	mid
	heiry.Node
}

type apex struct {
	top
	heiry.Node
}

SPEC

func main() {
	t := &top{}
	t.mid.base.label, t.mid.alt.label = "one", "two"
	t.mid.base.first.label, t.mid.base.second.label = "first part", "second part"
	t.mid.alt.first.label, t.mid.alt.second.label = "other first part", "other second part"
	if err := shapeInit(t); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(t.Node.Virtual[Namer]().Name())
}
`

const staleSpec = `
hierarchy shape top
level base public=Public private=Private
	node Namer
level mid parent=base
level top
`

type staleCase struct {
	name string
	// before and after rewrite the base spec. after is the author's edit.
	before, after func(string) string
	// oldOutput is printed by the fresh program before the edit.
	oldOutput string
	// newOutput is printed after the edit and a regeneration. When
	// regeneratedBuildError is set instead, the regenerated declaration no
	// longer fits the handwritten constructor and the build must say so.
	newOutput, regeneratedBuildError string
}

func replace(old, new string) func(string) string {
	return func(spec string) string {
		if strings.Count(spec, old) != 1 {
			panic("spec does not contain exactly one " + old)
		}
		return strings.Replace(spec, old, new, 1)
	}
}

func same(spec string) string { return spec }

var staleCases = []staleCase{
	{
		name:   "add a registration while an ancestor still supplies the operation",
		before: same, after: replace("level mid parent=base\n", "level mid parent=base\n\tnode Namer\n"),
		oldOutput: "base one", newOutput: "mid",
	},
	{
		name:      "remove a registration while an ancestor still supplies the operation",
		before:    replace("level mid parent=base\n", "level mid parent=base\n\tnode Namer\n"),
		after:     replace("level mid parent=base\n\tnode Namer\n", "level mid parent=base\n"),
		oldOutput: "mid", newOutput: "base one",
	},
	{
		name:      "change a component path between two same-type components",
		before:    replace("\tnode Namer\n", "\tcomponent first part Namer\n"),
		after:     replace("\tcomponent first part Namer\n", "\tcomponent second part Namer\n"),
		oldOutput: "first part", newOutput: "second part",
	},
	{
		name:   "select the other embedded parent",
		before: same, after: replace("level mid parent=base\n", "level mid parent=alt\n"),
		oldOutput: "base one", newOutput: "base two",
	},
	{
		name:   "widen a level's private contract",
		before: same, after: replace("level mid parent=base\n", "level mid parent=base public=Public private=WidePrivate\n"),
		oldOutput: "base one", newOutput: "error: MissingMethods: target main.top, level 2 (the target; no level has a valid registration), private Size capability, expected func() int",
	},
	{
		name:   "add a level to the hierarchy",
		before: same,
		after: func(spec string) string {
			return replace("hierarchy shape top\n", "hierarchy shape apex\n")(spec) + "level apex\n"
		},
		oldOutput: "base one", regeneratedBuildError: "cannot use t (variable of type *top) as *apex value",
	},
}

func staleModule(t *testing.T, carrier func(string) string, spec string) string {
	t.Helper()
	return tempModule(t, map[string]string{"main.go": strings.Replace(staleApp, "SPEC", carrier(spec), 1)})
}

func (tc staleCase) regenerated(t *testing.T, dir string) {
	t.Helper()
	mustGen(t, dir)
	if exit, message := gen("-check", dir); exit != 0 {
		t.Fatalf("check after regeneration exited %d:\n%s", exit, message)
	}
	if tc.regeneratedBuildError != "" {
		buildFails(t, dir, tc.regeneratedBuildError)
		return
	}
	if got := mustRun(t, dir); got != tc.newOutput {
		t.Fatalf("regenerated output = %q, want %q", got, tc.newOutput)
	}
}

// With the spec in a constant, every edit that is not followed by generation
// is a compile error in go build, go vet, and go test.
func TestStaleOutputWithSpecConstant(t *testing.T) {
	for _, tc := range staleCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := staleModule(t, constCarrier, tc.before(staleSpec))
			mustGen(t, dir)
			if got := mustRun(t, dir); got != tc.oldOutput {
				t.Fatalf("fresh output = %q, want %q", got, tc.oldOutput)
			}
			write(t, dir, "main.go", strings.Replace(staleApp, "SPEC", constCarrier(tc.after(tc.before(staleSpec))), 1))

			output := buildFails(t, dir, "duplicate key false in map literal")
			reportedIn(t, output, mainOutput, "duplicate key false in map literal")
			for _, command := range []string{"vet", "test"} {
				if output, err := goTool(t, dir, nil, command, "."); err == nil || !strings.Contains(output, "duplicate key false in map literal") {
					t.Fatalf("go %s on a stale spec: %v\n%s", command, err, output)
				} else {
					reportedIn(t, output, mainOutput, "duplicate key false in map literal")
				}
			}
			if exit, _ := gen("-check", dir); exit != 1 {
				t.Fatalf("check on a stale spec exited %d, want 1", exit)
			}
			tc.regenerated(t, dir)
		})
	}
}

// A specification in a comment is refused: the compiler cannot see a
// comment, so no generated assertion could detect that it changed.
func TestCommentSpecIsRefused(t *testing.T) {
	dir := staleModule(t, commentCarrier, staleSpec)
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "marks a specification only in the doc comment of a string constant") {
		t.Fatalf("exit %d:\n%s", exit, message)
	}
}

// These edits change the handwritten structs and leave the spec alone, so
// the generated file is exactly what the spec asks for and the check mode
// has nothing to report. The compiler still rejects the program, because the
// generated helpers name each field and each provider type.
func TestStructChangesBreakTheGeneratedHelpers(t *testing.T) {
	componentSpec := replace("\tnode Namer\n", "\tcomponent first part Namer\n")(staleSpec)
	for _, tc := range []struct {
		name, spec, old, new, want string
	}{
		{
			name: "rename a component field", spec: componentSpec,
			old: "\tfirst, second part\n", new: "\tprimary, second part\n",
			want: "leaf.mid.base.first undefined",
		},
		{
			name: "change a component field's type", spec: componentSpec,
			old: "\tfirst, second part\n", new: "\tfirst      otherPart\n\tsecond     part\n",
			want: "cannot use &leaf.mid.base.first (value of type *otherPart) as *part value in argument to shape.ComponentProvider[base, part]",
		},
		{
			name: "embed the parent by pointer", spec: staleSpec,
			old: "type mid struct {\n\tbase\n", new: "type mid struct {\n\t*base\n",
			want: "cannot use &leaf.mid.base (value of type **base) as *base value in argument to shape.NodeProvider[base]",
		},
		{
			name: "remove the parent embedding", spec: staleSpec,
			old: "type top struct {\n\tmid\n", new: "type top struct {\n\tinner mid\n",
			want: "leaf.mid undefined",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := staleModule(t, constCarrier, tc.spec)
			write(t, dir, "other.go", "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype otherPart struct {\n\theiry.Node\n\tlabel string\n}\n\nfunc (p *otherPart) Name() string { return p.label }\n")
			mustGen(t, dir)
			mustBuild(t, dir)
			edit(t, dir, "main.go", tc.old, tc.new)
			output, err := goTool(t, dir, nil, "build", "-o", "/dev/null", ".")
			if err == nil || !strings.Contains(output, tc.want) {
				t.Fatalf("go build: %v, want a failure mentioning %q:\n%s", err, tc.want, output)
			}
			if exit, message := gen("-check", dir); exit != 0 {
				t.Fatalf("the spec did not change, but check exited %d:\n%s", exit, message)
			}
		})
	}
}

// A spec that states something false is regenerated without complaint: the
// generator does not look at types. The compiler then rejects the generated
// code, through the Bind witness or through the typed reference.
func TestWrongSpecIsRejectedByTheCompiler(t *testing.T) {
	for _, tc := range []struct {
		name, old, new, want string
	}{
		{
			name: "a provider that does not implement the operation",
			old:  "level top\n", new: "level top\n\tnode Sizer\n",
			want: "*top does not implement Sizer (missing method Size)",
		},
		{
			name: "a component path whose field has another type",
			old:  "\tnode Namer\n", new: "\tcomponent first base Namer\n",
			want: "cannot use &leaf.mid.base.first (value of type *part) as *base value in argument to shape.ComponentProvider[base, base]",
		},
		{
			name: "a parent field that does not exist",
			old:  "level mid parent=base\n", new: "level mid parent=root\n",
			want: "leaf.mid.root undefined",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := staleModule(t, constCarrier, replace(tc.old, tc.new)(staleSpec))
			mustGen(t, dir)
			output := buildFails(t, dir, tc.want)
			if !strings.Contains(output, mainOutput) {
				t.Fatalf("the failure is not reported in the generated file:\n%s", output)
			}
		})
	}
}

// The generated references name each field on the path to a level, so the
// compiler rejects a path that no longer exists or has another type. The
// parent field's name is also a string in the declaration, and whether that
// field is an anonymous value embedding is checked by the library when the
// declaration is first prepared, not by the compiler.
func TestParentFieldChangesAreReportedAtValidation(t *testing.T) {
	dir := staleModule(t, constCarrier, staleSpec)
	mustGen(t, dir)
	edit(t, dir, "main.go", "type top struct {\n\tmid\n", "type top struct {\n\tmid mid\n")
	mustBuild(t, dir)
	if got := mustRun(t, dir); !strings.HasPrefix(got, "error: InvalidHierarchy") {
		t.Fatalf("output = %q, want the library's InvalidHierarchy", got)
	}
}
