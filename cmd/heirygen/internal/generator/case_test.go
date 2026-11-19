package generator

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// caseInsensitive reports whether dir's file system treats names that differ
// only in case as one file, and logs which kind the case ran on.
func caseInsensitive(t *testing.T, dir string) bool {
	t.Helper()
	write(t, dir, "case-probe", "")
	defer os.Remove(filepath.Join(dir, "case-probe"))
	_, err := os.Lstat(filepath.Join(dir, "CASE-PROBE"))
	t.Logf("file system at %s is case-insensitive: %t (%s)", dir, err == nil, runtime.GOOS)
	return err == nil
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var list []string
	for _, entry := range entries {
		list = append(list, entry.Name())
	}
	return list
}

// Renaming a source file in case only renames its output with it. On a
// case-insensitive file system the old and the new output name are one
// file, so the new output must not be removed as the old one's obsolete
// leftover.
func TestCaseOnlySourceRename(t *testing.T) {
	dir := sourceDir(t, map[string]string{"Main.go": shapeSource})
	caseInsensitive(t, dir)
	mustGen(t, dir)
	if err := os.Rename(filepath.Join(dir, "Main.go"), filepath.Join(dir, "main.go")); err != nil {
		t.Fatal(err)
	}
	mustGen(t, dir)
	got := strings.Join(names(t, dir), " ")
	if got != "main.go main.heirygen.go" {
		t.Fatalf("files after the rename: %s", got)
	}
	if content := read(t, dir, "main.heirygen.go"); !strings.HasPrefix(content, marker("main.go")) {
		t.Fatalf("output does not belong to main.go:\n%s", content)
	}
	if exit, message := gen("-check", dir); exit != 0 {
		t.Fatalf("check: exit %d:\n%s", exit, message)
	}
}

// An output name that differs only in case from an existing file is refused
// on every file system: the go command rejects such a pair of names, and on
// a case-insensitive file system they are one file.
func TestCaseEquivalentHandwrittenDestination(t *testing.T) {
	dir := sourceDir(t, map[string]string{"main.go": shapeSource, "main.HEIRYGEN.go": "package main\n\n// Written by hand.\n"})
	caseInsensitive(t, dir)
	before := snapshot(t, dir)
	for _, mode := range [][]string{{dir}, {"-check", dir}} {
		exit, message := gen(mode...)
		if exit != 1 || !strings.Contains(message, "main.heirygen.go differs only in case from the existing main.HEIRYGEN.go") {
			t.Errorf("%v: exit %d:\n%s", mode, exit, message)
		}
	}
	unchanged(t, dir, before)
}

// A symbolic link at an output path is never replaced or followed, whether
// it points at a handwritten file or nowhere.
func TestSymlinkAtOutputPath(t *testing.T) {
	for _, target := range []string{"handwritten.txt", "missing.txt"} {
		dir := sourceDir(t, map[string]string{"main.go": shapeSource, "handwritten.txt": "keep me"})
		if err := os.Symlink(target, filepath.Join(dir, "main.heirygen.go")); err != nil {
			t.Skipf("symbolic links are not available: %v", err)
		}
		before := snapshot(t, dir)
		for _, mode := range [][]string{{dir}, {"-check", dir}} {
			if exit, message := gen(mode...); exit != 1 || !strings.Contains(message, "main.heirygen.go is not a regular file") {
				t.Errorf("link to %s, %v: exit %d:\n%s", target, mode, exit, message)
			}
		}
		unchanged(t, dir, before)
	}
}

// interferingWriter lets another writer act on the directory after heirygen
// has read it and before it publishes, as a concurrent editor might.
type interferingWriter struct {
	osWriter
	interfere func()
	done      bool
}

func (w *interferingWriter) writeTemp(dir string, data []byte, perm os.FileMode) (string, error) {
	if !w.done {
		w.done = true
		w.interfere()
	}
	return w.osWriter.writeTemp(dir, data, perm)
}

