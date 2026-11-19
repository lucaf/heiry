// Command basic is a heirygen test fixture: the shipped basic example with
// its operation interfaces, declaration, references, and wrapper generated.
// The generator's tests require it to print exactly what examples/basic
// prints. It is a test fixture, not a usage example.
package main

//go:generate go run github.com/lucaf/heiry/cmd/heirygen

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lucaf/heiry"
)

// Contracts list the operations with their signatures. Public operations
// are offered to consumers; private ones exist only for the levels of the
// hierarchy. The generator derives one single-method interface per operation
// from them, named after the method: ProcessOp, DescribeOp, ValidateOp and
// TransformOp. Those interfaces are the keys used to register an
// implementation and to resolve one.
type (
	Public interface {
		Process(msg string) (string, error)
		Describe() string
	}
	Private interface {
		Validate(msg string) error
		Transform(msg string) string
	}
)

// base is the root level. It owns the processing template and a counter.
type base struct {
	heiry.Node
	processed int
}

func (b *base) Process(msg string) (string, error) {
	// Virtual calls: the most-derived ValidateOp and TransformOp are selected,
	// whatever the leaf type is. Each call goes through this level's own
	// holder, the embedded heiry.Node.
	if err := b.Node.Virtual[ValidateOp]().Validate(msg); err != nil {
		return "", err
	}
	b.processed++
	return b.Node.Virtual[TransformOp]().Transform(msg), nil
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
	// Parent call: the closest ValidateOp below this level.
	return l.Node.Parent[ValidateOp]().Validate(msg)
}

// tagged overrides Process, Describe and Transform, but not Validate.
type tagged struct {
	limited
	heiry.Node
	tag string
}

func (t *tagged) Process(msg string) (string, error) {
	// limited registers no ProcessOp, so this parent call reaches base.
	out, err := t.Node.Parent[ProcessOp]().Process(msg)
	if err != nil {
		return "", fmt.Errorf("%s: %w", t.tag, err)
	}
	return "[" + t.tag + "] " + out, nil
}

func (t *tagged) Describe() string {
	return "tagged > " + t.Node.Parent[DescribeOp]().Describe()
}

func (t *tagged) Transform(msg string) string {
	return strings.ToUpper(t.Node.Parent[TransformOp]().Transform(msg))
}

// hierarchySpec is the generator's input. The operations line asks for the
// operation interfaces of the two contracts. The hierarchy lists its levels
// from the root, and for each level the operations its own node registers.
// The wrapper line asks for the public wrapper type pipeline, which holds
// the object in the named field state and forwards every public operation.
//
//heiry:spec
const hierarchySpec = `
operations Public Private suffix=Op
hierarchy hierarchy tagged
level base public=Public private=Private
	node ProcessOp DescribeOp ValidateOp TransformOp
level limited
	node ValidateOp
level tagged
	node ProcessOp DescribeOp TransformOp
wrapper pipeline
`

// newTagged is an ordinary constructor: it allocates one object and sets its
// fields. The generated helper then passes the object and a reference to
// every level to Init, which checks the object against the hierarchy and
// gives every level its dispatch scope. The first Init also validates the
// declaration.
func newTagged(tag string, maxLen int) (*tagged, error) {
	t := &tagged{tag: tag}
	t.maxLen = maxLen
	if err := hierarchyInit(t); err != nil {
		return nil, err
	}
	return t, nil
}

// Pipeline is what consumers receive: the public operations and nothing else.
type Pipeline interface {
	Public
	Processed() int
}

// Processed is added by hand to the generated wrapper. It reads the counter
// that base.Process increments on the embedded base value of this same
// object.
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
	_, exposed := p.(ValidateOp)
	fmt.Fprintln(out, "wrapper exposes Validate:", exposed)

	// An ordinary Go call is not virtual: it runs the body the selector names.
	t, err := newTagged("direct", 12)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "direct call t.limited.base.Describe():", t.limited.base.Describe())
	fmt.Fprintln(out, "virtual call from the same base:", t.limited.base.Node.Virtual[DescribeOp]().Describe())

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
