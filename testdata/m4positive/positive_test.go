package m4positive

import (
	"testing"

	"github.com/lucaf/heiry"
)

type read interface{ Read() int }
type public interface{ Read() int }
type private interface{}

type root struct{ heiry.Node }

func (*root) Read() int { return 1 }

func TestPublicDeclarationCompilesAndValidates(t *testing.T) {
	definition := heiry.Define[root]().
		Level[root, public, private]("").
		Bind[read]((*root)(nil), ".").End()
	if err := definition.Validate(); err != nil {
		t.Fatalf("external declaration: %v", err)
	}
}
