package generator

import (
	"strconv"
	"strings"
	"testing"
)

// ADR 0014, part five: part and use. A part is a named group of component
// lines; use adds them to a level, at its own position, with paths relative
// to that level's node. The output is the written-out hierarchy's, apart
// from the freshness assertions and the probes' comments.

const confirmWidget = "package main\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/lucaf/heiry\"\n)\n\ntype (\n\tPublic interface {\n\t\tName() string\n\t\tConfirm() string\n\t}\n\tPrivate interface{}\n)\n\ntype confirmation struct{ heiry.Node }\n\nfunc (*confirmation) Confirm() string { return \"confirmed\" }\n\ntype base struct{ heiry.Node }\n\nfunc (*base) Name() string    { return \"base\" }\nfunc (*base) Confirm() string { return \"unconfirmed\" }\n\n" +
	"//heiry:spec\nconst opsSpec = `\noperations Public Private suffix=Op\n`\n\n" +
	"//heiry:spec\nconst widgetsChain = `\nchain widgets\nlevel base public=Public private=Private\n\tnode NameOp ConfirmOp\n`\n\n" +
	"func main() {\n\tb := &button{}\n\tif err := buttonsInit(b); err != nil {\n\t\tfmt.Println(\"error:\", err)\n\t\treturn\n\t}\n\tm := &menuItem{}\n\tif err := menuItemsInit(m); err != nil {\n\t\tfmt.Println(\"error:\", err)\n\t\treturn\n\t}\n\tfmt.Println(b.Node.Virtual[ConfirmOp]().Confirm(), m.Node.Virtual[ConfirmOp]().Confirm(), m.Node.Virtual[NameOp]().Name())\n}\n"

const confirmPart = "package main\n\n" + "//heiry:spec\nconst confirmingPart = `\npart confirming\n\tcomponent confirm confirmation ConfirmOp\n`\n"

const confirmButton = "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype button struct {\n\tbase\n\theiry.Node\n\tconfirm confirmation\n}\n\n" +
	"//heiry:spec\nconst buttonSpec = `\nhierarchy buttons button from=widgets\nlevel button\n\tuse confirming\n`\n"

// The menu item's hierarchy has no from=: it uses the part at its own root.
const confirmMenu = "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype menuItem struct {\n\theiry.Node\n\tconfirm confirmation\n}\n\nfunc (*menuItem) Name() string { return \"menu item\" }\n\n" +
	"//heiry:spec\nconst menuItemSpec = `\nhierarchy menuItems menuItem\nlevel menuItem public=Public private=Private\n\tnode NameOp\n\tuse confirming\n`\n"

func confirmFiles() map[string]string {
	return map[string]string{"widget.go": confirmWidget, "part.go": confirmPart, "button.go": confirmButton, "menu.go": confirmMenu}
}

// A part's lines generate, build, and dispatch at every level that uses
// them, and the outputs name the constants where a fix can go.
func TestPartSharedComponents(t *testing.T) {
	dir := tempModule(t, confirmFiles())
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "confirmed confirmed menu item" {
		t.Fatalf("output = %q", got)
	}
	button := read(t, dir, outputName("button.go"))
	for _, want := range []string{
		"// The declarations below also take lines from the constant widgetsChain.\n",
		"// The declarations below also take lines from the constant confirmingPart.\n",
		"// button declares the field confirm of its component confirm. Its component line is in confirmingPart, used in buttonSpec.\n",
	} {
		if !strings.Contains(button, want) {
			t.Errorf("%s lacks %q", outputName("button.go"), want)
		}
	}
	// A hierarchy without from= that uses a part gets the part's assertion
	// and the shared probe comments too.
	menu := read(t, dir, outputName("menu.go"))
	for _, want := range []string{
		"// The declarations below also take lines from the constant confirmingPart.\n",
		"missing from one of the constants that the probe's comment names.\n",
		"// menuItem declares the field confirm of its component confirm. Its component line is in confirmingPart, used in menuItemSpec.\n",
	} {
		if !strings.Contains(menu, want) {
			t.Errorf("%s lacks %q:\n%s", outputName("menu.go"), want, menu)
		}
	}
	// The part's own output holds its assertion and nothing for its lines.
	part := read(t, dir, outputName("part.go"))
	if strings.Count(part, "// The declarations below") != 1 || strings.Contains(part, "Define[") || strings.Contains(part, "\nimport") {
		t.Errorf("the part's output holds more than its assertion:\n%s", part)
	}
	// An edit to the part fails every output that takes its lines.
	edit(t, dir, "part.go", "part confirming\n", "part confirming\n// an edit\n")
	output := buildFails(t, dir, "duplicate key")
	for _, source := range []string{"part.go", "button.go", "menu.go"} {
		if !strings.Contains(output, outputName(source)) {
			t.Errorf("the edited part did not fail %s:\n%s", outputName(source), output)
		}
	}
}

