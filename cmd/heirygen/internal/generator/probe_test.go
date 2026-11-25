package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// probeApp is a three-level hierarchy whose root stores a component. Its
// structs are edited by the cases below; main touches no field, so only the
// generated file depends on the layout.
const probeApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type Namer interface{ Name() string }

type (
	Public  interface{ Namer }
	Private interface{}
)

type store struct{ heiry.Node }

func (s *store) Name() string { return "store" }

type base struct {
	heiry.Node
	store store
}

type mid struct {
	base
	heiry.Node
}

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

const probeSpec = `
hierarchy shape top
level base public=Public private=Private
	component store store Namer
level mid
level top
`

const (
	topEmbedsMid = "type top struct {\n\tmid\n"
	insertExtra  = "type extra struct {\n\tmid\n\theiry.Node\n}\n\ntype top struct {\n\textra\n"
	baseStore    = "type base struct {\n\theiry.Node\n\tstore store\n}"
	baseGroup    = "type holder struct{ store store }\n\ntype base struct {\n\theiry.Node\n\tgroup holder\n}"
)

// A field reached through promotion compiles in the generated references,
// and only the library's validation would report it. The probes make the
// build fail in the generated file instead, whether the specification left
// the level out when it was generated or a struct changed afterwards. In
// either case regenerating writes the same file, so the build still fails
// until the specification or the struct is corrected.
func TestALeftOutLevelFailsCompilation(t *testing.T) {
	for _, tc := range []struct {
		name string
		app  func(string) string // the structs when the file is generated
		spec string
		// old and new are a struct edit made after generation; without one,
		// the specification was wrong when the file was generated.
		old, new string
		want     string
	}{
		{name: "a level the specification leaves out", app: same, spec: replace("level mid\n", "")(probeSpec),
			want: "ambiguous selector probe.base"},
		{name: "a level inserted after generation", app: same, spec: probeSpec,
			old: topEmbedsMid, new: insertExtra,
			want: "ambiguous selector probe.mid"},
		{name: "two levels left out", app: replace(topEmbedsMid, insertExtra), spec: replace("level mid\n", "")(probeSpec),
			want: "cannot use &probe.base (value of type *struct{}) as *base value"},
		{name: "a component reached through an embedded struct", app: same, spec: probeSpec,
			old: baseStore, new: "type parts struct{ store store }\n\ntype base struct {\n\theiry.Node\n\tparts\n}",
			want: "ambiguous selector probe.store"},
		{name: "a component reached through two embedded structs", app: same, spec: probeSpec,
			old: baseStore, new: "type inner struct{ store store }\n\ntype parts struct{ inner }\n\ntype base struct {\n\theiry.Node\n\tparts\n}",
			want: "cannot use &probe.store (value of type *struct{}) as *store value"},
		{name: "the first step of a longer component path", app: replace(baseStore, baseGroup),
			spec: replace("\tcomponent store store Namer\n", "\tcomponent group.store store Namer\n")(probeSpec),
			old:  baseGroup, new: "type holder struct{ store store }\n\ntype wrap struct{ group holder }\n\ntype base struct {\n\theiry.Node\n\twrap\n}",
			want: "ambiguous selector probe.group"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempModule(t, map[string]string{"main.go": strings.Replace(tc.app(probeApp), "SPEC", constCarrier(tc.spec), 1)})
			mustGen(t, dir)
			if tc.old != "" {
				if got := mustRun(t, dir); got != "store" {
					t.Fatalf("fresh output = %q, want store", got)
				}
				edit(t, dir, "main.go", tc.old, tc.new)
				if exit, message := gen("-check", dir); exit != 0 {
					t.Fatalf("the specification did not change, but check exited %d:\n%s", exit, message)
				}
			}
			reportedIn(t, buildFails(t, dir, tc.want), mainOutput, tc.want)
			mustGen(t, dir)
			reportedIn(t, buildFails(t, dir, tc.want), mainOutput, tc.want)
		})
	}
}

// otherPackage holds a root level declared in another package of the module.
const otherPackage = "package other\n\nimport \"github.com/lucaf/heiry\"\n\ntype Base struct{ heiry.Node }\n\nfunc (b *Base) Name() string { return \"other\" }\n"

// Every layout the library accepts compiles with the probes and runs.
func TestProbeControls(t *testing.T) {
	for _, tc := range []struct {
		name, main, other, want string
	}{
		{name: "direct embedding", want: "store",
			main: strings.Replace(probeApp, "SPEC", constCarrier(probeSpec), 1)},
		{name: "a parent embedded through an alias, with parent=", want: "store",
			main: strings.NewReplacer(
				"type mid struct {\n\tbase\n", "type alt = base\n\ntype mid struct {\n\talt\n",
				"SPEC", constCarrier(replace("level mid\n", "level mid parent=alt\n")(probeSpec)),
			).Replace(probeApp)},
		{name: "a generic level", want: "store",
			main: strings.NewReplacer(
				"type mid struct {\n\tbase\n\theiry.Node\n}", "type box[T any] struct {\n\tbase\n\theiry.Node\n\tvalue T\n}",
				topEmbedsMid, "type top struct {\n\tbox[int]\n",
				"SPEC", constCarrier(replace("level mid\n", "level box[int]\n")(probeSpec)),
			).Replace(probeApp)},
		{name: "a level declared as a type definition", want: "store",
			main: strings.NewReplacer(
				"type mid struct {\n\tbase\n\theiry.Node\n}", "type record struct {\n\tbase\n\theiry.Node\n}\n\ntype mid record",
				"SPEC", constCarrier(probeSpec),
			).Replace(probeApp)},
		{name: "a root level from another package", want: "other", other: otherPackage,
			main: strings.NewReplacer(
				topEmbedsMid, "type top struct {\n\tother.Base\n",
				"\t\"github.com/lucaf/heiry\"\n", "\t\"github.com/lucaf/heiry\"\n\t\"probe/other\"\n",
				"SPEC", constCarrier("\nimport other \"probe/other\"\nhierarchy shape top\nlevel other.Base public=Public private=Private\n\tnode Namer\nlevel top\n"),
			).Replace(probeApp)},
		{name: "types and fields named like the probe's own names", want: "store",
			main: strings.NewReplacer(
				baseStore, "type probe struct {\n\theiry.Node\n\tdecoy store\n}",
				"type mid struct {\n\tbase\n", "type deeper struct {\n\tprobe\n",
				topEmbedsMid, "type subject struct {\n\tdeeper\n",
				"t := &top{}", "t := &subject{}",
				"SPEC", constCarrier("\nhierarchy shape subject\nlevel probe public=Public private=Private\n\tcomponent decoy store Namer\nlevel deeper\nlevel subject\n"),
			).Replace(probeApp)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempModule(t, map[string]string{"main.go": tc.main})
			if tc.other != "" {
				if err := os.Mkdir(filepath.Join(dir, "other"), 0o755); err != nil {
					t.Fatal(err)
				}
				write(t, dir, filepath.Join("other", "other.go"), tc.other)
			}
			mustGen(t, dir)
			mustBuild(t, dir)
			if got := mustRun(t, dir); got != tc.want {
				t.Fatalf("output = %q, want %q", got, tc.want)
			}
			if exit, message := gen("-check", dir); exit != 0 {
				t.Fatalf("check exited %d:\n%s", exit, message)
			}
		})
	}
}
