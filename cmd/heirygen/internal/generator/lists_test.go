package generator

import (
	"strconv"
	"strings"
	"testing"
)

// ADR 0014, part four: a problem found once the links between
// specifications are checked, reported at a line that hierarchies take,
// lists them: after "also taken by" when the message names the hierarchy it
// was found in, and after "taken by", each with its path, when it names
// none. A line's own hierarchy is never listed for it.

const listContracts = "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype (\n\tPublic  interface{ Name() string }\n\tPrivate interface{}\n)\n\ntype Namer interface{ Name() string }\n\ntype base struct{ heiry.Node }\n\n"

// listFiles is a chain taken by two hierarchies directly and by a third
// through the second. chainSpec, bSpec, and cSpec are the texts of the
// chain's constant and of the second and third hierarchies' constants.
func listFiles(chainSpec, bSpec, cSpec string) map[string]string {
	types := func(name, parent string) string {
		return "type " + name + " struct {\n\t" + parent + "\n\theiry.Node\n}\n\n"
	}
	return map[string]string{
		"chain.go": listContracts + specConst("wSpec", chainSpec),
		"a.go":     "package main\n\nimport \"github.com/lucaf/heiry\"\n\n" + types("atop", "base") + specConst("aSpec", "hierarchy as atop from=widgets\nlevel atop\n"),
		"b.go":     "package main\n\nimport \"github.com/lucaf/heiry\"\n\n" + types("btop", "base") + specConst("bSpec", bSpec),
		"c.go":     "package main\n\nimport \"github.com/lucaf/heiry\"\n\n" + types("ctop", "btop") + specConst("cSpec", cSpec),
	}
}

func TestTakenByLists(t *testing.T) {
	chainNamer := "chain widgets\nlevel base public=Public private=Private\n\tnode Namer\n"
	chainNameOp := "chain widgets\nlevel base public=Public private=Private\n\tnode NameOp\n"
	plainB := "hierarchy bs btop from=widgets\nlevel btop\n"
	plainC := "hierarchy cs ctop from=bs\nlevel ctop\n"
	wrappedC := "operations Public Private suffix=Op\nhierarchy cs ctop from=bs\nlevel ctop\nwrapper view\n"
	chainLine := strconv.Itoa(lineOf(t, listContracts+specConst("wSpec", chainNamer), "\tnode Namer"))
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"a problem of one hierarchy at a taken line, with the others that take it",
			listFiles(chainNamer, plainB, wrappedC),
			"chain.go:" + chainLine + ":2: wSpec, line 4 (taken by cSpec through from=bs, from=widgets; also taken by aSpec and bSpec): wrapper view forwards Name through NameOp, but hierarchy cs registers Name as Namer"},
		{"a problem of one hierarchy at its own line, which another takes",
			listFiles(chainNameOp, "operations Public Private suffix=Op\nhierarchy bs btop from=widgets\nlevel btop\n\tnode Namer\nwrapper view\n", plainC),
			"bSpec, line 5 (also taken by cSpec): wrapper view forwards Name through NameOp, but hierarchy bs registers Name as Namer"},
		{"a problem that names no hierarchy, at a chain's line",
			listFiles("chain widgets\nlevel base public=Public private=Private\n\tnode Missing\n", plainB, plainC),
			"wSpec, line 4 (taken by aSpec through from=widgets, and by bSpec through from=widgets, and by cSpec through from=bs, from=widgets): Missing is not declared in the handwritten files of package main"},
		{"a problem that names no hierarchy, at a hierarchy's own line, without that hierarchy",
			listFiles(chainNameOp, "hierarchy bs btop from=widgets\nlevel btop\n\tnode Missing\n", plainC),
			"bSpec, line 4 (taken by cSpec through from=bs): Missing is not declared in the handwritten files of package main"},
		{"a leaf mismatch at a hierarchy's header line, which another takes",
			listFiles(chainNameOp, "hierarchy bs atop from=widgets\nlevel btop\n", plainC),
			"bSpec, line 2 (taken by cSpec through from=bs): the last level is btop but the leaf is atop"},
		{"a build-rule error at a chain's header line",
			map[string]string{"h.go": chainSources + specConst("hSpec", "hierarchy hs top from=mids\nlevel top\n"), "mid.go": "package main\n\n" + specConst("mSpec", "chain mids from=widgets\nlevel base\n"), "widgets_test.go": "package main\n\n" + specConst("wSpec", "chain widgets\nlevel base public=Public private=Private\n")},
			"mSpec, line 2 (taken by hSpec through from=mids): from=widgets names the chain widgets, declared in widgets_test.go, which some builds that compile mid.go leave out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, tc.want, message)
			}
		})
	}
	// Problems found before the list is built get none: a link problem, and
	// a problem found while the files are read.
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"a link problem", listFiles("chain widgets from=nothing\nlevel base public=Public private=Private\n", plainB, plainC), "wSpec, line 2: from=nothing names no hierarchy or chain"},
		{"a grammar error in a chain's line", listFiles("chain widgets\nlevel base public=Public private=Private\n\tnodes Namer\n", plainB, plainC), "wSpec, line 4: unknown keyword"},
	} {
		t.Run("no list for "+tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) || strings.Contains(message, "taken by") {
				t.Errorf("exit %d, want %q and no list:\n%s", exit, tc.want, message)
			}
		})
	}
}

