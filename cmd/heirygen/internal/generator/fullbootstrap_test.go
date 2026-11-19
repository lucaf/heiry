package generator

import (
	"strings"
	"testing"
)

// extras.go is handwritten and uses generated types in declarations, not
// only in bodies: a method attached to the generated wrapper, struct fields
// of the wrapper and of an operation interface, an interface that embeds a
// generated operation interface, and a package-level variable of the
// definition's type.
const fullExtras = `package main

import "github.com/lucaf/heiry"

func (v *view) Sealed() string { return v.state.sealed }

type registry struct {
	current *view
	reader  ReadOp
}

type ReadAuditor interface {
	ReadOp
	AuditOp
}

var (
	definition *heiry.Definition[node] = records
	_          ReadAuditor             = (*view)(nil)
	_                                  = registry{}
)
`

// The extended generator still reads syntax only, so handwritten
// declarations may refer to generated operation interfaces and to the
// generated wrapper before those exist.
func TestBootstrapWithOperationsAndWrapper(t *testing.T) {
	module := func(t *testing.T) string {
		t.Helper()
		return tempModule(t, map[string]string{"main.go": fullApp, "probe.go": fullProbe, "extras.go": fullExtras})
	}

	t.Run("no generated output", func(t *testing.T) {
		dir := module(t)
		buildFails(t, dir, "undefined: view")
		mustGen(t, dir)
		mustBuild(t, dir)
		if got := mustRun(t, dir); !strings.Contains(got, "exposes Read: true Audit: true Seal: false") {
			t.Fatalf("output = %q", got)
		}
	})

	t.Run("generated output does not parse", func(t *testing.T) {
		dir := module(t)
		write(t, dir, mainOutput, marker("main.go")+"package main\n\nfunc {\n")
		mustGen(t, dir)
		mustBuild(t, dir)
	})

	// A method is renamed in the contract, the body, the spec, and the
	// handwritten uses. The old generated file no longer compiles.
	t.Run("generated output is stale and does not compile", func(t *testing.T) {
		dir := module(t)
		mustGen(t, dir)
		for _, replace := range [][2]string{
			{"\tRead(key string) (string, bool)\n}", "\tFetch(key string) (string, bool)\n}"},
			{"func (n *node) Read(key string)", "func (n *node) Fetch(key string)"},
			{"\tnode ReadOp AuditOp SealOp\n", "\tnode FetchOp AuditOp SealOp\n"},
		} {
			edit(t, dir, "main.go", replace[0], replace[1])
		}
		write(t, dir, "extras.go", strings.ReplaceAll(fullExtras, "ReadOp", "FetchOp"))
		buildFails(t, dir, "duplicate key false in map literal")
		mustGen(t, dir)
		mustBuild(t, dir)
	})

	// The one circular case: a contract cannot be defined in terms of an
	// interface that is generated from it.
	t.Run("a contract that embeds a generated operation interface", func(t *testing.T) {
		dir := module(t)
		edit(t, dir, "main.go", "\tSeal(reason string) error\n}", "\tSeal(reason string) error\n\tAuditOp\n}")
		exit, message := gen(dir)
		if exit != 1 || !strings.Contains(message, "contract AuditOp is not declared in the handwritten files") {
			t.Fatalf("exit %d:\n%s", exit, message)
		}
	})
}

// forwardApp has the signature shapes a forwarder must reproduce: a variadic
// parameter, named results, a parameter named like the forwarder's receiver,
// unnamed and blank parameters, a type from another package, no parameters
// and no results, and a holder embedded under another name.
const forwardApp = `package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lucaf/heiry"
)

type Public interface {
	Write(w io.Writer, parts ...string) (n int, err error)
	Lookup(string, int) (string, bool)
	Pick(_ int, heiry string) string
	Reset()
}

type Private interface{}

type Dispatch = heiry.Node

type sink struct {
	Dispatch
	resets int
}

func (s *sink) Write(w io.Writer, parts ...string) (int, error) {
	return fmt.Fprintln(w, strings.Join(parts, "+"))
}
func (s *sink) Lookup(key string, n int) (string, bool) { return strings.Repeat(key, n), n > 0 }
func (s *sink) Pick(n int, text string) string          { return fmt.Sprint(text, n) }
func (s *sink) Reset()                                  { s.resets++ }

//heiry:spec
const sinkSpec = ` + "`" + `
operations Public Private prefix=op
hierarchy sinks sink
level sink public=Public private=Private
	node opWrite opLookup opPick opReset
wrapper view field=object holder=Dispatch
` + "`" + `

func main() {
	s := &sink{}
	if err := sinksInit(s); err != nil {
		fmt.Println("error:", err)
		return
	}
	var public Public = &view{object: s}
	n, err := public.Write(os.Stdout, "a", "b", "c")
	fmt.Println(n, err)
	fmt.Println(public.Lookup("x", 3))
	fmt.Println(public.Pick(7, "p"))
	public.Reset()
	fmt.Println(s.resets)
}
`

func TestForwarderSignatures(t *testing.T) {
	dir := tempModule(t, map[string]string{"main.go": forwardApp})
	mustGen(t, dir)
	mustBuild(t, dir)
	if got := mustRun(t, dir); got != "a+b+c\n6 <nil>\nxxx true\np7\n1" {
		t.Fatalf("output = %q", got)
	}
	generated := read(t, dir, mainOutput)
	for _, want := range []string{
		`io "io"`,
		"opWrite interface {",
		"func (w_ *view) Write(w io.Writer, parts ...string) (int, error) {",
		"return w_.object.Dispatch.Virtual[opWrite]().Write(w, parts...)",
		"func (w *view) Lookup(arg0 string, arg1 int) (string, bool) {",
		"func (w *view) Pick(arg0 int, arg1 string) string {",
		"w.object.Dispatch.Virtual[opReset]().Reset()",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("generated file lacks %q", want)
		}
	}
	if t.Failed() {
		t.Log(generated)
	}
}
