// Command basic shows a three-level heiry hierarchy: selective overriding,
// inherited implementations, parent dispatch, state shared through value
// embedding, and a public wrapper that hides an inheritance-private operation.
//
// Run it with: go run ./examples/basic
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lucaf/heiry"
)

// Each operation has one canonical single-method interface. It is the key
// used to register an implementation and to resolve one.
type (
	Processor interface {
		Process(msg string) (string, error)
	}
	Describer   interface{ Describe() string }
	Validator   interface{ Validate(msg string) error }
	Transformer interface{ Transform(msg string) string }
)

// Contracts group the operations. Public operations are offered to
// consumers; private ones exist only for the levels of the hierarchy.
type (
	Public interface {
		Processor
		Describer
	}
	Private interface {
		Validator
		Transformer
	}
)

// base is the root level. It owns the processing template and a counter.
type base struct {
	heiry.Node
	processed int
}

func (b *base) Process(msg string) (string, error) {
	scope := b.Node.Scope()
	// Virtual calls: the most-derived Validator and Transformer are selected,
	// whatever the leaf type is.
	if err := heiry.Method[Validator](scope).Validate(msg); err != nil {
		return "", err
	}
	b.processed++
	return heiry.Method[Transformer](scope).Transform(msg), nil
}

func (b *base) Describe() string { return "base" }

func (b *base) Validate(msg string) error {
	if msg == "" {
		return errors.New("empty message")
	}
	return nil
}

func (b *base) Transform(msg string) string { return strings.TrimSpace(msg) }

// limited overrides Validate only. Everything else is inherited from base.
type limited struct {
	base
	heiry.Node
	maxLen int
}

func (l *limited) Validate(msg string) error {
	if len(msg) > l.maxLen {
		return fmt.Errorf("message longer than %d bytes", l.maxLen)
	}
	// Parent call: the closest Validator below this level.
	return heiry.ParentMethod[Validator](l.Node.Scope()).Validate(msg)
}

// tagged overrides Process, Describe and Transform, but not Validate.
type tagged struct {
	limited
	heiry.Node
	tag string
}

func (t *tagged) Process(msg string) (string, error) {
	// limited registers no Processor, so this parent call reaches base.
	out, err := heiry.ParentMethod[Processor](t.Node.Scope()).Process(msg)
	if err != nil {
		return "", fmt.Errorf("%s: %w", t.tag, err)
	}
	return "[" + t.tag + "] " + out, nil
}

func (t *tagged) Describe() string {
	return "tagged > " + heiry.ParentMethod[Describer](t.Node.Scope()).Describe()
}

func (t *tagged) Transform(msg string) string {
	return strings.ToUpper(heiry.ParentMethod[Transformer](t.Node.Scope()).Transform(msg))
}

// hierarchy is declared once, without any object. Each level names its node
// type, its contracts and the field that embeds its parent, then lists the
// operations that level implements. "." means the node itself provides them.
var hierarchy = heiry.Define[tagged]().
	Level[base, Public, Private]("").
	Bind[Processor]((*base)(nil), ".").
	Bind[Describer]((*base)(nil), ".").
	Bind[Validator]((*base)(nil), ".").
	Bind[Transformer]((*base)(nil), ".").End().
	Level[limited, Public, Private]("base").
	Bind[Validator]((*limited)(nil), ".").End().
	Level[tagged, Public, Private]("limited").
	Bind[Processor]((*tagged)(nil), ".").
	Bind[Describer]((*tagged)(nil), ".").
	Bind[Transformer]((*tagged)(nil), ".").End()

// newTagged is an ordinary constructor: it allocates one object and sets its
// fields. Init then checks the object against the hierarchy and gives every
// level its dispatch scope. The first Init also validates the declaration.
func newTagged(tag string, maxLen int) (*tagged, error) {
	t := &tagged{tag: tag}
	t.maxLen = maxLen
	err := hierarchy.Init(t,
		hierarchy.NodeProvider(&t.limited.base),
		hierarchy.NodeProvider(&t.limited),
		hierarchy.NodeProvider(t))
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Pipeline is what consumers receive: the public operations and nothing else.
type Pipeline interface {
	Public
	Processed() int
}

// pipeline is the public wrapper. It keeps the object in a named field, so
// no inherited or private method is promoted onto it, and it forwards each
// public operation through a virtual call.
type pipeline struct{ state *tagged }

func (p *pipeline) Process(msg string) (string, error) {
	return heiry.Method[Processor](p.state.Node.Scope()).Process(msg)
}

func (p *pipeline) Describe() string {
	return heiry.Method[Describer](p.state.Node.Scope()).Describe()
}

// Processed reads the counter that base.Process increments on the embedded
// base value of this same object.
func (p *pipeline) Processed() int { return p.state.processed }

func NewPipeline(tag string, maxLen int) (Pipeline, error) {
	t, err := newTagged(tag, maxLen)
	if err != nil {
		return nil, err
	}
	return &pipeline{state: t}, nil
}

func run(out io.Writer) error {
	p, err := NewPipeline("audit", 12)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "describe:", p.Describe())

	for _, msg := range []string{"  hello  ", "this message is too long", ""} {
		result, err := p.Process(msg)
		if err != nil {
			fmt.Fprintf(out, "process %q: error: %v\n", msg, err)
			continue
		}
		fmt.Fprintf(out, "process %q: %s\n", msg, result)
	}
	fmt.Fprintln(out, "processed:", p.Processed())

	// The private operations are not part of the wrapper's method set.
	_, exposed := p.(Validator)
	fmt.Fprintln(out, "wrapper exposes Validate:", exposed)

	// An ordinary Go call is not virtual: it runs the body the selector names.
	t, err := newTagged("direct", 12)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "direct call t.limited.base.Describe():", t.limited.base.Describe())
	fmt.Fprintln(out, "virtual call from the same base:", heiry.Method[Describer](t.limited.base.Node.Scope()).Describe())

	// Objects are independent: a second pipeline has its own state.
	fmt.Fprintln(out, "second object processed:", (&pipeline{state: t}).Processed())
	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
}
