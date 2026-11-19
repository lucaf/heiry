package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// staleHolderApp is valid when generated: the leaf embeds heiry.Node, the
// default holder. HOLDER is its field list, WRAPPER the wrapper line, and
// STALE what main does after initializing two objects.
const staleHolderApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type Dispatch = heiry.Node

type (
	Public  interface{ Read() string }
	Private interface{}
)

type node struct {
HOLDER	label string
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
	first, second := &node{label: "first"}, &node{label: "second"}
	for _, n := range []*node{first, second} {
		if err := recordsInit(n); err != nil {
			fmt.Println("error:", err)
			return
		}
	}
STALE	fmt.Println((&view{state: first}).Read())
}
`

func staleHolderSource(holder, wrapper, stale string) string {
	return strings.NewReplacer("HOLDER", holder, "WRAPPER", wrapper, "STALE", stale).Replace(staleHolderApp)
}

// The forwarders select the holder by the name it had when the wrapper was
// generated. A later edit of the leaf alone, which neither the specification
// nor the contracts see, can give that name to another field: a pointer,
// which may point into another initialized object, must then fail to
// compile. A named heiry.Node value compiles and fails at the first call;
// only -check or regenerating finds it.
func TestStaleHolderEdit(t *testing.T) {
	dir := tempModule(t, map[string]string{"main.go": staleHolderSource("\theiry.Node\n", "wrapper view", "")})
	mustGen(t, dir)
	if got := mustRun(t, dir); got != "first" {
		t.Fatalf("output = %q", got)
	}
	generated := read(t, dir, mainOutput)

	// The embedded holder becomes Dispatch, and Node a pointer into the
	// other object.
	edit(t, dir, "main.go", "\theiry.Node\n\tlabel string\n", "\tDispatch\n\tNode  *heiry.Node\n\tlabel string\n")
	edit(t, dir, "main.go", "\tfmt.Println((&view{state: first}).Read())\n", "\tfirst.Node = &second.Dispatch\n\tfmt.Println((&view{state: first}).Read())\n")
	if output, err := goTool(t, dir, nil, "build", "-o", "/dev/null", "."); err == nil {
		run, _ := goTool(t, dir, nil, "run", ".")
		t.Fatalf("the stale wrapper compiled, and the program printed %q, the other object's label", strings.TrimSpace(run))
	} else {
		t.Logf("after the stale edit:\n%s", output)
		reportedIn(t, output, mainOutput, "&leaf.Node (value of type **heiry.Node)")
	}

	// A named value field of that name compiles: the documented limit.
	edit(t, dir, "main.go", "\tNode  *heiry.Node\n", "\tNode  heiry.Node\n")
	edit(t, dir, "main.go", "\tfirst.Node = &second.Dispatch\n", "")
	if output, err := goTool(t, dir, nil, "build", "-o", "/dev/null", "."); err != nil {
		t.Fatalf("the named-value edit no longer compiles; update the documented limit:\n%s", output)
	}
	if got := read(t, dir, mainOutput); got != generated {
		t.Fatal("the output changed without regeneration")
	}
	if exit, message := gen("-check", dir); exit != 1 || !strings.Contains(message, "does not embed; its directly embedded heiry.Node is Dispatch") {
		t.Fatalf("check: exit %d:\n%s", exit, message)
	}
}

// The guard accepts the holder the generator establishes, under its own or
// an alias name, and the wrapper dispatches on its own object.
func TestHolderGuardControls(t *testing.T) {
	for _, tc := range []struct{ name, holder, wrapper string }{
		{"own name", "\theiry.Node\n", "wrapper view"},
		{"alias name", "\tDispatch\n", "wrapper view holder=Dispatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempModule(t, map[string]string{"main.go": staleHolderSource(tc.holder, tc.wrapper, "")})
			mustGen(t, dir)
			mustBuild(t, dir)
			if got := mustRun(t, dir); got != "first" {
				t.Fatalf("output = %q", got)
			}
		})
	}
}

// auditedOps lists the leaf's contracts after another contract that spells
// the shared method Wipe through an alias only this test file declares.
const auditedOps = "package main\n\ntype status = error\n\ntype Audited interface{ Wipe() status }\n\n//heiry:spec\nconst opsSpec = `\noperations Audited Public Private\n\texisting Read Reader\n\texisting Wipe Wiper\n`\n"

const auditedApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Reader interface{ Read() string }
	Wiper  interface{ Wipe() error }
)

type (
	Public  interface{ Read() string }
	Private interface{ Wipe() error }
	Local   interface{ Wipe() error }
)

type node struct{ heiry.Node }

func (n *node) Read() string { return "read" }
func (n *node) Wipe() error  { return nil }

//heiry:spec
const recordsSpec = ` + "`" + `
operations Local
	existing Wipe Wiper
hierarchy records node
level node public=Public private=Private
	node Reader Wiper
wrapper view
` + "`" + `

func main() {
	n := &node{}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println((&view{state: n}).Read(), n.Node.Virtual[Wiper]().Wipe())
}
`

