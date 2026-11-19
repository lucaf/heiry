package generator

import (
	"strings"
	"testing"
)

// fullApp uses everything the extended prototype generates: operation
// interfaces from contracts, one of which embeds another, the declaration,
// the helpers, and a public wrapper.
const fullApp = `package main

import (
	"fmt"

	"github.com/lucaf/heiry"
)

type Base interface {
	Read(key string) (string, bool)
}

type Public interface {
	Base
	Audit() string
}

type Private interface {
	Seal(reason string) error
}

type node struct {
	heiry.Node
	sealed string
}

func (n *node) Read(key string) (string, bool) { return key, true }
func (n *node) Audit() string                  { return "audit" }
func (n *node) Seal(reason string) error       { n.sealed = reason; return nil }

//heiry:spec
const recordsSpec = ` + "`" + `
operations Public Private suffix=Op
hierarchy records node
level node public=Public private=Private
	node ReadOp AuditOp SealOp
wrapper view
` + "`" + `

func main() {
	n := &node{}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	var consumer any = &view{state: n}
	_, whole := consumer.(Private)
	fmt.Println("implements the whole private contract:", whole)
	report(consumer)
}
`

// probe.go checks each operation individually, by an interface declared
// here, so the check does not depend on which interfaces are generated.
const fullProbe = `package main

import "fmt"

func report(consumer any) {
	_, read := consumer.(interface{ Read(key string) (string, bool) })
	_, audit := consumer.(interface{ Audit() string })
	_, seal := consumer.(interface{ Seal(reason string) error })
	fmt.Println("exposes Read:", read, "Audit:", audit, "Seal:", seal)
}
`

func fullModule(t *testing.T) string {
	t.Helper()
	dir := tempModule(t, map[string]string{"main.go": fullApp, "probe.go": fullProbe})
	mustGen(t, dir)
	const fresh = "implements the whole private contract: false\nexposes Read: true Audit: true Seal: false"
	if got := mustRun(t, dir); got != fresh {
		t.Fatalf("fresh output = %q", got)
	}
	return dir
}

// Each case changes handwritten source other than the spec constant and does
// not regenerate. The compiler must reject the stale generated file.
func TestStaleOperationsAndWrapper(t *testing.T) {
	for _, tc := range []struct {
		name string
		// The edit: a replacement in main.go, or a new handwritten file.
		old, new, added string
		// stale is the compile error without regeneration.
		stale string
		// After regeneration the program prints output, or still fails to
		// compile with rebuilt, or the generator refuses with refused.
		output, rebuilt, refused string
	}{
		{
			name:  "move a public operation into the private contract",
			old:   "\tBase\n\tAudit() string\n}\n\ntype Private interface {\n",
			new:   "\tBase\n}\n\ntype Private interface {\n\tAudit() string\n",
			stale: "(missing method Audit)",
			// Each private operation is checked on its own: Audit is gone
			// from the wrapper.
			output: "implements the whole private contract: false\nexposes Read: true Audit: false Seal: false",
		},
		{
			name: "remove a public operation",
			old:  "\tBase\n\tAudit() string\n", new: "\tBase\n",
			stale: "(missing method Audit)",
			// The spec still registers the operation, whose interface no
			// longer exists. The generator says so at the specification line.
			refused: "AuditOp is not declared in the handwritten files of package main",
		},
		{
			name: "add a private operation",
			old:  "\tSeal(reason string) error\n}", new: "\tSeal(reason string) error\n\tWipe()\n}",
			stale: "does not implement Private (missing method Wipe)",
			// Nothing registers it: validation reports it at first use, as
			// for a handwritten declaration.
			output: "error: MissingMethods: private Wipe capability <nil> at level 0 (expected func(), actual <nil>)",
		},
		{
			name: "change an operation's signature in the contract",
			old:  "\tSeal(reason string) error\n}", new: "\tSeal(reason string, force bool) error\n}",
			stale: "(wrong type for method Seal)",
			// The provider's method still has the old signature.
			rebuilt: "*node does not implement SealOp (wrong type for method Seal)",
		},
		{
			name: "change an embedded contract",
			old:  "\tRead(key string) (string, bool)\n}", new: "\tRead(key string) (string, bool)\n\tScan() []string\n}",
			stale:  "does not implement Public (missing method Scan)",
			output: "error: MissingMethods: public Scan capability <nil> at level 0 (expected func() []string, actual <nil>)",
		},
		{
			name:  "add a handwritten method to the wrapper that exposes a private operation",
			added: "package main\n\nfunc (v *view) Seal(reason string) error {\n\treturn v.state.Node.Virtual[SealOp]().Seal(reason)\n}\n",
			stale: "ambiguous selector",
			// Regeneration changes nothing: the method has to go.
			rebuilt: "ambiguous selector",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := fullModule(t)
			if tc.added != "" {
				write(t, dir, "extra.go", tc.added)
			} else {
				edit(t, dir, "main.go", tc.old, tc.new)
			}
			output := buildFails(t, dir, tc.stale)
			if !strings.Contains(output, mainOutput) {
				t.Fatalf("the failure is not reported in the generated file:\n%s", output)
			}
			for _, command := range []string{"vet", "test"} {
				if output, err := goTool(t, dir, nil, command, "."); err == nil || !strings.Contains(output, tc.stale) {
					t.Fatalf("go %s: %v\n%s", command, err, output)
				}
			}

			if tc.refused != "" {
				if exit, message := gen(dir); exit != 1 || !strings.Contains(message, tc.refused) {
					t.Fatalf("regeneration: exit %d, want %q:\n%s", exit, tc.refused, message)
				}
				return
			}
			mustGen(t, dir)
			if tc.rebuilt != "" {
				buildFails(t, dir, tc.rebuilt)
				return
			}
			if got := mustRun(t, dir); got != tc.output {
				t.Fatalf("regenerated output = %q, want %q", got, tc.output)
			}
		})
	}
}
