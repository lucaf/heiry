package generator

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ADR 0014, part three: chain and from=. A chain is a named run of levels
// that generates no declaration; from= on a hierarchy or chain line starts
// its levels with every level of the named hierarchy or chain. The output
// is the written-out hierarchy's, apart from the freshness assertions and
// the probes' comments.

// specConst is a marked constant holding text, whose first line is line 2.
func specConst(name, text string) string {
	return "//heiry:spec\nconst " + name + " = `\n" + text + "`\n"
}

// The widget program of the record, shortened: a chain with the root, a
// hierarchy that takes it and has a wrapper, and one that takes the first.
const widgetGo = "package main\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/lucaf/heiry\"\n)\n\ntype (\n\tPublic interface {\n\t\tName() string\n\t\tClick() string\n\t}\n\tPrivate interface{}\n)\n\ntype base struct{ heiry.Node }\n\nfunc (*base) Name() string  { return \"base\" }\nfunc (*base) Click() string { return \"base click\" }\n\n" +
	"//heiry:spec\nconst opsSpec = `\noperations Public Private suffix=Op\n`\n\n" +
	"//heiry:spec\nconst widgetsChain = `\nchain widgets\nlevel base public=Public private=Private\n\tnode NameOp ClickOp\n`\n\n" +
	"func main() {\n\tb := &button{}\n\tif err := buttonsInit(b); err != nil {\n\t\tfmt.Println(\"error:\", err)\n\t\treturn\n\t}\n\tl := &logged{}\n\tif err := loggedsInit(l); err != nil {\n\t\tfmt.Println(\"error:\", err)\n\t\treturn\n\t}\n\tw := &buttonView{state: b}\n\tfmt.Println(w.Name(), w.Click(), l.Node.Virtual[NameOp]().Name(), l.Node.Virtual[ClickOp]().Click())\n}\n"

const buttonGo = "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype button struct {\n\tbase\n\theiry.Node\n}\n\nfunc (*button) Click() string { return \"button click\" }\n\n" +
	"//heiry:spec\nconst buttonSpec = `\nhierarchy buttons button from=widgets\nlevel button\n\tnode ClickOp\nwrapper buttonView\n`\n"

const loggedGo = "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype logged struct {\n\tbutton\n\theiry.Node\n}\n\nfunc (*logged) Name() string { return \"logged\" }\n\n" +
	"//heiry:spec\nconst loggedSpec = `\nhierarchy loggeds logged from=buttons\nlevel logged\n\tnode NameOp\n`\n"

func widgetFiles() map[string]string {
	return map[string]string{"widget.go": widgetGo, "button.go": buttonGo, "logged.go": loggedGo}
}

// The shared levels generate, build, and dispatch, and the outputs say
// which constants a fix can go in.
func TestChainSharedLevels(t *testing.T) {
	dir := tempModule(t, widgetFiles())
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "base button click logged button click" {
		t.Fatalf("output = %q", got)
	}

	logged := read(t, dir, outputName("logged.go"))
	for _, want := range []string{
		"// The declarations below were generated from the constant loggedSpec.\n",
		"// The declarations below also take lines from the constant widgetsChain.\n// If the map literal fails to compile with a duplicate key, widgetsChain has\n// changed since then: run go generate. If the line above it fails, widgetsChain\n// is no longer a constant.\n",
		"// The declarations below also take lines from the constant buttonSpec.\n",
		"// Each probe below compiles only while a struct declares a field itself, as\n// the library requires: a level's parent field, and the first field of each\n// component's path. If a selector below is ambiguous or has another type,\n// that field is gone, has another type, or is reached only through another\n// embedded struct; for a parent field, a level between the two is then\n// missing from one of the constants that the probe's comment names.\n// Regenerating writes the same probes, so first correct a constant that the\n// probe's comment names, or the struct; then run go generate.\n",
		"// button declares its parent field base, of type base. Its level is in buttonSpec, after the levels of widgetsChain.\n",
		"// logged declares its parent field button, of type button. Its level is in loggedSpec, after the levels of buttonSpec.\n",
	} {
		if !strings.Contains(logged, want) {
			t.Errorf("%s lacks:\n%s", outputName("logged.go"), want)
		}
	}
	// The added assertions follow the hierarchy's own, in the order the
	// written-out hierarchy first uses a line of each constant.
	own := strings.Index(logged, "constant loggedSpec.")
	chain := strings.Index(logged, "constant widgetsChain.")
	button := strings.Index(logged, "constant buttonSpec.")
	if !(own < chain && chain < button) {
		t.Errorf("assertions out of order: loggedSpec %d, widgetsChain %d, buttonSpec %d", own, chain, button)
	}
	if strings.Count(logged, "// The declarations below") != 3 {
		t.Errorf("want exactly three assertions in %s:\n%s", outputName("logged.go"), logged)
	}

	// The chain's own output holds its assertion and the operations, and
	// nothing for the chain's levels.
	widget := read(t, dir, outputName("widget.go"))
	if !strings.Contains(widget, "constant widgetsChain.") || strings.Contains(widget, "Define[") || strings.Contains(widget, "\"github.com/lucaf/heiry\"") {
		t.Errorf("the chain's output declares something for its levels:\n%s", widget)
	}
}

// A level inserted between base and button in the structs alone fails the
// build of every output that has button's probe.
func TestChainProbeOnATakenLevel(t *testing.T) {
	dir := tempModule(t, widgetFiles())
	mustGen(t, dir)
	edit(t, dir, "button.go", "type button struct {\n\tbase\n", "type mid struct {\n\tbase\n\theiry.Node\n}\n\ntype button struct {\n\tmid\n")
	output := buildFails(t, dir, "ambiguous selector")
	for _, out := range []string{outputName("button.go"), outputName("logged.go")} {
		if !strings.Contains(output, out) {
			t.Errorf("the build did not fail in %s:\n%s", out, output)
		}
	}
	// A level that is second in its own constant names that constant only.
	files := widgetFiles()
	files["widget.go"] = strings.Replace(files["widget.go"], "level base public=Public private=Private\n\tnode NameOp ClickOp\n", "level root public=Public private=Private\nlevel base\n\tnode NameOp ClickOp\n", 1)
	files["widget.go"] = strings.Replace(files["widget.go"], "type base struct{ heiry.Node }", "type root struct{ heiry.Node }\n\ntype base struct {\n\troot\n\theiry.Node\n}", 1)
	dir = tempModule(t, files)
	mustGen(t, dir)
	mustBuild(t, dir)
	if want := "// base declares its parent field root, of type root. Its level is in widgetsChain.\n"; !strings.Contains(read(t, dir, outputName("button.go")), want) {
		t.Errorf("want %q in:\n%s", want, read(t, dir, outputName("button.go")))
	}
}