// A file that appears at a new output's path after heirygen read the
// directory is not replaced.
func TestDestinationCreatedAfterPlanning(t *testing.T) {
	for _, name := range []string{"main.heirygen.go", "MAIN.heirygen.go"} {
		dir := sourceDir(t, map[string]string{"main.go": shapeSource})
		insensitive := caseInsensitive(t, dir)
		if name != "main.heirygen.go" && !insensitive {
			continue // two distinct files on this file system
		}
		withWriter(t, &interferingWriter{interfere: func() { write(t, dir, name, "// Written by hand meanwhile.\n") }})
		_, err := Generate(dir)
		if err == nil || !strings.Contains(err.Error(), "was created after heirygen read the directory") {
			t.Fatalf("%s: err = %v", name, err)
		}
		if got := read(t, dir, name); got != "// Written by hand meanwhile.\n" {
			t.Fatalf("%s was replaced:\n%s", name, got)
		}
		noTemporaryFiles(t, dir)
	}
}

// An owned output that is replaced by another file after heirygen read the
// directory is neither replaced nor removed in turn.
func TestOutputReplacedAfterPlanning(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"a stale output", map[string]string{"main.go": shapeSource}},
		// main.go has no specification any more, so its output is obsolete;
		// a.go's output is written first, which is when the file changes.
		{"an obsolete output", map[string]string{"main.go": "package main\n", "a.go": strings.Replace(shapeSource, "const s", "const a", 1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.files["main.heirygen.go"] = marker("main.go") + "\npackage main\n\n// stale\n"
			dir := sourceDir(t, tc.files)
			path := filepath.Join(dir, "main.heirygen.go")
			withWriter(t, &interferingWriter{interfere: func() {
				os.Remove(path)
				write(t, dir, "main.heirygen.go", "// Written by hand meanwhile.\n")
			}})
			if _, err := Generate(dir); err == nil || !strings.Contains(err.Error(), "changed after heirygen read it") {
				t.Fatalf("err = %v", err)
			}
			if got := read(t, dir, "main.heirygen.go"); got != "// Written by hand meanwhile.\n" {
				t.Fatalf("the new file was replaced or removed:\n%s", got)
			}
			noTemporaryFiles(t, dir)
		})
	}
}

// Two sources whose names differ only in case can exist only on a
// case-sensitive file system. Their outputs would too, and the go command
// rejects the pair, so the generator refuses.
func TestOutputsThatDifferOnlyInCase(t *testing.T) {
	dir := sourceDir(t, map[string]string{"main.go": shapeSource})
	if caseInsensitive(t, dir) {
		t.Skip("two sources that differ only in case cannot exist on this file system")
	}
	write(t, dir, "Main.go", strings.Replace(shapeSource, "hierarchy shape", "hierarchy other", 1))
	before := snapshot(t, dir)
	if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "differ only in case, because their sources do") {
		t.Fatalf("exit %d:\n%s", exit, message)
	}
	unchanged(t, dir, before)
}

// stepWriter fails the nth operation that renames or links a file whose
// name is, in any case, one of names.
type stepWriter struct {
	osWriter
	names []string
	fail  int
	steps int
}

func (w *stepWriter) involved(paths ...string) bool {
	for _, path := range paths {
		for _, name := range w.names {
			if strings.EqualFold(filepath.Base(path), name) {
				return true
			}
		}
	}
	return false
}

func (w *stepWriter) step(paths ...string) error {
	if w.involved(paths...) {
		if w.steps++; w.steps == w.fail {
			return errInjected
		}
	}
	return nil
}

func (w *stepWriter) rename(from, to string) error {
	if err := w.step(from, to); err != nil {
		return err
	}
	return w.osWriter.rename(from, to)
}

func (w *stepWriter) link(from, to string) error {
	if err := w.step(from, to); err != nil {
		return err
	}
	return w.osWriter.link(from, to)
}

