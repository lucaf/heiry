package main

import (
	"fmt"
	"os"
)

// Example runs the program and checks its complete output.
func Example() {
	if err := run(os.Stdout); err != nil {
		fmt.Println("error:", err)
	}
	// Output:
	// describe: tagged > base
	// process "  hello  ": [audit] HELLO
	// process "this message is too long": error: audit: message longer than 12 bytes
	// process "": error: audit: empty message
	// processed: 1
	// wrapper exposes Validate: false
	// direct call t.limited.base.Describe(): base
	// virtual call from the same base: tagged > base
	// second object processed: 0
}
