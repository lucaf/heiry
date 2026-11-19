package generator

import (
	"strings"
	"testing"
)

// dispatchApp registers its operations as SPEC says and then calls every
// public operation through the generated wrapper. A forwarder that resolves
// an interface the hierarchy did not register panics with UnknownCapability.
const dispatchApp = `package main

import (
	"errors"
	"fmt"

	"github.com/lucaf/heiry"
)

DECLS

SPEC

func main() {
	l := LEAF
	if err := shapeInit(l); err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() {
		var diagnostic *heiry.Error
		if err, ok := recover().(error); ok && errors.As(err, &diagnostic) {
			fmt.Println("panic:", diagnostic.Code, diagnostic.Capability)
		}
	}()
	w := &view{state: l}
	fmt.Println(CALLS)
}
`

type dispatchCase struct {
	decls, spec, leaf, calls string
}

func (c dispatchCase) source() string {
	return strings.NewReplacer("DECLS", c.decls, "SPEC", "//heiry:spec\nconst shapeSpec = `\n"+c.spec+"`", "LEAF", c.leaf, "CALLS", c.calls).Replace(dispatchApp)
}

// The public contract embeds a handwritten operation interface, and the
// hierarchy registers that interface. The operations line, without an
// existing line, gives the method a generated interface of its own.
var embeddedNamer = dispatchCase{
	decls: `type Namer interface{ Name() string }

type (
	Public interface {
		Namer
		Describe() string
	}
	Private interface{}
)

type node struct{ heiry.Node }

func (n *node) Name() string     { return "node" }
func (n *node) Describe() string { return "described" }`,
	spec: "operations Public Private suffix=Op\nhierarchy shape node\nlevel node public=Public private=Private\n\tnode Namer DescribeOp\nwrapper view\n",
	leaf: "&node{}", calls: "w.Describe(), w.Name()",
}

// The leaf extends the public contract, and the extension is listed on an
// operations line of its own, which gives Describe a second interface.
var extendedContract = dispatchCase{
	decls: `type (
	Public     interface{ Describe() string }
	WidePublic interface {
		Public
		Extra() string
	}
	Private interface{}
)

type base struct{ heiry.Node }

func (b *base) Describe() string { return "base" }

type top struct {
	base
	heiry.Node
}

func (t *top) Extra() string { return "extra" }`,
	spec: "operations Public Private suffix=Op\noperations WidePublic prefix=Wide suffix=Op\nhierarchy shape top\nlevel base public=Public private=Private\n\tnode DescribeOp\nlevel top public=WidePublic private=Private\n\tnode WideExtraOp\nwrapper view\n",
	leaf: "&top{}", calls: "w.Describe(), w.Extra()",
}

// The hierarchy registers an imported interface, whose method the
// generator cannot read.
var importedStringer = dispatchCase{
	decls: `type (
	Public  interface{ String() string }
	Private interface{}
)

type node struct{ heiry.Node }

func (n *node) String() string { return "node" }`,
	spec: "import fmt \"fmt\"\noperations Public Private suffix=Op\nhierarchy shape node\nlevel node public=Public private=Private\n\tnode fmt.Stringer\nwrapper view\n",
	leaf: "&node{}", calls: "w.String()",
}

// A forwarder resolves the interface the operations line gives its method.
// When the hierarchy registers that method under another interface, the
// generator refuses and names the existing line that makes them one.
func TestForwarderMustResolveTheRegisteredInterface(t *testing.T) {
	for _, tc := range []struct {
		name string
		app  dispatchCase
		want []string
	}{
		{"embedded handwritten interface", embeddedNamer, []string{"wrapper view forwards Name through NameOp, but hierarchy shape registers Name as Namer", "existing Name Namer"}},
		{"extension on its own operations line", extendedContract, []string{"wrapper view forwards Describe through WideDescribeOp, but hierarchy shape registers Describe as DescribeOp", "existing Describe DescribeOp"}},
		{"imported interface", importedStringer, []string{"cannot tell which operation fmt.Stringer identifies", "existing String fmt.Stringer"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := sourceDir(t, map[string]string{"main.go": tc.app.source()})
			exit, message := gen(dir)
			for _, want := range tc.want {
				if exit != 1 || !strings.Contains(message, want) {
					t.Errorf("exit %d, want a message containing %q:\n%s", exit, want, message)
				}
			}
		})
	}
}

