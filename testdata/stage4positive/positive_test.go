package stage4positive

import (
	"testing"

	"github.com/lucaf/heiry/testdata/stage4boundary"
)

type publicValue struct{}

func (*publicValue) Read() string { return "public" }

func TestPublicConsumer(t *testing.T) {
	got, private := stage4boundary.Consume(&publicValue{})
	if got != "public" {
		t.Fatalf("Read = %q", got)
	}
	if private {
		t.Fatal("private method leaked")
	}
}
