// Command comparison implements one small application twice: with a
// hand-written self interface and with heiry. It runs both on the same
// inputs and checks that they behave identically.
//
// The application formats list items. A base level renders an item after
// checking it; a strict level adds a length limit through a component; a
// fancy level changes the prefix and wraps the base rendering.
//
// Run it with: go run ./examples/comparison
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// Renderer is the behavior both implementations provide.
type Renderer interface {
	Render(item string) (string, error)
	// Rejected reports how many items the length limit refused.
	Rejected() int
}

var (
	errEmpty   = errors.New("empty item")
	errTooLong = errors.New("item too long")
)

func describe(r Renderer, item string) string {
	out, err := r.Render(item)
	if err != nil {
		return "error: " + err.Error()
	}
	return out
}

func run(out io.Writer) error {
	const limit = 10
	self := newSelfRenderer(limit)
	plan, err := fancy.Prepare()
	if err != nil {
		return err
	}
	withHeiry, err := newHeiryRenderer(plan, limit)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "%-20s %-22s %s\n", "item", "self interface", "heiry")
	for _, item := range []string{"milk", "", "a dozen eggs"} {
		a, b := describe(self, item), describe(withHeiry, item)
		if a != b {
			return fmt.Errorf("item %q: self interface gave %q, heiry gave %q", item, a, b)
		}
		fmt.Fprintf(out, "%-20q %-22s %s\n", item, a, b)
	}
	if self.Rejected() != withHeiry.Rejected() {
		return fmt.Errorf("rejected counts differ: %d and %d", self.Rejected(), withHeiry.Rejected())
	}
	fmt.Fprintf(out, "rejected by the limit: self interface %d, heiry %d\n", self.Rejected(), withHeiry.Rejected())
	fmt.Fprintln(out, "both implementations agree")
	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
}