// A component probe names the constant that holds the slot's lines, once,
// also when several lines with one path merge into the slot.
func TestChainComponentProbeComments(t *testing.T) {
	chain := chainSources + specConst("cSpec", "chain c\nlevel base public=Public private=Private\n\tcomponent store storage NameOp\n\tcomponent store storage ClickOp\n\tcomponent log logger NameOp\n")
	out := read(t, generatedDir(t, map[string]string{"chain.go": chain, "main.go": "package main\n\ntype (\n\tNameOp  interface{}\n\tClickOp interface{}\n)\n\ntype storage struct{}\n\ntype logger struct{}\n\n" + specConst("hSpec", "hierarchy things top from=c\nlevel top\n")}), mainOutput)
	for _, want := range []string{
		"// base declares the field store of its component store. Its component lines are in cSpec.\n",
		"// base declares the field log of its component log. Its component line is in cSpec.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
}

// withoutSharedDifferences drops what the record lets the shared form
// differ in: the freshness assertions and the probes' comments.
func withoutSharedDifferences(output string) string {
	lines := strings.SplitAfter(output, "\n")
	var kept []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "// The declarations below "):
			for i < len(lines) && lines[i] != "}\n" {
				i++
			}
			continue
		case strings.HasPrefix(line, "// Each probe below compiles"):
			for i+1 < len(lines) && strings.HasPrefix(lines[i+1], "//") {
				i++
			}
			kept = append(kept, "// PROBES\n")
			continue
		case strings.HasPrefix(line, "// ") && (strings.Contains(line, " declares its parent field ") || strings.Contains(line, " declares the field ")):
			if cut := strings.Index(line, ". Its "); cut >= 0 {
				line = line[:cut+1] + "\n"
			}
		}
		if line == "\n" && len(kept) != 0 && kept[len(kept)-1] == "\n" {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "")
}

// compareWrittenOut generates the shared form and the written-out form, in
// which source's constant holds the hierarchy written out, and requires
// equal outputs: source's apart from the freshness assertions and the
// probes' comments, and exactly that of every other file that takes no
// lines, and so cannot depend on source.
func compareWrittenOut(t *testing.T, shared map[string]string, source, written string) {
	t.Helper()
	sharedDir := sourceDir(t, shared)
	if exit, message := gen(sharedDir); exit != 0 {
		t.Fatalf("shared form: %s", message)
	}
	out := map[string]string{}
	for name, content := range shared {
		out[name] = content
	}
	out[source] = written
	writtenDir := sourceDir(t, out)
	if exit, message := gen(writtenDir); exit != 0 {
		t.Fatalf("written-out form: %s", message)
	}
	for name := range shared {
		if strings.HasSuffix(name, ".heirygen.go") || !strings.HasSuffix(name, ".go") || (name != source && strings.Contains(shared[name], "from=")) {
			continue
		}
		a, b := read(t, sharedDir, outputName(name)), read(t, writtenDir, outputName(name))
		if name == source {
			a, b = withoutSharedDifferences(a), withoutSharedDifferences(b)
		}
		if a != b {
			t.Errorf("%s differs:\n--- shared\n%s\n--- written out\n%s", outputName(name), a, b)
		}
	}
}

const shapeContracts = "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype (\n\tPublic  interface{}\n\tPrivate interface{}\n)\n\n"

// shapeTop is the file of the hierarchy that takes the lines.
const shapeTop = "package main\n\nimport \"github.com/lucaf/heiry\"\n\n"

