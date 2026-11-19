# Proposal: optional code generation

Status: **superseded** on 2026-11-19 by [ADR 0013](../decisions/0013-optional-code-generator.md) and milestone [G1](../roadmap.md#g1-optional-code-generator). The owner approved production implementation of the generator this document investigated; its required behavior is now [specification S10](../specification.md#s10-optional-code-generation), its design the [architecture](../architecture.md#optional-code-generator), its acceptance the [testing plan](../testing.md#generator-acceptance), and its use the [user documentation](../user/generation.md). This document is kept below as the historical record of the investigation, unchanged apart from these opening paragraphs, the labels on removed paths, commands, and status statements, and the section on running the prototype checks. Present-tense statements below describe the investigation as it ended, not the current state.

The prototype paths it names under `testdata/genprobe/` were removed when the maintained command in `cmd/heirygen` replaced them; its regressions were migrated there. The production design differs from what is described here in its output file names (`x.heirygen.go` instead of `heirygen_x.go`, which kept platform selection wrong for some source names), in establishing file ownership by marker and source, in whole-file publication of each output, in its exit statuses, and in several further diagnostics.

Previous status, for the record: unapproved proposal, updated on 2026-11-19 after a second investigation pass that extended the prototype to operation interfaces and public wrappers, and again the same day after the owner reviewed the prototype and found three gaps, which are fixed.

Every code block below is copied from files that were compiled and run. The prototype, its workloads, and its tests were under `testdata/genprobe/`, removed in G1; commands and raw outputs are in the [investigation evidence](../evidence/generator-investigation/README.md). Measured results and estimates are kept apart, and the [Evidence](#evidence) section separates what was observed from what was reasoned.

## Summary and recommendation

**1. Does the extended approach materially improve authoring?** For the setup and forwarding code, yes. For the program as a whole, modestly. For what an author has to understand, no.

| Measured, handwritten code lines | Basic | Components |
| --- | --- | --- |
| Setup and forwarding boilerplate today | 29 | 30 |
| With the first prototype (declaration and initialization generated) | 23 | 21 |
| With the extended prototype (operation interfaces and wrapper generated too) | 12 (−58.6%) | 13 (−56.7%) |
| Whole program today | 148 | 154 |
| Whole program with the extended prototype | 130 (−12.2%) | 138 (−10.4%) |

Three kinds of repetition leave the handwritten text: a separate interface per operation, a forwarder per public operation, and a reference per level and component on every initialization. The declaration becomes a short table. Dispatch calls in method bodies keep their length; only the name inside the brackets changes. The author still needs every concept the library has, and a small specification language in addition.

**2. The handwritten code, side by side,** is in [Basic example, three versions](#basic-example-three-versions) and [Components example, three versions](#components-example-three-versions).

**3. Safety.** Every stale-output case that was tested and that changes what the generated code should be is rejected by ordinary compilation: an edited spec constant, a spec constant turned into a variable, a public operation moved into the private contract or removed, a private operation added, a changed signature, a changed embedded contract, a retargeted alias, changed struct fields, and a handwritten wrapper method named after a private operation. The mechanisms were demonstrated on handwritten stand-ins before the generator relied on them.

The owner's review of the prototype on 2026-11-19 found that two of these guarantees did not hold as stated and that one supported-signature claim was wrong. All three were reproduced as failing tests and fixed; see [Owner review](#owner-review). The claims in this document describe the prototype after those fixes.

Two things are not rejected by compilation, and no design found here can make them so:

- A generated file that differs from what its spec asks for, by a hand edit or a generator defect, where the difference is well typed. Dropping a registration that an ancestor also supplies is the example. Check mode detects a hand edit; it cannot detect a defect of the generator that computes the expected text. Compiler guards verify particular properties. They cannot prove that a generator translated every instruction faithfully, so tests, review, and output checking remain necessary.
- A handwritten wrapper method with another name that calls a private operation. No compiler-visible property distinguishes it from any other method. This is equally true of handwritten wrappers today.

In three respects the generated code is checked more strictly than the shipped handwritten examples: every registration is individually compiler-checked, the wrapper cannot declare a method named after a private operation, and a nil object is reported by the library instead of by a nil dereference.

**4. Status and recommendation, at the end of the investigation.** Production implementation was later approved under [ADR 0013](../decisions/0013-optional-code-generator.md); the status recorded here was: production implementation is not approved. After the review the owner's position is that roughly halving the setup and forwarding boilerplate is worthwhile, that the residual trust boundary in the first bullet is real and separate from the fixed gaps, and that one decision comes first: whether the string-based authoring experience is attractive enough to justify maintaining the tool. The recommendation here was to retain the prototype under `testdata/genprobe/`, since removed, as evidence and do no further generator work until that decision. The material for it is in [Which facts move into the spec string](#which-facts-move-into-the-spec-string) and [Maintenance cost](#maintenance-cost).

## Basic example, three versions

**A** is the shipped example, `examples/basic/main.go`. **B** is the same program as input of the first prototype, `testdata/genprobe/basic/main.go` (removed). **C** is the input of the extended prototype, `testdata/genprobe/basicfull/main.go` (removed). All three print the same output. Level types and method bodies are identical except for the dispatch calls listed below.

### Operation interfaces and contracts

A and B, identical:

```go
// Each operation has one canonical single-method interface. It is the key
// used to register an implementation and to resolve one.
type (
	Processor interface {
		Process(msg string) (string, error)
	}
	Describer   interface{ Describe() string }
	Validator   interface{ Validate(msg string) error }
	Transformer interface{ Transform(msg string) string }
)

// Contracts group the operations. Public operations are offered to
// consumers; private ones exist only for the levels of the hierarchy.
type (
	Public interface {
		Processor
		Describer
	}
	Private interface {
		Validator
		Transformer
	}
)
```

C:

```go
// Contracts list the operations with their signatures. Public operations
// are offered to consumers; private ones exist only for the levels of the
// hierarchy. The generator derives one single-method interface per operation
// from them, named after the method: ProcessOp, DescribeOp, ValidateOp and
// TransformOp. Those interfaces are the keys used to register an
// implementation and to resolve one.
type (
	Public interface {
		Process(msg string) (string, error)
		Describe() string
	}
	Private interface {
		Validate(msg string) error
		Transform(msg string) string
	}
)
```

### Dispatch calls in method bodies

Every call whose spelling changes, in order of appearance. A and B:

```go
if err := b.Node.Virtual[Validator]().Validate(msg); err != nil {
return b.Node.Virtual[Transformer]().Transform(msg), nil
return l.Node.Parent[Validator]().Validate(msg)
out, err := t.Node.Parent[Processor]().Process(msg)
return "tagged > " + t.Node.Parent[Describer]().Describe()
return strings.ToUpper(t.Node.Parent[Transformer]().Transform(msg))
_, exposed := p.(Validator)
fmt.Fprintln(out, "virtual call from the same base:", t.limited.base.Node.Virtual[Describer]().Describe())
```

C:

```go
if err := b.Node.Virtual[ValidateOp]().Validate(msg); err != nil {
return b.Node.Virtual[TransformOp]().Transform(msg), nil
return l.Node.Parent[ValidateOp]().Validate(msg)
out, err := t.Node.Parent[ProcessOp]().Process(msg)
return "tagged > " + t.Node.Parent[DescribeOp]().Describe()
return strings.ToUpper(t.Node.Parent[TransformOp]().Transform(msg))
_, exposed := p.(ValidateOp)
fmt.Fprintln(out, "virtual call from the same base:", t.limited.base.Node.Virtual[DescribeOp]().Describe())
```

The call form is the existing holder method in all three. Its cost is unchanged: the holder, `Virtual` or `Parent`, the operation interface, and the method name again.

### Declaration and constructor

A:

```go
// hierarchy is declared once, without any object. The root names its
// contracts; each Extend adds the next level, which keeps those contracts and
// finds its parent among its embedded fields. BindNode states that the level's
// own node implements one operation, and the compiler checks that it does.
var hierarchy = heiry.Define[tagged]().
	Level[base, Public, Private]("").
	BindNode[Processor]((*base)(nil)).
	BindNode[Describer]((*base)(nil)).
	BindNode[Validator]((*base)(nil)).
	BindNode[Transformer]((*base)(nil)).
	Extend[limited]().
	BindNode[Validator]((*limited)(nil)).
	Extend[tagged]().
	BindNode[Processor]((*tagged)(nil)).
	BindNode[Describer]((*tagged)(nil)).
	BindNode[Transformer]((*tagged)(nil))

// newTagged is an ordinary constructor: it allocates one object and sets its
// fields. Init then checks the object against the hierarchy and gives every
// level its dispatch scope. The first Init also validates the declaration.
func newTagged(tag string, maxLen int) (*tagged, error) {
	t := &tagged{tag: tag}
	t.maxLen = maxLen
	err := hierarchy.Init(t,
		hierarchy.NodeProvider(&t.limited.base),
		hierarchy.NodeProvider(&t.limited),
		hierarchy.NodeProvider(t))
	if err != nil {
		return nil, err
	}
	return t, nil
}
```

B:

```go
// hierarchySpec is the generator's input: one hierarchy, its levels in order
// from the root, the contracts, and the operations each level's own node
// registers. The generator turns it into the declaration named hierarchy,
// with one compiler-checked Bind per operation, and into the helper
// hierarchyInit.
//
//heiry:spec
const hierarchySpec = `
hierarchy hierarchy tagged
level base public=Public private=Private
	node Processor Describer Validator Transformer
level limited
	node Validator
level tagged
	node Processor Describer Transformer
`

// newTagged is an ordinary constructor: it allocates one object and sets its
// fields. The generated helper then passes the object and a reference to
// every level to Init, which checks the object against the hierarchy and
// gives every level its dispatch scope. The first Init also validates the
// declaration.
func newTagged(tag string, maxLen int) (*tagged, error) {
	t := &tagged{tag: tag}
	t.maxLen = maxLen
	if err := hierarchyInit(t); err != nil {
		return nil, err
	}
	return t, nil
}
```

C:

```go
// hierarchySpec is the generator's input. The operations line asks for the
// operation interfaces of the two contracts. The hierarchy lists its levels
// from the root, and for each level the operations its own node registers.
// The wrapper line asks for the public wrapper type pipeline, which holds
// the object in the named field state and forwards every public operation.
//
//heiry:spec
const hierarchySpec = `
operations Public Private suffix=Op
hierarchy hierarchy tagged
level base public=Public private=Private
	node ProcessOp DescribeOp ValidateOp TransformOp
level limited
	node ValidateOp
level tagged
	node ProcessOp DescribeOp TransformOp
wrapper pipeline
`

// newTagged is an ordinary constructor: it allocates one object and sets its
// fields. The generated helper then passes the object and a reference to
// every level to Init, which checks the object against the hierarchy and
// gives every level its dispatch scope. The first Init also validates the
// declaration.
func newTagged(tag string, maxLen int) (*tagged, error) {
	t := &tagged{tag: tag}
	t.maxLen = maxLen
	if err := hierarchyInit(t); err != nil {
		return nil, err
	}
	return t, nil
}
```

### Wrapper

A and B, identical:

```go
// Pipeline is what consumers receive: the public operations and nothing else.
type Pipeline interface {
	Public
	Processed() int
}

// pipeline is the public wrapper. It keeps the object in a named field, so
// no inherited or private method is promoted onto it, and it forwards each
// public operation through a virtual call.
type pipeline struct{ state *tagged }

func (p *pipeline) Process(msg string) (string, error) {
	return p.state.Node.Virtual[Processor]().Process(msg)
}

func (p *pipeline) Describe() string { return p.state.Node.Virtual[Describer]().Describe() }

// Processed reads the counter that base.Process increments on the embedded
// base value of this same object.
func (p *pipeline) Processed() int { return p.state.processed }

func NewPipeline(tag string, maxLen int) (Pipeline, error) {
	t, err := newTagged(tag, maxLen)
	if err != nil {
		return nil, err
	}
	return &pipeline{state: t}, nil
}
```

C:

```go
// Pipeline is what consumers receive: the public operations and nothing else.
type Pipeline interface {
	Public
	Processed() int
}

// Processed is added by hand to the generated wrapper. It reads the counter
// that base.Process increments on the embedded base value of this same
// object.
func (p *pipeline) Processed() int { return p.state.processed }

func NewPipeline(tag string, maxLen int) (Pipeline, error) {
	t, err := newTagged(tag, maxLen)
	if err != nil {
		return nil, err
	}
	return &pipeline{state: t}, nil
}
```

In C the type `pipeline`, its field `state`, and the two forwarders are generated. `Processed` is a handwritten method on the generated type, and the constructor builds the generated struct by its field name.

## Components example, three versions

**A** is `examples/components/main.go`, **B** is `testdata/genprobe/components/main.go`, **C** is `testdata/genprobe/componentsfull/main.go`, both removed. Component and level types and their method bodies are identical except for the dispatch calls.

### Operation interfaces and contracts

A and B, identical:

```go
type (
	Getter interface {
		Get(key string) (string, bool)
	}
	Putter   interface{ Put(key, value string) }
	Recorder interface{ Record(event string) }
)

type (
	Public interface {
		Getter
		Putter
	}
	Private interface{ Recorder }
)
```

C:

```go
// The contracts list the operations with their signatures. The generator
// derives one single-method interface per operation from them: GetOp, PutOp
// and RecordOp.
type (
	Public interface {
		Get(key string) (string, bool)
		Put(key, value string)
	}
	Private interface{ Record(event string) }
)
```

### Dispatch calls in method bodies

A and B:

```go
s.Node.Virtual[Recorder]().Record("put " + key)
value, ok := c.Node.Parent[Getter]().Get(key)
c.Node.Virtual[Recorder]().Record("cache miss " + key)
c.Node.Parent[Putter]().Put(key, value)
s.Node.Parent[Recorder]().Record(fmt.Sprintf("#%d %s", s.count, event))
r.cache.Node.Virtual[Recorder]() == Recorder(&r.stamper))
r.cache.Node.Parent[Recorder]() == Recorder(&r.repository.log))
```

C:

```go
s.Node.Virtual[RecordOp]().Record("put " + key)
value, ok := c.Node.Parent[GetOp]().Get(key)
c.Node.Virtual[RecordOp]().Record("cache miss " + key)
c.Node.Parent[PutOp]().Put(key, value)
s.Node.Parent[RecordOp]().Record(fmt.Sprintf("#%d %s", s.count, event))
r.cache.Node.Virtual[RecordOp]() == RecordOp(&r.stamper))
r.cache.Node.Parent[RecordOp]() == RecordOp(&r.repository.log))
```

### Declaration and references

A:

```go
// Each registration names the field that holds its provider. BindGroup
// registers several operations for one component; whether the component
// implements each of them is checked when the declaration is validated. Bind
// registers one operation and lets the compiler check that the provider
// implements it. Extend adds the derived level with the same contracts.
var hierarchy = heiry.Define[cachedRepository]().
	Level[repository, Public, Private]("").
	BindGroup((*store)(nil), "store", heiry.Of[Getter](), heiry.Of[Putter]()).
	Bind[Recorder]((*auditLog)(nil), "log").
	Extend[cachedRepository]().
	BindGroup((*cache)(nil), "cache", heiry.Of[Getter](), heiry.Of[Putter]()).
	Bind[Recorder]((*stamper)(nil), "stamper")

// references lists one pointer per level node and one per registered
// component. Init compares each pointer with the address it expects inside r.
func references(r *cachedRepository) []heiry.ProviderRef[cachedRepository] {
	return []heiry.ProviderRef[cachedRepository]{
		hierarchy.NodeProvider(&r.repository),
		hierarchy.ComponentProvider[repository]("store", &r.repository.store),
		hierarchy.ComponentProvider[repository]("log", &r.repository.log),
		hierarchy.NodeProvider(r),
		hierarchy.ComponentProvider[cachedRepository]("cache", &r.cache),
		hierarchy.ComponentProvider[cachedRepository]("stamper", &r.stamper),
	}
}
```

B:

```go
// hierarchySpec is the generator's input. Each component line names the
// field that holds the provider, the provider's type, and the operations it
// registers. The generator emits one compiler-checked Bind per operation and
// the references for every level node and component.
//
//heiry:spec
const hierarchySpec = `
hierarchy hierarchy cachedRepository
level repository public=Public private=Private
	component store store Getter Putter
	component log auditLog Recorder
level cachedRepository
	component cache cache Getter Putter
	component stamper stamper Recorder
`
```

C:

```go
// hierarchySpec is the generator's input. Each component line names the
// field that holds the provider, the provider's type, and the operations it
// registers. The wrapper line asks for the public wrapper type wrapper.
//
//heiry:spec
const hierarchySpec = `
operations Public Private suffix=Op
hierarchy hierarchy cachedRepository
level repository public=Public private=Private
	component store store GetOp PutOp
	component log auditLog RecordOp
level cachedRepository
	component cache cache GetOp PutOp
	component stamper stamper RecordOp
wrapper wrapper
`
```

### Wrapper and constructor

A:

```go
// Repository is the consumer-facing view.
type Repository interface {
	Public
	Events() []string
}

type wrapper struct{ state *cachedRepository }

func (w *wrapper) Get(key string) (string, bool) { return w.state.Node.Virtual[Getter]().Get(key) }

func (w *wrapper) Put(key, value string) { w.state.Node.Virtual[Putter]().Put(key, value) }

func (w *wrapper) Events() []string { return w.state.log.events }

// NewRepository allocates one object, binds it and returns the public view.
func NewRepository() (Repository, error) {
	r := newObject()
	if err := hierarchy.Init(r, references(r)...); err != nil {
		return nil, err
	}
	return &wrapper{state: r}, nil
}
```

B:

```go
// Repository is the consumer-facing view.
type Repository interface {
	Public
	Events() []string
}

type wrapper struct{ state *cachedRepository }

func (w *wrapper) Get(key string) (string, bool) { return w.state.Node.Virtual[Getter]().Get(key) }

func (w *wrapper) Put(key, value string) { w.state.Node.Virtual[Putter]().Put(key, value) }

func (w *wrapper) Events() []string { return w.state.log.events }

// NewRepository allocates one object, binds it and returns the public view.
func NewRepository() (Repository, error) {
	r := newObject()
	if err := hierarchyInit(r); err != nil {
		return nil, err
	}
	return &wrapper{state: r}, nil
}
```

C:

```go
// Repository is the consumer-facing view.
type Repository interface {
	Public
	Events() []string
}

// Events is added by hand to the generated wrapper.
func (w *wrapper) Events() []string { return w.state.log.events }

// NewRepository allocates one object, binds it and returns the public view.
func NewRepository() (Repository, error) {
	r := newObject()
	if err := hierarchyInit(r); err != nil {
		return nil, err
	}
	return &wrapper{state: r}, nil
}
```

The demonstration code of the example, which replaces one reference with a pointer into another object, uses `references(r)` in A and the generated `hierarchyRefs(r)` in B and C.

## Measured effect

The removed measurement tool, run as `go run ./testdata/genprobe/measure`, counted lines that hold at least one token and the non-blank bytes of those tokens. Comments and blank lines are not counted; each non-blank line of a spec constant counts as a line. "Boilerplate" is exactly the declarations and calls the tool names: handwritten operation interfaces, the declaration or spec constant, reference lists and initialization calls, and the wrapper type with its forwarders. Contracts, level types, method bodies, constructors, handwritten wrapper methods, and the examples' demonstration code are "everything else". The tool prints every line it counted as boilerplate; that list is in the evidence.

| Handwritten code | Basic A | Basic B | Basic C | Components A | Components B | Components C |
| --- | --- | --- | --- | --- | --- | --- |
| Operation interfaces | 8 | 8 | 0 | 7 | 7 | 0 |
| Declaration or spec | 12 | 9 | 11 | 7 | 9 | 11 |
| Reference lists and initialization calls | 4 | 1 | 1 | 13 | 2 | 2 |
| Wrapper type and forwarders | 5 | 5 | 0 | 3 | 3 | 0 |
| **Boilerplate, lines** | **29** | **23** | **12** | **30** | **21** | **13** |
| Boilerplate, bytes | 913 | 630 | 277 | 1,249 | 643 | 348 |
| Everything else, lines | 119 | 118 | 118 | 124 | 125 | 125 |
| **Whole program, lines** | **148** | **141** | **130** | **154** | **146** | **138** |
| Whole program, bytes | 3,304 | 3,010 | 2,722 | 3,807 | 3,234 | 2,983 |

Generated code, which nobody writes and which is not included above:

| Generated file, code lines | Basic | Components |
| --- | --- | --- |
| First prototype | 48 | 47 |
| Extended prototype | 96 | 86 |

Against the shipped examples the extended prototype removes 17 of 29 boilerplate lines (58.6%) and 18 of 148 program lines (12.2%) for basic, and 17 of 30 (56.7%) and 16 of 154 (10.4%) for components. In bytes, boilerplate falls by 69.7% and 72.1%. The generated file is about eight times the lines of the spec that produces it.

These are two small programs. The saving grows with the number of operations, registrations, and slots, and does not grow with the amount of code in method bodies. No larger hierarchy was measured.

### Which repeated facts disappear

- **An operation's signature.** Today it is written in the operation interface, again in each implementing method, and again in the forwarder of a public operation. In C it is written in the contract and in each implementing method.
- **The operation as a declared type.** One single-method interface per operation, and its name embedded in a contract, become a method written in the contract.
- **The typed-nil witness.** `(*base)(nil)` on every registration, eight times in basic, is derived from the level or component line.
- **The object's layout at initialization.** Each level's path as a Go selector, and for a component its owner type, path string, and selector, are derived from the level and component lines.
- **The wrapper.** Its struct, one forwarder per public operation, and the operation each forwarder resolves.

### Which facts move into the spec string

The order and types of the levels; the contracts of each level; which operations each node or component registers; component paths and provider types; a parent field that is not named after its type; the wrapper's name; and the naming rule of the operation interfaces.

A spec is a string. The compiler compares it with the text it was generated from and otherwise does not look inside it. An editor does not highlight or complete it, and a rename does not reach into it; a renamed type makes the generated file stop compiling until the spec is edited and regenerated.

### What is still written more than once

- An operation's name: in the spec for each registration, and inside the brackets of each dispatch call.
- The hierarchy's shape: each level embeds its parent in its struct, and the spec lists the levels. The compiler and the library check that they agree.
- A component: its field in the struct, and its path in the spec.

### What the author still has to understand

Generation removes typing, not concepts. The author still needs value embedding of levels and one directly embedded `heiry.Node` per provider; the public and private contracts and their extension rules; that each operation is identified by one interface type, now with a generated name; that a registration is an explicit ownership assertion per level and provider, and that an unregistered method is inherited however it is declared; component paths and which level owns a component; virtual and parent calls through the provider's own holder; that the wrapper holds the object in a named field; Definition, Plan, and per-object initialization, lazy, eager, and precomputed; and the copying limitation.

New with generation: the spec grammar; the generated names, which appear in handwritten code but are declared in a file the author does not write; the generate and check workflow; and how to read the compile errors that say the output is stale or a guard was tripped.

## The spec

One marked string constant holds operations lines, one hierarchy, or both.

```text
import <name> "<path>"
operations <Contract>... [prefix=<p>] [suffix=<s>]
existing <Method> <Type>
hierarchy <Name> <LeafType>
level <NodeType> [parent=<Field>] [public=<Type> private=<Type>]
node <Operation>...
component <path> <ProviderType> <Operation>...
wrapper <Type> [field=<name>] [holder=<Field>]
```

| Line | Meaning |
| --- | --- |
| `operations` | Generate one single-method interface per method of the listed handwritten contracts, named prefix + method + suffix. A method that several listed contracts have gets one interface. |
| `existing` | Belongs to the `operations` line above it. The named method is identified by an interface that already exists, handwritten or imported; none is generated for it. |
| `hierarchy`, `level`, `node`, `component` | As in the first pass: the definition's name and leaf; the levels in order with their contracts and parent field; the operations each node or component registers, by operation interface. Nothing else is registered. |
| `wrapper` | Generate the public wrapper type for the hierarchy: a struct with one named field holding the leaf, and a forwarder for every method of the leaf's public contract. Both of the leaf's contracts, public and private, must be listed by an `operations` line, because the wrapper depends on both and only listed contracts are guarded. |
| `import` | A package that type expressions in the spec use. |

The registrations, paths, parents, contract names, and level order come from the spec text alone. The method signatures of operation interfaces and forwarders are read from the handwritten contract declarations, by syntax only. The generator still never type-checks.

## What is generated

For the basic example, `testdata/genprobe/basicfull/heirygen_main.go` (removed), complete:

```go
// Code generated by heirygen (investigation prototype) from main.go; DO NOT EDIT.

package main

import (
	"github.com/lucaf/heiry"
)

// The declarations below were generated from the constant hierarchySpec.
// If the map literal fails to compile with a duplicate key, hierarchySpec has
// changed since then: run go generate. If the line above it fails, hierarchySpec
// is no longer a constant.
func _() {
	const generatedFrom = "" +
		"\n" +
		"operations Public Private suffix=Op\n" +
		"hierarchy hierarchy tagged\n" +
		"level base public=Public private=Private\n" +
		"\tnode ProcessOp DescribeOp ValidateOp TransformOp\n" +
		"level limited\n" +
		"\tnode ValidateOp\n" +
		"level tagged\n" +
		"\tnode ProcessOp DescribeOp TransformOp\n" +
		"wrapper pipeline\n"
	const unchanged = hierarchySpec == generatedFrom
	_ = map[interface{}]struct{}{1 == 0: {}, unchanged: {}}
}

// Operation interfaces of Public, Private: one per method, with the
// signature written in the contract.
type (
	ProcessOp interface {
		Process(msg string) (string, error)
	}
	DescribeOp  interface{ Describe() string }
	ValidateOp  interface{ Validate(msg string) error }
	TransformOp interface{ Transform(msg string) string }
)

// Each contract must have exactly the methods of its operation interfaces.
// If an assignment below fails to compile, a contract no longer has the
// methods this file was generated for: run go generate. If it still fails
// afterwards, two listed contracts give one method name different
// signatures, or an existing interface does not have the contract's.
func _() {
	var _ interface {
		ProcessOp
		DescribeOp
	} = Public(nil)
	var _ Public = interface {
		ProcessOp
		DescribeOp
	}(nil)
	var _ interface {
		ValidateOp
		TransformOp
	} = Private(nil)
	var _ Private = interface {
		ValidateOp
		TransformOp
	}(nil)
}

// hierarchy is the receiver-free declaration of the hierarchy whose leaf is tagged.
var hierarchy = heiry.Define[tagged]().
	Level[base, Public, Private]("").
	Bind[ProcessOp]((*base)(nil), ".").
	Bind[DescribeOp]((*base)(nil), ".").
	Bind[ValidateOp]((*base)(nil), ".").
	Bind[TransformOp]((*base)(nil), ".").
	End().
	Level[limited, Public, Private]("base").
	Bind[ValidateOp]((*limited)(nil), ".").
	End().
	Level[tagged, Public, Private]("limited").
	Bind[ProcessOp]((*tagged)(nil), ".").
	Bind[DescribeOp]((*tagged)(nil), ".").
	Bind[TransformOp]((*tagged)(nil), ".").
	End()

// hierarchyRefs returns one reference for every level node and registered
// component of the object, created by hierarchy.
//
// References cannot point into a nil object. For one, they point into a
// scratch value instead: Init checks references before the object, and
// well-formed references let it reach its own diagnostic for a nil object.
func hierarchyRefs(leaf *tagged) []heiry.ProviderRef[tagged] {
	if leaf == nil {
		leaf = new(tagged)
	}
	return []heiry.ProviderRef[tagged]{
		hierarchy.NodeProvider[base](&leaf.limited.base),
		hierarchy.NodeProvider[limited](&leaf.limited),
		hierarchy.NodeProvider[tagged](leaf),
	}
}

// hierarchyInit binds one object through hierarchy, preparing it on first use.
func hierarchyInit(leaf *tagged) error {
	return hierarchy.Init(leaf, hierarchyRefs(leaf)...)
}

// hierarchyInitPlan binds one object through a Plan prepared from hierarchy or
// converted from such a Plan, dynamic or precomputed. A Plan of another
// definition is rejected by Init, because the references carry the lineage
// of hierarchy.
func hierarchyInitPlan(plan *heiry.Plan[tagged], leaf *tagged) error {
	return plan.Init(leaf, hierarchyRefs(leaf)...)
}

// pipeline is the public wrapper of hierarchy. It holds the object in a named field,
// so nothing is promoted onto it, and forwards every operation of Public
// through a virtual call.
type pipeline struct{ state *tagged }

// *pipeline has every method of Public. This shows satisfaction only; it says
// nothing about other methods of the wrapper.
var _ Public = (*pipeline)(nil)

func (w *pipeline) Process(msg string) (string, error) {
	return w.state.Node.Virtual[ProcessOp]().Process(msg)
}

func (w *pipeline) Describe() string {
	return w.state.Node.Virtual[DescribeOp]().Describe()
}

// No method of pipeline may be named after an operation of Private.
// If a selector below is ambiguous, a handwritten method of the wrapper
// has the name of a private operation: remove or rename that method.
func _() {
	type private interface {
		Validate()
		Transform()
	}
	type probe struct {
		*pipeline
		private
	}
	_ = probe.Validate
	_ = probe.Transform
}
```

The other generated files were `testdata/genprobe/componentsfull/heirygen_main.go` and, for the first prototype, `heirygen_main.go` in `testdata/genprobe/basic` and `testdata/genprobe/components`, all removed.

| Generated | For |
| --- | --- |
| `func _()` with a constant comparison and a map literal | Each spec constant: it is still a constant, and it still has the text the file was generated from. |
| Operation interfaces | Each `operations` line, except methods with an `existing` line. |
| `func _()` with two assignments per contract | Each `operations` line: every listed contract has exactly the methods of its operation interfaces. |
| `H`, `HRefs`, `HInit`, `HInitPlan` | Each hierarchy `H`: the definition, the reference list, and the two initialization helpers. |
| The wrapper type, `var _ Public = (*W)(nil)`, forwarders | Each `wrapper` line. A forwarder keeps the contract's parameter names where they are safe and drops result names, which could collide with its receiver or capture a name its body uses. |
| `func _()` with ambiguous-selector probes | Each `wrapper` line whose private contract has methods: no method of the wrapper is named after a private operation. |

Everything goes through the existing public API: `Define`, `Level`, `Bind`, `End`, `NodeProvider`, `ComponentProvider`, `Init`, and `Node.Virtual`. Nothing of the library's internals is copied, and there is no second resolver.

## Safety mechanisms

The spec-constant assertion comes from the first pass. Three further compile-time mechanisms carry the extension; each was demonstrated on handwritten stand-ins, with no generator involved, before generated code used it.

### Spec constant freshness

```go
func _() {
	const generatedFrom = "" + /* the spec text, line by line */
	const unchanged = hierarchySpec == generatedFrom
	_ = map[interface{}]struct{}{1 == 0: {}, unchanged: {}}
}
```

The comparison is bound to a constant, so the compiler must evaluate it. When the texts differ, `unchanged` is false, the map literal has the key false twice, and the build fails with `duplicate key false in map literal`.

The earlier form put the comparison directly into the map key. The owner's review found that it could be disarmed: if the handwritten declaration changed from `const` to `var`, the key was no longer a constant, the compiler stopped checking it for duplicates, and a stale spec built. `TestFreshnessAssertionCannotBeDisarmed` reproduces that and shows the present form failing with `is not constant` as soon as the declaration is a variable, edited or not. The re-audit found a second, far-fetched way: a package that redeclares `false` inverted the old assertion. The present form names neither `false` nor `bool`, and the same test covers it.

### Contract and operation equality

```go
var _ interface { ReadOp; AuditOp } = Public(nil)
var _ Public = interface { ReadOp; AuditOp }(nil)
```

An interface value is assignable to an interface type only if it has every method of that type with an identical signature. One assignment in each direction is therefore accepted only when the contract and the union of its operation interfaces have the same methods with the same signatures. `TestContractEqualityAssertion` shows each direction rejecting a change:

| Change to the contract | Compiler output |
| --- | --- |
| A public method moved into the private contract | `missing method Audit` |
| A public method removed | `missing method Audit` |
| A private method added | `does not implement Private (missing method Wipe)` |
| A signature changed | `wrong type for method Seal` |
| A method added to an embedded contract | `does not implement Public (missing method Scan)` |
| An embedded contract replaced | `missing method Read` |
| A parameter renamed | Compiles: the method set is unchanged |

The compiler evaluates these assignments against the contracts as they are, with full type information. The generator reads contracts by syntax; this is what checks what it read.

The generator itself does not decide whether two spellings of a method are the same signature. Parameter names, result names, and type names can all differ between two spellings of one signature: `Read(x string)` and `Read(y text)`, with `text` an alias of `string`, are identical. The generator refuses only what cannot be equal, a different number of parameters or results, and leaves the rest to these assignments. An earlier version compared the printed text and so rejected valid contracts; the review found it, and `TestSignaturesThatDifferOnlyInSpelling` covers it.

### Absence of private operation names on the wrapper

```go
func _() {
	type private interface { Seal() }
	type probe struct {
		*view
		private
	}
	_ = probe.Seal
}
```

If the wrapper `view` declares a method `Seal`, the selector `probe.Seal` names two methods at the same depth and is ambiguous, which is a compile error. `TestPrivateNameAbsenceAssertion` shows it rejecting a method of that name with a pointer or value receiver and with any signature, and accepting an additional method with another name.

The names come from the leaf's private contract, which must be listed by an `operations` line. That requirement is what keeps the guard current: a private operation added later breaks the contract's equality assertion. The first extended prototype required only the public contract to be listed. The review found the consequence, which was then observed: with the private contract unlisted, a new private operation together with a wrapper method of its name built without error. `TestWrapperNeedsBothContractsListed` covers it.

Its limits, each observed:

- It sees names, not behavior. A wrapper method with another name that calls a private operation's implementation compiles.
- It compares methods one embedding deep. A wrapper type that itself embeds a type with the private method promotes it from deeper and is not rejected. A generated wrapper struct has only its one named field, so it promotes nothing; adding an embedded field would be a hand edit of the generated file.
- It is not a check that the wrapper has no additional methods. Additional methods are intended: `Processed` and `Events` in the examples.

### What the satisfaction assertion is not

`var _ Public = (*W)(nil)` proves that the wrapper has every public method. It proves nothing about other methods: `TestSatisfactionAssertionAcceptsExtraMethods` compiles a wrapper that carries it and exposes a private operation. The generated file keeps the assertion for what it does prove and does not rely on it for visibility.

There is no exact method-set check of the wrapper in this design, and the earlier proposal's phrase for one was inaccurate. What the compiler enforces is: the public contract equals the forwarded operations, the wrapper has every public method, and the wrapper declares no method named after a private operation.

### Stale output of the extended generator

`TestStaleOperationsAndWrapper` changes handwritten source other than the spec constant and does not regenerate. Before the two guards existed, five of these six cases built successfully; that was the failing observation that preceded them.

| Change | Without regeneration | After regeneration |
| --- | --- | --- |
| Move a public operation into the private contract | Compile error, `missing method Audit` | The wrapper no longer has `Audit`; checked individually, no private operation is exposed |
| Remove a public operation | Compile error, `missing method Audit` | Compile error: the spec still registers `AuditOp`, which no longer exists |
| Add a private operation | Compile error, `missing method Wipe` | `MissingMethods` at first initialization, as for a handwritten declaration |
| Change an operation's signature in the contract | Compile error, `wrong type for method Seal` | Compile error: the provider's method does not implement the regenerated interface |
| Change an embedded contract | Compile error, `missing method Scan` | `MissingMethods` at first initialization |
| Add a handwritten wrapper method named after a private operation | Compile error, `ambiguous selector` | The same error: the method has to go |

Each failure is reported in the generated file, by `go build`, `go vet`, and `go test`. Visibility is checked per operation throughout: the probe program and the workload tests assert each private operation separately, because a wrapper can fail to implement the whole private contract and still expose one of its methods.

### Guarantees

| Guarantee | Where the check runs with generated code | Evidence |
| --- | --- | --- |
| Provider implements each registered operation | Compile time: one `Bind[T]` with a typed-nil witness per operation. The shipped components example checks its grouped registrations at validation. | Generated output; `TestWrongSpecIsRejectedByTheCompiler`; `TestCompilerCheckedPropertiesOfGeneratedCode` |
| Canonical operation identity | One interface per method per `operations` line, equal to the contract by the equality assertion. A contract listed by two lines is refused. | `TestEqualMethodNamesInTwoHierarchies`, `TestOperationsSharedByDefinitionsOverOneLeaf` |
| Typed invocation | Unchanged: bodies and forwarders call `Node.Virtual[T]` and `Node.Parent[T]` and then an ordinary method. | All four workloads print the shipped output |
| Explicit registration ownership | The spec lists every registration. The generator does not type-check and reads no method set of a provider. | `TestBootstrap`; generator source |
| Contracts, extension rules | Unchanged: definition validation at first preparation. | `MissingMethods` rows above |
| Consumer visibility | Compile time for names; per-operation tests for the dynamic method set. | `TestStaleOperationsAndWrapper`; `TestWrapperExposesNoPrivateOperation` in both extended workloads |
| Reference lineage, slot claims | Unchanged `Init`. References are created by the definition with explicit owner and path. | `TestForeignPlanIsRejected`, `TestTwoDefinitionsOverOneLeaf` |
| Exact provider type | Compile time through explicit type arguments on each factory call, and again at `Init`. | `TestStructChangesBreakTheGeneratedHelpers` |
| Physical ownership | Unchanged: every `Init` compares addresses. | `TestOwnershipAndNilObject`: each of six references replaced in turn |
| Atomic initialization, repeated-initialization rejection, structured errors | Unchanged `Init`. | `TestHelpersMatchTheHandwrittenIdiom` |
| Receiver-free reusable Plans, separate bindings, both modes | The declaration holds types, strings, and typed nils. | `TestGeneratedOperationsInBothModes` in both extended workloads |
| Parent-call semantics | Unchanged resolution. | The same tests: parent of a component's operation is below the owning level; a parent call skips a level that registers nothing |
| Validation timing | Unchanged: invalid definitions are reported at first preparation, before any object is bound. | `TestInvalidDefinitionTakesPrecedence` |
| No `unsafe`, no reflective invocation | The generated file imports the library and the packages that signatures and the spec name. | Generated output |

## Freshness across all inputs

"Compilation" means `go build`, `go vet`, or `go test` failing to compile. "Check mode" means `heirygen -check`, a tool run that must be configured somewhere. "Tests" means a test the application has and runs. A generator check, a test, and a CI step are each a different enforcement boundary from compilation; none is described here as a compiler guarantee.

### Input changed, output not regenerated

| Change | Compilation | Check mode | Tests | Nothing |
| --- | --- | --- | --- | --- |
| Spec constant edited: registration, path, parent, contract name, level, `operations`, `wrapper` | Yes | Yes | | |
| Spec declaration changed from a constant to a variable | Yes | Yes: the generator refuses it | | |
| Method set changed in a contract an `operations` line lists, which includes both contracts of every generated wrapper: moved, removed, added, signature, embedded contract, alias retargeted | Yes | Yes | | |
| Method set changed in a contract no `operations` line lists, in a hierarchy without a generated wrapper | No: nothing generated depends on it | No | Definition validation at first preparation, as for handwritten code | |
| Contract text changed, method set unchanged: parameter renamed, methods reordered | No, and nothing is wrong | Yes | | |
| Struct changed: component field renamed or retyped, parent embedded by pointer or removed, parent alias retargeted | Yes | No: the output is still what the spec asks for | | |
| Source file's build constraint changed | Yes, in every build that selects only one of the two files; the others are correct | Yes | | |
| New handwritten package-level name equal to a generated one | Yes | Yes | | |
| New handwritten name equal to a generated parameter or local | No, and nothing is wrong | Yes | | |
| Handwritten wrapper method named after a private operation | Yes | No | | |
| Handwritten wrapper method with another name that calls a private operation | No | No | Only a test written for that method | Otherwise yes |
| Handwritten interface that duplicates a generated operation interface | No | No | Yes, if a test reaches the call: `UnknownCapability`. At validation if both are registered: `CapabilityIdentity` | |
| A spec that is wrong but consistent: an omitted override, a provider that only promotes the method | No | No | Only a behavior test | Otherwise yes |

The last three rows are not staleness. They exist identically for handwritten declarations and wrappers; specification S4 is unchanged.

### Generated code manually edited

| Edit | Compilation | Check mode | Tests | Nothing |
| --- | --- | --- | --- | --- |
| Not well typed | Yes | Yes | | |
| Well typed: a registration removed while an ancestor supplies the operation, a guard removed, an embedded field added to the wrapper | No | Yes | Only a test that observes the effect | Without check mode and such a test, yes |

`TestManuallyEditedGeneratedFile` removes the equality guard by hand and then moves an operation into the private contract. The program compiles and the wrapper exposes the operation. Check mode reports the edit, and a test that checks each private operation reports the exposure.

### The generator emitted incorrect code

Simulated by making the generated file wrong in the way a defect would.

| Defect | Compilation | Check mode | Tests | Nothing |
| --- | --- | --- | --- | --- |
| Operation interface with a miscopied signature | Yes, equality assertion | No | | |
| Registration for a provider without the method | Yes, `Bind` witness | No | | |
| Forwarder that resolves another operation | Yes: that interface lacks the forwarded method | No | | |
| Reference with the wrong provider type | Yes, explicit type arguments | No | | |
| Wrapper without a public forwarder | Yes, satisfaction assertion | No | | |
| Wrong owner or path string in a reference, wrong parent field or contract in a level | No | No | Yes: typed error at first preparation or initialization | |
| A registration dropped or added where an ancestor supplies the operation; a guard not emitted | No | No: the same generator computes the expected text | Only the generator's own tests, or a behavior test | Otherwise yes |

No universal detection of generator defects is claimed.

### The generator version changed

| Situation | Compilation | Check mode | Tests | Nothing |
| --- | --- | --- | --- | --- |
| Output of an earlier version stays in the tree | No, while the library accepts the calls it makes | Yes, if the new version writes different text for the same input | | If the text is the same, nothing needs detecting |

The prototype stamps no version in the header. A version stamp would make check mode demand regeneration after every upgrade; whether that is wanted belongs to a production design.

### The trust boundary

What the compiler verifies about a generated file, independently of the generator:

- Each registered provider implements its operation.
- The operation interfaces equal the contracts, when the equality assertion is present.
- The wrapper has every public method, and each forwarder calls the method of the operation it resolves.
- Each reference has the provider type the spec states.
- The spec declaration is a constant and has the text recorded in the file.

What the library verifies at run time, unchanged: structure, contracts, coverage, duplicates, slots, lineage, ownership, and single atomic initialization.

What rests on the generator being correct: that the registrations are exactly those the spec lists, and that the guards are present. The evidence for this is the generator's tests, the four workloads printing the shipped examples' output, and check mode on committed output. That is evidence about the cases tested. It is not a proof, and nothing in the workflow turns it into a compiler check.

## Bootstrap, naming, and reuse

The extended generator still parses and never type-checks. `TestBootstrapWithOperationsAndWrapper` adds a handwritten file whose declarations use generated types: a method attached to the generated wrapper, struct fields of the wrapper and of an operation interface, an interface embedding two generated operation interfaces, and a variable of the definition's type.

| Situation | Observed |
| --- | --- |
| No generated output | `undefined: view`; one run makes the package compile |
| Generated output that does not parse | One run repairs it |
| A method renamed in contract, body, spec, and handwritten uses; old output no longer compiles | One run repairs it |
| A contract that embeds an operation interface generated from it | Refused by the generator: the interface is not declared in handwritten files |

Operation identity and naming:

| Case | Behavior | Evidence |
| --- | --- | --- |
| Several hierarchies in one package | Each family of contracts takes its own `operations` line. | `TestEqualMethodNamesInTwoHierarchies` |
| Equal method names with different signatures | Different lines with different prefix or suffix give different interfaces; resolving one family's operation on the other's object panics with `UnknownCapability`. One line for both is refused when the number of parameters or results differs, and otherwise fails at the equality assertion; two lines that would produce the same name are refused. | The same test |
| Several definitions over one leaf type | They share one `operations` line, because they share bodies; each has its own registrations, helpers, and wrapper. A contract listed by two lines is refused. | `TestOperationsSharedByDefinitionsOverOneLeaf` |
| Existing handwritten or imported operation interfaces | An `existing` line keeps the interface as the operation's identity; the equality assertion checks its signature. A hierarchy can also keep all its handwritten interfaces and use only the first-pass generation. | `TestExistingOperationInterfaces`; matrix fixture |
| A handwritten duplicate of a generated interface | Not detectable by generated code. The library reports it as for two handwritten interfaces. | `TestHandwrittenDuplicateOfAGeneratedOperation` |
| Generated identifiers that collide with handwritten ones | Reported by the generator, with both positions, before writing: operation interfaces, wrapper types, definitions, helpers. Parameters, locals, receivers, and the import name are renamed. | `TestGeneratedTypeNameCollisions`, `TestGeneratedNameCollisionsAreReported`, `TestGeneratedNamesDoNotCaptureApplicationIdentifiers`, `TestForwarderSignatures` |

## Initialization semantics

The first pass's helper gave a nil object to `Init` with typed-nil references. That produced a library diagnostic, but not the one `Init` defines for a nil object. This pass compared the helpers with direct library calls field by field and changed two behaviors.

For every input the handwritten idiom can be called with, the helpers return the identical diagnostic, context included (`TestHelpersMatchTheHandwrittenIdiom`):

| Input | Helper and handwritten call |
| --- | --- |
| Valid object; lazy, dynamic Plan, precomputed Plan | No error |
| Initialized object, either entry, either mode | `AlreadyInitialized`, level 0, path `limited.base` |
| Object of which an inner level was initialized before | `AlreadyInitialized`, level 0, path `limited.base` |
| Nil Plan or unprepared Plan, with a valid or an initialized object | `InvalidPlan`, target only |
| Invalid definition, any object | The definition's errors; no scope installed |

The handwritten idiom cannot be called with a nil object: it panics while taking addresses. `Init` itself can, and what it returns depends on the references:

| Call | Diagnostic |
| --- | --- |
| `Init(nil, well-formed references)` | `InvalidProvider`, target only: no level, path, expected, or actual type |
| `Init(nil, typed-nil references)`, the first pass's helper | `InvalidProvider`, level 0, path `limited.base`, expected and actual `*base` |
| `Init(nil)` with no references | `BindingMismatch`, level 0, path `limited.base` |

The first row is `Init`'s own check of the object. The helpers now reproduce it exactly, for lazy, dynamic, and precomputed use, and after an unusable Plan or an invalid definition has been reported first. They do so by building well-formed references into a scratch value when the object is nil, because `Init` checks references before the object.

Two semantic choices are reported for later approval. Neither determines whether to proceed.

- **How a helper reports a nil object.** Prototyped: exactly `Init`'s nil-object diagnostic, through references into a scratch value. Alternatives: typed-nil references, which give a slot-level diagnostic; or no handling, which panics as handwritten code does. A consequence of the prototyped choice is that `HRefs(nil)` returns references that belong to no usable object; `Init` rejects them for any real object with `ProviderOwnership`.
- **Whether `HInitPlan` accepts a Plan of another definition.** The first pass created references with the Plan's own factories. `TestForeignPlanIsRejected` showed the consequence: a Plan of another definition of the same leaf type, with the same slots, initialized the object through the helper. The helper now creates references with its own definition, so `Init` rejects a foreign Plan with `BindingMismatch`. The cost, read from the source and not measured, is that each reference creation takes the definition's lock briefly, where Plan-created references take none.

## Supported configurations and explicit limitations

Exercised: everything the first pass listed, and operation interfaces from contracts with embedded contracts, aliases, and several listed contracts, including a private contract that grows at a later level together with a wrapper; existing interfaces from this or another package; forwarders for methods with variadic parameters, named results under any name, unnamed and blank parameters, signatures written over several lines, types from other packages, and no parameters or results; one method written in several contracts with different parameter names or type spellings; a holder embedded under another name; handwritten methods on generated wrappers; several wrappers and definitions over one leaf type.

Unsupported, each refused with a message unless noted. None narrows what the library accepts from handwritten code: an author can always write the affected part by hand.

1. A contract method obtained by embedding an interface of another package. The method must be written out in the contract; an `existing` line can keep the imported interface as the operation's identity.
2. Generic contracts, methods with type parameters, and type-set elements in contracts.
3. A contract declared more than once in the directory, for example once per platform. The generator does not evaluate build constraints.
4. A contract embedding an operation interface that is generated from it.
5. A contract listed by two `operations` lines.
6. A wrapper for a hierarchy whose public or private leaf contract no `operations` line lists.
7. More than one generated wrapper per hierarchy. Further wrappers can be handwritten, without the generated guards.
8. A qualified type in a signature whose import, in the contract's file, has no explicit name and a path that does not end in the package name. A type that reaches the contract's file through a dot import is not refused: the copied signature fails to compile in the generated file.
9. From the first pass: type expressions in a spec contain no spaces; a spec is one string literal; one generated name per package directory across build configurations; no grouped registration; a definition assembled conditionally stays handwritten.
10. Not refused, reported later by the compiler or the library as for handwritten code: a duplicate registration, an undeclared operation, a wrong parent field.

## Build configuration and dependencies

Unchanged from the first pass: the generated file repeats its source file's `//go:build` line and keeps the source file's name as the end of its own; output is deterministic; the prototype uses the standard library only (`go/parser`, `go/ast`, `go/token`, `go/printer`, `go/format`); no module dependency was added. New in this pass: imports needed by copied signatures are taken from the contract's file and written with explicit names.

## Maintenance cost

- The prototype generator is 1,554 lines and its tests 2,580 lines. A production tool would add documentation, message design, and version handling.
- The generator must follow the library's declaration, reference, and holder API, and Go's syntax for interfaces and signatures.
- Its tests build and break throwaway modules with the go command and take about a minute, against a few seconds for the library's suite.
- Consumers gain a workflow: generate after editing a spec or a contract, and run check mode wherever hand edits must be caught.

## Alternatives

| Alternative | Outcome |
| --- | --- |
| Stop after the first prototype | Safe and small: 20.7% and 30.0% of boilerplate lines. Leaves operation interfaces and wrappers, where the remaining repetition is. |
| Signatures in the spec, contracts handwritten against generated interfaces | Not prototyped. It would put the most-read part of the API into a string and repeat nothing less. |
| Type-checking generator for signatures | Not pursued. Handwritten methods on the generated wrapper are exactly the declarations a type checker cannot resolve before first generation; see the first pass's probe. Syntax plus the equality assertion gives the same result without the restriction. |
| Typed virtual and parent accessors | Excluded from this pass. A provider type can serve several slots, levels, and definitions with different operations, so one method set per type cannot describe it. |
| Comment directives, external spec files, a Go-typed spec behind a build tag | Unchanged from the first pass: silent staleness, no compile-time assertion, or new library API. |
| Rejected earlier by the owner and not reopened | Bare-pointer initialization, registration without a witness, variadic optional arguments, inferring overrides from method sets. |

## Owner review

On 2026-11-19 the owner reviewed the prototype by reading it, without running it, and reported three findings. Each was reproduced as a failing test before it was fixed.

| Finding | Reproduced as | Fix |
| --- | --- | --- |
| The wrapper's private contract did not have to be listed, so it had no equality assertion. A private operation added later did not fail compilation, and the wrapper's guard did not know its name. | The generator accepted a wrapper with only the public contract listed. A build with a new private operation and a wrapper method of its name succeeded. | A wrapper requires both of the leaf's contracts to be listed; the guard's names are taken from the listed contract. |
| The freshness assertion did not enforce that its input stays a constant. | With `const` changed to `var`, a stale spec built. | The comparison is bound to a generated constant first, which does not compile for a variable. |
| Valid signatures were rejected, or produced wrappers that did not compile. | Two contracts that write `Read(x string)` and `Read(y text)` were refused. A method returning `(w int)` produced a forwarder whose receiver and result were both `w`. | The generator compares only the number of parameters and results; forwarders drop result names. |

The same classes of gap were then looked for elsewhere:

- **Other ways to disarm a guard from handwritten code.** Redeclaring `false` inverted the old freshness assertion; fixed and tested. Redeclaring `nil` cannot disarm the other guards: the generated file stops compiling, which was observed.
- **Other inputs read by syntax without a guard.** After the first fix, every contract whose methods reach generated code is listed and therefore guarded. Imports copied for signatures are checked by the equality assertion, because a wrong import changes a type's identity.
- **Other signature shapes.** Signatures written over several lines and named results that collide with generated names are covered. A type that arrives through a dot import is not supported and fails loudly, which was observed.

This audit was done by the author of the prototype, by reading and by the tests named above. It is not an independent review, and it does not show that no further gap exists.

## Decisions

Historical: these were the open decisions when the investigation ended. [ADR 0013](../decisions/0013-optional-code-generator.md) records what the owner decided.

The owner has not approved production implementation and has named the decision that comes first:

- **Is the string-based authoring experience attractive enough to justify maintaining the tool?** What it buys is measured above: boilerplate from 29 to 12 and from 30 to 13 lines. What it costs: the hierarchy's shape and registrations live in a string that editors and refactoring tools do not look inside; generated names appear in handwritten code; and a tool of about 1,500 lines with about 2,600 lines of tests has to follow the library's API.

Two further points would have to be settled before any production design, and are not asked now:

- **The enforcement boundary.** The correspondence between a spec and its generated file is enforced by check mode, tests, and review, not by the compiler. The owner has acknowledged this boundary as real; no decision on it is recorded.
- **Contracts read by syntax.** Contracts are read from their declarations without type-checking, with the compiler verifying the result and with the unsupported forms listed above.

Everything else is detail for a production design: the grammar's keywords and defaults, generated names, the two initialization choices, a method-origin report, tool location and versioning, and whether the prototype stays under `testdata/genprobe/`.

## Running the prototype checks

Historical. The commands that ran the prototype named packages under `testdata/genprobe/`, which no longer exist; they do not run any more. The maintained generator's checks are the [generator gates](../testing.md#generator-acceptance) of the testing plan, which the CI workflow runs.

## Evidence

Recorded on 2026-11-18 and 2026-11-19 with Go 1.27.1 on darwin/arm64. Commands, raw outputs, and source identities are in the [evidence directory](../evidence/generator-investigation/README.md). The final run passed 69 tests and 65 subtests in seven test packages.

**Observed failing, then fixed.** Seventeen in all, each listed with its failing output in the evidence. Nine belong to the first pass. Four belong to the second:

1. The helpers' nil-object diagnostic differed in context from the one `Init` defines.
2. A Plan of another definition with the same slots was accepted through `HInitPlan`.
3. The generator refused the extended workloads' specs; after the extension both workloads print the shipped output.
4. Five of six stale operation and wrapper cases built successfully before the guards were emitted.

Four followed the owner's review:

1. A wrapper was generated with its private contract unlisted.
2. A stale spec built once its declaration was a variable; with `false` redeclared, fresh output failed and stale output would have passed.
3. Two spellings of one signature were refused.
4. A forwarder's receiver collided with a named result.

**Passed on first run (acceptance coverage and recorded observations).** The three mechanism probes; identity and naming across hierarchies, definitions, existing interfaces, and a contract that grows at a later level; the handwritten-duplicate characterization; type-name collisions; bootstrap with operations and wrapper; forwarder signatures; the freshness matrix cases for cosmetic contract changes, aliases, build constraints, new identifiers, hand edits, simulated generator defects, and another generator version; the redeclared `nil` and dot-import cases of the re-audit; and the extended workloads' behavior, visibility, and ownership tests in both modes. Test expectations were corrected where the emitted text changed or where an expectation of mine was wrong; those corrections are listed in the evidence and are not Red evidence.

**Reasoned or read from source, not executed.** That the definition's lock makes definition-created references briefly serialized. That a leaf whose holder is embedded under another name, with an ancestor's `Node` promoted, would still dispatch virtually through a forwarder that names `Node`. That a retargeted alias of a provider type needs no detection because generated code refers to it by name. The alternatives that were not prototyped.

**Estimates.** None of the figures in this document is an estimate. The first pass's estimate of what later outputs would remove (13 and 10 lines) is superseded by the measured 11 and 8 further lines.

**Not established.**

| Question | Status |
| --- | --- |
| Behavior on the Go 1.27.x matrix entry and in remote CI | Not run |
| Execution on Linux and Windows | Type-checked with `go vet` only |
| Authoring effect on hierarchies larger than the examples | Not measured |
| Whether authors find the stale-output compile errors clear enough | Not assessed |
| Whether the prototype has further guard gaps | An author's re-audit found one more and fixed it; no independent review has been done since the fixes |
| The cost of the definition's lock in the Plan helper | Not measured |
