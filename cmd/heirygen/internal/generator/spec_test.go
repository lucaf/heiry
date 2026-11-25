package generator

import (
	"path/filepath"
	"strings"
	"testing"
)

// Diagnostics about a specification point at the file line of the offending
// specification line. In a raw string literal every specification line is a
// file line; an interpreted literal is reported at the literal itself.
func TestDiagnosticPositions(t *testing.T) {
	const head = "package main\n\n//heiry:spec\nconst s = `\n" // the literal starts on line 4
	for _, tc := range []struct {
		name, source, want string
	}{
		{"unknown keyword", head + "hierarchy shape leaf\nlevle leaf public=P private=Q\n`\n", "main.go:6:1: s, line 3: unknown keyword \"levle\""},
		{"indented line", head + "hierarchy shape leaf\nlevel leaf public=P private=Q\n\tnode fmt.Stringer\n`\n", "main.go:7:2: s, line 4: fmt.Stringer uses package fmt"},
		{"leaf mismatch", head + "\nhierarchy shape top\nlevel leaf public=P private=Q\n`\n", "main.go:6:1: s, line 3: the last level is leaf but the leaf is top"},
		{"interpreted literal", "package main\n\n//heiry:spec\nconst s = \"hierarchy shape leaf\\nlevle leaf\\n\"\n", "main.go:4:11: s, line 2: unknown keyword \"levle\""},
	} {
		dir := sourceDir(t, map[string]string{"main.go": tc.source})
		_, err := Generate(dir)
		want := filepath.Join(dir, tc.want)
		if err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("%s:\n got %v\nwant %s...", tc.name, err, want)
		}
	}
}

// An unqualified type in a specification is written into the generated
// file as is, so it must be a type the package declares, a type the
// generator itself generates, or a predeclared type. Anything else could
// only produce output that does not compile.
func TestUndeclaredSpecificationTypes(t *testing.T) {
	const decls = "\ntype leaf struct{}\ntype P interface{ Do() }\ntype Q interface{}\n"
	for _, tc := range []struct{ name, spec, want string }{
		{"node operation", "hierarchy shape leaf\nlevel leaf public=P private=Q\n\tnode Missing\n", "s, line 5: Missing is not declared in the handwritten files of package main"},
		{"component provider", "hierarchy shape leaf\nlevel leaf public=P private=Q\n\tcomponent c part DoOp\n", "s, line 5: part is not declared"},
		{"contract", "hierarchy shape leaf\nlevel leaf public=P private=R\n", "s, line 4: R is not declared"},
		{"type argument", "hierarchy shape leaf\nlevel leaf public=P private=Q\n\tcomponent c box[item] DoOp\n", "item is not declared"},
	} {
		source := "package main\n" + decls + "\ntype box[T any] struct{}\n\n//heiry:spec\nconst s = `\noperations P Q suffix=Op\n" + tc.spec + "`\n"
		dir := sourceDir(t, map[string]string{"main.go": source})
		if exit, message := gen(dir); exit != 1 || !strings.Contains(message, tc.want) {
			t.Errorf("%s: exit %d, want %q:\n%s", tc.name, exit, tc.want, message)
		}
	}

	// Declared, generated, predeclared, and qualified names are accepted.
	source := "package main\n" + decls + "\ntype box[T any] struct{}\n\n//heiry:spec\nconst s = `\nimport fmt \"fmt\"\noperations P Q suffix=Op\nhierarchy shape leaf\nlevel leaf public=P private=Q\n\tnode DoOp\n\tcomponent c box[int] fmt.Stringer\n`\n"
	dir := sourceDir(t, map[string]string{"main.go": source})
	if exit, message := gen(dir); exit != 0 {
		t.Fatalf("exit %d:\n%s", exit, message)
	}
}

// A marker written with a space is ordinary prose to Go, so the constant
// would silently not be a specification. The generator says so.
func TestMisspelledMarker(t *testing.T) {
	source := strings.Replace(shapeSource, "//heiry:spec", "// heiry:spec", 1)
	dir := sourceDir(t, map[string]string{"main.go": source})
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "write //heiry:spec without a space") {
		t.Fatalf("exit %d:\n%s", exit, message)
	}
}