// The output of a hierarchy that uses a part is the written-out
// hierarchy's.
func TestPartWrittenOutForm(t *testing.T) {
	contracts := "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype (\n\tPublic  interface{}\n\tPrivate interface{}\n)\n\ntype subject struct{ heiry.Node }\n\n"
	part := "package main\n\n" + specConst("pSpec", "import plan \"example.com/plan\"\npart p\n\tcomponent store subject plan.leaf\n")
	t.Run("a hierarchy without from= that uses a part at its own level", func(t *testing.T) {
		top := contracts + "type top struct {\n\theiry.Node\n\tstore subject\n}\n\ntype StoreOp interface{}\n\n"
		files := map[string]string{"part.go": part, "main.go": top + specConst("tSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tcomponent store subject StoreOp\n\tuse p\n")}
		compareWrittenOut(t, files, "main.go", top+specConst("tSpec", "hierarchy things top\nimport plan \"example.com/plan\"\nlevel top public=Public private=Private\n\tcomponent store subject StoreOp\n\tcomponent store subject plan.leaf\n"))
		out := read(t, generatedDir(t, files), mainOutput)
		for _, want := range []string{
			"// The declarations below also take lines from the constant pSpec.\n",
			"\tplan \"example.com/plan\"\n",
			"plan_ *heiry.Plan[top], leaf_ *top",
			"missing from one of the constants that the probe's comment names.\n",
			"// top declares the field store of its component store. Its component lines are in tSpec and in pSpec, used in tSpec.\n",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("want %q in:\n%s", want, out)
			}
		}
	})
	t.Run("a part used through a chain", func(t *testing.T) {
		chain := contracts + "type base struct {\n\theiry.Node\n\tstore subject\n}\n\n" + specConst("cSpec", "chain c\nlevel base public=Public private=Private\n\tuse p\n")
		top := "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype top struct {\n\tbase\n\theiry.Node\n}\n\n"
		files := map[string]string{"part.go": part, "chain.go": chain, "main.go": top + specConst("tSpec", "hierarchy things top from=c\nlevel top\n")}
		compareWrittenOut(t, files, "main.go", top+specConst("tSpec", "hierarchy things top\nimport plan \"example.com/plan\"\nlevel base public=Public private=Private\n\tcomponent store subject plan.leaf\nlevel top\n"))
		if out := read(t, generatedDir(t, files), mainOutput); !strings.Contains(out, "// base declares the field store of its component store. Its component line is in pSpec, used in cSpec.\n") {
			t.Errorf("the probe does not name the use line's constant:\n%s", out)
		}
	})
	t.Run("a part used at two levels, with one assertion for it", func(t *testing.T) {
		types := contracts + "type mid struct {\n\theiry.Node\n\tstore subject\n}\n\ntype top struct {\n\tmid\n\theiry.Node\n\tstore subject\n}\n\n"
		files := map[string]string{"part.go": part, "main.go": types + specConst("tSpec", "hierarchy things top\nlevel mid public=Public private=Private\n\tuse p\nlevel top\n\tuse p\n")}
		compareWrittenOut(t, files, "main.go", types+specConst("tSpec", "hierarchy things top\nimport plan \"example.com/plan\"\nlevel mid public=Public private=Private\n\tcomponent store subject plan.leaf\nlevel top\n\tcomponent store subject plan.leaf\n"))
		if out := read(t, generatedDir(t, files), mainOutput); strings.Count(out, "also take lines from the constant pSpec.") != 1 {
			t.Errorf("want one assertion for pSpec:\n%s", out)
		}
	})
}

// partSources is a package for the refusals; it needs to parse, not to
// compile.
const partSources = chainSources + "type store struct{}\n\ntype other struct{}\n\n"

