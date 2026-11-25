package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR 0014: a hierarchy's leaf must be its last level. A leaf spelled exactly
// as its last level is accepted at once; otherwise the two are compared as
// types, with package names replaced by import paths, parentheses dropped,
// and local aliases followed, also inside type arguments.

// leafProgram is a module's main.go for the accepted cases. DECLS and SPEC
// are replaced by each case.
const leafProgram = "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype (\n\tPublic  interface{}\n\tPrivate interface{}\n)\n\ntype node struct{ heiry.Node }\n\nDECLS\n\n//heiry:spec\nconst thingsSpec = `\nSPEC`\n\nfunc main() {}\n"

const leafLib = "package lib\n\nimport \"github.com/lucaf/heiry\"\n\ntype Thing struct{ heiry.Node }\n\ntype T int\n"

func leafModule(t *testing.T, decls, spec string, extra map[string]string) string {
	t.Helper()
	source := strings.Replace(strings.Replace(leafProgram, "DECLS", decls, 1), "SPEC", spec, 1)
	dir := tempModule(t, map[string]string{"main.go": source})
	for name, content := range extra {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, dir, name, content)
	}
	return dir
}

// Spellings refused today only because of how they are written are
// accepted, and the output compiles: it passes the leaf to the last level's
// reference factory, so a leaf of another type would not.
func TestLeafSpellingsOfOneType(t *testing.T) {
	lib := map[string]string{"lib/lib.go": leafLib}
	for _, tc := range []struct {
		name, decls, spec string
		extra             map[string]string
	}{
		{"two import names of one package", "", "hierarchy things a.Thing\nimport a \"probe/lib\"\nimport b \"probe/lib\"\nlevel b.Thing public=Public private=Private\n", lib},
		{"two import names inside a type argument", "type box[T any] struct{ heiry.Node }", "hierarchy things box[a.T]\nimport a \"probe/lib\"\nimport b \"probe/lib\"\nlevel box[b.T] public=Public private=Private\n", lib},
		{"a local alias", "type Btn = node", "hierarchy things Btn\nlevel node public=Public private=Private\n", nil},
		{"a local alias as the last level", "type Btn = node", "hierarchy things node\nlevel Btn public=Public private=Private\n", nil},
		{"an alias of an alias", "type (\n\tBtn = Mid\n\tMid = node\n)", "hierarchy things Btn\nlevel node public=Public private=Private\n", nil},
		{"an alias inside a type argument", "type box[T any] struct{ heiry.Node }\n\ntype MyInt = int", "hierarchy things box[MyInt]\nlevel box[int] public=Public private=Private\n", nil},
		{"an alias spelled with spaces", "type gen2[A, B any] struct{ heiry.Node }\n\ntype IB = gen2[int, string]", "hierarchy things IB\nlevel gen2[int,string] public=Public private=Private\n", nil},
		{"a parenthesized leaf", "", "hierarchy things (node)\nlevel node public=Public private=Private\n", nil},
		{"a parenthesized alias", "type Btn = (node)", "hierarchy things Btn\nlevel node public=Public private=Private\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := leafModule(t, tc.decls, tc.spec, tc.extra)
			mustGen(t, dir)
			mustBuild(t, dir)
		})
	}
}

// Aliases are followed inside pointer, slice, array, map, and channel types
// in type arguments too, while two different composite types stay refused,
// with no reason to add.
func TestLeafCompositeTypeArguments(t *testing.T) {
	const decls = "type box[T any] struct{ heiry.Node }\n\ntype MyInt = int\n\ntype ch = box[chan MyInt]"
	for _, tc := range []struct{ name, leaf, last string }{
		{"a pointer", "box[*MyInt]", "box[*int]"},
		{"a slice", "box[[]MyInt]", "box[[]int]"},
		{"an array", "box[[2]MyInt]", "box[[2]int]"},
		{"a map", "box[map[MyInt]MyInt]", "box[map[int]int]"},
		{"a channel in an alias", "ch", "box[chan(int)]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := leafModule(t, decls, "hierarchy things "+tc.leaf+"\nlevel "+tc.last+" public=Public private=Private\n", nil)
			mustGen(t, dir)
			mustBuild(t, dir)
		})
	}
	for _, tc := range []struct{ name, leaf, last string }{
		{"a pointer and a slice", "box[*int]", "box[[]int]"},
		{"arrays of two lengths", "box[[2]int]", "box[[3]int]"},
		{"two channel directions", "box[chan(int)]", "box[<-chan(int)]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := leafDir(t, "type box[T any] struct{}", "hierarchy things "+tc.leaf+"\nlevel "+tc.last+" public=Public private=Private\n", nil)
			want := "thingsSpec, line 2: the last level is " + tc.last + " but the leaf is " + tc.leaf + "\n"
			if exit, message := gen(dir); exit != 1 || !strings.Contains(message+"\n", want) {
				t.Errorf("exit %d, want a message ending %q:\n%s", exit, want, message)
			}
		})
	}
}