// One faulty line taken by several hierarchies: the problem found first, in
// the order of the steps and within a step in file order, is reported, with
// the other hierarchies that take the line listed.
func TestFaultyLineTakenBySeveralHierarchies(t *testing.T) {
	// The chain registers Sealer, whose only existing line is in a test
	// file and uses a name only that file declares. A hierarchy in an
	// ordinary file therefore fails the repeated-guard check on the line,
	// and one in a test file with a wrapper fails the forwarder check.
	contracts := "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype (\n\tPublic  interface{ Seal() error }\n\tPrivate interface{}\n)\n\ntype Sealer interface{ Seal() error }\n\ntype base struct{ heiry.Node }\n\n"
	chain := contracts + specConst("cSpec", "chain c\nlevel base public=Public private=Private\n\tnode Sealer\n")
	sealerLine := strconv.Itoa(lineOf(t, chain, "\tnode Sealer"))
	hierarchy := func(constName, name, leaf string, wrapped bool) string {
		spec := "hierarchy " + name + " " + leaf + " from=c\nlevel " + leaf + "\n"
		if wrapped {
			spec = "operations Public Private suffix=Op\n" + spec + "wrapper view\n"
		}
		return "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype " + leaf + " struct {\n\tbase\n\theiry.Node\n}\n\n" + specConst(constName, spec)
	}
	// The same line with an import, for a hierarchy whose file binds that
	// name to another path and so fails only the import check of step 9.
	libChain := contracts + specConst("cSpec", "import lib \"example.com/b\"\nchain c\nlevel base public=Public private=Private\n\tnode lib.Sealer\n")
	libLine := strconv.Itoa(lineOf(t, libChain, "\tnode lib.Sealer"))
	libAudit := "package main\n\ntype status = error\n\ntype Audited interface{ Seal() status }\n\n" + specConst("auditSpec", "import lib \"example.com/b\"\noperations Audited\n\texisting Seal lib.Sealer\n")
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"in step 8, the first in file order, which fails the repeated-guard check",
			map[string]string{"chain.go": chain, "audit_test.go": auditDeclaration, "main.go": hierarchy("oneSpec", "ones", "top1", false), "x_test.go": hierarchy("twoSpec", "twos", "top2", true)},
			"chain.go:" + sealerLine + ":2: cSpec, line 4 (taken by oneSpec through from=c; also taken by twoSpec): registration Sealer needs a guard in the output of main.go"},
		{"in step 8, the first in file order, which fails the forwarder check",
			map[string]string{"chain.go": chain, "audit_test.go": auditDeclaration, "a_test.go": hierarchy("twoSpec", "twos", "top2", true), "main.go": hierarchy("oneSpec", "ones", "top1", false)},
			"chain.go:" + sealerLine + ":2: cSpec, line 4 (taken by twoSpec through from=c; also taken by oneSpec): wrapper view forwards Seal through SealOp, but hierarchy twos registers Seal as Sealer"},
		{"across steps 8 and 9, the one that fails step 8, later in file order",
			map[string]string{"chain.go": libChain, "audit_test.go": libAudit, "a_test.go": "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype top1 struct {\n\tbase\n\theiry.Node\n}\n\n" + specConst("aSpec", "import lib \"example.com/a\"\nhierarchy ones top1 from=c\nlevel top1\n\tnode lib.Other\n"), "b.go": hierarchy("bSpec", "twos", "top2", false)},
			"chain.go:" + libLine + ":2: cSpec, line 5 (taken by bSpec through from=c; also taken by aSpec): registration lib.Sealer needs a guard in the output of b.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exit, message := gen(sourceDir(t, tc.files))
			if exit != 1 || !strings.Contains(message, tc.want) {
				t.Errorf("exit %d, want a message containing %q:\n%s", exit, tc.want, message)
			}
		})
	}
	t.Run("a link problem in a later file, before a build-rule error, without a list", func(t *testing.T) {
		files := map[string]string{
			"a.go":      chainSources + specConst("hSpec", "hierarchy things top from=widgets\nlevel top\n"),
			"a_test.go": "package main\n\n" + specConst("wSpec", "chain widgets\nlevel base public=Public private=Private\n"),
			"b.go":      "package main\n\n" + specConst("bSpec", "chain loop from=loop\nlevel top\n"),
		}
		exit, message := gen(sourceDir(t, files))
		if exit != 1 || !strings.Contains(message, "bSpec, line 2: from=loop makes a cycle of from=") || strings.Contains(message, "taken by") {
			t.Errorf("exit %d, want the cycle without a list:\n%s", exit, message)
		}
	})
}