func TestPartRefusals(t *testing.T) {
	part := specConst("pSpec", "part p\n\tcomponent store store Public\n")
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"a use that names a hierarchy",
			map[string]string{"a.go": partSources + specConst("aSpec", "hierarchy things base\nlevel base public=Public private=Private\n") + specConst("bSpec", "hierarchy others top\nlevel top public=Public private=Private\n\tuse things\n")},
			"bSpec, line 4: use things names the hierarchy things, but use names a part"},
		{"a use that names a chain",
			map[string]string{"a.go": partSources + specConst("aSpec", "chain c\nlevel base public=Public private=Private\n") + specConst("bSpec", "hierarchy others top\nlevel top public=Public private=Private\n\tuse c\n")},
			"bSpec, line 4: use c names the chain c, but use names a part"},
		{"a from= that names a part",
			map[string]string{"a.go": partSources + part + specConst("bSpec", "hierarchy others top from=p\nlevel top\n")},
			"bSpec, line 2: from=p names the part p, but from= names a hierarchy or chain"},
		{"an unknown part",
			map[string]string{"a.go": partSources + specConst("bSpec", "hierarchy others top\nlevel top public=Public private=Private\n\tuse nothing\n")},
			"bSpec, line 4: use nothing names no part of package main"},
		{"a part of the other package clause",
			map[string]string{"a.go": partSources + part, "x_test.go": "package main_test\n\n" + specConst("xSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse p\n")},
			"xSpec, line 4: use p names the part p of package main, but this specification is in package main_test; neither can see the other's constants"},
		{"a part and a chain of one name",
			map[string]string{"a.go": partSources + specConst("aSpec", "chain p\nlevel base public=Public private=Private\n") + part},
			"pSpec, line 2: the name p is already declared by the chain at "},
		{"a use line before any level",
			map[string]string{"a.go": partSources + part + specConst("bSpec", "hierarchy things top\nuse p\n")},
			"bSpec, line 3: a use line needs a level line above it"},
		{"a use line after the wrapper",
			map[string]string{"a.go": partSources + part + specConst("bSpec", "hierarchy things top\nlevel top public=Public private=Private\nwrapper view\n\tuse p\n")},
			"bSpec, line 5: the wrapper line follows the last level"},
		{"a use line with no hierarchy or chain line",
			map[string]string{"a.go": partSources + part + specConst("bSpec", "use p\n")},
			"bSpec, line 2: use needs a preceding line: hierarchy <Name> <LeafType> or chain <Name>"},
		{"a use line in a part",
			map[string]string{"a.go": partSources + specConst("pSpec", "part p\n\tcomponent store store Public\n\tuse q\n")},
			"pSpec, line 4: only component lines follow a part line: a part holds no use line"},
		{"a level line in a part",
			map[string]string{"a.go": partSources + specConst("pSpec", "part p\nlevel top\n")},
			"pSpec, line 3: only component lines follow a part line: a part holds no level line"},
		{"an import line after the part line",
			map[string]string{"a.go": partSources + specConst("pSpec", "part p\nimport lib \"example.com/lib\"\n\tcomponent store store Public\n")},
			"pSpec, line 3: only component lines follow a part line: write the import line before it"},
		{"a part line in a hierarchy's constant",
			map[string]string{"a.go": partSources + specConst("pSpec", "hierarchy things top\nlevel top public=Public private=Private\npart p\n")},
			"pSpec, line 4: a specification holds at most one hierarchy, chain, or part; use another constant for another one"},
		{"a part line after a chain line",
			map[string]string{"a.go": partSources + specConst("pSpec", "chain c\nlevel top public=Public private=Private\npart p\n")},
			"pSpec, line 4: a specification holds at most one hierarchy, chain, or part; use another constant for another one"},
		{"a hierarchy line after a part line",
			map[string]string{"a.go": partSources + specConst("pSpec", "part p\n\tcomponent store store Public\nhierarchy things top\n")},
			"pSpec, line 4: a specification holds at most one hierarchy, chain, or part; use another constant for another one"},
		{"a chain line after a part line",
			map[string]string{"a.go": partSources + specConst("pSpec", "part p\n\tcomponent store store Public\nchain c\n")},
			"pSpec, line 4: a specification holds at most one hierarchy, chain, or part; use another constant for another one"},
		{"a part with no component line",
			map[string]string{"a.go": partSources + specConst("pSpec", "part p\n")},
			"pSpec, line 2: part p has no component line"},
		{"a malformed part line",
			map[string]string{"a.go": partSources + specConst("pSpec", "part p q\n")},
			"pSpec, line 2: want: part <Name>"},
		{"a malformed use line",
			map[string]string{"a.go": partSources + part + specConst("bSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse\n")},
			"bSpec, line 4: want: use <Part>"},
		{"two provider types for one path in a part, as for a level",
			map[string]string{"a.go": partSources + specConst("pSpec", "part p\n\tcomponent store store Public\n\tcomponent store other Public\n")},
			"pSpec, line 4: component store is declared with provider types store and other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, tc.want, message)
			}
		})
	}
	// A part name declares nothing in Go, so it may equal a Go name.
	files := map[string]string{"a.go": partSources + specConst("pSpec", "part store\n\tcomponent store store Public\n") + specConst("bSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse store\n")}
	if exit, message := gen(sourceDir(t, files)); exit != 0 {
		t.Errorf("a part named like a type of the package was refused:\n%s", message)
	}
}