// wrapperLeafProgram dispatches through a generated wrapper whose leaf is
// written LEAF.
const wrapperLeafProgram = "package main\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/lucaf/heiry\"\n)\n\ntype (\n\tPublic  interface{ Read() string }\n\tPrivate interface{}\n)\n\nNODE\n\nfunc (*node) Read() string { return \"node\" }\n\nDECLS\n\n//heiry:spec\nconst thingsSpec = `\noperations Public Private suffix=Op\nhierarchy things LEAF\nlevel node public=Public private=Private\n\tnode ReadOp\nwrapper view\n`\n\nfunc main() {\n\tn := &node{}\n\tif err := thingsInit(n); err != nil {\n\t\tfmt.Println(\"error:\", err)\n\t\treturn\n\t}\n\tfmt.Println((&view{state: n}).Read())\n}\n"

// A parenthesized leaf gets its wrapper, as the leaf node does: the
// generator finds the leaf's embedded heiry.Node through parentheses around
// the leaf, on an alias's right side, and around the struct the leaf is
// declared as.
func TestLeafWrapperThroughParentheses(t *testing.T) {
	const plain, parenthesized = "type node struct{ heiry.Node }", "type node (struct{ heiry.Node })"
	for _, tc := range []struct{ name, node, decls, leaf string }{
		{"the leaf node", plain, "", "node"},
		{"a parenthesized leaf", plain, "", "(node)"},
		{"an alias whose right side is parenthesized", plain, "type Btn = (node)", "Btn"},
		{"a leaf declared as a parenthesized struct", parenthesized, "", "node"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempModule(t, map[string]string{"main.go": strings.NewReplacer("NODE", tc.node, "DECLS", tc.decls, "LEAF", tc.leaf).Replace(wrapperLeafProgram)})
			mustGen(t, dir)
			if got := mustRun(t, dir); got != "node" {
				t.Fatalf("the wrapper dispatched to %q, want %q", got, "node")
			}
		})
	}
}

// A leaf accepted through an alias stays checked: the references pass the
// leaf to the last level's reference factory, so an alias that later names
// another type fails the build in the generated file, although the
// specification did not change.
func TestLeafAliasThatChanges(t *testing.T) {
	dir := leafModule(t, "type Btn = node\n\ntype other struct{ heiry.Node }", "hierarchy things Btn\nlevel node public=Public private=Private\n", nil)
	mustGen(t, dir)
	mustBuild(t, dir)
	edit(t, dir, "main.go", "type Btn = node", "type Btn = other")
	if output := buildFails(t, dir, "cannot use leaf"); !strings.Contains(output, mainOutput) {
		t.Fatalf("the build failed outside %s:\n%s", mainOutput, output)
	}
}

// An alias declared only in a file that some builds leave out is followed
// like any other, as for any type declared that way: generation succeeds,
// and a build that leaves the file out fails in the generated file.
func TestLeafAliasInATestFile(t *testing.T) {
	dir := leafModule(t, "", "hierarchy things Btn\nlevel node public=Public private=Private\n", map[string]string{"main_test.go": "package main\n\ntype Btn = node\n"})
	mustGen(t, dir)
	if output, err := goTool(t, dir, nil, "vet", "."); err != nil {
		t.Fatalf("go vet, which compiles the test file, failed:\n%s", output)
	}
	if output := buildFails(t, dir, "undefined: Btn"); !strings.Contains(output, mainOutput) {
		t.Fatalf("the build failed outside %s:\n%s", mainOutput, output)
	}
}

// leafSources is the handwritten package of the refusal cases. It needs to
// parse, not to compile.
const leafSources = "package main\n\ntype (\n\tPublic  interface{}\n\tPrivate interface{}\n)\n\ntype node struct{}\n\nDECLS\n\n//heiry:spec\nconst thingsSpec = `\nSPEC`\n"

func leafDir(t *testing.T, decls, spec string, extra map[string]string) string {
	t.Helper()
	files := map[string]string{"main.go": strings.Replace(strings.Replace(leafSources, "DECLS", decls, 1), "SPEC", spec, 1)}
	for name, content := range extra {
		files[name] = content
	}
	return sourceDir(t, files)
}

