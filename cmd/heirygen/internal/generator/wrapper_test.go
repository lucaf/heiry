package generator

import (
	"strings"
	"testing"
)

// holderApp's leaf embeds one heiry.Node, as the library requires, and
// declares one more Node field by name, which the library never installs.
const holderApp = `package main

import (
	"errors"
	"fmt"

	"github.com/lucaf/heiry"
)

type Dispatch = heiry.Node

type (
	Public  interface{ Read() string }
	Private interface{}
)

type node struct {
	EMBEDDED
	NAMED heiry.Node
}

func (n *node) Read() string { return "read" }

//heiry:spec
const recordsSpec = ` + "`" + `
operations Public Private suffix=Op
hierarchy records node
level node public=Public private=Private
	node ReadOp
WRAPPER
` + "`" + `

func main() {
	n := &node{}
	if err := recordsInit(n); err != nil {
		fmt.Println("error:", err)
		return
	}
	defer func() {
		var diagnostic *heiry.Error
		if err, ok := recover().(error); ok && errors.As(err, &diagnostic) {
			fmt.Println("panic:", diagnostic.Code)
		}
	}()
	fmt.Println(VIEW.Read())
}
`

func holderSource(embedded, named, wrapper, view string) string {
	return strings.NewReplacer("EMBEDDED", embedded, "NAMED", named, "WRAPPER", wrapper, "VIEW", view).Replace(holderApp)
}

// Specification S8: the wrapper is unexported and keeps the object in named
// unexported storage.
func TestWrapperIsUnexported(t *testing.T) {
	for _, tc := range []struct{ wrapper, view, want string }{
		{"wrapper View", "(&View{state: n})", "wrapper View must be unexported"},
		{"wrapper view field=State", "(&view{State: n})", "field=State must be unexported"},
		{"wrapper _", "(&view{state: n})", "wrapper _ must be named"},
		{"wrapper view field=_", "(&view{state: n})", "field=_ must be named"},
	} {
		dir := sourceDir(t, map[string]string{"main.go": holderSource("heiry.Node", "Aux", tc.wrapper, tc.view)})
		if exit, message := gen(dir); exit != 1 || !strings.Contains(message, tc.want) {
			t.Errorf("%s: exit %d, want %q:\n%s", tc.wrapper, exit, tc.want, message)
		}
	}
}

// The forwarder dispatches through the leaf's directly embedded Node, the
// holder the library installs. A Node field that is merely named is never
// installed, and dispatching through it would fail at run time.
func TestForwarderUsesTheEmbeddedHolder(t *testing.T) {
	for _, tc := range []struct{ name, embedded, named, wrapper, want string }{
		{"a named holder selected explicitly", "heiry.Node", "Aux", "wrapper view holder=Aux", "holder=Aux names a field that node does not embed; its directly embedded heiry.Node is Node"},
		{"a named holder selected by default", "Dispatch", "Node", "wrapper view", "the default holder=Node names a field that node does not embed; its directly embedded heiry.Node is Dispatch: write holder=Dispatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := sourceDir(t, map[string]string{"main.go": holderSource(tc.embedded, tc.named, tc.wrapper, "(&view{state: n})")})
			if exit, message := gen(dir); exit != 1 || !strings.Contains(message, tc.want) {
				t.Fatalf("exit %d, want %q:\n%s", exit, tc.want, message)
			}
		})
	}
}

// Controls: the embedded holder, under its own name or an alias name, with
// another Node field beside it, initializes and dispatches.
func TestForwarderHolderControls(t *testing.T) {
	for _, tc := range []struct{ name, embedded, named, wrapper string }{
		{"embedded Node beside a named one", "heiry.Node", "Aux", "wrapper view"},
		{"embedded alias beside a field named Node", "Dispatch", "Node", "wrapper view holder=Dispatch"},
		{"embedded alias, explicit holder", "Dispatch", "Aux", "wrapper view holder=Dispatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempModule(t, map[string]string{"main.go": holderSource(tc.embedded, tc.named, tc.wrapper, "(&view{state: n})")})
			mustGen(t, dir)
			mustBuild(t, dir)
			if got := mustRun(t, dir); got != "read" {
				t.Fatalf("output = %q", got)
			}
		})
	}
}
