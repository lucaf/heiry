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
	// repeated Prepare returns the same Plan: true
	// dynamic     Dear Ada! Hello, Ada (Dr)
	// precomputed Dear Ada! Hello, Ada (Dr)
	// lazy        Dear Ada! Hello, Ada (Dr)
	// second object: Dear Alan! Hello, Alan (Prof)
	// greetings counted per object: first=2 second=1
	// second Init of one object: AlreadyInitialized
	// incomplete declaration: MissingMethods Sign
}