// A constant that a use line names must be compiled in every build that
// compiles the one that names it, with the error at the use line.
func TestPartBuildRule(t *testing.T) {
	part := "package main\n\n" + specConst("pSpec", "part p\n\tcomponent store store Public\n")
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"a hierarchy that uses a test file's part",
			map[string]string{"main.go": partSources + specConst("hSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse p\n"), "part_test.go": part},
			"hSpec, line 4: use p names the part p, declared in part_test.go, which some builds that compile main.go leave out; declare it in main.go, in a non-test file without build constraints or a platform suffix, or in a file with exactly the conditions of main.go"},
		{"a hierarchy that uses a part some builds leave out",
			map[string]string{"main.go": partSources + specConst("hSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse p\n"), "part_linux.go": part},
			"hSpec, line 4: use p names the part p, declared in part_linux.go, which some builds that compile main.go leave out"},
		{"a chain that uses a test file's part",
			map[string]string{"main.go": partSources + specConst("cSpec", "chain c\nlevel top public=Public private=Private\n\tuse p\n"), "part_test.go": part},
			"cSpec, line 4: use p names the part p, declared in part_test.go, which some builds that compile main.go leave out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, tc.want, message)
			}
		})
	}
	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"a file with the same conditions", map[string]string{"main_linux.go": partSources + specConst("hSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse p\n"), "part_linux.go": part}},
		{"a test hierarchy that uses an ordinary part", map[string]string{"main.go": partSources, "part.go": part, "main_test.go": "package main\n\n" + specConst("hSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse p\n")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if exit, message := gen(sourceDir(t, tc.files)); exit != 0 {
				t.Errorf("refused:\n%s", message)
			}
		})
	}
}