// The output of a hierarchy that takes lines is the written-out
// hierarchy's, on the shapes most likely to show a mistake.
func TestChainWrittenOutForm(t *testing.T) {
	t.Run("the widget program", func(t *testing.T) {
		files := widgetFiles()
		compareWrittenOut(t, files, "button.go", strings.Replace(buttonGo, "hierarchy buttons button from=widgets\n", "hierarchy buttons button\nlevel base public=Public private=Private\n\tnode NameOp ClickOp\n", 1))
		compareWrittenOut(t, files, "logged.go", strings.Replace(loggedGo, "hierarchy loggeds logged from=buttons\n", "hierarchy loggeds logged\nlevel base public=Public private=Private\n\tnode NameOp ClickOp\nlevel button\n\tnode ClickOp\n", 1))
	})
	t.Run("several own levels, and a chain that takes a hierarchy's levels", func(t *testing.T) {
		root := shapeContracts + "type base struct{ heiry.Node }\n\n" + specConst("rootSpec", "hierarchy roots base\nlevel base public=Public private=Private\n")
		mid := shapeTop + "type mid struct {\n\tbase\n\theiry.Node\n}\n\n" + specConst("midSpec", "chain mids from=roots\nlevel mid\n")
		top := shapeTop + "type low struct {\n\tmid\n\theiry.Node\n}\n\ntype top struct {\n\tlow\n\theiry.Node\n}\n\n"
		files := map[string]string{"root.go": root, "mid.go": mid, "top.go": top + specConst("tSpec", "hierarchy things top from=mids\nlevel low\nlevel top\n")}
		compareWrittenOut(t, files, "top.go", top+specConst("tSpec", "hierarchy things top\nlevel base public=Public private=Private\nlevel mid\nlevel low\nlevel top\n"))
	})
	t.Run("taken lines that use the output's default local names", func(t *testing.T) {
		chain := shapeContracts + "type leaf struct{ heiry.Node }\n\ntype generatedFrom struct{ heiry.Node }\n\n" + specConst("cSpec", "import plan \"example.com/plan\"\nimport probe \"example.com/probe\"\nchain c\nlevel leaf public=Public private=Private\n\tnode plan.Op\nlevel generatedFrom\n\tcomponent subject probe.Subject probe.Op\n")
		top := shapeTop + "type top struct {\n\tgeneratedFrom\n\theiry.Node\n}\n\n"
		files := map[string]string{"chain.go": chain, "top.go": top + specConst("tSpec", "hierarchy things top from=c\nlevel top\n")}
		compareWrittenOut(t, files, "top.go", top+specConst("tSpec", "hierarchy things top\nimport plan \"example.com/plan\"\nimport probe \"example.com/probe\"\nlevel leaf public=Public private=Private\n\tnode plan.Op\nlevel generatedFrom\n\tcomponent subject probe.Subject probe.Op\nlevel top\n"))
		shared := read(t, generatedDir(t, files), outputName("top.go"))
		for _, want := range []string{"leaf_ *top", "\tplan \"example.com/plan\"\n"} {
			if !strings.Contains(shared, want) {
				t.Errorf("want %q in:\n%s", want, shared)
			}
		}
	})
	t.Run("untaken lines that use default local names or an import of their own", func(t *testing.T) {
		buttons := strings.Replace(shapeContracts, "Public  interface{}", "Public  interface{ Name() string }", 1) + "type button struct{ heiry.Node }\n\n" + specConst("buttonSpec", "import lib \"example.com/lib\"\noperations Public Private\n\texisting Name lib.leaf\nhierarchy buttons button\nlevel button public=Public private=Private\n")
		top := shapeTop + "type top struct {\n\tbutton\n\theiry.Node\n}\n\n"
		files := map[string]string{"button.go": buttons, "top.go": top + specConst("tSpec", "hierarchy things top from=buttons\nlevel top\n")}
		compareWrittenOut(t, files, "top.go", top+specConst("tSpec", "hierarchy things top\nlevel button public=Public private=Private\nlevel top\n"))
		shared := read(t, generatedDir(t, files), outputName("top.go"))
		if strings.Contains(shared, "\tlib \"example.com/lib\"\n") || strings.Contains(shared, "leaf_") {
			t.Errorf("the output depends on lines it does not take:\n%s", shared)
		}
	})
	t.Run("a last taken level that is qualified or generic", func(t *testing.T) {
		for _, tc := range []struct{ imports, last string }{
			{"import lib \"example.com/lib\"\n", "lib.Base"},
			{"", "box[int]"},
		} {
			chain := shapeContracts + "type box[T any] struct{ heiry.Node }\n\n" + specConst("cSpec", tc.imports+"chain c\nlevel "+tc.last+" public=Public private=Private\n")
			top := shapeTop + "type top struct{ heiry.Node }\n\n"
			files := map[string]string{"chain.go": chain, "top.go": top + specConst("tSpec", "hierarchy things top from=c\nlevel top\n")}
			compareWrittenOut(t, files, "top.go", top+specConst("tSpec", "hierarchy things top\n"+tc.imports+"level "+tc.last+" public=Public private=Private\nlevel top\n"))
		}
	})
	t.Run("an inherited contract that is qualified", func(t *testing.T) {
		chain := shapeContracts + "type base struct{ heiry.Node }\n\n" + specConst("cSpec", "import lib \"example.com/lib\"\nchain c\nlevel base public=lib.Pub private=lib.Priv\n")
		top := shapeTop + "type top struct {\n\tbase\n\theiry.Node\n}\n\n"
		files := map[string]string{"chain.go": chain, "top.go": top + specConst("tSpec", "hierarchy things top from=c\nlevel top\n")}
		compareWrittenOut(t, files, "top.go", top+specConst("tSpec", "hierarchy things top\nimport lib \"example.com/lib\"\nlevel base public=lib.Pub private=lib.Priv\nlevel top\n"))
		if shared := read(t, generatedDir(t, files), outputName("top.go")); !strings.Contains(shared, "Level[top, lib.Pub, lib.Priv]") || !strings.Contains(shared, "\tlib \"example.com/lib\"\n") {
			t.Errorf("the inherited contract is not read through the chain's import:\n%s", shared)
		}
	})
	t.Run("a taken registration whose existing line is only in the chain's file", func(t *testing.T) {
		chain := "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype (\n\tPublic  interface{ Name() string }\n\tPrivate interface{}\n)\n\ntype Namer interface{ Name() string }\n\ntype base struct{ heiry.Node }\n\n" + specConst("opsSpec", "operations Public Private\n\texisting Name Namer\n") + "\n" + specConst("cSpec", "chain c\nlevel base public=Public private=Private\n\tnode Namer\n")
		top := "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype top struct {\n\tbase\n\theiry.Node\n}\n\n"
		files := map[string]string{"chain.go": chain, "top.go": top + specConst("tSpec", "hierarchy things top from=c\nlevel top\n")}
		compareWrittenOut(t, files, "top.go", top+specConst("tSpec", "hierarchy things top\nlevel base public=Public private=Private\n\tnode Namer\nlevel top\n"))
		if shared := read(t, generatedDir(t, files), outputName("top.go")); !strings.Contains(shared, "= Namer(nil)") {
			t.Errorf("the output does not repeat the guard of Namer:\n%s", shared)
		}
	})
}

// generatedDir returns a directory with files, generated.
func generatedDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := sourceDir(t, files)
	if exit, message := gen(dir); exit != 0 {
		t.Fatalf("generate: %s", message)
	}
	return dir
}

// Every output that still holds an old line of an edited constant fails to
// compile, also after a run that put only some of the new outputs in place.
func TestChainFreshness(t *testing.T) {
	dir := tempModule(t, widgetFiles())
	mustGen(t, dir)
	mustBuild(t, dir)
	edit(t, dir, "widget.go", "chain widgets\n", "chain widgets\n// an edit\n")
	output := buildFails(t, dir, "duplicate key")
	for _, source := range []string{"widget.go", "button.go", "logged.go"} {
		if !strings.Contains(output, outputName(source)) {
			t.Errorf("the edited chain did not fail %s:\n%s", outputName(source), output)
		}
	}
	// A run that fails partway: only the chain's and buttons' outputs are
	// new, so loggeds' old output still holds the chain's old lines.
	fresh := t.TempDir()
	for _, source := range []string{"widget.go", "button.go", "logged.go"} {
		write(t, fresh, source, read(t, dir, source))
	}
	if exit, message := gen(fresh); exit != 0 {
		t.Fatal(message)
	}
	for _, source := range []string{"widget.go", "button.go"} {
		write(t, dir, outputName(source), read(t, fresh, outputName(source)))
	}
	output = buildFails(t, dir, "duplicate key")
	if !strings.Contains(output, outputName("logged.go")) || strings.Contains(output, outputName("button.go")) {
		t.Errorf("want the build to fail in the stale %s only:\n%s", outputName("logged.go"), output)
	}
}

