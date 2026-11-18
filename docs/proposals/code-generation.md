# Proposal: optional code generation

Status: unapproved design proposal, preserved on 2026-11-18 so a later session can continue the investigation. Nothing here is implemented, approved, or scheduled. It is not a decision record; an ADR is written only if the owner approves an actual design. The [specification](../specification.md), [architecture](../architecture.md), and [testing plan](../testing.md) are unchanged by this document.

All code in this document is a sketch. None of it has been compiled or run.

## Purpose

Remove the handwritten ceremony that the library's safety checks force on authors, while keeping the checked library underneath:

- Registrations with a typed-nil witness per operation.
- Provider references at every initialization.
- One single-method interface per operation.
- Public wrapper forwarders.

Generation would be optional. The handwritten API stays fully supported and is what the generated code calls.

## Why a handwritten API cannot do this

The investigation that preceded this proposal looked for a handwritten API that removes the same ceremony without weakening a check, and found none. Four things force the text:

| Ceremony | What forces it |
| --- | --- |
| A typed-nil witness on every registration | Go cannot state that a type implements an interface when the interface is a type parameter, so the compiler check needs a value converted to that interface, once per operation |
| An interface per operation and a forwarder per public method | Go cannot synthesize an implementation of an interface at run time |
| A pointer per level and component at every initialization | Reflection can compare the addresses of unexported fields but cannot produce a usable typed pointer to them without `unsafe` |
| An explicit operation list per level | At run time a declared method cannot be told from a promoted one |

A generator runs on source, inside the application's package, so it can write each of these mechanically.

