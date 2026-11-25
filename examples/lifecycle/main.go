// Command lifecycle shows the three separate steps of using a hierarchy:
// declaring it, preparing a reusable Plan, and initializing each object.
// It prepares eagerly and lazily, reuses one Plan for several objects, and
// compares dynamic with precomputed resolution.
//
// Run it with: go run ./examples/lifecycle
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/lucaf/heiry"
)

type (
	Greeter interface{ Greet(name string) string }
	Signer  interface{ Sign() string }

	Public  interface{ Greeter }
	Private interface{ Signer }
)

type plain struct {
	heiry.Node
	greetings int
}

func (p *plain) Greet(name string) string {
	p.greetings++
	return "Hello, " + name + p.Node.Virtual[Signer]().Sign()
}

func (p *plain) Sign() string { return "." }

type formal struct {
	plain
	heiry.Node
	title string
}

func (f *formal) Greet(name string) string {
	return "Dear " + name + "! " + f.Node.Parent[Greeter]().Greet(name)
}

func (f *formal) Sign() string { return " (" + f.title + ")" }

// declare describes the hierarchy. It needs no object and runs no
// application code; the result holds only types, paths and registrations.
// BindGroup with the path "." registers several operations for a level's own
// node; the declaration's validation checks that the node implements each of
// them. End returns from each level to the definition.
func declare() *heiry.Definition[formal] {
	return heiry.Define[formal]().
		Level[plain, Public, Private]("").
		BindGroup((*plain)(nil), ".", heiry.Of[Greeter](), heiry.Of[Signer]()).End().
		Extend[formal]().
		BindGroup((*formal)(nil), ".", heiry.Of[Greeter](), heiry.Of[Signer]()).
		End()
}

// newFormal allocates an object with an ordinary constructor signature and
// binds it to an already prepared Plan. Only this step is repeated per object.
func newFormal(plan *heiry.Plan[formal], title string) (*formal, error) {
	f := &formal{title: title}
	err := plan.Init(f, plan.NodeProvider(&f.plain), plan.NodeProvider(f))
	return f, err
}

func greet(f *formal, name string) string { return f.Node.Virtual[Greeter]().Greet(name) }

func run(out io.Writer) error {
	// Declaration. Validate checks it without freezing it or creating a Plan.
	definition := declare()
	if err := definition.Validate(); err != nil {
		return err
	}

	// Eager preparation: validate once and build the Plan at startup, before
	// any object exists.
	dynamic, err := definition.Prepare()
	if err != nil {
		return err
	}
	again, err := definition.Prepare()
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "repeated Prepare returns the same Plan:", again == dynamic)

	// Optional precomputation derives a second immutable Plan. The dynamic
	// Plan stays usable.
	precomputed, err := dynamic.Precompute()
	if err != nil {
		return err
	}

	// Lazy preparation: a definition that was never prepared is validated and
	// prepared by its first Init.
	lazyDefinition := declare()
	lazy := &formal{title: "Dr"}
	if err := lazyDefinition.Init(lazy, lazyDefinition.NodeProvider(&lazy.plain), lazyDefinition.NodeProvider(lazy)); err != nil {
		return err
	}

	first, err := newFormal(dynamic, "Dr")
	if err != nil {
		return err
	}
	fast, err := newFormal(precomputed, "Dr")
	if err != nil {
		return err
	}

	results := []struct {
		mode   string
		object *formal
	}{{"dynamic", first}, {"precomputed", fast}, {"lazy", lazy}}
	var want string
	for index, result := range results {
		got := greet(result.object, "Ada")
		if index == 0 {
			want = got
		} else if got != want {
			return fmt.Errorf("%s result %q differs from dynamic %q", result.mode, got, want)
		}
		fmt.Fprintf(out, "%-12s%s\n", result.mode, got)
	}

	// One Plan, independent objects: each has its own state and scopes.
	second, err := newFormal(dynamic, "Prof")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "second object:", greet(second, "Alan"))
	greet(first, "Grace")
	fmt.Fprintf(out, "greetings counted per object: first=%d second=%d\n", first.greetings, second.greetings)

	// An initialized object cannot be initialized again.
	err = dynamic.Init(first, dynamic.NodeProvider(&first.plain), dynamic.NodeProvider(first))
	fmt.Fprintln(out, "second Init of one object:", code(err))

	// An invalid declaration is reported by preparation, before any object is
	// involved. Here nothing implements Sign.
	_, err = heiry.Define[formal]().
		Level[plain, Public, Private]("").
		Bind[Greeter]((*plain)(nil), ".").End().
		Extend[formal]().
		Bind[Greeter]((*formal)(nil), ".").End().
		Prepare()
	var diagnostic *heiry.Error
	if !errors.As(err, &diagnostic) {
		return fmt.Errorf("incomplete declaration was accepted: %v", err)
	}
	fmt.Fprintf(out, "incomplete declaration: %s %s\n", diagnostic.Code, diagnostic.Name)
	return nil
}

// code returns the stable code of a heiry error.
func code(err error) heiry.Code {
	var diagnostic *heiry.Error
	if errors.As(err, &diagnostic) {
		return diagnostic.Code
	}
	return ""
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
}