// Component lines with one path at one level merge into one slot, from the
// level and from parts, as in the written-out form. Two provider types for
// one slot, from lines of different constants, are reported at the later
// line in written-out order, naming both constants.
func TestPartComponentLines(t *testing.T) {
	part := func(constName, name, provider string) string {
		return specConst(constName, "part "+name+"\n\tcomponent store "+provider+" Public\n")
	}
	partFile := "package main\n\n" + part("pSpec", "p", "other")
	partLine := strconv.Itoa(lineOf(t, partFile, "\tcomponent store other"))
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"a part's line after the level's own",
			map[string]string{"p.go": partFile, "main.go": partSources + specConst("hSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tcomponent store store Public\n\tuse p\n")},
			"p.go:" + partLine + ":2: pSpec, line 3 (taken by hSpec through use p): component store is declared with provider types store, in hSpec, and other, in pSpec"},
		{"the level's own line after a part's",
			map[string]string{"p.go": partFile, "main.go": partSources + specConst("hSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse p\n\tcomponent store store Public\n")},
			"hSpec, line 5: component store is declared with provider types other, in pSpec, and store, in hSpec"},
		{"two parts at one level",
			map[string]string{"p.go": partFile + "\n" + part("qSpec", "q", "store"), "main.go": partSources + specConst("hSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse p\n\tuse q\n")},
			"qSpec, line 3 (taken by hSpec through use q): component store is declared with provider types other, in pSpec, and store, in qSpec"},
		{"a chain that nothing uses",
			map[string]string{"p.go": partFile, "main.go": partSources + specConst("cSpec", "chain unused\nlevel top public=Public private=Private\n\tcomponent store store Public\n\tuse p\n")},
			"pSpec, line 3: component store is declared with provider types store, in cSpec, and other, in pSpec"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, tc.want, message)
			}
		})
	}
	t.Run("a slot merged from two parts", func(t *testing.T) {
		parts := "package main\n\n" + part("pSpec", "p", "store") + part("qSpec", "q", "store")
		files := map[string]string{"p.go": parts, "main.go": partSources + specConst("hSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse p\n\tuse q\n")}
		want := "// top declares the field store of its component store. Its component lines are in pSpec, used in hSpec, and in qSpec, used in hSpec.\n"
		if out := read(t, generatedDir(t, files), mainOutput); !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	})
	t.Run("a merged slot", func(t *testing.T) {
		files := map[string]string{"p.go": "package main\n\n" + part("pSpec", "p", "store"), "main.go": partSources + "type StoreOp interface{}\n\n" + specConst("hSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tcomponent log other StoreOp\n\tcomponent store store StoreOp\n\tuse p\n")}
		out := read(t, generatedDir(t, files), mainOutput)
		for _, want := range []string{
			"Bind[StoreOp]((*store)(nil), \"store\").\n\tBind[Public]((*store)(nil), \"store\")",
			"// top declares the field store of its component store. Its component lines are in hSpec and in pSpec, used in hSpec.\n",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("want %q in:\n%s", want, out)
			}
		}
		if strings.Count(out, "declares the field store of its component store") != 1 {
			t.Errorf("want one probe for the merged slot:\n%s", out)
		}
	})
}

// The added assertions and the imports of taken lines follow the order in
// which each level's lines are written, node lines included, with each use
// line replaced by its part's lines; a conflict between two taken lines is
// reported at the one written second.
func TestPartWrittenOrder(t *testing.T) {
	level := "hierarchy things top\nlevel top public=Public private=Private\n\tcomponent a store Public\n\tuse p\n\tuse q\n"
	t.Run("the assertions, when a later part's line merges into an earlier slot", func(t *testing.T) {
		parts := "package main\n\n" + specConst("pSpec", "part p\n\tcomponent b store Public\n") + specConst("qSpec", "part q\n\tcomponent a store Private\n")
		out := read(t, generatedDir(t, map[string]string{"parts.go": parts, "main.go": partSources + specConst("hSpec", level)}), mainOutput)
		p, q := strings.Index(out, "also take lines from the constant pSpec."), strings.Index(out, "also take lines from the constant qSpec.")
		if p < 0 || q < 0 || p > q {
			t.Errorf("want pSpec's assertion before qSpec's, as their lines are written:\n%s", out)
		}
	})
	t.Run("an import conflict between two parts, at the line written second", func(t *testing.T) {
		parts := "package main\n\n" + specConst("pSpec", "import lib \"example.com/one\"\npart p\n\tcomponent b store lib.Op\n") + specConst("qSpec", "import lib \"example.com/two\"\npart q\n\tcomponent a store lib.Op\n")
		exit, message := gen(sourceDir(t, map[string]string{"parts.go": parts, "main.go": partSources + specConst("hSpec", level)}))
		want := "parts.go:" + strconv.Itoa(lineOf(t, parts, "\tcomponent a store lib.Op")) + ":2: qSpec, line 4 (taken by hSpec through use q): import name lib is used for \"example.com/one\" and \"example.com/two\" in one generated file, by pSpec, line 2: import lib \"example.com/one\", and by qSpec, line 2: import lib \"example.com/two\"; give one of the imports another name"
		if exit != 1 || !strings.Contains(message, want) {
			t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
		}
	})
	t.Run("an import conflict between a part and a later node line, at the node line", func(t *testing.T) {
		part := "package main\n\n" + specConst("pSpec", "import lib \"example.com/one\"\npart p\n\tcomponent store store lib.Op\n")
		chain := partSources + specConst("cSpec", "import lib \"example.com/two\"\nchain c\nlevel base public=Public private=Private\n\tuse p\n\tnode lib.Op\n")
		exit, message := gen(sourceDir(t, map[string]string{"part.go": part, "chain.go": chain, "main.go": "package main\n\n" + specConst("hSpec", "hierarchy things top from=c\nlevel top\n")}))
		want := "chain.go:" + strconv.Itoa(lineOf(t, chain, "\tnode lib.Op")) + ":2: cSpec, line 6 (taken by hSpec through from=c): import name lib is used for \"example.com/one\" and \"example.com/two\" in one generated file, by pSpec, line 2: import lib \"example.com/one\", and by cSpec, line 2: import lib \"example.com/two\"; give one of the imports another name"
		if exit != 1 || !strings.Contains(message, want) {
			t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
		}
	})
}

