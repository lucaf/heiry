package generator

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func twoSources(t *testing.T) string {
	t.Helper()
	other := strings.Replace(shapeSource, "hierarchy shape leaf", "hierarchy other leaf", 1)
	return sourceDir(t, map[string]string{
		"a.go": shapeSource, "b.go": strings.Replace(other, "const s", "const t", 1),
		// a's output exists and is stale; b's does not exist yet.
		"a.heirygen.go": marker("a.go") + "\npackage main\n\n// stale\n",
	})
}

func noTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	for name := range snapshot(t, dir) {
		if strings.HasPrefix(name, ".heirygen-") {
			t.Errorf("temporary file %s was left behind", name)
		}
	}
}

// When one output cannot be written, no output changes: every output is
// written to a temporary file before any is put in place.
func TestFailedWriteChangesNoOutput(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("relies on POSIX directory permissions for a non-root user")
	}
	dir := twoSources(t)
	before := snapshot(t, dir)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	if _, err := Generate(dir); err == nil {
		t.Fatal("generation succeeded in a read-only directory")
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	unchanged(t, dir, before)
	noTemporaryFiles(t, dir)
}

// A replaced output keeps its permission bits; a new one gets 0644.
func TestOutputPermissions(t *testing.T) {
	dir := twoSources(t)
	if err := os.Chmod(filepath.Join(dir, "a.heirygen.go"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Written) != 2 {
		t.Fatalf("written = %v", report.Written)
	}
	for name, want := range map[string]os.FileMode{"a.heirygen.go": 0o600, "b.heirygen.go": 0o644} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode %v, want %v", name, got, want)
		}
	}
	noTemporaryFiles(t, dir)

	// A second run finds everything up to date and rewrites nothing.
	if report, err := Generate(dir); err != nil || len(report.Written)+len(report.Removed) != 0 {
		t.Fatalf("second run: %+v, %v", report, err)
	}
}

// failingWriter makes one chosen operation fail and passes the others on.
type failingWriter struct {
	osWriter
	failTemp, failRename, failRemove, failLink int // 1-based call to fail, 0 for none
	temps, renames, removes, links             int
	// linkErr is the error a failing link returns; errInjected by default.
	linkErr error
}

func (w *failingWriter) link(from, to string) error {
	if w.links++; w.links == w.failLink {
		if w.linkErr != nil {
			return w.linkErr
		}
		return errInjected
	}
	return w.osWriter.link(from, to)
}

func (w *failingWriter) writeTemp(dir string, data []byte, perm os.FileMode) (string, error) {
	if w.temps++; w.temps == w.failTemp {
		return "", errInjected
	}
	return w.osWriter.writeTemp(dir, data, perm)
}

func (w *failingWriter) rename(from, to string) error {
	if w.renames++; w.renames == w.failRename {
		return errInjected
	}
	return w.osWriter.rename(from, to)
}

func (w *failingWriter) remove(path string) error {
	if !strings.Contains(path, ".heirygen-") {
		if w.removes++; w.removes == w.failRemove {
			return errInjected
		}
	}
	return w.osWriter.remove(path)
}

var errInjected = &Error{Msg: "injected failure"}

func withWriter(t *testing.T, w fileWriter) {
	t.Helper()
	previous := writer
	writer = w
	t.Cleanup(func() { writer = previous })
}

func TestInjectedWriteFailures(t *testing.T) {
	t.Run("the second temporary file", func(t *testing.T) {
		dir := twoSources(t)
		before := snapshot(t, dir)
		withWriter(t, &failingWriter{failTemp: 2})
		if _, err := Generate(dir); err == nil || !strings.Contains(err.Error(), "no output was changed") {
			t.Fatalf("err = %v", err)
		}
		unchanged(t, dir, before)
		noTemporaryFiles(t, dir)
	})

	// Publishing a new output after an existing one was replaced leaves each
	// output whole: one is new, the other not created, and the message says
	// which. a.heirygen.go is replaced by renaming; b.heirygen.go is new and
	// is given its name with a link.
	t.Run("publishing the second output", func(t *testing.T) {
		dir := twoSources(t)
		// Link 1 checks that the file system has hard links; link 2 gives
		// b.heirygen.go its name.
		withWriter(t, &failingWriter{failLink: 2, linkErr: &os.LinkError{Op: "link", Err: fs.ErrPermission}})
		report, err := Generate(dir)
		if err == nil || !strings.Contains(err.Error(), "already put in place: a.heirygen.go") {
			t.Fatalf("err = %v", err)
		}
		if len(report.Written) != 1 || report.Written[0] != "a.heirygen.go" {
			t.Fatalf("written = %v", report.Written)
		}
		if _, present := snapshot(t, dir)["b.heirygen.go"]; present {
			t.Fatal("b.heirygen.go exists although its rename failed")
		}
		noTemporaryFiles(t, dir)
		// Running again completes the work.
		withWriter(t, osWriter{})
		mustGen(t, dir)
		if exit, message := gen("-check", dir); exit != 0 {
			t.Fatalf("check: %s", message)
		}
	})

	// A new output is given its name with a hard link, so that it never
	// replaces a file. Without hard links the command fails before it
	// changes any output, rather than write the final file in place.
	t.Run("a file system without hard links", func(t *testing.T) {
		dir := twoSources(t)
		before := snapshot(t, dir)
		withWriter(t, &failingWriter{failLink: 1, linkErr: &os.LinkError{Op: "link", Err: errors.ErrUnsupported}})
		_, err := Generate(dir)
		if err == nil || !strings.Contains(err.Error(), "putting a new output in place needs a hard link") || !strings.Contains(err.Error(), "no output was changed") {
			t.Fatalf("err = %v", err)
		}
		unchanged(t, dir, before)
		noTemporaryFiles(t, dir)
	})

	t.Run("removing an obsolete output", func(t *testing.T) {
		dir := twoSources(t)
		write(t, dir, "c.heirygen.go", marker("c.go")+"\npackage main\n")
		withWriter(t, &failingWriter{failRemove: 1})
		if _, err := Generate(dir); err == nil || !strings.Contains(err.Error(), "removing obsolete") {
			t.Fatalf("err = %v", err)
		}
		if exit, message := gen("-check", dir); exit != 1 || !strings.Contains(message, "c.heirygen.go: no specification") {
			t.Fatalf("check: exit %d:\n%s", exit, message)
		}
		noTemporaryFiles(t, dir)
	})
}

// Check mode never writes, renames, or removes.
func TestCheckModeWritesNothing(t *testing.T) {
	dir := twoSources(t)
	write(t, dir, "c.heirygen.go", marker("c.go")+"\npackage main\n")
	before := snapshot(t, dir)
	withWriter(t, &failingWriter{failTemp: 1, failRename: 1, failRemove: 1})
	findings, err := Check(dir)
	if err != nil || len(findings) != 3 {
		t.Fatalf("findings %v, err %v", findings, err)
	}
	unchanged(t, dir, before)
}