// A spelling that is still refused gets the reason after the message that
// names both spellings. Spelled identically, each of these is accepted, as
// today, before any alias is followed.
func TestLeafRefusalReasons(t *testing.T) {
	twice := map[string]string{"a.go": "package main\n\ntype Btn = node\n", "b.go": "package main\n\ntype Btn = node\n"}
	dot := map[string]string{"alias.go": "package main\n\nimport . \"example.com/lib\"\n\ntype Btn = Base\n"}
	unread := map[string]string{"alias.go": "package main\n\nimport \"example.com/go-ui\"\n\ntype Btn = ui.Base\n"}
	for _, tc := range []struct {
		name, decls, imports, leaf, last string
		extra                            map[string]string
		reason                           string
	}{
		{"a name that is not declared", "", "", "Btn", "node", nil, "Btn is not declared"},
		{"a type of its own", "type Btn struct{}", "", "Btn", "node", nil, "Btn is declared as a type of its own, not as an alias"},
		{"an alias declared twice", "", "", "Btn", "node", twice, "Btn is declared more than once"},
		{"an alias with type parameters", "type box[T any] struct{}\n\ntype Btn[T any] = box[T]", "", "Btn[int]", "box[int]", nil, "Btn has type parameters"},
		{"an alias in another package", "", "import lib \"example.com/lib\"\n", "lib.Btn", "node", nil, "lib.Btn is declared in another package, whose aliases the generator does not read"},
		{"a dot-imported name", "", "import lib \"example.com/lib\"\n", "Btn", "lib.Base", dot, "Btn's right side uses Base from a dot import in alias.go; import that package with a name"},
		{"a cycle of aliases", "type (\n\tBtn  = Base\n\tBase = Btn\n)", "", "Btn", "node", nil, "the alias Btn is in a cycle of aliases: Btn, Base, Btn"},
		{"an unreadable import", "", "import ui \"example.com/go-ui\"\n", "Btn", "ui.Base", unread, "Btn's right side uses ui, but the generator cannot tell which import of alias.go provides ui; give that import the explicit name ui"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := leafDir(t, tc.decls, "hierarchy things "+tc.leaf+"\n"+tc.imports+"level "+tc.last+" public=Public private=Private\n", tc.extra)
			want := "thingsSpec, line 2: the last level is " + tc.last + " but the leaf is " + tc.leaf + ": " + tc.reason
			if exit, message := gen(dir); exit != 1 || !strings.Contains(message, want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
			}
			// The control: the leaf spelled as its own last level. Its import
			// line is kept only when the leaf uses it.
			imports := ""
			if strings.Contains(tc.leaf, ".") {
				imports = tc.imports
			}
			control := leafDir(t, tc.decls, "hierarchy things "+tc.leaf+"\n"+imports+"level "+tc.leaf+" public=Public private=Private\n", tc.extra)
			exit, message := gen(control)
			if strings.Contains(message, "the last level is") {
				t.Fatalf("identical spellings %s were refused by the leaf comparison:\n%s", tc.leaf, message)
			}
			if tc.name == "a name that is not declared" {
				// Accepted by the leaf comparison, the name is refused by the
				// later check of undeclared names.
				if exit != 1 || !strings.Contains(message, "Btn is not declared in the handwritten files") {
					t.Errorf("exit %d, want the undeclared-name refusal:\n%s", exit, message)
				}
			} else if exit != 0 {
				t.Errorf("identical spellings %s: exit %d:\n%s", tc.leaf, exit, message)
			}
		})
	}
}

// mismatchFile is a file whose constant has a leaf that is not its last
// level: "the last level is base but the leaf is top", at the hierarchy
// line. before goes above the hierarchy line, inside after the level line,
// and after the constant. With decls, the file declares the types too.
type mismatchFile struct {
	constName, before, inside, after string
	decls                            bool
}

func (f mismatchFile) String() string {
	source := "package main\n\n"
	if f.decls {
		source += "type (\n\tP interface{}\n\tQ interface{}\n)\n\ntype top struct{}\n\ntype base struct{}\n\n"
	}
	return source + "//heiry:spec\nconst " + f.constName + " = `\n" + f.before + "hierarchy shape top\nlevel base public=P private=Q\n" + f.inside + "`\n" + f.after
}

