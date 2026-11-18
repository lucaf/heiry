package main

import (
	"fmt"
	"os"
	"testing"
)

// Example runs the program and checks its complete output.
func Example() {
	if err := run(os.Stdout); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// item                 self interface         heiry
	// "milk"               <* milk>               <* milk>
	// ""                   error: empty item      error: empty item
	// "a dozen eggs"       error: item too long   error: item too long
	// rejected by the limit: self interface 1, heiry 1
	// both implementations agree
}

var sink string

// BenchmarkRender measures the same successful Render call through each
// implementation. Every variant does the same checks and string building, so
// the difference is the cost of the dispatch mechanism for this call shape.
func BenchmarkRender(b *testing.B) {
	plan := fancy.MustPrepare()
	dynamic, err := newHeiryRenderer(plan, 10)
	if err != nil {
		b.Fatal(err)
	}
	precomputed, err := newHeiryRenderer(plan.MustPrecompute(), 10)
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		renderer Renderer
	}{
		{"self-interface", newSelfRenderer(10)},
		{"heiry-dynamic", dynamic},
		{"heiry-precomputed", precomputed},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var out string
			for b.Loop() {
				out, _ = tc.renderer.Render("milk")
			}
			sink = out
		})
	}
}
