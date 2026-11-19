package generator

import (
	"strings"
	"testing"
)

// Generation reads the handwritten files' syntax only. These cases start
// from a package that does not compile and require one generator run to
// make it compile.
func TestBootstrap(t *testing.T) {
	t.Run("no generated output", func(t *testing.T) {
		dir := shapeModule(t, constCarrier)
		buildFails(t, dir, "undefined: shapeInit")
		mustGen(t, dir)
		mustBuild(t, dir)
	})

	t.Run("generated output does not parse", func(t *testing.T) {
		dir := shapeModule(t, constCarrier)
		write(t, dir, mainOutput, marker("main.go")+"package main\n\nfunc {\n")
		buildFails(t, dir, "syntax error")
		mustGen(t, dir)
		mustBuild(t, dir)
	})

	t.Run("generated output does not type-check", func(t *testing.T) {
		dir := shapeModule(t, constCarrier)
		mustGen(t, dir)
		edit(t, dir, "main.go", "type mid struct {", "type middle struct {")
		edit(t, dir, "main.go", "func (m *mid) Name()", "func (m *middle) Name()")
		edit(t, dir, "main.go", "\tmid\n\theiry.Node\n}", "\tmiddle\n\theiry.Node\n}")
		edit(t, dir, "main.go", "level mid\n", "level middle\n")
		buildFails(t, dir, "undefined: mid")
		mustGen(t, dir)
		mustBuild(t, dir)
		if got := mustRun(t, dir); got != "base" {
			t.Fatalf("output = %q, want base", got)
		}
	})

	// Package-level declarations, not only bodies, may use generated names:
	// an eagerly prepared Plan, a struct field of the definition's type, and
	// a method value. None of them exists before the first generation.
	t.Run("handwritten declarations use generated names", func(t *testing.T) {
		dir := shapeModule(t, constCarrier)
		write(t, dir, "eager.go", `package main

import "github.com/lucaf/heiry"

var eagerPlan = shape.MustPrepare().MustPrecompute()

type registry struct {
	definition *heiry.Definition[top]
	bind       func(*top) error
}

var defaults = registry{definition: shape, bind: shapeInit}
`)
		buildFails(t, dir, "undefined: shape")
		mustGen(t, dir)
		mustBuild(t, dir)
	})

	t.Run("unrelated type errors elsewhere", func(t *testing.T) {
		dir := shapeModule(t, constCarrier)
		write(t, dir, "broken.go", "package main\n\nvar broken missingType = notDeclared()\n")
		mustGen(t, dir)
		buildFails(t, dir, "undefined: missingType")
		if !strings.Contains(read(t, dir, mainOutput), "func shapeInit(") {
			t.Fatal("generation did not write the helpers")
		}
	})

	// The one requirement: handwritten files must be syntactically valid Go.
	t.Run("syntax error in a handwritten file", func(t *testing.T) {
		dir := shapeModule(t, constCarrier)
		write(t, dir, "broken.go", "package main\n\nfunc {\n")
		exit, message := gen(dir)
		if exit != 1 || !strings.Contains(message, "broken.go") {
			t.Fatalf("exit %d:\n%s", exit, message)
		}
	})
}