// chainSources is a package for the refusals; it needs to parse, not to
// compile.
const chainSources = "package main\n\ntype (\n\tPublic  interface{}\n\tPrivate interface{}\n)\n\ntype base struct{}\n\ntype top struct{}\n\n"

func TestChainRefusals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"a chain that takes its own levels",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain a from=a\nlevel top\n")},
			"aSpec, line 2: from=a makes a cycle of from=: a, a"},
		{"two chains that take each other's levels, at the first from= in file order",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain a from=b\nlevel top\n"), "b.go": "package main\n\n" + specConst("bSpec", "chain b from=a\nlevel base\n")},
			"aSpec, line 2: from=b makes a cycle of from=: a, b, a"},
		{"three chains that take each other's levels",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain a from=b\nlevel top\n"), "b.go": "package main\n\n" + specConst("bSpec", "chain b from=c\nlevel base\n"), "c.go": "package main\n\n" + specConst("cSpec", "chain c from=a\nlevel top\n")},
			"aSpec, line 2: from=b makes a cycle of from=: a, b, c, a"},
		{"a name of the other package clause",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain widgets\nlevel base public=Public private=Private\n"), "x_test.go": "package main_test\n\n" + specConst("xSpec", "hierarchy things top from=widgets\nlevel top\n")},
			"xSpec, line 2: from=widgets names the chain widgets of package main, but this specification is in package main_test; neither can see the other's constants"},
		{"a name of the external test package's clause",
			map[string]string{"a.go": chainSources + specConst("aSpec", "hierarchy things top from=widgets\nlevel top\n"), "x_test.go": "package main_test\n\n" + specConst("xSpec", "chain widgets\nlevel base public=Public private=Private\n")},
			"aSpec, line 2: from=widgets names the chain widgets of package main_test, but this specification is in package main; neither can see the other's constants"},
		{"a name declared twice, at its second declaration",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain widgets\nlevel base public=Public private=Private\n"), "b.go": "package main\n\n" + specConst("bSpec", "chain widgets\nlevel top public=Public private=Private\n")},
			"bSpec, line 2: the name widgets is already declared by the chain at "},
		{"a hierarchy and a chain of one name",
			map[string]string{"a.go": chainSources + specConst("aSpec", "hierarchy widgets top\nlevel top public=Public private=Private\n") + specConst("bSpec", "chain widgets\nlevel base public=Public private=Private\n")},
			"bSpec, line 2: the name widgets is already declared by the hierarchy at "},
		{"two hierarchies of one name, with the message of the generated-name check",
			map[string]string{"a.go": chainSources + specConst("aSpec", "hierarchy things top\nlevel top public=Public private=Private\n"), "b.go": "package main\n\n" + specConst("bSpec", "hierarchy things base\nlevel base public=Public private=Private\n")},
			"bSpec would generate things, which the specification at "},
		{"a first own level whose parent field cannot be derived",
			map[string]string{"a.go": chainSources + specConst("cSpec", "chain c\nlevel []int public=Public private=Private\n") + specConst("tSpec", "hierarchy things top from=c\nlevel top\n")},
			"tSpec, line 3: the parent field of top cannot be derived from []int; write parent=<Field>"},
		{"an unknown name",
			map[string]string{"a.go": chainSources + specConst("aSpec", "hierarchy things top from=widgetz\nlevel top\n")},
			"aSpec, line 2: from=widgetz names no hierarchy or chain of package main"},
		{"an unknown name in a chain that nothing uses",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain unused from=widgetz\nlevel top\n")},
			"aSpec, line 2: from=widgetz names no hierarchy or chain of package main"},
		{"a chain with omit=",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain c omit=InitPlan\nlevel base public=Public private=Private\n")},
			"aSpec, line 2: unknown chain option \"omit=InitPlan\"; the options are from="},
		{"a chain with a wrapper",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain c\nlevel base public=Public private=Private\nwrapper view\n")},
			"aSpec, line 4: a chain has no wrapper line; give the wrapper to a hierarchy that takes the chain's levels with from="},
		{"a chain with no level",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain buttons2\n")},
			"aSpec, line 2: chain buttons2 has no level"},
		{"a chain with from= and no level of its own",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain buttons2 from=buttons\n")},
			"aSpec, line 2: chain buttons2 has no level of its own; write from=buttons in the hierarchy instead"},
		{"a from= that names a hierarchy with no level of its own",
			map[string]string{"a.go": chainSources + specConst("aSpec", "hierarchy buttons base\nlevel base public=Public private=Private\n") + specConst("bSpec", "hierarchy buttons2 base from=buttons\n") + specConst("cSpec", "hierarchy things top from=buttons2\nlevel top\n")},
			"cSpec, line 2: buttons2 has no level of its own, so from= cannot name it; name its target directly, with from=buttons"},
		{"a hierarchy and a chain in one constant",
			map[string]string{"a.go": chainSources + specConst("aSpec", "hierarchy things top\nchain c\n")},
			"aSpec, line 3: a specification holds at most one hierarchy, chain, or part; use another constant for another one"},
		{"a chain line that is malformed",
			map[string]string{"a.go": chainSources + specConst("aSpec", "chain _\n")},
			"aSpec, line 2: want: chain <Name> [from=<Name>]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, tc.want, message)
			}
		})
	}
	// A chain name declares nothing in Go, so it may equal a Go name.
	files := map[string]string{"a.go": chainSources + specConst("aSpec", "chain base\nlevel base public=Public private=Private\n") + specConst("bSpec", "hierarchy things top from=base\nlevel top\n")}
	if exit, message := gen(sourceDir(t, files)); exit != 0 {
		t.Errorf("a chain named like a type of the package was refused:\n%s", message)
	}
}