// The leaf comparison runs after every file is read. A leaf mismatch is now
// reported after every problem that reading finds in a later constant or a
// later file, after any misplaced marker in the same file, and after every
// import or type-expression problem in the same constant, wherever its line
// is. A problem in an earlier file or an earlier constant, a grammar error
// in the same constant, and a Go syntax or build-constraint error anywhere
// in the same file still come first, and problems found by later checks
// still come after it.
func TestLeafMismatchOrder(t *testing.T) {
	grammar := func(constName string) string {
		return "//heiry:spec\nconst " + constName + " = `\nlevle x\n`\n"
	}
	a := mismatchFile{constName: "aSpec", decls: true}
	with := func(f mismatchFile, edit func(*mismatchFile)) string {
		edit(&f)
		return f.String()
	}
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		// Each of these reported the leaf mismatch first before ADR 0014.
		{"a grammar error in a later file",
			map[string]string{"a.go": a.String(), "b.go": "package main\n\n" + grammar("bSpec")},
			"bSpec, line 2: unknown keyword"},
		{"an unused import in the same constant",
			map[string]string{"a.go": with(a, func(f *mismatchFile) { f.before = "import fmt \"fmt\"\n" })},
			"aSpec, line 2: import fmt is not used"},
		{"a grammar error in a later constant of the same file",
			map[string]string{"a.go": with(a, func(f *mismatchFile) { f.after = "\n" + grammar("cSpec") })},
			"cSpec, line 2: unknown keyword"},
		{"a Go syntax error in a later file",
			map[string]string{"a.go": a.String(), "b.go": "package main\n\nfunc (\n"},
			"b.go:3:"},
		{"a misplaced marker later in the same file",
			map[string]string{"a.go": with(a, func(f *mismatchFile) { f.after = "\n// heiry:spec\nvar x = 1\n" })},
			"is not a marker"},
		{"a misplaced marker earlier in the same file, since markers are checked after the constants",
			map[string]string{"a.go": strings.Replace(a.String(), "package main\n", "package main\n\n// heiry:spec\n", 1)},
			"is not a marker"},
		{"a type expression that does not parse, in the same constant",
			map[string]string{"a.go": with(a, func(f *mismatchFile) { f.inside = "\tnode P)\n" })},
			"aSpec, line 4: \"P)\" is not a Go type expression"},
		{"a package name with no import line, in the same constant",
			map[string]string{"a.go": with(a, func(f *mismatchFile) { f.inside = "\tnode fmt.Stringer\n" })},
			"aSpec, line 4: fmt.Stringer uses package fmt"},
		{"a duplicate import name above the hierarchy line",
			map[string]string{"a.go": with(a, func(f *mismatchFile) { f.before = "import x \"a\"\nimport x \"b\"\n" })},
			"aSpec, line 3: import name x is declared twice"},
		{"a hierarchy with no level, in a later file",
			map[string]string{"a.go": a.String(), "b.go": "package main\n\n//heiry:spec\nconst bSpec = `\nhierarchy other top\n`\n"},
			"bSpec, line 2: hierarchy other has no level"},
		{"two //go:build lines in a later file",
			map[string]string{"a.go": a.String(), "b.go": "//go:build linux\n//go:build darwin\n\npackage main\n"},
			"more than one //go:build line"},
		{"a marked constant that is not one string literal, in a later file",
			map[string]string{"a.go": a.String(), "b.go": "package main\n\n//heiry:spec\nconst bSpec = \"a\" + \"b\"\n"},
			"constant is one string literal"},

		// Each of these is reported first, as before ADR 0014.
		{"a problem in an earlier file",
			map[string]string{"a.go": "package main\n\n" + grammar("aSpec"), "b.go": mismatchFile{constName: "bSpec", decls: true}.String()},
			"aSpec, line 2: unknown keyword"},
		{"a problem in an earlier constant of the same file",
			map[string]string{"a.go": strings.Replace(mismatchFile{constName: "bSpec", decls: true}.String(), "//heiry:spec\n", grammar("aSpec")+"\n//heiry:spec\n", 1)},
			"aSpec, line 2: unknown keyword"},
		{"a grammar error in the same constant",
			map[string]string{"a.go": with(a, func(f *mismatchFile) { f.inside = "levle x\n" })},
			"aSpec, line 4: unknown keyword"},
		{"a Go syntax error in the same file",
			map[string]string{"a.go": with(a, func(f *mismatchFile) { f.after = "\nfunc (\n" })},
			"expected"},
		{"a build-constraint error in the same file",
			map[string]string{"a.go": "//go:build linux\n//go:build darwin\n\n" + a.String()},
			"more than one //go:build line"},
		{"a problem found by a later check, in an earlier file",
			map[string]string{"a.go": "package main\n\ntype (\n\tP interface{}\n\tQ interface{}\n)\n\ntype top struct{}\n\ntype base struct{}\n\n//heiry:spec\nconst aSpec = `\nhierarchy other top\nlevel top public=P private=Q\n\tnode Missing\n`\n", "b.go": mismatchFile{constName: "bSpec"}.String()},
			"bSpec, line 2: the last level is base but the leaf is top"},
		{"two leaf mismatches",
			map[string]string{"a.go": a.String(), "b.go": strings.Replace(mismatchFile{constName: "bSpec"}.String(), "hierarchy shape top", "hierarchy other top", 1)},
			"aSpec, line 2: the last level is base but the leaf is top"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, tc.want, message)
			}
		})
	}
}