// An option given twice on one line is refused rather than resolved by
// order.
func TestRepeatedOption(t *testing.T) {
	source := strings.Replace(shapeSource, "level leaf public=P private=Q", "level leaf public=P private=Q public=P", 1)
	dir := sourceDir(t, map[string]string{"main.go": source})
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "option public= is given twice") {
		t.Fatalf("exit %d:\n%s", exit, message)
	}
}

// Contract forms the generator cannot read as syntax are refused with a
// diagnostic at the contract, never turned into output.
func TestUnsupportedContracts(t *testing.T) {
	const spec = "\n//heiry:spec\nconst s = `\noperations Public Private suffix=Op\n`\n"
	for _, tc := range []struct{ name, decls, want string }{
		{"generic contract", "type Public[T any] interface{ Get() T }\ntype Private interface{}\n", "contract Public is generic"},
		{"type-set element", "type Public interface{ ~int | string }\ntype Private interface{}\n", "contract Public has the element ~int | string"},
		{"alias of an imported interface", "import \"io\"\n\ntype Public = io.Reader\ntype Private interface{}\n", "Public is declared as io.Reader"},
		{"embedded imported interface", "import \"io\"\n\ntype Public interface{ io.Reader }\ntype Private interface{}\n", "contract Public embeds io.Reader, whose methods cannot be read from this package"},
		{"embedded predeclared error", "type Public interface{ error }\ntype Private interface{}\n", "main.go:3:24: contract Public embeds the predeclared interface error, whose method the generator does not read; write it out in Public: Error() string"},
		{"embedded predeclared any", "type Public interface{ any }\ntype Private interface{}\n", "main.go:3:24: contract Public embeds the predeclared any, which the generator does not read as a contract; remove it from Public"},
		{"self-embedding", "type Public interface{ Private }\ntype Private interface{ Public }\n", "embeds itself"},
		{"method with type parameters", "type Public interface{ Get[T any]() T }\ntype Private interface{}\n", ""},
		{"import name not in its path", "import yaml \"gopkg.in/yaml.v3\"\n\nvar _ yaml.Node\n\ntype Public interface{ Decode(n *yaml.Node) error }\ntype Private interface{}\n", ""},
		{"unexported method", "type Public interface{ get() int }\ntype Private interface{}\n", "operation get is not exported"},
		{"undeclared contract", "type Private interface{}\n", "contract Public is not declared in the handwritten files of this package"},
	} {
		source := "package main\n\n" + tc.decls + spec
		dir := sourceDir(t, map[string]string{"main.go": source})
		exit, message := gen(dir)
		switch {
		case tc.name == "method with type parameters":
			// Go itself rejects the declaration, so it does not parse.
			if exit != 1 || !strings.Contains(message, "interface method must have no type parameters") {
				t.Errorf("%s: exit %d:\n%s", tc.name, exit, message)
			}
		case tc.name == "import name not in its path":
			// An explicitly named import is read correctly.
			if exit != 0 {
				t.Errorf("%s: exit %d:\n%s", tc.name, exit, message)
			}
		case exit != 1 || !strings.Contains(message, tc.want):
			t.Errorf("%s: exit %d, want %q:\n%s", tc.name, exit, tc.want, message)
		}
	}

	// An unnamed import whose package name differs from its last path
	// element cannot be resolved from syntax; the diagnostic says how to fix
	// it.
	source := "package main\n\nimport \"gopkg.in/yaml.v3\"\n\nvar _ yaml.Node\n\ntype Public interface{ Decode(n *yaml.Node) error }\ntype Private interface{}\n" + spec
	dir := sourceDir(t, map[string]string{"main.go": source})
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "cannot tell which import of main.go provides yaml; give that import the explicit name yaml") {
		t.Errorf("unnamed import: exit %d:\n%s", exit, message)
	}
}