// A merged slot's provider keeps each line's meaning: it is read at every
// line that adds to the slot, with that line's constant's imports, so a
// spelling whose import name two constants bind to different paths is
// refused as an import-name conflict, in either order, at the taken line,
// or at the second taken line when both are taken.
func TestPartMergedProviderImports(t *testing.T) {
	partFile := "package main\n\n" + specConst("pSpec", "import lib \"example.com/one\"\npart p\n\tcomponent store lib.Store Public\n")
	partLine := strconv.Itoa(lineOf(t, partFile, "\tcomponent store lib.Store Public"))
	ownLine, useLine := "\tcomponent store lib.Store Private\n", "\tuse p\n"
	hierarchy := func(lines string) string {
		return partSources + specConst("hSpec", "import lib \"example.com/two\"\nhierarchy things top\nlevel top public=Public private=Private\n"+lines)
	}
	chain := func(lines string) string {
		return partSources + specConst("cSpec", "import lib \"example.com/two\"\nchain c\nlevel base public=Public private=Private\n"+lines)
	}
	takesChain := "package main\n\n" + specConst("hSpec", "hierarchy things top from=c\nlevel top\n")
	chainUseFirst := chain(useLine + ownLine)
	parts := partFile + "\n" + specConst("qSpec", "import lib \"example.com/two\"\npart q\n\tcomponent store lib.Store Private\n")
	// conflict is the message at a position, with the bindings in the order
	// they are added.
	conflict := func(at, first, firstPath, second, secondPath string) string {
		return at + ": import name lib is used for \"" + firstPath + "\" and \"" + secondPath + "\" in one generated file, by " + first + ": import lib \"" + firstPath + "\", and by " + second + ": import lib \"" + secondPath + "\"; give one of the imports another name"
	}
	atPartLine := conflict("p.go:"+partLine+":2: pSpec, line 4 (taken by hSpec through use p)", "hSpec, line 2", "example.com/two", "pSpec, line 2", "example.com/one")
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"at a level of the hierarchy's constant, its own line first",
			map[string]string{"p.go": partFile, "main.go": hierarchy(ownLine + useLine)}, atPartLine},
		{"at a level of the hierarchy's constant, the use line first",
			map[string]string{"p.go": partFile, "main.go": hierarchy(useLine + ownLine)}, atPartLine},
		{"at a chain's level, its own line first",
			map[string]string{"p.go": partFile, "chain.go": chain(ownLine + useLine), "main.go": takesChain},
			conflict("p.go:"+partLine+":2: pSpec, line 4 (taken by hSpec through from=c, use p)", "cSpec, line 2", "example.com/two", "pSpec, line 2", "example.com/one")},
		{"at a chain's level, the use line first",
			map[string]string{"p.go": partFile, "chain.go": chainUseFirst, "main.go": takesChain},
			conflict("chain.go:"+strconv.Itoa(lineOf(t, chainUseFirst, ownLine))+":2: cSpec, line 6 (taken by hSpec through from=c)", "pSpec, line 2", "example.com/one", "cSpec, line 2", "example.com/two")},
		{"two parts whose lines merge into one slot",
			map[string]string{"parts.go": parts, "main.go": partSources + specConst("hSpec", "hierarchy things top\nlevel top public=Public private=Private\n\tuse p\n\tuse q\n")},
			conflict("parts.go:"+strconv.Itoa(lineOf(t, parts, "\tcomponent store lib.Store Private"))+":2: qSpec, line 4 (taken by hSpec through use q)", "pSpec, line 2", "example.com/one", "qSpec, line 2", "example.com/two")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, tc.want, message)
			}
		})
	}
	t.Run("the same path in both constants", func(t *testing.T) {
		files := map[string]string{"p.go": partFile, "main.go": strings.Replace(hierarchy(ownLine+useLine), "example.com/two", "example.com/one", 1)}
		compareWrittenOut(t, files, "main.go", partSources+specConst("hSpec", "import lib \"example.com/one\"\nhierarchy things top\nlevel top public=Public private=Private\n\tcomponent store lib.Store Private\n\tcomponent store lib.Store Public\n"))
	})
}