// The wrapper's code, forwarders and contract guards, copies the signatures
// of the leaf's own contracts, which its Level call names in every build that
// compiles the wrapper. Another contract on the same operations line may
// spell a shared method with names those builds lack.
func TestWrapperCopiesItsOwnContracts(t *testing.T) {
	dir := tempModule(t, map[string]string{"main.go": auditedApp, "ops_test.go": auditedOps})
	mustGen(t, dir)
	if output, err := goTool(t, dir, nil, "build", "-o", "/dev/null", "."); err != nil {
		t.Fatalf("the ordinary build failed:\n%s", output)
	}
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "read <nil>" {
		t.Fatalf("output = %q", got)
	}
	generated := read(t, dir, mainOutput)
	if !strings.Contains(generated, "var _ interface{ Wipe() error } = Private(nil)") {
		t.Fatalf("the private contract guard does not copy Private:\n%s", generated)
	}

	// A stale change of the private contract still fails on its guard.
	edit(t, dir, "main.go", "\tPrivate interface{ Wipe() error }\n", "\tPrivate interface {\n\t\tWipe() error\n\t\tSeal() error\n\t}\n")
	edit(t, dir, "main.go", "func (n *node) Wipe() error  { return nil }\n", "func (n *node) Wipe() error  { return nil }\nfunc (n *node) Seal() error  { return nil }\n")
	output := buildFails(t, dir, "missing method Seal")
	reportedIn(t, output, mainOutput, "does not implement Private (missing method Seal)")
}

// promotedHolderApp has a second hierarchy, over peer, whose objects are
// initialized on their own. FIELDS is node's field list and STALE what main
// does before calling the wrapper.
const promotedHolderApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type Dispatch = heiry.Node

type (
	Public  interface{ Read() string }
	Private interface{}
)

type node struct {
FIELDS	label string
}

func (n *node) Read() string { return n.label }

type peer struct {
	Dispatch
	label string
}

func (p *peer) Read() string { return p.label }

//heiry:spec
const recordsSpec = ` + "`" + `
operations Public Private suffix=Op
hierarchy records node
level node public=Public private=Private
	node ReadOp
wrapper view holder=Dispatch
` + "`" + `

//heiry:spec
const peersSpec = ` + "`" + `
hierarchy peers peer
level peer public=Public private=Private
	node ReadOp
` + "`" + `

func main() {
	first, other := &node{label: "first"}, &peer{label: "peer"}
	if err := recordsInit(first); err != nil {
		fmt.Println("error:", err)
		return
	}
	if err := peersInit(other); err != nil {
		fmt.Println("error:", err)
		return
	}
STALE	fmt.Println((&view{state: first}).Read())
}
`

// A holder can also be replaced by one promoted from an embedded field, by
// pointer or by value, one or more levels down, while the leaf embeds
// heiry.Node directly: the library installs the direct one, and the stale
// forwarders reach the promoted one, which another hierarchy initialized.
// The guard accepts only a holder declared in the leaf itself.
func TestStalePromotedHolder(t *testing.T) {
	initEmbedded := func(path string) string {
		return "\tfirst." + path + ".label = \"embedded peer\"\n\tif err := peersInit(&first." + path + "); err != nil {\n\t\tfmt.Println(\"error:\", err)\n\t\treturn\n\t}\n"
	}
	for _, tc := range []struct{ name, fields, stale, want string }{
		{"through a pointer", "\theiry.Node\n\t*peer\n", "\tfirst.peer = other\n", "ambiguous selector"},
		{"through a value", "\theiry.Node\n\tpeer\n", initEmbedded("peer"), "ambiguous selector"},
		{"two levels down", "\theiry.Node\n\touter\n", initEmbedded("outer.peer"), "value of type *struct{}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			valid := strings.NewReplacer("FIELDS", "\tDispatch\n", "STALE", "").Replace(promotedHolderApp)
			dir := tempModule(t, map[string]string{"main.go": valid})
			mustGen(t, dir)
			if got := mustRun(t, dir); got != "first" {
				t.Fatalf("output = %q", got)
			}
			edit(t, dir, "main.go", "type node struct {\n\tDispatch\n", "type outer struct{ peer }\n\ntype node struct {\n"+tc.fields)
			edit(t, dir, "main.go", "\tfmt.Println((&view{state: first}).Read())\n", tc.stale+"\tfmt.Println((&view{state: first}).Read())\n")
			output, err := goTool(t, dir, nil, "build", "-o", "/dev/null", ".")
			if err == nil {
				run, _ := goTool(t, dir, nil, "run", ".")
				t.Fatalf("the stale wrapper compiled, and the program printed %q instead of its own object's label", strings.TrimSpace(run))
			}
			t.Logf("after the stale edit:\n%s", output)
			reportedIn(t, output, mainOutput, tc.want)
		})
	}
}

// nodeLeafApp's leaf type is named Node, like the heiry.Node it embeds and
// the default holder. DECLS declares it, and LEAF is its type expression.
const nodeLeafApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type (
	Public  interface{ Read() string }
	Private interface{}
)

DECLS

//heiry:spec
const recordsSpec = ` + "`" + `
operations Public Private suffix=Op
hierarchy records LEAF
level LEAF public=Public private=Private
	node ReadOp
wrapper view
` + "`" + `

func main() {
	n := &LEAF{label: "first"}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println((&view{state: n}).Read())
}
`