The small redundancies that could be removed safely were delivered separately as [ADR 0012](../decisions/0012-additive-authoring-conveniences.md). Shorter forms that the owner rejected because they weakened a check are listed under [Alternatives](#alternatives).

## Inputs and outputs

### What the author would write

- Contracts as ordinary Go interfaces with their methods written out.
- Level and component types as ordinary structs, each embedding `heiry.Node`.
- Method bodies as ordinary Go.
- Explicit metadata naming:
  - The hierarchy levels and the selected leaf.
  - The public and private contracts, where they change.
  - The parent field, where inference is ambiguous.
  - Component ownership paths.
  - The operations each provider supplies.

### What would be generated

Deterministic Go files in the application's own package, containing:

- Canonical single-method operation interfaces.
- A receiver-free hierarchy definition built from compiler-checked `Bind[T]` registrations.
- Initialization helpers that use the existing typed reference factories.
- A public wrapper exposing only the public operations.
- Optionally, typed dispatch helpers.

Generating inside the application's package is what allows direct references to unexported fields. The generated code still goes through the library's validation, ownership checks, and binder.

### Explicit forms are preferable in generated code

Generated code has no reason to be short. It should use the forms that carry the most cross-checks: `Level` with the parent field named, and `Bind` with the witness and the path. The conveniences of ADR 0012 are for handwritten code.

## Directive sketch

The names and placement below are placeholders, not a selected format.

| Directive | Placement | Meaning |
| --- | --- | --- |
| `//heiry:level [public=X private=Y] [parent=Field]` | Above a struct type | Marks a level. The root names both contracts; later levels inherit unless they name new ones. `parent=` selects the field when inference is ambiguous. |
| `//heiry:implements Name...` | Above a level type, or on a component field | Lists the operations that provider supplies. This is the author's explicit ownership statement. |
| `//heiry:leaf` | Above a level type | Requests a definition, initialization helpers, and a public wrapper for that level as the target |

### Handwritten input (uncompiled sketch)

```go
//go:generate go run <generator>

type Public interface {
	Process(msg string) (string, error)
	Describe() string
}

type Private interface {
	Validate(msg string) error
	Transform(msg string) string
}

//heiry:level public=Public private=Private
//heiry:implements Process Describe Validate Transform
type base struct {
	heiry.Node
	processed int
}

//heiry:level
//heiry:implements Validate
type limited struct {
	base
	heiry.Node
	maxLen int
}

//heiry:level
//heiry:leaf
//heiry:implements Process Describe Transform
type tagged struct {
	limited
	heiry.Node
	tag string
}

// Components are designated on the field that holds them.
//heiry:level public=StorePublic private=StorePrivate
type repository struct {
	heiry.Node
	store store    //heiry:implements Get Put
	log   auditLog //heiry:implements Record
}
```

A contract change at a later level:

```go
type LimitedPrivate interface {
	Private
	Limit() int
}

//heiry:level private=LimitedPrivate
//heiry:implements Validate Limit
type limited struct{ /* ... */ }
```

### Representative generated output (uncompiled sketch)

It uses only the existing API.

```go
// Code generated by <generator>. DO NOT EDIT.

type (
	processOp interface {
		Process(msg string) (string, error)
	}
	describeOp  interface{ Describe() string }
	validateOp  interface{ Validate(msg string) error }
	transformOp interface{ Transform(msg string) string }
)

var taggedHierarchy = heiry.Define[tagged]().
	Level[base, Public, Private]("").
	Bind[processOp]((*base)(nil), ".").
	Bind[describeOp]((*base)(nil), ".").
	Bind[validateOp]((*base)(nil), ".").
	Bind[transformOp]((*base)(nil), ".").End().
	Level[limited, Public, Private]("base").
	Bind[validateOp]((*limited)(nil), ".").End().
	Level[tagged, Public, Private]("limited").
	Bind[processOp]((*tagged)(nil), ".").
	Bind[describeOp]((*tagged)(nil), ".").
	Bind[transformOp]((*tagged)(nil), ".").End()

// bindTagged binds t to a prepared Plan, dynamic or precomputed.
func bindTagged(plan *heiry.Plan[tagged], t *tagged) error {
	return plan.Init(t,
		plan.NodeProvider(&t.limited.base),
		plan.NodeProvider(&t.limited),
		plan.NodeProvider(t))
}

type taggedPublic struct{ state *tagged }

var _ Public = (*taggedPublic)(nil)

func (w *taggedPublic) Process(msg string) (string, error) {
	return heiry.Method[processOp](w.state.Node.Scope()).Process(msg)
}
```

For components the generated initialization helper would carry the full slot claims:

```go
h.ComponentProvider[repository]("store", &r.repository.store)
```

The sketch of `bindTagged` is incomplete in one known respect: it takes the address of fields of `t` before the library sees `t`, so a nil `t` would be a plain nil dereference instead of the library's typed error. See [nil handling](#nil-handling-in-generated-helpers).

### Independent instances and standalone Plans

Generated code would expose the definition, so eager preparation, precomputation, and Plan reuse work as they do today:

```go
plan := taggedHierarchy.MustPrepare()
fast := plan.MustPrecompute()
errA := bindTagged(plan, a)
errB := bindTagged(fast, b)
```

## Source of ownership

The author's `implements` list is the source. The generator would verify it and infer nothing:

- Each listed name must be an operation of that level's effective contracts.
- The provider type must declare that method itself, rather than obtaining it by promotion. Source-level type information can distinguish the two, which the library cannot.

This is a source-inspection policy and needs explicit owner approval. It must not be extended to inferring overrides from the mere existence of a method.

Verifying that a method is directly declared narrows the trust boundary of [specification S4](../specification.md#s4-implementation-ownership-formal-trust-boundary). It does not prove that the body is a genuine implementation, that it delegates correctly, or that it terminates.

## Safety position

The intended property is that every existing check still runs, because the generated code makes the same calls an author would write. That property is necessary but not sufficient. "The emitted code calls heiry" does not by itself establish the same guarantees: a generator introduces failure modes the library has never had to consider, listed under [Unresolved risks](#unresolved-risks). If the workflow cannot preserve a guarantee, the proposal must say so rather than advertise equivalence.

| Guarantee | How generated code would keep it | Caveat |
| --- | --- | --- |
| Provider implements each registered operation | `Bind[T]` with a typed-nil witness, emitted for every operation | Only as fresh as the generated file |
| Typed calls | Generated forwarders and helpers call `Method[T]` and `ParentMethod[T]` | Same |
| Structure, contracts, duplicates, coverage | Unchanged library validation at preparation | None |
| Exact provider type, ownership, slot claims, lineage | The existing reference factories and `Init` | Helpers must not dereference before the library checks |
| Atomic and single initialization | Unchanged `Init` | Same |
| Receiver-free Plans | The generated declaration contains only typed nils | None |
| Public/private visibility | A generated wrapper with a named field and public forwarders only | Stale output can expose an operation that has since become private |
| No `unsafe`, no reflective invocation | The generator emits ordinary calls | None |

## Unresolved risks

These must be settled before any approval. Each is an open question, not a solved problem.

### Stale generated code

A comment directive does not run during `go build`. Stale output can keep old registrations or keep exposing a method that has become private.

- Some signature, type, or contract changes make stale generated code fail compilation; others remain compilable and require a freshness check.
- Adding an `implements` entry for an operation that an ancestor still supplies compiles and runs the old behavior. This is silent.
- A check run by tests or CI is a test, not a compiler guarantee, and must not be described as one.

Open: how consumer builds detect staleness.

### First-generation bootstrapping

Generation must work when no output exists yet.

- Handwritten bodies would call generated names. Skipping function bodies during type checking avoids errors from those calls.
- That does not cover declarations. A handwritten method attached to a generated wrapper type, or a handwritten struct that embeds one, names a type that does not exist yet. Skipping bodies does not solve undefined receiver or field types.

Open: whether the generator can work from a package with such errors, or whether authors must be barred from attaching declarations to generated types.

### Naming collisions and operation identity

- Generated and handwritten code must agree on the operation types. If an author also declares an interface for the same operation, the two are different capability keys.
- Unrelated operations with the same method name, in different hierarchies of one package, must not collide on one generated interface.
- Generated identifiers must not collide with application identifiers.

Open: the naming scheme and the rule for operations shared between hierarchies.

### Provider types reused across hierarchies or levels

One provider type can appear at several levels or in several hierarchies. Per-type helper methods, if generated, would then be ambiguous or duplicated.

Open: whether typed dispatch helpers can be generated at all without changing semantics, or whether a precise limitation must be reported instead. The library itself supports several definitions over one leaf type; a directive attached to a type describes only one.

### Nil handling in generated helpers

A generated helper that takes field addresses of its argument dereferences it before the library can report its typed error. Helpers must preserve nil handling, diagnostics, and initialization atomicity exactly.

Open: the shape of helpers that hand a nil object to the library unchanged.

### Consumer-side freshness checks

Open: what a consumer runs, when, and what it guarantees. Candidates are a generator check mode invoked from a test, a CI step that regenerates and compares, or both. None is a compile-time mechanism.

### Visibility checking

Tests of generated wrappers must check individual private operations. Checking that the wrapper does not satisfy the whole private interface is weaker: a wrapper could expose one private operation and still fail that assertion.

### Build configuration

Open: build tags, platform-specific files, imports required by operation signatures, tool versioning, and byte-for-byte deterministic output.

### Tooling and maintenance

Open: where the generator lives, and whether it can be written with the standard library alone so the library module stays free of dependencies. A generator must follow new language syntax, including generic methods.

## Evidence

| Claim | Status |
| --- | --- |
| The Go 1.27.1 standard library type checker offers an option to skip function bodies, lists a named type's methods, and exposes an interface's embedded types | Confirmed by reading the installed source. Behavior was not executed. |
| A type's method list excludes promoted methods | Reasoned from documentation. Not executed. |
| The existing API calls in the generated sketch exist with those signatures | Confirmed against the package |
| The handwritten inputs plus generated output compile and reproduce the shipped examples' output | Not established |
| The generator can run on a package that does not yet compile | Not established; see bootstrapping |
| Handwritten line counts would fall by roughly a fifth for the two shipped examples | Estimate from uncompiled drafts |

## Alternatives

| Alternative | Outcome |
| --- | --- |
| A better handwritten API only | Delivered as ADR 0012. It removes small redundancies; the four forced items remain. |
| Initialization from bare pointers typed as `any` | Rejected by the owner: pointer arguments stop being compiler-checked, and reference lineage and slot claims disappear |
| Registration without a witness | Rejected by the owner: the provider/interface check moves from compilation to validation, and a wrong component path can bind silently when two components implement the same operation |
| Optional arguments as variadics | Rejected by the owner: an arity compile error becomes a run-time rejection |
| A sealed interface accepting any holder-bearing receiver | Withdrawn: method promotion lets embedded pointers and embedded interfaces satisfy it, so it cannot restrict its arguments to valid providers |
| Typed selector functions at declaration | Not pursued: the library cannot establish that a function captures nothing, so receiver-free Plans would rest on author discipline |
| Reaching unexported fields through reflection's unsafe-pointer accessors | Not pursued: it avoids importing `unsafe` but defeats the prohibition |
| Inferring overrides from method sets | Excluded by specification S1 and S4 |

## Proposed validation

If the owner authorizes further work, investigate in this order, stopping after each step:

1. **Registration and initialization generation.** This is the largest reduction and avoids the type-reuse questions.
2. **Wrappers and operation interfaces.**
3. **Typed dispatch helpers**, only if provider reuse can be handled without changing semantics.

Narrow probes that would resolve the main uncertainties:

- Whether the type checker, skipping bodies, can analyze a package whose handwritten code refers to names and types that are not generated yet.
- Whether declared and promoted methods are distinguished as documented.
- Whether the two shipped examples, rewritten as handwritten input plus generated output, compile and produce exactly their current checked output.
- Whether the generator can avoid dependencies outside the standard library.

## Decisions an approval would have to make

1. Whether to pursue generation at all.
2. The directive vocabulary and placement.
3. The source-inspection policy, including whether a declared but unlisted method that matches an operation is an error.
4. The freshness mechanism and what it is allowed to claim.
5. Which outputs are generated, each separately.
6. The generator's location and dependencies.
