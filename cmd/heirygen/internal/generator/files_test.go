package generator

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// sourceDir creates a plain directory with the given files. Cases that need
// no go command use it, so they also run under -short.
func sourceDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		write(t, dir, name, content)
	}
	return dir
}

func TestOutputNames(t *testing.T) {
	for source, want := range map[string]string{
		"main.go":          "main.heirygen.go",
		"main_test.go":     "main.heirygen_test.go",
		"linux.go":         "linux.heirygen.go",
		"x_linux_amd64.go": "x_linux_amd64.heirygen.go",
		"x_linux_test.go":  "x_linux.heirygen_test.go",
		"a.b_linux.go":     "a.b_linux.heirygen.go",
	} {
		if got := outputName(source); got != want {
			t.Errorf("outputName(%q) = %q, want %q", source, got, want)
		}
		if !isOutput(want) {
			t.Errorf("isOutput(%q) = false", want)
		}
	}
}

// A hierarchy whose helpers would be named like test functions is refused
// when its specification is in a test file: go test would treat TestXRefs
// as a test with the wrong signature.
func TestHelperNamesInTestFiles(t *testing.T) {
	spec := func(name string) string {
		return "package main\n\ntype (\n\tleaf struct{}\n\tP    interface{}\n\tQ    interface{}\n)\n\n//heiry:spec\nconst s = `\nhierarchy " + name + " leaf\nlevel leaf public=P private=Q\n`\n"
	}
	for _, name := range []string{"TestShape", "BenchmarkShape", "FuzzShape", "ExampleShape", "Test"} {
		dir := sourceDir(t, map[string]string{"shape_test.go": spec(name)})
		exit, message := gen(dir)
		if exit != 1 || !strings.Contains(message, "go test treats") || !strings.Contains(message, name+"Refs") {
			t.Errorf("%s: exit %d:\n%s", name, exit, message)
		}
	}
	// The same names are fine outside test files, and so are names whose
	// prefix is followed by a lower-case letter.
	for _, files := range []map[string]string{
		{"shape.go": spec("TestShape")},
		{"shape_test.go": spec("Testshape")},
	} {
		dir := sourceDir(t, files)
		if exit, message := gen(dir); exit != 0 {
			t.Errorf("%v: exit %d:\n%s", files, exit, message)
		}
	}
}

// shapeSource is a minimal handwritten file with one specification. The
// generator does not type-check, so the types need only be declared.
const shapeSource = "package main\n\ntype (\n\tleaf struct{}\n\tP    interface{}\n\tQ    interface{}\n)\n\n//heiry:spec\nconst s = `\nhierarchy shape leaf\nlevel leaf public=P private=Q\n`\n"

// snapshot records every file of dir, so a case can require that nothing
// changed.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string)
	for _, entry := range entries {
		switch {
		case entry.IsDir():
			files[entry.Name()+"/"] = ""
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			files[entry.Name()] = "-> " + target
		default:
			files[entry.Name()] = read(t, dir, entry.Name())
		}
	}
	return files
}

func unchanged(t *testing.T, dir string, before map[string]string) {
	t.Helper()
	if after := snapshot(t, dir); !maps.Equal(before, after) {
		t.Fatalf("the directory changed:\nbefore %v\nafter  %v", slices.Sorted(maps.Keys(before)), slices.Sorted(maps.Keys(after)))
	}
}

