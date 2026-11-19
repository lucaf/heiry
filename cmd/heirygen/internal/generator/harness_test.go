package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot is the heiry module directory: this package is
// cmd/heirygen/internal/generator inside it.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "declaration.go")); err != nil {
		t.Fatalf("heiry module not found at %s: %v", root, err)
	}
	return root
}

// tempModule creates a throwaway module outside the repository that imports
// the actual heiry package through a replace directive. Tests edit and break
// its files, so they never touch maintained sources. These cases spend their
// time in child go commands, so -short skips them.
func tempModule(t *testing.T, files map[string]string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds a throwaway module with the go command")
	}
	dir := t.TempDir()
	write(t, dir, "go.mod", "module probe\n\ngo 1.27.1\n\nrequire github.com/lucaf/heiry v0.0.0\n\nreplace github.com/lucaf/heiry => "+repoRoot(t)+"\n")
	for name, content := range files {
		write(t, dir, name, content)
	}
	return dir
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// edit replaces one occurrence of old in a file and fails if it is absent,
// so a case cannot pass because its edit silently did nothing.
func edit(t *testing.T, dir, name, old, new string) {
	t.Helper()
	content := read(t, dir, name)
	if strings.Count(content, old) != 1 {
		t.Fatalf("%s: want exactly one %q", name, old)
	}
	write(t, dir, name, strings.Replace(content, old, new, 1))
}

// goTool runs the go command in dir, offline, and returns its combined output.
func goTool(t *testing.T, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command("go", args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off", "GOPROXY=off")
	command.Env = append(command.Env, env...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func mustBuild(t *testing.T, dir string) {
	t.Helper()
	if output, err := goTool(t, dir, nil, "vet", "."); err != nil {
		t.Fatalf("go vet failed:\n%s", output)
	}
}

func mustRun(t *testing.T, dir string) string {
	t.Helper()
	output, err := goTool(t, dir, nil, "run", ".")
	if err != nil {
		t.Fatalf("go run failed:\n%s", output)
	}
	return strings.TrimSpace(output)
}

// buildFails requires go build to fail with a diagnostic containing want.
func buildFails(t *testing.T, dir, want string) string {
	t.Helper()
	output, err := goTool(t, dir, nil, "build", "-o", os.DevNull, ".")
	if err == nil {
		t.Fatalf("go build succeeded, want a failure mentioning %q", want)
	}
	if !strings.Contains(output, want) {
		t.Fatalf("go build failed without mentioning %q:\n%s", want, output)
	}
	return output
}

// gen runs the generator on dir, or checks dir when the first argument is
// "-check". It returns 0 on success and 1 with the message for an error or
// for findings, so that a case can assert both outcome and text.
func gen(args ...string) (int, string) {
	if len(args) == 2 && args[0] == "-check" {
		findings, err := Check(args[1])
		if err != nil {
			return 1, err.Error()
		}
		var lines []string
		for _, finding := range findings {
			lines = append(lines, finding.String())
		}
		if len(lines) != 0 {
			return 1, strings.Join(lines, "\n")
		}
		return 0, ""
	}
	if _, err := Generate(args[len(args)-1]); err != nil {
		return 1, err.Error()
	}
	return 0, ""
}

func mustGen(t *testing.T, dir string) {
	t.Helper()
	if exit, output := gen(dir); exit != 0 {
		t.Fatalf("generate %s:\n%s", dir, output)
	}
}

// mainOutput is the output generated for main.go.
var mainOutput = outputName("main.go")
