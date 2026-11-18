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
	// Virtual calls: the most-derived Validator and Transformer are selected,
	// whatever the leaf type is. Each call goes through this level's own
	// holder, the embedded heiry.Node.
	if err := b.Node.Virtual[Validator]().Validate(msg); err != nil {
		return "", err
	}
	b.processed++
	return b.Node.Virtual[Transformer]().Transform(msg), nil
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
	return l.Node.Parent[Validator]().Validate(msg)
}

// tagged overrides Process, Describe and Transform, but not Validate.
type tagged struct {
	limited
	heiry.Node
	tag string
}

func (t *tagged) Process(msg string) (string, error) {
	// limited registers no Processor, so this parent call reaches base.
	out, err := t.Node.Parent[Processor]().Process(msg)
	if err != nil {
		return "", fmt.Errorf("%s: %w", t.tag, err)
	}
	return "[" + t.tag + "] " + out, nil
}

func (t *tagged) Describe() string {
	return "tagged > " + t.Node.Parent[Describer]().Describe()
}

func (t *tagged) Transform(msg string) string {
	return strings.ToUpper(t.Node.Parent[Transformer]().Transform(msg))
}

// hierarchy is declared once, without any object. The root names its
// contracts; each Extend adds the next level, which keeps those contracts and
// finds its parent among its embedded fields. BindNode states that the level's
// own node implements one operation, and the compiler checks that it does.
var hierarchy = heiry.Define[tagged]().
	Level[base, Public, Private]("").
	BindNode[Processor]((*base)(nil)).
	BindNode[Describer]((*base)(nil)).
	BindNode[Validator]((*base)(nil)).
	BindNode[Transformer]((*base)(nil)).
	Extend[limited]().
	BindNode[Validator]((*limited)(nil)).
	Extend[tagged]().
	BindNode[Processor]((*tagged)(nil)).
	BindNode[Describer]((*tagged)(nil)).
	BindNode[Transformer]((*tagged)(nil))

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
	return p.state.Node.Virtual[Processor]().Process(msg)
}

func (p *pipeline) Describe() string { return p.state.Node.Virtual[Describer]().Describe() }

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
	fmt.Fprintln(out, "virtual call from the same base:", t.limited.base.Node.Virtual[Describer]().Describe())

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