// A part's own output holds its freshness assertion, beside what the
// operations lines of its constant generate, and imports nothing for its
// lines.
func TestPartOwnOutput(t *testing.T) {
	t.Run("a part alone in its file whose lines use an import", func(t *testing.T) {
		labels := "package main\n\n" + specConst("labelingPart", "import fmt \"fmt\"\npart labeling\n\tcomponent label labeler fmt.Stringer\n")
		main := "package main\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/lucaf/heiry\"\n)\n\ntype (\n\tPublic  interface{ String() string }\n\tPrivate interface{}\n)\n\ntype labeler struct{ heiry.Node }\n\nfunc (*labeler) String() string { return \"a label\" }\n\ntype top struct {\n\theiry.Node\n\tlabel labeler\n}\n\n" +
			specConst("topSpec", "hierarchy tops top\nlevel top public=Public private=Private\n\tuse labeling\n") +
			"\nfunc main() {\n\tt := &top{}\n\tif err := topsInit(t); err != nil {\n\t\tfmt.Println(\"error:\", err)\n\t\treturn\n\t}\n\tfmt.Println(t.Node.Virtual[fmt.Stringer]().String())\n}\n"
		dir := tempModule(t, map[string]string{"labels.go": labels, "main.go": main})
		mustGen(t, dir)
		mustBuild(t, dir)
		if got := mustRun(t, dir); got != "a label" {
			t.Errorf("output = %q", got)
		}
		if out := read(t, dir, outputName("labels.go")); strings.Count(out, "// The declarations below") != 1 || strings.Contains(out, "\nimport") {
			t.Errorf("the part's output holds more than its assertion:\n%s", out)
		}
	})
	t.Run("a part beside an operations line in one constant", func(t *testing.T) {
		part := "package main\n\ntype (\n\tPublic  interface{ Name() string }\n\tPrivate interface{}\n)\n\n" + specConst("pSpec", "import ext \"example.com/ext\"\nimport lib \"example.com/lib\"\noperations Public Private\n\texisting Name ext.Namer\npart p\n\tcomponent store lib.Store lib.Op\n")
		out := read(t, generatedDir(t, map[string]string{"part.go": part}), outputName("part.go"))
		if !strings.Contains(out, "\text \"example.com/ext\"\n") || strings.Contains(out, "\tlib \"example.com/lib\"\n") {
			t.Errorf("want the import of the existing line and not the part's:\n%s", out)
		}
	})
}

// The lists name the use steps of a path.
func TestPartLists(t *testing.T) {
	part := "package main\n\n" + specConst("pSpec", "part p\n\tcomponent store store Missing\n")
	files := map[string]string{
		"p.go": part,
		"a.go": partSources + specConst("aSpec", "hierarchy as top\nlevel top public=Public private=Private\n\tuse p\n"),
		"b.go": "package main\n\ntype btop struct{}\n\n" + specConst("cSpec", "chain c\nlevel base public=Public private=Private\n\tuse p\n") + specConst("bSpec", "hierarchy bs btop from=c\nlevel btop\n"),
	}
	exit, message := gen(sourceDir(t, files))
	want := "pSpec, line 3 (taken by aSpec through use p, and by bSpec through from=c, use p): Missing is not declared in the handwritten files of package main"
	if exit != 1 || !strings.Contains(message, want) {
		t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
	}
	// A build-rule error at a chain's use line lists the hierarchies that
	// take the chain's lines.
	files = map[string]string{
		"main.go":      partSources + specConst("cSpec", "chain c\nlevel base public=Public private=Private\n\tuse p\n") + specConst("hSpec", "hierarchy things top from=c\nlevel top\n"),
		"part_test.go": "package main\n\n" + specConst("pSpec", "part p\n\tcomponent store store Public\n"),
	}
	exit, message = gen(sourceDir(t, files))
	want = "cSpec, line 4 (taken by hSpec through from=c): use p names the part p, declared in part_test.go"
	if exit != 1 || !strings.Contains(message, want) {
		t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
	}
}
