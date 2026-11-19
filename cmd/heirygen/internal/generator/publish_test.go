package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// cleanupWriter refuses to remove the hidden files heirygen makes whose names
// end with suffix, and records them. With partialTemp, that call of writeTemp
// leaves a partial file behind and fails.
type cleanupWriter struct {
	failingWriter
	suffix      string
	partialTemp int
	partials    int
	refused     []string
}

func (w *cleanupWriter) writeTemp(dir string, data []byte, perm os.FileMode) (string, error) {
	if w.partials++; w.partials == w.partialTemp {
		path, err := w.osWriter.writeTemp(dir, data[:len(data)/2], perm)
		if err != nil {
			return "", err
		}
		return path, errInjected
	}
	return w.failingWriter.writeTemp(dir, data, perm)
}

func (w *cleanupWriter) remove(path string) error {
	if w.suffix != "" && strings.HasPrefix(filepath.Base(path), ".heirygen-") && strings.HasSuffix(path, w.suffix) {
		w.refused = append(w.refused, path)
		return errInjected
	}
	return w.failingWriter.remove(path)
}

// hiddenFiles returns, in name order, the hidden files heirygen made in dir.
func hiddenFiles(t *testing.T, dir string) []string {
	t.Helper()
	var hidden []string
	for _, name := range names(t, dir) {
		if strings.HasPrefix(name, ".heirygen-") {
			hidden = append(hidden, filepath.Join(dir, name))
		}
	}
	return hidden
}

// A failure while removing obsolete outputs comes after every new or changed
// output is in place, and the error says so, with the outputs already removed.
func TestObsoleteRemovalFailureReportsProgress(t *testing.T) {
	dir := twoSources(t)
	write(t, dir, "c.heirygen.go", marker("c.go")+"\npackage main\n")
	write(t, dir, "d.heirygen.go", marker("d.go")+"\npackage main\n")
	withWriter(t, &failingWriter{failRemove: 2})
	_, err := Generate(dir)
	if err == nil {
		t.Fatal("the injected failure was not reported")
	}
	t.Logf("error: %v", err)
	for _, want := range []string{filepath.Join(dir, "d.heirygen.go"), "already put in place: a.heirygen.go, b.heirygen.go", "already removed: c.heirygen.go"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the error does not say %q: %v", want, err)
		}
	}
	if exit, message := gen("-check", dir); exit != 1 || message != filepath.Join(dir, "d.heirygen.go")+": no specification produces this file; run go generate" {
		t.Fatalf("check: exit %d:\n%s", exit, message)
	}
	noTemporaryFiles(t, dir)
}

// A hidden file that heirygen cannot remove is reported with its path, and
// the error says which outputs were put in place, also when every output is.
func TestCleanupFailuresNameThePathsLeftBehind(t *testing.T) {
	for _, tc := range []struct {
		name   string
		writer *cleanupWriter
		want   []string // in the error, besides the paths left behind
		check  int      // exit of check mode afterwards
	}{
		{"temporary file of a linked output", &cleanupWriter{suffix: ".tmp"}, []string{"every output is up to date"}, 0},
		{"hard-link probe", &cleanupWriter{suffix: ".tmp.link"}, []string{"every output is up to date"}, 0},
		{"temporary file discarded after a failure", &cleanupWriter{failingWriter: failingWriter{failLink: 2}, suffix: ".tmp"}, []string{"already put in place: a.heirygen.go"}, 1},
		{"partial temporary file", &cleanupWriter{partialTemp: 2, suffix: ".tmp"}, []string{"no output was changed"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := twoSources(t)
			withWriter(t, tc.writer)
			_, err := Generate(dir)
			if err == nil {
				t.Fatal("generation succeeded although a hidden file was left behind")
			}
			t.Logf("error: %v", err)
			left := hiddenFiles(t, dir)
			refused := slices.Sorted(slices.Values(tc.writer.refused))
			if len(left) == 0 || !slices.Equal(left, refused) {
				t.Fatalf("left behind %v, refused to remove %v", left, tc.writer.refused)
			}
			for _, want := range append(tc.want, left...) {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("the error does not say %q: %v", want, err)
				}
			}
			withWriter(t, osWriter{})
			if exit, message := gen("-check", dir); exit != tc.check {
				t.Fatalf("check: exit %d, want %d:\n%s", exit, tc.check, message)
			}
		})
	}
}

// After a case-only rename the new output is in place. If its hidden copy of
// the old output cannot be removed, the outputs after it are still put in
// place, and the error says that every output is up to date and names that
// file.
func TestCaseOnlyRenameCleanupFailure(t *testing.T) {
	other := strings.Replace(strings.Replace(shapeSource, "hierarchy shape leaf", "hierarchy other leaf", 1), "const s", "const t", 1)
	dir := sourceDir(t, map[string]string{"Main.go": shapeSource, "z.go": other})
	if !caseInsensitive(t, dir) {
		t.Skip("a case-only rename is two files on this file system")
	}
	mustGen(t, dir)
	old := read(t, dir, "Main.heirygen.go")
	// z's output is put in place after main's.
	if err := os.Remove(filepath.Join(dir, "z.heirygen.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "Main.go"), filepath.Join(dir, "main.go")); err != nil {
		t.Fatal(err)
	}
	w := &cleanupWriter{suffix: ".tmp.old"}
	withWriter(t, w)
	_, err := Generate(dir)
	if err == nil {
		t.Fatal("generation succeeded although the old output was left behind")
	}
	t.Logf("error: %v", err)
	if len(w.refused) != 1 || !strings.Contains(err.Error(), w.refused[0]) || !strings.Contains(err.Error(), "every output is up to date") {
		t.Fatalf("refused %v; err = %v", w.refused, err)
	}
	if got, _ := os.ReadFile(w.refused[0]); string(got) != old {
		t.Fatalf("%s does not hold the old output", w.refused[0])
	}
	if got := strings.Join(names(t, dir), " "); !strings.HasSuffix(got, "main.go main.heirygen.go z.go z.heirygen.go") {
		t.Fatalf("files: %s", got)
	}
	withWriter(t, osWriter{})
	if exit, message := gen("-check", dir); exit != 0 {
		t.Fatalf("check: exit %d:\n%s", exit, message)
	}
}

// The output of a source renamed in case only takes the old output's place
// and keeps its permission bits, whether the file system sees one file or two.
func TestCaseOnlyRenameKeepsPermissions(t *testing.T) {
	dir := sourceDir(t, map[string]string{"Main.go": shapeSource})
	mustGen(t, dir)
	if err := os.Chmod(filepath.Join(dir, "Main.heirygen.go"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "Main.go"), filepath.Join(dir, "main.go")); err != nil {
		t.Fatal(err)
	}
	mustGen(t, dir)
	if got := strings.Join(names(t, dir), " "); got != "main.go main.heirygen.go" {
		t.Fatalf("files: %s", got)
	}
	info, err := os.Stat(filepath.Join(dir, "main.heirygen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode %v, want %v", got, os.FileMode(0o600))
	}
	noTemporaryFiles(t, dir)
}