// A file is the generator's to replace or delete only when it has the output
// form, its first line is the marker, and the marker names the source that
// output name belongs to. Anything else is refused by both modes, and
// nothing changes.
func TestOutputOwnership(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name:  "a handwritten file with an output name",
			files: map[string]string{"main.go": shapeSource, "main.heirygen.go": "package main\n\n// Written by hand.\n"},
			want:  "main.heirygen.go has the name of a heirygen output but does not start with its marker",
		},
		{
			name:  "an unrecognized file that no source would produce",
			files: map[string]string{"main.go": shapeSource, "stray.heirygen.go": "package main\n"},
			want:  "stray.heirygen.go has the name of a heirygen output but does not start with its marker",
		},
		{
			name:  "a marker that names another source",
			files: map[string]string{"main.go": shapeSource, "main.heirygen.go": marker("other.go") + "\npackage main\n"},
			want:  "main.heirygen.go has the name of a heirygen output but does not start with its marker",
		},
		{
			name:  "a marker on a file with another name",
			files: map[string]string{"main.go": shapeSource, "copy.go": marker("main.go") + "\npackage main\n"},
			want:  "copy.go starts with a heirygen marker but is not named as the output of main.go",
		},
		{
			name:  "output of the investigation prototype",
			files: map[string]string{"main.go": shapeSource, "heirygen_main.go": "// Code generated by heirygen (investigation prototype) from main.go; DO NOT EDIT.\n\npackage main\n"},
			want:  "heirygen_main.go starts with a heirygen marker",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := sourceDir(t, tc.files)
			before := snapshot(t, dir)
			for _, mode := range [][]string{{dir}, {"-check", dir}} {
				exit, message := gen(mode...)
				if exit != 1 || !strings.Contains(message, tc.want) {
					t.Errorf("%v: exit %d, want a message containing %q:\n%s", mode, exit, tc.want, message)
				}
			}
			unchanged(t, dir, before)
		})
	}

	t.Run("a directory where an output belongs", func(t *testing.T) {
		dir := sourceDir(t, map[string]string{"main.go": shapeSource})
		if err := os.Mkdir(filepath.Join(dir, "main.heirygen.go"), 0o755); err != nil {
			t.Fatal(err)
		}
		before := snapshot(t, dir)
		if exit, message := gen(dir); exit != 1 || !strings.Contains(message, "main.heirygen.go is not a regular file") {
			t.Errorf("exit %d:\n%s", exit, message)
		}
		unchanged(t, dir, before)
	})
}

// Owned outputs whose source no longer holds a specification, or no longer
// exists, are obsolete: check mode reports them and generation removes them.
func TestObsoleteOwnedOutputs(t *testing.T) {
	dir := sourceDir(t, map[string]string{
		"main.go":                 shapeSource,
		"plain.go":                "package main\n",
		"plain.heirygen.go":       marker("plain.go") + "\npackage main\n",
		"gone.heirygen_test.go":   marker("gone_test.go") + "\npackage main\n",
		"notes.txt":               "not Go",
		"main.heirygen.go.backup": "not an output",
	})
	exit, message := gen("-check", dir)
	for _, want := range []string{"main.heirygen.go: missing", "plain.heirygen.go: no specification produces this file", "gone.heirygen_test.go: no specification produces this file"} {
		if exit != 1 || !strings.Contains(message, want) {
			t.Errorf("check: exit %d, want %q:\n%s", exit, want, message)
		}
	}
	mustGen(t, dir)
	after := snapshot(t, dir)
	for _, name := range []string{"plain.heirygen.go", "gone.heirygen_test.go"} {
		if _, present := after[name]; present {
			t.Errorf("%s was not removed", name)
		}
	}
	for _, name := range []string{"main.heirygen.go", "plain.go", "notes.txt", "main.heirygen.go.backup"} {
		if _, present := after[name]; !present {
			t.Errorf("%s is missing", name)
		}
	}
	if exit, message := gen("-check", dir); exit != 0 {
		t.Fatalf("check after generation: exit %d:\n%s", exit, message)
	}
}

// Files the go command ignores are ignored here too, even when they do not
// parse or carry a marker.
func TestIgnoredFiles(t *testing.T) {
	dir := sourceDir(t, map[string]string{
		"main.go":        shapeSource,
		"_scratch.go":    "this is not Go",
		".editor.go":     "neither is this",
		"_x.heirygen.go": "package main\n",
	})
	mustGen(t, dir)
	if _, present := snapshot(t, dir)["_x.heirygen.go"]; !present {
		t.Fatal("an ignored file was removed")
	}
}