// A case-only rename that fails partway, on a file system that keeps the old
// spelling of a replaced file, must leave a state the next run completes.
// Ownership stays exact: an output whose marker names a source of another
// spelling is still refused.
func TestInterruptedCaseOnlyRename(t *testing.T) {
	dir := sourceDir(t, map[string]string{"Main.go": shapeSource})
	if !caseInsensitive(t, dir) {
		t.Skip("a case-only rename is two files on this file system")
	}
	mustGen(t, dir)
	if err := os.Rename(filepath.Join(dir, "Main.go"), filepath.Join(dir, "main.go")); err != nil {
		t.Fatal(err)
	}
	withWriter(t, &stepWriter{names: []string{"main.heirygen.go"}, fail: 2})
	if _, err := Generate(dir); err == nil || !strings.Contains(err.Error(), "the old output Main.heirygen.go was restored") {
		t.Fatalf("err = %v", err)
	}
	t.Logf("after the failure: %s", strings.Join(names(t, dir), " "))
	noTemporaryFiles(t, dir)
	withWriter(t, osWriter{})
	if exit, message := gen(dir); exit != 0 {
		t.Fatalf("rerun: exit %d:\n%s", exit, message)
	}
	if got := strings.Join(names(t, dir), " "); got != "main.go main.heirygen.go" {
		t.Fatalf("files after the rerun: %s", got)
	}
	if content := read(t, dir, "main.heirygen.go"); !strings.HasPrefix(content, marker("main.go")) {
		t.Fatalf("output does not belong to main.go:\n%s", content)
	}
	if exit, message := gen("-check", dir); exit != 0 {
		t.Fatalf("check: exit %d:\n%s", exit, message)
	}

	stale := sourceDir(t, map[string]string{"main.go": shapeSource, "Main.heirygen.go": marker("main.go") + "\npackage main\n"})
	if exit, message := gen(stale); exit != 1 || !strings.Contains(message, "Main.heirygen.go has the name of a heirygen output but does not start with its marker") {
		t.Fatalf("an output named for another spelling: exit %d:\n%s", exit, message)
	}
}

// asideWriter creates a handwritten file at a name right after the old
// output of a case-only rename is moved aside, before the new one is linked
// into place, as an editor or another run might.
type asideWriter struct {
	osWriter
	dir, name, content string
	aside              string // where the old output was moved
}

func (w *asideWriter) rename(from, to string) error {
	if err := w.osWriter.rename(from, to); err != nil {
		return err
	}
	if strings.HasSuffix(to, ".old") && w.aside == "" {
		w.aside = to
		return os.WriteFile(filepath.Join(w.dir, w.name), []byte(w.content), 0o644)
	}
	return nil
}

// A file that appears at the destination of a case-only rename while the old
// output is aside is not replaced, neither by the new output nor by putting
// the old one back. The old output is kept under the hidden name the error
// reports, and following the reported step recovers it.
func TestCaseOnlyRenameKeepsAFileThatAppears(t *testing.T) {
	dir := sourceDir(t, map[string]string{"Main.go": shapeSource})
	if !caseInsensitive(t, dir) {
		t.Skip("a case-only rename is two files on this file system")
	}
	mustGen(t, dir)
	old := read(t, dir, "Main.heirygen.go")
	if err := os.Rename(filepath.Join(dir, "Main.go"), filepath.Join(dir, "main.go")); err != nil {
		t.Fatal(err)
	}
	const handwritten = "// Written by hand meanwhile.\n"
	w := &asideWriter{dir: dir, name: "main.heirygen.go", content: handwritten}
	withWriter(t, w)
	_, err := Generate(dir)
	if err == nil {
		t.Fatal("generation succeeded although a file appeared at its output")
	}
	if w.aside == "" {
		t.Fatal("the old output was never moved aside")
	}
	t.Logf("error: %v", err)
	if got := read(t, dir, "main.heirygen.go"); got != handwritten {
		t.Fatalf("the handwritten file was replaced:\n%s", got)
	}
	if !strings.Contains(err.Error(), "was kept as "+w.aside) || !strings.Contains(err.Error(), "rename "+w.aside+" to "+filepath.Join(dir, "Main.heirygen.go")) {
		t.Fatalf("the error does not report the kept old output and how to recover it: %v", err)
	}
	if got, err := os.ReadFile(w.aside); err != nil || string(got) != old {
		t.Fatalf("the old output is not recoverable at %s: %v\n%s", w.aside, err, got)
	}

	// Recovery, as the error says: deal with the new file, then put the old
	// output back and run again.
	withWriter(t, osWriter{})
	if err := os.Remove(filepath.Join(dir, "main.heirygen.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(w.aside, filepath.Join(dir, "Main.heirygen.go")); err != nil {
		t.Fatal(err)
	}
	mustGen(t, dir)
	if got := strings.Join(names(t, dir), " "); got != "main.go main.heirygen.go" {
		t.Fatalf("files after recovery: %s", got)
	}
	if exit, message := gen("-check", dir); exit != 0 {
		t.Fatalf("check: exit %d:\n%s", exit, message)
	}
}