// A constant that a hierarchy or chain takes lines from must be compiled
// in every build that compiles the one that names it, with the error at the
// line that names it.
func TestChainBuildRule(t *testing.T) {
	chain := specConst("cSpec", "chain widgets\nlevel base public=Public private=Private\n")
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"a hierarchy that takes a test file's chain",
			map[string]string{"main.go": chainSources + specConst("hSpec", "hierarchy things top from=widgets\nlevel top\n"), "chain_test.go": "package main\n\n" + chain},
			"hSpec, line 2: from=widgets names the chain widgets, declared in chain_test.go, which some builds that compile main.go leave out; declare it in main.go, in a non-test file without build constraints or a platform suffix, or in a file with exactly the conditions of main.go"},
		{"a hierarchy that takes a chain some builds leave out",
			map[string]string{"main.go": chainSources + specConst("hSpec", "hierarchy things top from=widgets\nlevel top\n"), "chain_linux.go": "package main\n\n" + chain},
			"hSpec, line 2: from=widgets names the chain widgets, declared in chain_linux.go, which some builds that compile main.go leave out"},
		{"a chain that takes a test file's chain",
			map[string]string{"main.go": chainSources + specConst("mSpec", "chain mid from=widgets\nlevel top\n"), "chain_test.go": "package main\n\n" + chain},
			"mSpec, line 2: from=widgets names the chain widgets, declared in chain_test.go, which some builds that compile main.go leave out"},
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
		{"a file with the same conditions", map[string]string{"main_linux.go": chainSources + specConst("hSpec", "hierarchy things top from=widgets\nlevel top\n"), "chain_linux.go": "package main\n\n" + chain}},
		{"a test hierarchy that takes an ordinary chain", map[string]string{"main.go": chainSources + chain, "main_test.go": "package main\n\n" + specConst("hSpec", "hierarchy things top from=widgets\nlevel top\n")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if exit, message := gen(sourceDir(t, tc.files)); exit != 0 {
				t.Errorf("refused:\n%s", message)
			}
		})
	}
}

// lineOf returns the 1-based line of the first occurrence of text.
func lineOf(t *testing.T, content, text string) int {
	t.Helper()
	at := strings.Index(content, text)
	if at < 0 {
		t.Fatalf("no %q", text)
	}
	return strings.Count(content[:at], "\n") + 1
}

// A taken line keeps the meaning its own constant gives its package names;
// the output imports exactly what its code uses, under the names it uses,
// and binds each name to one path.
func TestChainImports(t *testing.T) {
	t.Run("a taken line's import, which the hierarchy does not bind", func(t *testing.T) {
		chain := chainSources + specConst("cSpec", "import lib \"example.com/lib\"\nchain c\nlevel base public=Public private=Private\n\tnode lib.Op\n")
		dir := generatedDir(t, map[string]string{"chain.go": chain, "main.go": "package main\n\n" + specConst("hSpec", "hierarchy things top from=c\nlevel top\n")})
		if out := read(t, dir, mainOutput); !strings.Contains(out, "\tlib \"example.com/lib\"\n") {
			t.Errorf("the taken line's import is missing:\n%s", out)
		}
		// The chain's own output declares nothing for its levels, so it
		// imports nothing for them.
		if out := read(t, dir, outputName("chain.go")); strings.Contains(out, "\tlib \"example.com/lib\"\n") {
			t.Errorf("the chain's output imports what only its levels use:\n%s", out)
		}
	})
	t.Run("a chain beside an operations line in one constant", func(t *testing.T) {
		chain := "package main\n\ntype (\n\tPublic  interface{ Name() string }\n\tPrivate interface{}\n)\n\ntype base struct{}\n\n" + specConst("cSpec", "import ext \"example.com/ext\"\nimport lib \"example.com/lib\"\noperations Public Private\n\texisting Name ext.Namer\nchain c\nlevel base public=Public private=Private\n\tnode lib.Op\n")
		out := read(t, generatedDir(t, map[string]string{"chain.go": chain}), outputName("chain.go"))
		if !strings.Contains(out, "\text \"example.com/ext\"\n") || strings.Contains(out, "\tlib \"example.com/lib\"\n") {
			t.Errorf("want the import of the existing line and not the chain's:\n%s", out)
		}
	})
	t.Run("an import that only a taken line would use", func(t *testing.T) {
		chain := chainSources + specConst("cSpec", "import lib \"example.com/lib\"\nchain c\nlevel base public=Public private=Private\n\tnode lib.Op\n")
		files := map[string]string{"chain.go": chain, "main.go": "package main\n\n" + specConst("hSpec", "hierarchy things top from=c\nimport lib \"example.com/lib\"\nlevel top\n")}
		if exit, message := gen(sourceDir(t, files)); exit != 1 || !strings.Contains(message, "hSpec, line 3: import lib is not used") {
			t.Errorf("exit %d, want the unused import refused:\n%s", exit, message)
		}
	})
	t.Run("a name the hierarchy binds to another path", func(t *testing.T) {
		chain := chainSources + specConst("cSpec", "import lib \"example.com/b\"\nchain c\nlevel base public=Public private=Private\n\tnode lib.Op\n")
		main := "package main\n\n" + specConst("hSpec", "hierarchy things top from=c\nimport lib \"example.com/a\"\nlevel top\n\tnode lib.Other\n")
		exit, message := gen(sourceDir(t, map[string]string{"chain.go": chain, "main.go": main}))
		want := "chain.go:" + strconv.Itoa(lineOf(t, chain, "\tnode lib.Op")) + ":2: cSpec, line 5 (taken by hSpec through from=c): import name lib is used for \"example.com/a\" and \"example.com/b\" in one generated file, by hSpec, line 3: import lib \"example.com/a\", and by cSpec, line 2: import lib \"example.com/b\"; give one of the imports another name"
		if exit != 1 || !strings.Contains(message, want) {
			t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
		}
	})
	t.Run("two specifications of one file", func(t *testing.T) {
		main := chainSources + specConst("aSpec", "import lib \"example.com/a\"\nhierarchy a top\nlevel top public=Public private=Private\n\tnode lib.Op\n") + specConst("bSpec", "import lib \"example.com/b\"\nhierarchy b base\nlevel base public=Public private=Private\n\tnode lib.Op\n")
		exit, message := gen(sourceDir(t, map[string]string{"main.go": main}))
		want := "import name lib is used for \"example.com/a\" and \"example.com/b\" in one generated file, by aSpec, line 2: import lib \"example.com/a\", and by bSpec, line 2: import lib \"example.com/b\"; give one of the imports another name"
		if exit != 1 || !strings.Contains(message, want) {
			t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
		}
	})
	t.Run("a specification and a copied signature", func(t *testing.T) {
		contracts := "package main\n\nimport lib \"example.com/c\"\n\ntype (\n\tPublic  interface{ Name() lib.Text }\n\tPrivate interface{}\n)\n\ntype base struct{}\n"
		main := "package main\n\n" + specConst("aSpec", "import lib \"example.com/a\"\noperations Public Private suffix=Op\nhierarchy a base\nlevel base public=Public private=Private\n\tnode lib.Op\n")
		exit, message := gen(sourceDir(t, map[string]string{"contracts.go": contracts, "main.go": main}))
		if exit != 1 || !strings.Contains(message, "by aSpec, line 2: import lib \"example.com/a\", and by ") || !strings.Contains(message, "contracts.go:3:8: import lib \"example.com/c\"") {
			t.Errorf("exit %d, want both bindings named:\n%s", exit, message)
		}
	})
}