// The holder probe embeds the leaf, and an embedded field is named after its
// type. A leaf type named like its holder, such as Node with the default
// holder=Node, must not make the probe select that embedded field.
func TestHolderGuardForALeafNamedLikeItsHolder(t *testing.T) {
	for _, tc := range []struct{ name, decls, leaf string }{
		{"leaf named Node", "type Node struct {\n\theiry.Node\n\tlabel string\n}\n\nfunc (n *Node) Read() string { return n.label }", "Node"},
		{"generic leaf named Node", "type Node[T any] struct {\n\theiry.Node\n\tlabel string\n\textra T\n}\n\nfunc (n *Node[T]) Read() string { return n.label }", "Node[int]"},
		{"leaf alias named Node", "type record struct {\n\theiry.Node\n\tlabel string\n}\n\ntype Node = record\n\nfunc (n *record) Read() string { return n.label }", "Node"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempModule(t, map[string]string{"main.go": strings.NewReplacer("DECLS", tc.decls, "LEAF", tc.leaf).Replace(nodeLeafApp)})
			mustGen(t, dir)
			if output, err := goTool(t, dir, nil, "vet", "."); err != nil {
				t.Fatalf("the generated wrapper does not compile:\n%s", output)
			}
			if got := mustRun(t, dir); got != "first" {
				t.Fatalf("output = %q", got)
			}
			// The probe still refuses a holder promoted from an embedded
			// struct for such a leaf.
			edit(t, dir, "main.go", "\theiry.Node\n\tlabel string\n", "\tinner\n\tlabel string\n")
			write(t, dir, "inner.go", "package main\n\nimport \"github.com/lucaf/heiry\"\n\ntype inner struct{ heiry.Node }\n")
			output := buildFails(t, dir, "ambiguous selector")
			reportedIn(t, output, mainOutput, "ambiguous selector")
		})
	}
}

// resetApp's private contract is named Reset, like its one operation.
// CONTRACT declares it.
const resetApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type Public interface{ Read() string }

CONTRACT

type node struct {
	heiry.Node
	count int
}

func (n *node) Read() string { return fmt.Sprint("read ", n.count) }
func (n *node) Reset()       { n.count = 0 }

//heiry:spec
const recordsSpec = ` + "`" + `
operations Public Reset suffix=Op
hierarchy records node
level node public=Public private=Reset
	node ReadOp ResetOp
wrapper view
` + "`" + `

func main() {
	n := &node{count: 3}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	n.Node.Virtual[ResetOp]().Reset()
	fmt.Println((&view{state: n}).Read())
}
`

// The private-name probe embeds the private contract, and an embedded field
// is named after its type. A private operation named like the contract, such
// as Reset in Reset, must not make the probe's selector find that field.
func TestPrivateProbeForAnOperationNamedLikeItsContract(t *testing.T) {
	for _, tc := range []struct{ name, contract, emptied string }{
		{"contract named Reset", "type Reset interface{ Reset() }", "type Reset interface{}"},
		{"local alias named Reset", "type resetter interface{ Reset() }\n\ntype Reset = resetter", "type resetter interface{}\n\ntype Reset = resetter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempModule(t, map[string]string{"main.go": strings.Replace(resetApp, "CONTRACT", tc.contract, 1)})
			mustGen(t, dir)
			if output, err := goTool(t, dir, nil, "vet", "."); err != nil {
				t.Fatalf("the generated wrapper does not compile:\n%s", output)
			}
			if got := mustRun(t, dir); got != "read 0" {
				t.Fatalf("output = %q", got)
			}

			// A handwritten wrapper method of the operation's name.
			write(t, dir, "exposed.go", "package main\n\nfunc (v *view) Reset() { v.state.Reset() }\n")
			output := buildFails(t, dir, "ambiguous selector")
			reportedIn(t, output, mainOutput, "ambiguous selector probe.Reset")

			// The operation removed from the contract, without regenerating.
			if err := os.Remove(filepath.Join(dir, "exposed.go")); err != nil {
				t.Fatal(err)
			}
			edit(t, dir, "main.go", tc.contract, tc.emptied)
			output = buildFails(t, dir, "Reset")
			reportedIn(t, output, mainOutput, "probe.Reset undefined")
		})
	}
}
