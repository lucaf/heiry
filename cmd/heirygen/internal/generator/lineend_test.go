package generator

import (
	"strings"
	"testing"
)

// A checkout can convert line endings to CRLF. heirygen recognizes its
// marker on such a file and compares line endings as LF, so an output that
// differs only in line endings is up to date; it always writes LF.
func TestCRLFOutputs(t *testing.T) {
	dir := sourceDir(t, map[string]string{"main.go": shapeSource})
	mustGen(t, dir)
	lf := read(t, dir, "main.heirygen.go")
	write(t, dir, "main.heirygen.go", strings.ReplaceAll(lf, "\n", "\r\n"))
	before := snapshot(t, dir)
	if exit, message := gen("-check", dir); exit != 0 {
		t.Fatalf("check on a CRLF output: exit %d:\n%s", exit, message)
	}
	if report, err := Generate(dir); err != nil || len(report.Written)+len(report.Removed) != 0 {
		t.Fatalf("generate on a CRLF output: %+v, %v", report, err)
	}
	unchanged(t, dir, before)

	// A stale CRLF output is reported, check mode leaves it alone, and
	// generation replaces it with LF.
	stale := strings.ReplaceAll(strings.Replace(lf, "// The declarations below", "// Stale: the declarations below", 1), "\n", "\r\n")
	write(t, dir, "main.heirygen.go", stale)
	before = snapshot(t, dir)
	if exit, message := gen("-check", dir); exit != 1 || !strings.Contains(message, "main.heirygen.go: differs from the expected output") {
		t.Fatalf("check on a stale CRLF output: exit %d:\n%s", exit, message)
	}
	unchanged(t, dir, before)
	mustGen(t, dir)
	if got := read(t, dir, "main.heirygen.go"); got != lf {
		t.Fatalf("regenerated output is not the LF output:\n%q", got)
	}
}
