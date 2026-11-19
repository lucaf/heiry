package generator

import (
	"strings"
	"testing"
)

const taggedSource = `BUILD
package main

import "github.com/lucaf/heiry"

type NAMEOp interface{ NAME() string }

type (
	NAMEPublic  interface{ NAMEOp }
	NAMEPrivate interface{}
)

type NAMENode struct{ heiry.Node }

func (n *NAMENode) NAME() string { return "NAME" }

//heiry:spec
const NAMESpec = ` + "`" + `
hierarchy NAMEHierarchy NAMENode
level NAMENode public=NAMEPublic private=NAMEPrivate
	node NAMEOp
` + "`" + `

func init() {
	n := &NAMENode{}
	if err := NAMEHierarchyInit(n); err != nil {
		panic(err)
	}
	selected = append(selected, n.Node.Virtual[NAMEOp]().NAME())
}
`

func variant(name, build string) string {
	return strings.ReplaceAll(strings.ReplaceAll(taggedSource, "NAME", name), "BUILD", build)
}

// A generated file is selected by exactly the builds that select its source
// file: it repeats the //go:build line and keeps the source file's name,
// including a platform or test suffix, at the end of its own.
func TestGeneratedFilesFollowBuildConstraints(t *testing.T) {
	dir := tempModule(t, map[string]string{
		"main.go": `package main

import (
	"fmt"
	"sort"
	"strings"
)

var selected []string

func main() {
	sort.Strings(selected)
	fmt.Println(strings.Join(selected, " "))
}
`,
		"always.go":       variant("Always", ""),
		"special.go":      variant("Special", "//go:build special && !never"),
		"plat_linux.go":   variant("Linux", ""),
		"plat_windows.go": variant("Windows", ""),
		"plat_darwin.go":  variant("Darwin", ""),
		"main_test.go": `package main

import "testing"

type TestOp interface{ Test() string }

type (
	TestPublic  interface{ TestOp }
	TestPrivate interface{}
)

//heiry:spec
const testSpec = ` + "`" + `
hierarchy testHierarchy AlwaysNode
level AlwaysNode public=AlwaysPublic private=AlwaysPrivate
	node AlwaysOp
` + "`" + `

func TestGenerated(t *testing.T) {
	n := &AlwaysNode{}
	if err := testHierarchyInit(n); err != nil {
		t.Fatal(err)
	}
}
`,
	})
	mustGen(t, dir)
	for name, want := range map[string]string{
		outputName("always.go"):       "",
		outputName("special.go"):      "//go:build special && !never",
		outputName("plat_linux.go"):   "",
		outputName("plat_windows.go"): "",
		outputName("plat_darwin.go"):  "",
		outputName("main_test.go"):    "",
	} {
		content := read(t, dir, name)
		if strings.Contains(content, "//go:build") != (want != "") || !strings.Contains(content, want) {
			t.Fatalf("%s: build constraint, want %q:\n%s", name, want, content)
		}
	}

	for _, tc := range []struct {
		env  []string
		tags string
		want string
	}{
		{env: []string{"GOOS=linux", "GOARCH=amd64"}},
		{env: []string{"GOOS=windows", "GOARCH=amd64"}},
		{env: []string{"GOOS=darwin", "GOARCH=arm64"}},
		{env: []string{"GOOS=linux", "GOARCH=amd64"}, tags: "special"},
		{env: []string{"GOOS=linux", "GOARCH=amd64"}, tags: "special,never"},
	} {
		if output, err := goTool(t, dir, tc.env, "vet", "-tags", tc.tags, "."); err != nil {
			t.Fatalf("go vet %v -tags %q:\n%s", tc.env, tc.tags, output)
		}
	}

	// On this platform, with and without the tag, the program runs exactly
	// the hierarchies its build selects, and the test file's hierarchy exists
	// only under go test.
	host, err := goTool(t, dir, nil, "env", "GOOS")
	if err != nil {
		t.Fatal(err)
	}
	platform := map[string]string{"linux": "Linux", "windows": "Windows", "darwin": "Darwin"}[strings.TrimSpace(host)]
	if platform == "" {
		t.Skipf("no platform file for %s", host)
	}
	if got := mustRun(t, dir); got != "Always "+platform {
		t.Fatalf("output = %q", got)
	}
	output, err := goTool(t, dir, nil, "run", "-tags", "special", ".")
	if err != nil || strings.TrimSpace(output) != "Always "+platform+" Special" {
		t.Fatalf("tagged output = %q, %v", output, err)
	}
	if output, err := goTool(t, dir, nil, "test", "."); err != nil {
		t.Fatalf("go test:\n%s", output)
	}
	if exit, message := gen("-check", dir); exit != 0 {
		t.Fatalf("check exited %d:\n%s", exit, message)
	}
}

