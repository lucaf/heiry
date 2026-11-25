package generator

import (
	"regexp"
	"strings"
	"testing"
)

// generated returns the output generated for main.go in a plain directory.
func generated(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := sourceDir(t, files)
	mustGen(t, dir)
	return read(t, dir, mainOutput)
}

// freshnessBlock matches the assertion that holds the specification text,
// which differs whenever the text does.
var freshnessBlock = regexp.MustCompile(`(?s)\n// The declarations below were generated from the constant .*?\n}\n`)

// initPlanBlock matches the Plan helper and its comment.
var initPlanBlock = regexp.MustCompile(`(?s)\n// shapeInitPlan binds .*?\n}\n`)

// omit=InitPlan leaves out the Plan helper and nothing else: the output is
// the one without the option, less that helper, apart from the assertion
// that holds the specification text.
func TestOmitInitPlan(t *testing.T) {
	plain := generated(t, map[string]string{"main.go": shapeSource})
	omitted := generated(t, map[string]string{"main.go": strings.Replace(shapeSource, "hierarchy shape leaf\n", "hierarchy shape leaf omit=InitPlan\n", 1)})
	if !strings.Contains(plain, "func shapeInitPlan(") {
		t.Fatalf("without the option the Plan helper is missing:\n%s", plain)
	}
	if strings.Contains(omitted, "shapeInitPlan") {
		t.Fatalf("omit=InitPlan still generates the Plan helper:\n%s", omitted)
	}
	for _, helper := range []string{"func shapeRefs(", "func shapeInit("} {
		if !strings.Contains(omitted, helper) {
			t.Fatalf("omit=InitPlan dropped %s:\n%s", helper, omitted)
		}
	}
	want := initPlanBlock.ReplaceAllString(freshnessBlock.ReplaceAllString(plain, "\n"), "")
	if got := freshnessBlock.ReplaceAllString(omitted, "\n"); got != want {
		t.Fatalf("omit=InitPlan changed more than the Plan helper:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestOmitOptionErrors(t *testing.T) {
	for _, tc := range []struct{ name, line, want string }{
		{"another helper", "hierarchy shape leaf omit=Init", "s, line 2: omit= takes InitPlan, the only helper that can be left out"},
		{"repeated", "hierarchy shape leaf omit=InitPlan omit=InitPlan", "s, line 2: option omit= is given twice"},
		{"unknown option", "hierarchy shape leaf plan=no", `s, line 2: unknown hierarchy option "plan=no"; the options are from=, omit=`},
	} {
		dir := sourceDir(t, map[string]string{"main.go": strings.Replace(shapeSource, "hierarchy shape leaf", tc.line, 1)})
		if exit, message := gen(dir); exit != 1 || !strings.Contains(message, tc.want) {
			t.Errorf("%s: exit %d, want %q:\n%s", tc.name, exit, tc.want, message)
		}
	}
}

// An omitted helper is not generated, so its name is free for the package
// and is not checked as a test name.
func TestOmittedHelperNameIsFree(t *testing.T) {
	const handwritten = "package main\n\nfunc shapeInitPlan() {}\n"
	dir := sourceDir(t, map[string]string{"main.go": shapeSource, "other.go": handwritten})
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "would generate shapeInitPlan, which") {
		t.Fatalf("control: exit %d:\n%s", exit, message)
	}
	dir = sourceDir(t, map[string]string{
		"main.go":  strings.Replace(shapeSource, "hierarchy shape leaf\n", "hierarchy shape leaf omit=InitPlan\n", 1),
		"other.go": handwritten,
	})
	if exit, message := gen(dir); exit != 0 {
		t.Fatalf("exit %d:\n%s", exit, message)
	}
}

// A call left behind after the option is added does not build.
func TestLeftoverInitPlanCallFailsToBuild(t *testing.T) {
	app := strings.Replace(shapeApp, "SPEC", constCarrier(strings.Replace(shapeSpecText, "hierarchy shape top\n", "hierarchy shape top omit=InitPlan\n", 1)), 1)
	app = strings.Replace(app, "\tfmt.Println(t.Node.Virtual[Namer]().Name())\n", "\tfmt.Println(t.Node.Virtual[Namer]().Name())\n\t_ = shapeInitPlan\n", 1)
	dir := tempModule(t, map[string]string{"main.go": app})
	mustGen(t, dir)
	buildFails(t, dir, "undefined: shapeInitPlan")
}
