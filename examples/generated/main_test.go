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
	// send: hello -- Ada
	// sent: [bob: hello -- Ada]
	// wrapper exposes Format: false Record: false
	// precomputed: hi -- Grace
	// precomputed: hi -- Linus
	// second initialization: AlreadyInitialized
	// nil object: InvalidProvider
}