// With the existing line, the forwarder resolves the registered interface:
// the program initializes the object and calls every public operation
// through the wrapper. A registration that spells the same type through an
// alias is the same operation; the compiler confirms that the two are
// identical.
func TestForwarderControls(t *testing.T) {
	alias := embeddedNamer
	alias.decls = strings.Replace(alias.decls, "type Namer interface{ Name() string }", "type Namer interface{ Name() string }\n\ntype NamerAlias = Namer", 1)
	alias.spec = strings.Replace(alias.spec, "\tnode Namer DescribeOp", "\tnode NamerAlias DescribeOp", 1)
	for _, tc := range []struct {
		name, old, new, output string
		app                    dispatchCase
	}{
		{"embedded handwritten interface", "operations Public Private suffix=Op\n", "operations Public Private suffix=Op\n\texisting Name Namer\n", "described node", embeddedNamer},
		{"registration through an alias", "operations Public Private suffix=Op\n", "operations Public Private suffix=Op\n\texisting Name Namer\n", "described node", alias},
		{"extension on its own operations line", "operations WidePublic prefix=Wide suffix=Op\n", "operations WidePublic prefix=Wide suffix=Op\n\texisting Describe DescribeOp\n", "base extra", extendedContract},
		{"extension on the same operations line", "operations Public Private suffix=Op\noperations WidePublic prefix=Wide suffix=Op\n", "operations Public WidePublic Private suffix=Op\n", "base extra", extendedContract},
		{"imported interface", "operations Public Private suffix=Op\n", "operations Public Private suffix=Op\n\texisting String fmt.Stringer\n", "node", importedStringer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := tc.app
			app.spec = strings.Replace(app.spec, tc.old, tc.new, 1)
			if tc.name == "extension on the same operations line" {
				app.spec = strings.Replace(app.spec, "WideExtraOp", "ExtraOp", 1)
			}
			dir := tempModule(t, map[string]string{"main.go": app.source()})
			mustGen(t, dir)
			mustBuild(t, dir)
			if got := mustRun(t, dir); got != tc.output {
				t.Fatalf("output = %q, want %q", got, tc.output)
			}
			if tc.name == "registration through an alias" && !strings.Contains(read(t, dir, mainOutput), "var _ *Namer = (*NamerAlias)(nil)") {
				t.Fatalf("no identity assertion for the alias:\n%s", read(t, dir, mainOutput))
			}
		})
	}
}

// The compiler decides type identity where spellings differ. When the alias
// becomes a type of its own and nobody regenerates, the identity assertion
// stops the build; regenerating then refuses the mismatch.
func TestIdentityAssertionCatchesAChangedAlias(t *testing.T) {
	app := embeddedNamer
	app.decls = strings.Replace(app.decls, "type Namer interface{ Name() string }", "type Namer interface{ Name() string }\n\ntype NamerAlias = Namer", 1)
	app.spec = strings.Replace(app.spec, "\tnode Namer DescribeOp", "\tnode NamerAlias DescribeOp", 1)
	app.spec = strings.Replace(app.spec, "operations Public Private suffix=Op\n", "operations Public Private suffix=Op\n\texisting Name Namer\n", 1)
	dir := tempModule(t, map[string]string{"main.go": app.source()})
	mustGen(t, dir)
	mustBuild(t, dir)
	edit(t, dir, "main.go", "type NamerAlias = Namer", "type NamerAlias Namer")
	output := buildFails(t, dir, "cannot use (*NamerAlias)(nil) (value of type *NamerAlias) as *Namer value")
	if !strings.Contains(output, mainOutput) {
		t.Fatalf("the failure is not reported in the generated file:\n%s", output)
	}
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "registers Name as NamerAlias") {
		t.Fatalf("regeneration: exit %d:\n%s", exit, message)
	}
}
