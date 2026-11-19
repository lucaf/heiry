package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// otherObjectApp's leaf is a type definition of a local struct, so its own
// declaration shows no fields. The struct embeds the holder under the alias
// name Dispatch and also has a named *heiry.Node field, which main points
// at the holder of another initialized object.
const otherObjectApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type Dispatch = heiry.Node

type (
	Public  interface{ Read() string }
	Private interface{}
)

type record struct {
	Dispatch
	Node  *heiry.Node
	label string
}

type node record

func (n *node) Read() string { return n.label }

//heiry:spec
const recordsSpec = ` + "`" + `
operations Public Private suffix=Op
hierarchy records node
level node public=Public private=Private
	node ReadOp
WRAPPER
` + "`" + `

func main() {
	first, second := &node{label: "first"}, &node{label: "second"}
	for _, n := range []*node{first, second} {
		if err := recordsInit(n); err != nil {
			fmt.Println("error:", err)
			return
		}
	}
	first.Node = &second.Dispatch
	fmt.Println(CALL)
}
`

// A forwarder written through the named pointer, as the default holder=Node
// spells it, reads the other object. The generator must refuse to write it:
// it reads the leaf through the type definition and finds that the holder
// the library installs is Dispatch.
func TestHolderOfALeafDeclaredAsATypeDefinition(t *testing.T) {
	// The hazard, with a handwritten forwarder of exactly that form.
	handwritten := strings.NewReplacer("WRAPPER\n", "", "CALL", "(&view{state: first}).Read()").Replace(otherObjectApp) +
		"\ntype view struct{ state *node }\n\nfunc (w *view) Read() string { return w.state.Node.Virtual[ReadOp]().Read() }\n"
	dir := tempModule(t, map[string]string{"main.go": handwritten})
	mustGen(t, dir)
	if got := mustRun(t, dir); got != "second" {
		t.Fatalf("dispatch through the named pointer printed %q, want the other object's %q", got, "second")
	}

	// The generator refuses that wrapper.
	refused := strings.NewReplacer("WRAPPER", "wrapper view", "CALL", "(&view{state: first}).Read()").Replace(otherObjectApp)
	dir = sourceDir(t, map[string]string{"main.go": refused})
	want := "the default holder=Node names a field that node does not embed; its directly embedded heiry.Node is Dispatch: write holder=Dispatch"
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, want) {
		t.Fatalf("exit %d, want %q:\n%s", exit, want, message)
	}

	// Control: the alias-named holder dispatches on the object itself.
	control := strings.NewReplacer("WRAPPER", "wrapper view holder=Dispatch", "CALL", "(&view{state: first}).Read()").Replace(otherObjectApp)
	dir = tempModule(t, map[string]string{"main.go": control})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "first" {
		t.Fatalf("output = %q, want first", got)
	}
}

// unestablishedApp's leaf embeds its holder through an alias declared in
// another package, which syntax cannot follow. The library accepts it.
const unestablishedApp = `package main

import (
	"fmt"

	"probe/nodes"
)

type (
	Public  interface{ Read() string }
	Private interface{}
)

type node struct {
	nodes.Holder
	label string
}

func (n *node) Read() string { return n.label }

//heiry:spec
const recordsSpec = ` + "`" + `
operations Public Private suffix=Op
hierarchy records node
level node public=Public private=Private
	node ReadOp
WRAPPER
` + "`" + `

func main() {
	n := &node{label: "read"}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(CALL)
}
`

// When the generator cannot establish the leaf's directly embedded holder,
// it refuses the wrapper and says how to proceed; without the wrapper line
// the declaration and the helpers are still generated.
func TestUnestablishedHolderRefusesTheWrapper(t *testing.T) {
	const how = "remove the wrapper line and write the wrapper by hand; the declaration and helpers are generated without it"
	for _, tc := range []struct{ name, source, want string }{
		{"a holder aliased in another package", strings.NewReplacer("WRAPPER", "wrapper view holder=Holder", "CALL", "0").Replace(unestablishedApp), "node embeds no field that the generator can show is heiry.Node"},
		{"a leaf declared in another package", "package main\n\nimport \"probe/shapes\"\n\ntype (\n\tPublic  interface{ Read() string }\n\tPrivate interface{}\n)\n\n//heiry:spec\nconst s = `\nimport shapes \"probe/shapes\"\noperations Public Private suffix=Op\nhierarchy records shapes.Leaf\nlevel shapes.Leaf public=Public private=Private\n\tnode ReadOp\nwrapper view\n`\n\nvar _ shapes.Leaf\n", "shapes.Leaf is not a struct type declared in this package"},
		{"a leaf that embeds a pointer", strings.Replace(strings.NewReplacer("WRAPPER", "wrapper view", "CALL", "0").Replace(unestablishedApp), "\tnodes.Holder\n", "\t*nodes.Holder\n", 1), "node embeds no field that the generator can show is heiry.Node"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := sourceDir(t, map[string]string{"main.go": tc.source})
			if exit, message := gen(dir); exit != 1 || !strings.Contains(message, tc.want) || !strings.Contains(message, how) {
				t.Fatalf("exit %d, want %q and %q:\n%s", exit, tc.want, how, message)
			}
		})
	}

	// Without the wrapper line the declaration and helpers are generated,
	// and a handwritten wrapper names the holder itself.
	source := strings.NewReplacer("WRAPPER\n", "", "CALL", "(&view{state: n}).Read()").Replace(unestablishedApp) +
		"\ntype view struct{ state *node }\n\nfunc (w *view) Read() string { return w.state.Holder.Virtual[ReadOp]().Read() }\n"
	dir := tempModule(t, map[string]string{"main.go": source})
	if err := os.Mkdir(filepath.Join(dir, "nodes"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "nodes/nodes.go", "package nodes\n\nimport \"github.com/lucaf/heiry\"\n\ntype Holder = heiry.Node\n")
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "read" {
		t.Fatalf("output = %q", got)
	}
}