// Go selects files for a platform by the part of the name before the first
// dot, ignoring what precedes the first underscore. The output of a source
// must be selected exactly when the source is, also for sources whose own
// names are platform names.
func TestOutputNamesKeepPlatformSelection(t *testing.T) {
	sources := map[string]string{
		"linux.go":         variant("Plain", ""),      // no platform constraint
		"amd64.go":         variant("Arch", ""),       // no platform constraint
		"linux_amd64.go":   variant("Amd", ""),        // amd64 only
		"x_linux_arm64.go": variant("Pair", ""),       // linux and arm64
		"y_windows.go":     variant("Win", ""),        // windows only
		"darwin_test.go":   variant("Probe", ""),      // a test file, no constraint
		"z_linux_test.go":  variant("LinuxProbe", ""), // a linux test file
		"w.tag_linux.go":   variant("Dotted", ""),     // no constraint: cut at the first dot
	}
	files := map[string]string{"main.go": "package main\n\nvar selected []string\n\nfunc main() {}\n"}
	for name, content := range sources {
		files[name] = content
	}
	dir := tempModule(t, files)
	mustGen(t, dir)

	for _, platform := range [][2]string{{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "amd64"}, {"linux", "arm64"}, {"windows", "amd64"}} {
		env := []string{"GOOS=" + platform[0], "GOARCH=" + platform[1]}
		listed, err := goTool(t, dir, env, "list", "-f", "{{join .GoFiles \" \"}} {{join .TestGoFiles \" \"}}", ".")
		if err != nil {
			t.Fatalf("go list %v:\n%s", env, listed)
		}
		selected := make(map[string]bool)
		for _, name := range strings.Fields(listed) {
			selected[name] = true
		}
		for source := range sources {
			if selected[source] != selected[outputName(source)] {
				t.Errorf("%v: %s selected %t, its output %s selected %t", env, source, selected[source], outputName(source), selected[outputName(source)])
			}
		}
		if output, err := goTool(t, dir, env, "vet", "."); err != nil {
			t.Errorf("go vet %v:\n%s", env, output)
		}
	}
}

// A file constrained by legacy +build lines, or by importing "C", is built
// only in some configurations. Its output must be built in exactly the same
// ones, which a //go:build line in the output achieves.
func TestOutputsFollowLegacyAndCgoConstraints(t *testing.T) {
	cgo := func(source string) string {
		return strings.Replace(source, "import \"github.com/lucaf/heiry\"", "// #include <stdlib.h>\nimport \"C\"\n\nimport \"github.com/lucaf/heiry\"", 1)
	}
	sources := map[string]string{
		"legacy.go":    variant("Legacy", "// +build special\n"),
		"legacytwo.go": variant("LegacyTwo", "// +build special\n// +build linux\n"),
		"legacyor.go":  variant("LegacyOr", "// +build special,linux darwin\n"),
		"misplaced.go": strings.Replace(variant("Misplaced", ""), "package main", "// Misplaced is ignored by the go command.\n// +build special\npackage main", 1),
		"cgo.go":       cgo(variant("Cgo", "")),
		"cgotagged.go": cgo(variant("CgoTagged", "//go:build linux\n")),
		"cgolegacy.go": cgo(variant("CgoLegacy", "// +build special\n")),
		// An import path written as a raw string is the same import.
		"rawcgo.go": strings.Replace(variant("RawCgo", ""), "import \"github.com/lucaf/heiry\"", "// #include <stdlib.h>\nimport `C`\n\nimport \"github.com/lucaf/heiry\"", 1),
		// The go command removes a leading UTF-8 byte order mark before it
		// reads the header.
		"bomgobuild.go": "\ufeff" + variant("BomGoBuild", "//go:build special\n"),
		"bomplus.go":    "\ufeff" + variant("BomPlus", "// +build special\n"),
	}
	files := map[string]string{"main.go": "package main\n\nvar selected []string\n\nfunc main() {}\n"}
	for name, content := range sources {
		files[name] = content
	}
	dir := tempModule(t, files)
	mustGen(t, dir)
	for _, cgoEnabled := range []string{"0", "1"} {
		for _, platform := range [][2]string{{"darwin", "arm64"}, {"linux", "amd64"}} {
			for _, tags := range []string{"", "special"} {
				env := []string{"GOOS=" + platform[0], "GOARCH=" + platform[1], "CGO_ENABLED=" + cgoEnabled}
				listed, err := goTool(t, dir, env, "list", "-tags", tags, "-f", "{{join .GoFiles \" \"}} {{join .CgoFiles \" \"}}", ".")
				if err != nil {
					t.Fatalf("go list %v -tags %q:\n%s", env, tags, listed)
				}
				selected := make(map[string]bool)
				for _, name := range strings.Fields(listed) {
					selected[name] = true
				}
				for source := range sources {
					if selected[source] != selected[outputName(source)] {
						t.Errorf("%v -tags %q: %s selected %t, its output selected %t", env, tags, source, selected[source], selected[outputName(source)])
					}
				}
			}
		}
	}
}
