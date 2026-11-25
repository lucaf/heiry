package generator

import (
	"path/filepath"
	"testing"
)

// fixtures are the maintained packages whose generated output is committed.
var fixtures = []string{
	"testdata/basic",
	"testdata/components",
	"testdata/matrix",
	"testdata/chains",
	"../../../../examples/generated",
}

// Committed generated output is what the generator writes from its source.
func TestCommittedOutputsAreFresh(t *testing.T) {
	for _, dir := range fixtures {
		findings, err := Check(dir)
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		for _, finding := range findings {
			t.Errorf("%s", finding)
		}
	}
}

// The example fixtures generate what the shipped examples write by hand,
// and their programs print exactly the same output.
func TestFixturesBehaveLikeTheHandwrittenExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("runs programs with the go command")
	}
	root := repoRoot(t)
	for fixture, example := range map[string]string{"testdata/basic": "examples/basic", "testdata/components": "examples/components"} {
		path, err := filepath.Abs(fixture)
		if err != nil {
			t.Fatal(err)
		}
		generated, err := goTool(t, root, nil, "run", path)
		if err != nil {
			t.Fatalf("%s:\n%s", fixture, generated)
		}
		handwritten, err := goTool(t, root, nil, "run", "./"+example)
		if err != nil {
			t.Fatalf("%s:\n%s", example, handwritten)
		}
		if generated != handwritten {
			t.Errorf("%s prints\n%s\nbut %s prints\n%s", fixture, generated, example, handwritten)
		}
	}
}