// A hierarchy with no level of its own generates a second definition; its
// leaf is the last taken level, compared as types, and a mismatch names the
// taken line.
func TestChainLeaf(t *testing.T) {
	files := widgetFiles()
	files["second.go"] = "package main\n\ntype Btn = button\n\n" + specConst("secondSpec", "hierarchy plainButtons button from=buttons\n") + specConst("thirdSpec", "hierarchy aliasButtons Btn from=buttons\n")
	dir := tempModule(t, files)
	mustGen(t, dir)
	mustBuild(t, dir)

	files["second.go"] = "package main\n\n" + specConst("secondSpec", "hierarchy plainButtons logged from=buttons\n")
	exit, message := gen(sourceDir(t, files))
	want := "secondSpec, line 2: the last level is button, at buttonSpec, line 3, but the leaf is logged: logged is declared as a type of its own, not as an alias"
	if exit != 1 || !strings.Contains(message, want) {
		t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
	}
}

// Link problems come before every leaf mismatch and every per-constant
// check, and the per-constant checks take constants in file and line order.
func TestChainOrder(t *testing.T) {
	mismatch := specConst("mSpec", "hierarchy other top\nlevel base public=Public private=Private\n")
	chain := specConst("cSpec", "chain widgets\nlevel base public=Public private=Private\n")
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"an unknown name in a later file, before a leaf mismatch",
			map[string]string{"a.go": chainSources + mismatch, "b.go": "package main\n\n" + specConst("bSpec", "hierarchy third top from=widgetz\nlevel top\n")},
			"bSpec, line 2: from=widgetz names no hierarchy or chain"},
		{"a cycle in a later file, before a build-rule error",
			map[string]string{"a.go": chainSources + specConst("hSpec", "hierarchy things top from=widgets\nlevel top\n"), "a_test.go": "package main\n\n" + chain, "b.go": "package main\n\n" + specConst("bSpec", "chain loop from=loop\nlevel top\n")},
			"bSpec, line 2: from=loop makes a cycle of from="},
		{"a build-rule error in an earlier file, before a leaf mismatch",
			map[string]string{"a.go": chainSources + specConst("hSpec", "hierarchy things top from=widgets\nlevel top\n"), "a_test.go": "package main\n\n" + chain, "b.go": "package main\n\n" + mismatch},
			"hSpec, line 2: from=widgets names the chain widgets, declared in a_test.go"},
		{"a leaf mismatch in an earlier file, before a build-rule error",
			map[string]string{"a.go": chainSources + mismatch, "b.go": "package main\n\n" + specConst("hSpec", "hierarchy things top from=widgets\nlevel top\n"), "b_test.go": "package main\n\n" + chain},
			"mSpec, line 2: the last level is base but the leaf is top"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, tc.want, message)
			}
		})
	}
}

// A problem that appears only in a hierarchy that takes the line is
// reported at the taken line, naming the hierarchy and the names it went
// through.
func TestChainCombinedPositions(t *testing.T) {
	t.Run("a forwarder that would resolve another interface", func(t *testing.T) {
		contracts := "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype (\n\tPublic  interface{ Name() string }\n\tPrivate interface{}\n)\n\ntype Namer interface{ Name() string }\n\ntype base struct{ heiry.Node }\n\ntype top struct {\n\tbase\n\theiry.Node\n}\n\n"
		chain := "package main\n\n" + specConst("cSpec", "chain c\nlevel base public=Public private=Private\n\tnode Namer\n") + specConst("midSpec", "chain mid from=c\nlevel top\n")
		main := contracts + specConst("hSpec", "operations Public Private suffix=Op\nhierarchy things top from=mid\nwrapper view\n")
		exit, message := gen(sourceDir(t, map[string]string{"chain.go": chain, "main.go": main}))
		want := "chain.go:" + strconv.Itoa(lineOf(t, chain, "\tnode Namer")) + ":2: cSpec, line 4 (taken by hSpec through from=mid, from=c): wrapper view forwards Name through NameOp, but hierarchy things registers Name as Namer"
		if exit != 1 || !strings.Contains(message, want) {
			t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
		}
	})
	t.Run("a repeated guard that some build cannot compile", func(t *testing.T) {
		main := chainSources + "type Sealer interface{ Seal() error }\n\n" + specConst("hSpec", "hierarchy things top from=c\nlevel top\n")
		chain := "package main\n\n" + specConst("cSpec", "chain c\nlevel base public=Public private=Private\n\tnode Sealer\n")
		exit, message := gen(sourceDir(t, map[string]string{"main.go": main, "chain.go": chain, "audit_test.go": auditDeclaration}))
		want := "chain.go:" + strconv.Itoa(lineOf(t, chain, "\tnode Sealer")) + ":2: cSpec, line 4 (taken by hSpec through from=c): registration Sealer needs a guard in the output of main.go"
		if exit != 1 || !strings.Contains(message, want) {
			t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
		}
	})
}

// fmtWidget is a package whose chain's lines use an import, fmt, for the
// cases of a chain that takes them with from=.
const fmtWidget = "package main\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/lucaf/heiry\"\n)\n\ntype (\n\tPublic interface {\n\t\tName() string\n\t\tString() string\n\t}\n\tPrivate interface{}\n)\n\ntype base struct{ heiry.Node }\n\nfunc (*base) Name() string   { return \"base\" }\nfunc (*base) String() string { return \"base\" }\n\n" +
	"//heiry:spec\nconst opsSpec = `\nimport fmt \"fmt\"\noperations Public Private suffix=Op\n\texisting String fmt.Stringer\n`\n\n" +
	"//heiry:spec\nconst widgetsChain = `\nimport fmt \"fmt\"\nchain widgets\nlevel base public=Public private=Private\n\tnode NameOp fmt.Stringer\n`\n\n" +
	"func main() { fmt.Println() }\n"

