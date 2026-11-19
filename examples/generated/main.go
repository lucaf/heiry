// Command generated shows the optional heirygen workflow. The handwritten
// code below holds the contracts, the level and component types, their
// method bodies, a constructor, and one specification string. heirygen
// writes the rest into main.heirygen.go: the operation interfaces, the
// declaration with one compiler-checked registration per operation, the
// initialization helpers, and the public wrapper.
//
// Regenerate it with: go generate ./examples/generated
// Run it with:        go run ./examples/generated
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

// The contracts list the operations with their signatures. heirygen derives
// one single-method interface per method, named by the suffix the
// specification gives: SendOp, FormatOp and RecordOp.
type (
	Public  interface{ Send(to, text string) string }
	Private interface {
		Format(text string) string
		Record(line string)
	}
)

// sender is the root level. Its node implements Send and Format; its log
// component implements Record.
type sender struct {
	heiry.Node
	log journal
}

type journal struct {
	heiry.Node
	lines []string
}

func (j *journal) Record(line string) { j.lines = append(j.lines, line) }

func (s *sender) Send(to, text string) string {
	// Virtual calls select the most-derived Format and Record.
	message := s.Node.Virtual[FormatOp]().Format(text)
	s.Node.Virtual[RecordOp]().Record(to + ": " + message)
	return message
}

func (s *sender) Format(text string) string { return strings.TrimSpace(text) }

// signed overrides Format and extends the root's through a parent call.
type signed struct {
	sender
	heiry.Node
	signature string
}

func (s *signed) Format(text string) string {
	return s.Node.Parent[FormatOp]().Format(text) + " -- " + s.signature
}

// notifierSpec is heirygen's input. The operations line asks for the
// operation interfaces. The hierarchy lists the levels from the root and,
// for each, the operations its node and its components register. The
// wrapper line asks for the public wrapper type mailer.
//
//heiry:spec
const notifierSpec = `
operations Public Private suffix=Op
hierarchy notifier signed
level sender public=Public private=Private
	node SendOp FormatOp
	component log journal RecordOp
level signed
	node FormatOp
wrapper mailer
`

// Sent is a handwritten method on the generated wrapper.
func (m *mailer) Sent() []string { return m.state.log.lines }

// newMailer is an ordinary constructor with its own arguments. The generated
// helper initializes the object, preparing the declaration on first use.
func newMailer(signature string) (*mailer, error) {
	s := &signed{signature: signature}
	if err := notifierInit(s); err != nil {
		return nil, err
	}
	return &mailer{state: s}, nil
}

func run(out io.Writer) error {
	m, err := newMailer("Ada")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "send:", m.Send("bob", "  hello  "))
	fmt.Fprintln(out, "sent:", m.Sent())

	// The private operations are not part of the wrapper. Each is checked
	// on its own.
	var consumer any = m
	_, format := consumer.(FormatOp)
	_, record := consumer.(RecordOp)
	fmt.Fprintln(out, "wrapper exposes Format:", format, "Record:", record)

	// One precomputed Plan initializes independent objects.
	plan, err := notifier.Prepare()
	if err != nil {
		return err
	}
	fast := plan.MustPrecompute()
	for _, name := range []string{"Grace", "Linus"} {
		s := &signed{signature: name}
		if err := notifierInitPlan(fast, s); err != nil {
			return err
		}
		fmt.Fprintln(out, "precomputed:", (&mailer{state: s}).Send("eve", "hi"))
	}

	// Setup errors are the library's typed errors.
	var diagnostic *heiry.Error
	if errors.As(notifierInit(m.state), &diagnostic) {
		fmt.Fprintln(out, "second initialization:", diagnostic.Code)
	}
	if errors.As(notifierInit(nil), &diagnostic) {
		fmt.Fprintln(out, "nil object:", diagnostic.Code)
	}
	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
}
