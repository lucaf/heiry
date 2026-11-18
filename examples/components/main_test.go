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
	// foreign component rejected: ProviderOwnership at repository.store
	// object untouched by the failed Init: true
	// get color: "blue" true
	// get color: "blue" true
	// get shape: "" false
	// get color after update: "red" true
	// event: #1 put color
	// event: #2 cache miss color
	// event: #3 cache miss shape
	// event: #4 put color
	// event: #5 cache miss color
	// virtual Recorder from cache is the sibling stamper: true
	// parent Recorder from cache is the root log: true
}

// A write must be visible to later reads even when the key was cached.
func TestPutReplacesCachedValue(t *testing.T) {
	repo, err := NewRepository()
	if err != nil {
		t.Fatal(err)
	}
	repo.Put("color", "blue")
	if got, ok := repo.Get("color"); got != "blue" || !ok {
		t.Fatalf("first read = %q %t, want blue", got, ok)
	}
	repo.Put("color", "red")
	if got, ok := repo.Get("color"); got != "red" || !ok {
		t.Fatalf("read after update = %q %t, want red", got, ok)
	}
}