const fmtMid = "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype mid struct {\n\tbase\n\theiry.Node\n}\n\n" +
	"//heiry:spec\nconst midChain = `\nchain mids from=widgets\nlevel mid\n`\n"

// A chain's own output holds its freshness assertion and nothing for its
// lines, also for the lines it takes with from=.
func TestChainWithFromOwnOutput(t *testing.T) {
	t.Run("a chain alone in its file", func(t *testing.T) {
		dir := tempModule(t, map[string]string{"widget.go": fmtWidget, "mid.go": fmtMid})
		mustGen(t, dir)
		mustBuild(t, dir)
		out := read(t, dir, outputName("mid.go"))
		if strings.Count(out, "// The declarations below") != 1 || !strings.Contains(out, "constant midChain.") || strings.Contains(out, "import") {
			t.Errorf("the chain's output holds more than its own assertion:\n%s", out)
		}
	})
	t.Run("a chain beside a hierarchy that binds the same import name to another path", func(t *testing.T) {
		other := "package other\n\nimport \"github.com/lucaf/heiry\"\n\ntype Thing struct{ heiry.Node }\n"
		mid := fmtMid + "\n" + specConst("thingSpec", "import fmt \"probe/other\"\nhierarchy things fmt.Thing\nlevel fmt.Thing public=Public private=Private\n")
		dir := tempModule(t, map[string]string{"widget.go": fmtWidget, "mid.go": mid})
		if err := os.MkdirAll(filepath.Join(dir, "other"), 0o755); err != nil {
			t.Fatal(err)
		}
		write(t, dir, "other/other.go", other)
		mustGen(t, dir)
		mustBuild(t, dir)
	})
}

// An import conflict that a taken line causes is reported at that taken
// line, naming the hierarchy that takes it, whichever binding comes first;
// when both come from taken lines, at the one added second.
func TestChainImportConflictAtTheTakenLine(t *testing.T) {
	chainB := chainSources + specConst("cSpec", "import lib \"example.com/b\"\nchain c\nlevel base public=Public private=Private\n\tnode lib.Op\n")
	chainNoPublic := strings.Replace(chainB, "\tPublic  interface{}\n", "", 1)
	chainD := "package main\n\n" + specConst("dSpec", "import lib \"example.com/a\"\nchain d\nlevel base public=Public private=Private\n\tnode lib.Op\n")
	nodeLine := func(content string) string { return strconv.Itoa(lineOf(t, content, "\tnode lib.Op")) }
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"the taken line's binding first, a later specification's second",
			map[string]string{"chain.go": chainB, "main.go": "package main\n\n" + specConst("aSpec", "hierarchy first top from=c\nlevel top\n") + specConst("bSpec", "import lib \"example.com/a\"\nhierarchy second base\nlevel base public=Public private=Private\n\tnode lib.Other\n")},
			"chain.go:" + nodeLine(chainB) + ":2: cSpec, line 5 (taken by aSpec through from=c): import name lib is used for \"example.com/b\" and \"example.com/a\" in one generated file, by cSpec, line 2: import lib \"example.com/b\", and by bSpec, line 2: import lib \"example.com/a\"; give one of the imports another name"},
		{"the taken line's binding first, a copied signature's second",
			map[string]string{"chain.go": chainNoPublic, "contracts.go": "package main\n\nimport lib \"example.com/c\"\n\ntype Public interface{ Name() lib.Text }\n", "main.go": "package main\n\n" + specConst("aSpec", "operations Public Private suffix=Op\nhierarchy first top from=c\nlevel top\n")},
			"chain.go:" + nodeLine(chainNoPublic) + ":2: cSpec, line 5 (taken by aSpec through from=c): import name lib is used for \"example.com/b\" and \"example.com/c\" in one generated file, by cSpec, line 2: import lib \"example.com/b\", and by "},
		{"both bindings from taken lines, at the one added second",
			map[string]string{"chain.go": chainB, "d.go": chainD, "main.go": "package main\n\n" + specConst("aSpec", "hierarchy first top from=c\nlevel top\n") + specConst("bSpec", "hierarchy second top from=d\nlevel top\n")},
			"d.go:" + nodeLine(chainD) + ":2: dSpec, line 5 (taken by bSpec through from=d): import name lib is used for \"example.com/b\" and \"example.com/a\" in one generated file, by cSpec, line 2: import lib \"example.com/b\", and by dSpec, line 2: import lib \"example.com/a\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, tc.want, message)
			}
		})
	}
}

// The advice to name the target directly is left out when that target
// would lead back to the hierarchy or chain that names it.
func TestChainTargetThatLeadsBack(t *testing.T) {
	files := map[string]string{"a.go": chainSources + specConst("cSpec", "chain c from=one\nlevel top\n") + specConst("oneSpec", "hierarchy one base from=c\n")}
	exit, message := gen(sourceDir(t, files))
	if exit != 1 || !strings.Contains(message, "cSpec, line 2: one has no level of its own, so from= cannot name it") || strings.Contains(message, "name its target directly") {
		t.Errorf("exit %d, want the refusal without the advice:\n%s", exit, message)
	}
}

// A level, node, or use line before any hierarchy or chain line says that
// either can precede it, a component line that a part line can too; a
// wrapper line, which only a hierarchy has, names the hierarchy line alone.
func TestPrecedingLineMessages(t *testing.T) {
	for _, tc := range []struct{ line, want string }{
		{"level base public=Public private=Private", "level needs a preceding line: hierarchy <Name> <LeafType> or chain <Name>"},
		{"node NameOp", "node needs a preceding line: hierarchy <Name> <LeafType> or chain <Name>"},
		{"component store storage NameOp", "component needs a preceding line: hierarchy <Name> <LeafType>, chain <Name>, or part <Name>"},
		{"wrapper view", "wrapper needs a preceding line: hierarchy <Name> <LeafType>"},
	} {
		exit, message := gen(sourceDir(t, map[string]string{"a.go": chainSources + specConst("aSpec", tc.line+"\n")}))
		if exit != 1 || !strings.HasSuffix(message, "aSpec, line 2: "+tc.want) {
			t.Errorf("%s: exit %d, want a message ending %q:\n%s", tc.line, exit, tc.want, message)
		}
	}
}

