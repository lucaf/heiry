package generator

import (
	"strings"
	"testing"
)

// An invalid declaration is reported before anything about the object, as
// Definition.Init reports it, whatever the object is.
func TestInvalidDefinitionTakesPrecedence(t *testing.T) {
	// No level registers Namer, so the definition has a missing method.
	app := strings.Replace(shapeApp, "SPEC", constCarrier(strings.Replace(shapeSpecText, "\tnode Namer\n", "", 1)), 1)
	app = strings.Replace(app, `	t := &top{}
	if err := shapeInit(t); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(t.Node.Virtual[Namer]().Name())
`, `	report := func(name string, err error) {
		var diagnostic *heiry.Error
		if !errors.As(err, &diagnostic) {
			fmt.Println(name, "unexpected:", err)
			return
		}
		fmt.Println(name, diagnostic.Code)
	}
	t := &top{}
	report("nil object:", shapeInit(nil))
	report("valid object:", shapeInit(t))
	report("nil Plan:", shapeInitPlan(nil, t))
	fmt.Println("untouched:", t.Node.Scope() == heiry.Scope{})
`, 1)
	app = strings.Replace(app, "import (\n\t\"fmt\"\n", "import (\n\t\"errors\"\n\t\"fmt\"\n", 1)
	dir := tempModule(t, map[string]string{"main.go": app})
	mustGen(t, dir)
	const want = "nil object: MissingMethods\nvalid object: MissingMethods\nnil Plan: InvalidPlan\nuntouched: true"
	if got := mustRun(t, dir); got != want {
		t.Fatalf("output:\n%s\nwant:\n%s", got, want)
	}
}