// Acceptance cases of shared levels that already work.
func TestChainAcceptance(t *testing.T) {
	t.Run("one package under two import names, and a leaf against a taken last level", func(t *testing.T) {
		chain := "package main\n\n" + specConst("cSpec", "import lib \"probe/lib\"\nchain c\nlevel lib.Thing public=Public private=Private\n")
		dir := leafModule(t, "", "hierarchy things other.Thing from=c\nimport other \"probe/lib\"\n", map[string]string{"lib/lib.go": leafLib, "chain.go": chain})
		mustGen(t, dir)
		mustBuild(t, dir)
		if out := read(t, dir, mainOutput); !strings.Contains(out, "\tlib \"probe/lib\"\n") || !strings.Contains(out, "\tother \"probe/lib\"\n") {
			t.Errorf("want the one path under both names:\n%s", out)
		}
	})
	t.Run("a hierarchy with from=, no level of its own, a wrapper and omit=InitPlan", func(t *testing.T) {
		files := widgetFiles()
		files["second.go"] = "package main\n\n" + specConst("secondSpec", "hierarchy plainButtons button from=buttons omit=InitPlan\nwrapper plainView\n")
		dir := tempModule(t, files)
		mustGen(t, dir)
		mustBuild(t, dir)
		out := read(t, dir, outputName("second.go"))
		if strings.Contains(out, "plainButtonsInitPlan") || !strings.Contains(out, "type plainView struct") {
			t.Errorf("want the wrapper and no Plan helper:\n%s", out)
		}
	})
	t.Run("the build rule for a chain that takes a chain of a platform file", func(t *testing.T) {
		chain := "package main\n\n" + specConst("cSpec", "chain widgets\nlevel base public=Public private=Private\n")
		refused := map[string]string{"chain_linux.go": chain, "main.go": chainSources + specConst("mSpec", "chain mids from=widgets\nlevel top\n")}
		want := "mSpec, line 2: from=widgets names the chain widgets, declared in chain_linux.go, which some builds that compile main.go leave out"
		if exit, message := gen(sourceDir(t, refused)); exit != 1 || !strings.Contains(message, want) {
			t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
		}
		accepted := map[string]string{"chain_linux.go": chain, "main.go": chainSources, "mid_linux.go": "package main\n\n" + specConst("mSpec", "chain mids from=widgets\nlevel top\n")}
		if exit, message := gen(sourceDir(t, accepted)); exit != 0 {
			t.Errorf("the chain with the same condition was refused:\n%s", message)
		}
	})
	t.Run("the advice to name the target directly, left out only when it leads back", func(t *testing.T) {
		for _, tc := range []struct {
			name, specs, want string
			advice            bool
		}{
			{"a hierarchy names the target", specConst("hSpec", "hierarchy things top from=one\nlevel top\n") + specConst("oneSpec", "hierarchy one base from=things\n"), "hSpec, line 2: one has no level of its own, so from= cannot name it", false},
			{"a three-step cycle", specConst("cSpec", "chain c from=one\nlevel top\n") + specConst("oneSpec", "hierarchy one base from=x\n") + specConst("xSpec", "chain x from=c\nlevel base public=Public private=Private\n"), "cSpec, line 2: one has no level of its own, so from= cannot name it", false},
			{"a target whose from= path loops elsewhere", specConst("cSpec", "chain c from=one\nlevel top\n") + specConst("oneSpec", "hierarchy one base from=x\n") + specConst("xSpec", "chain x from=y\nlevel base\n") + specConst("ySpec", "chain y from=x\nlevel top\n"), "cSpec, line 2: one has no level of its own, so from= cannot name it; name its target directly, with from=x", true},
		} {
			exit, message := gen(sourceDir(t, map[string]string{"a.go": chainSources + tc.specs}))
			if exit != 1 || !strings.Contains(message, tc.want) || strings.Contains(message, "name its target directly") != tc.advice {
				t.Errorf("%s: exit %d, want %q, advice %t:\n%s", tc.name, exit, tc.want, tc.advice, message)
			}
		}
	})
	t.Run("the wrapper of a hierarchy with from= and no level of its own, called", func(t *testing.T) {
		files := widgetFiles()
		files["second.go"] = "package main\n\n" + specConst("secondSpec", "hierarchy plainButtons button from=buttons\nwrapper plainView\n")
		files["widget.go"] = strings.Replace(widgetGo, "\tw := &buttonView{state: b}\n", "\tp := &button{}\n\tif err := plainButtonsInit(p); err != nil {\n\t\tfmt.Println(\"error:\", err)\n\t\treturn\n\t}\n\tv := &plainView{state: p}\n\tfmt.Println(v.Name(), v.Click())\n\tw := &buttonView{state: b}\n", 1)
		dir := tempModule(t, files)
		mustGen(t, dir)
		if got := mustRun(t, dir); got != "base button click\nbase button click logged button click" {
			t.Errorf("output = %q", got)
		}
	})
	t.Run("a partial run over two from= steps", func(t *testing.T) {
		top := "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype top struct {\n\tmid\n\theiry.Node\n}\n\n" + specConst("topSpec", "hierarchy tops top from=mids\nlevel top\n")
		// The taken lines use no import, so the case does not depend on the
		// imports of a chain's own output.
		widget := strings.Replace(fmtWidget, "import fmt \"fmt\"\nchain widgets\nlevel base public=Public private=Private\n\tnode NameOp fmt.Stringer\n", "chain widgets\nlevel base public=Public private=Private\n\tnode NameOp\n", 1)
		files := map[string]string{"widget.go": widget, "mid.go": fmtMid, "top.go": top}
		dir := tempModule(t, files)
		mustGen(t, dir)
		mustBuild(t, dir)
		edit(t, dir, "widget.go", "chain widgets\n", "chain widgets\n// an edit\n")
		fresh := t.TempDir()
		for source := range files {
			write(t, fresh, source, read(t, dir, source))
		}
		if exit, message := gen(fresh); exit != 0 {
			t.Fatal(message)
		}
		for _, source := range []string{"widget.go", "mid.go"} {
			write(t, dir, outputName(source), read(t, fresh, outputName(source)))
		}
		output := buildFails(t, dir, "duplicate key")
		for _, source := range []string{"widget.go", "mid.go"} {
			if strings.Contains(output, outputName(source)) {
				t.Errorf("the new output %s failed:\n%s", outputName(source), output)
			}
		}
		if !strings.Contains(output, outputName("top.go")) {
			t.Errorf("the stale %s did not fail:\n%s", outputName("top.go"), output)
		}
	})
}
