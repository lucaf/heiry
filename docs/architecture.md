# Architecture

This document describes how the library implements the behavior in the [specification](specification.md). Development order and verification belong to [testing](testing.md). Significant decisions and rejected alternatives are recorded in [docs/decisions](decisions/). ADRs explain the design without overriding the specification.

## Components

The typed resolution kernel receives immutable level/capability/provider information. It does not know application method names or signatures. A builder describes bindings and supplies validated information to this kernel; it is not the inheritance mechanism itself.

| Component | Contents | Lifetime |
| --- | --- | --- |
| Schema definition | Target type; ordered level types; direct parent fields; provider-relative paths; effective contracts at each level; canonical capability types; provider owner assertions | Mutable while being described |
| Hierarchy validation | Checks of an immutable definition snapshot, independent of live receiver identity | Definition-level work before a usable Plan is published |
| Plan | Validated schema metadata, including types and paths; optional destination indices | Immutable, receiver-free, shareable |
| Fluent configuration handles | Hierarchy/level configuration and the selected lifecycle stage | Mutable during configuration; prepared configuration is frozen |
| Initialization work | Existing target, selected Plan, and temporary scope installation sites | Internal to initialization, coordinated by the builder |
| Dispatch binding | Actual provider pointers for one object and its Plan reference | Initialized object lifetime |
| Node / Scope | Once-installed holder / immutable complete-object-and-level handle | Attached to the actual receiver |
| Public wrapper | Public typed forwarding methods with named internal storage | Consumer-facing object lifetime |

The implementation lives in the root heiry package. The components above are internal responsibilities, not a requirement to export one public type for each row. They do not constitute a class or constructor registry.

## Validation, plan preparation, and instance initialization

These are three responsibilities with separate inputs and effects. [Specification S6](specification.md#s6-validation-plan-preparation-and-reuse) owns their required behavior and eager/lazy timing. Their separation does not imply three public methods or three exported types.

### Hierarchy validation

Validation consumes a stable definition snapshot describing **types, paths, contracts, and explicit registrations**. Structural inspection checks parent/provider storage, Node declarations, contract composition, capability signatures, and registration coverage. Once the chain is structurally valid, invalid node-provider registrations are reported and omitted from contract analysis. Contract validation records invalid registrations and counts only valid bindings toward coverage, so it can report missing methods alongside registration errors when the contracts remain analyzable. If a later contract is invalid, analysis stops but retains established registration diagnostics. Its result establishes definition validity or describes errors. It does not associate receivers with the hierarchy or install scopes, and it cannot replace physical checks on a later object.

### Plan preparation

Preparation turns a successfully validated snapshot into reusable metadata: level order, validated storage paths and types, effective contracts, and operation-to-provider ownership information. Dynamic resolution needs this metadata without precomputed destinations. Optional precomputation derives destinations from the same validated definition, as described under dispatch representations.

Validation and preparation may be coordinated within one internal flow, but a declaration alone is not proof of validity, and successful validation alone does not install an object. Publish a Plan only after its required metadata is complete. No application constructor or method is invoked to perform either responsibility.

### Eager and lazy lifecycle

One hierarchy definition supports either preparation schedule. The eager path starts validation and preparation at startup, independently of object allocation. The lazy path starts the same work when the definition is first needed; object setup then waits for a usable Plan before binding that object. Later objects reuse the completed Plan in either case. A cache miss on the lazy path is a preparation trigger, not an UnvalidatedSchema error by itself.

The runtime never needs two hierarchies or duplicate level/contract/binding declarations to provide these strategies. Definition-level metadata and object-specific references remain separate even when one fluent flow coordinates them. If a concrete object is temporarily available during preparation, only instance-independent type/path information may enter the reusable result; its pointers and scopes remain local to that object's setup.

### Receiver-free schema declaration API

The receiver-free declaration uses the individual signatures approved in [ADR 0007](decisions/0007-fluent-receiver-free-declaration.md) and grouped signatures approved in [ADR 0009](decisions/0009-grouped-fluent-registration.md):

```go
func Define[Leaf any]() *Definition[Leaf]
func (d *Definition[Leaf]) Level[Node, Public, Private any](parentField string) *LevelHandle[Leaf]
func (l *LevelHandle[Leaf]) Bind[Operation any](provider Operation, providerPath string) *LevelHandle[Leaf]
type OperationToken struct { /* unexported capability metadata */ }
func Of[T any]() OperationToken
func (l *LevelHandle[Leaf]) BindGroup[Provider any](provider *Provider, providerPath string, operations ...OperationToken) *LevelHandle[Leaf]
func (l *LevelHandle[Leaf]) End() *Definition[Leaf]
func (d *Definition[Leaf]) Validate() error
```

`Definition[Leaf]` owns ordered levels; each concrete `LevelHandle[Leaf]` retains its original level after later levels are added. `End` returns the definition. `Bind` accepts a typed-nil witness, immediately reduces it to provider type metadata, and uses its generic parameter for the compiler's provider/interface check. Invalid capability, nonnil witness, or malformed path calls add no registration and retain receiver-free structured diagnostics. `Validate` checks the current configuration without freezing it or preparing a Plan; later edits are checked afresh. Recorded errors remain visible, while further checks run only when safe. In particular, invalid provider registrations are omitted from contract coverage so missing operations remain reportable when the contracts can be analyzed.

The final node type must equal `Leaf` exactly. After validating the complete direct anonymous value-parent chain, validation derives leaf-relative node paths from its explicit parent field names. Provider paths are relative to the owning level; `"."` identifies the node itself. The declaration stores types, paths, contracts, and operation metadata, never receivers, bound methods, or object-capturing closures. Its validated snapshot feeds the existing hierarchy and contract validators, then the same Plan and instance-binding machinery. Validation of the declaration alone does not publish a Plan or install scopes.

Grouped registration uses the same handle and expands accepted token types into the existing per-operation entries. The group checks malformed call inputs atomically and stores only extracted type metadata; the existing validator checks provider/interface assertions and duplicates. [ADR 0009](decisions/0009-grouped-fluent-registration.md) records its approved signatures and failure rules. The dynamic fluent lifecycle uses the Definition and Plan operations approved in [ADR 0008](decisions/0008-dynamic-plan-lifecycle.md). Component-provider and precomputed lifecycle signatures remain unresolved. [ADR 0007](decisions/0007-fluent-receiver-free-declaration.md) records earlier descriptor and selector alternatives; [ADR 0006](decisions/0006-receiver-free-schema.md) remains unapproved history.

[ADR 0008](decisions/0008-dynamic-plan-lifecycle.md) approves the dynamic preparation and node-provider lifecycle signatures. A definition reference records lineage and node type without validating or freezing; a Plan reference uses the same lineage. Plan preparation consumes one validated snapshot and shares its capability identities and registration layout. Instance binding supplies actual receivers and checks ownership before installing scopes. Prepared Plan methods read only immutable metadata and instance inputs. Component factories, precomputation, and concurrent first-use coordination remain separate decisions.

### Instance initialization

The input to this stage is an existing leaf object and a completed Plan, whether obtained eagerly, through lazy preparation, or by reuse. Initialization follows validated paths to the actual value subobjects, checks type and address identity, nils, ownership, directly embedded holders, and prior initialization, and collects installation sites before writing any scopes. If configuration supplies provider pointers, they must identify those actual subobjects.

Initialization checks all collected holders before installing the per-object dispatch reference and each holder's level. Publication follows successful installation. Scopes inside that object refer to its own dispatch binding; another independently constructed object has a separate binding. Provider pointers are taken only from the object being initialized. The shared Plan cannot replace these instance checks, even on a cache hit.

The data flow is:

```text
one hierarchy definition
          │
  eager startup OR lazy first need
          │
  hierarchy validation
          │
  immutable Plan preparation ──optional precomputation──> equivalent Plan
          │                                                   │
          └──────────── reuse a selected Plan ─────────────────┘
                                  │
                       + each concrete leaf object
                                  │
                       instance checks and scope installation
                                  │
                       initialized object / public wrapper
```

None of these responsibilities calls application methods or constructors. Application constructors allocate their leaf and initialize its fields using their own native signatures. Their arguments and application state are not part of reusable hierarchy metadata.

## Plan ownership and caching

Validation and metadata construction belong to the definition, while receivers and scope installation belong to each instance. A retained Plan is the simplest reuse boundary; a definition-scoped holder or a separate cache are possible internal mechanisms. This architecture selects none of them and requires no global registry, cache, or process-wide exactly-once preparation guarantee. Warm reuse must not repeat successful definition validation for each new object.

A lookup must enforce the complete definition identity specified in S6. Changes to an intermediate contract, explicit operation owner, or ownership path can change validity or dispatch even when the leaf Go type is unchanged. Grouped and individual declarations feed the same operation metadata. Resolution mode belongs to the selected Plan variant; optional precomputed data cannot become shared mutable state in a dynamic Plan.

Any retained Plan, metadata snapshot, or cached preparation result must be receiver-free. Instance bindings can reference a Plan; the Plan and cache must not reference those bindings or the objects that prompted preparation. This one-way ownership also applies to any stored preparation diagnostic: it must not retain a failed instance through an error payload or closure.

Concurrent readers use completed immutable metadata. Lazy first-use coordination protects construction and publication of that metadata, not application fields. No consumer may observe a partially populated Plan. Independent objects then perform their own initialization checks and scope installation; one object's setup failure leaves the shared Plan and other objects intact. Same-target concurrent initialization remains unsupported under S8.

The synchronization primitive, cache key representation, lifetime/eviction policy, and whether failed preparation is retained or retried are not specified here. All choices must preserve the behavioral guarantees in S6. No cache API or public lifecycle signature is selected by this description.

## Fluent builder

The design goal and illustrative syntax belong to [specification S1](specification.md#fluent-configuration). The builder is a configuration layer over the same schema validator, instance binder, and dispatch kernel. It does not introduce a separate inheritance engine.

The declaration's `Definition[Leaf]` and `LevelHandle[Leaf]` support chaining and separate statements over the same metadata. The dynamic path chains through their existing End, Prepare/MustPrepare, and Init/MustInit operations. Future precomputed lifecycle handles remain undecided. Both styles feed the same validators and dispatch kernel.

Public and private contracts remain per-level metadata, so the chain can describe their independent extension through the hierarchy. A level can hold several provider descriptions, each associated with its explicitly selected operations. `BindGroup` expands into the same per-operation entries as individual registration; it does not create a different resolver or change override precedence. Both paths use the same duplicate, contract, and ownership validation.

Individual `Bind[Operation]` has the compiler's provider/interface check. `BindGroup` checks pointer syntax at compile time and provider/interface compatibility through definition validation. Runtime invocation remains typed in either form.

Preparation freezes a coherent configuration. Lifecycle operations return the current context or a handle for the next stage, allowing preparation, optional precomputation, and initialization to compose. Exact handle types are open; returning the same pointer from every operation is not a requirement. Error-returning paths and Must variants expose the same setup failures, and a failed stage must not allow later stages to publish an initialized object.

A fluent precompute step delegates to the Plan conversion mechanism and carries the selected result into initialization. It does not mutate the source Plan or initialized objects. Omitting that step retains dynamic dispatch. The builder may carry a live target during instance setup, but no such target enters a shared Plan. The flow may obtain a Plan eagerly or lazily; it must complete validation and Plan preparation before initializing the target. The names in the existing illustration do not assign a public method to each internal responsibility.

## State machines

These tables describe the implementation of the lifecycle guarantees in [specification S8](specification.md#s8-visibility-lifecycle-and-concurrency).

### Hierarchy definition and Plan

| From | Operation | To / result |
| --- | --- | --- |
| Definition being configured | Add metadata | Same configuration |
| Unprepared definition | Eager request or lazy first need | Stable snapshot enters validation |
| Definition under validation, valid | Complete validation | Validated metadata; no object initialized |
| Definition under validation, invalid | Report failure | Structured error; no Plan or scopes published |
| Validated definition | Complete Plan preparation | Immutable dynamic Plan ready for reuse |
| Preparation incomplete or failed | Reuse request | No partial Plan exposed; resolve completion or report failure |
| Prepared definition | Subsequent request in its reuse context | Existing Plan reused without revalidating for the new object |
| Valid dynamic Plan | Optional conversion | New precomputed Plan; original handle remains valid |
| Precomputed Plan | Conversion again | Equivalent result; may reuse the handle |
| Nil/unvalidated Plan | Attempt conversion or bypass preparation | InvalidPlan/UnvalidatedSchema; no binding |
| Either valid Plan | Initialize another object | Same Plan, separate instance binding |

### Instance setup

| From | Operation | To / result |
| --- | --- | --- |
| Configuring instance, definition not yet prepared | Lazy preparation | Obtain a validated Plan before binding; failure leaves scopes untouched |
| Configuring instance, Plan ready | Check selected definition and object references | Frozen setup context; target scopes remain uninitialized |
| Configuring instance | No valid definition/Plan, or mismatched configuration | Setup error; no prepared result or scope writes |
| Frozen prepared configuration | Attempt configuration edit | Rejected; prepared result unchanged |
| Prepared configuration | Optional precompute | Setup context selecting converted Plan; source Plan unchanged |
| Uninitialized target | Attempt binding before a valid Plan is obtained | InvalidPlan/UnvalidatedSchema; target untouched |
| Uninitialized target | Init with invalid target/storage | BindingMismatch/ownership error; target untouched |
| Uninitialized target, valid | Init | Initialized target; all holders installed before publication |
| Initialized target through any alias | Init again | AlreadyInitialized; existing bindings preserved |
| Uninitialized scope | Invocation | Uninitialized panic |

Initialization uses temporary internal installation records and exclusive target access. The builder coordinates these transitions; the tables describe lifecycle states, not mandatory exported type or method names. In particular, “prepared configuration” does not require a public type literally named Prepared. Hierarchy validation and plan preparation are distinct logical responsibilities even if the public API eventually combines their trigger.

## Dispatch representations

The observable rules are defined in [specification S7](specification.md#s7-resolution-and-precomputation). Dynamic lookup scans explicit registrations backward. A selected entry contains a pointer to its actual provider, whose own Node identifies the executing level.

Precomputed metadata stores `owner(before, capability)`: a level index or a missing sentinel. An array of depth+1 entries per capability can hold every parent destination followed by the virtual destination. One forward pass per capability records the previous owner before incorporating each level's own registration. The per-object provider map then selects the actual receiver at the stored level.

This representation handles operations absent at intermediate levels and parent calls to a different operation without changing execution state. Component providers at one level use distinct operation entries with the same level number; they do not become additional parent steps.

For d levels, m methods, and r registrations, dynamic metadata takes O(d+m+r) storage and lookup takes O(d). Dense precomputation adds O(d×m) shared indices and conversion work. Hash-map based precomputed lookup is expected O(1) with respect to depth; the call still includes type lookup/assertion and ordinary interface invocation. Per-object provider storage is separate from shared indices.

## State binding and safe reflection

Schema metadata stores the level node's type/path and a separate provider type/path for each explicitly registered operation. The provider may be the node itself or an owned component; its type is not required to match the node type. Several operation entries may identify the same provider at one level.

Instance binding follows those paths from the target and compares the resulting subobject with any supplied provider. It excludes the designated parent subtree when locating a component owned by a level. Pointer, interface, slice, and map edges are not ownership paths. The binder collects each distinct node/provider holder once, checks that it belongs to only one level, and installs that level's scope after all checks succeed. A1 and A2 at level A therefore share a level index while retaining separate receiver addresses and application state. The ownership requirements themselves are specified in S5.

An unexported component field can be addressable without being interfaceable or settable. `CanAddr` does not imply `CanInterface` or `CanSet`: `Interface()` and setters can panic on values reached through unexported fields, and taking `Addr()` does not remove that restriction. An exported Node reached through a named unexported component can inherit the restriction. Check validity, kind, and the relevant capability before using reflection operations. See the [reflection access checks](https://pkg.go.dev/reflect#Value.CanInterface), [CanSet](https://pkg.go.dev/reflect#Value.CanSet), and [Addr implementation](https://go.dev/src/reflect/value.go).

For a named unexported component, code in its declaring package can supply a provider pointer, for example `&object.hiddenComponent`. Ownership inspection could compare the expected field's exact type and `Addr().Pointer()` with that supplied pointer, without extracting the restricted field through `Interface()`. Keep the owning object and provider strongly reachable during address comparisons; never convert numeric addresses back into pointers or retain them as reusable Plan metadata. Address/type checks must still enforce the declared path and parent-subtree exclusion. See [Value.Pointer](https://pkg.go.dev/reflect#Value.Pointer). This is a feasibility inference, not a validated integration path.

Scope installation could then start from the independently supplied pointer through `reflect.ValueOf(provider).Elem()`, rather than the restricted value obtained by traversing the containing object. The directly declared exported Node can provide a checked `*Node` for ordinary library-side scope assignment; its unexported internals must not be set through reflection. This inference from the [ValueOf and Field implementation](https://go.dev/src/reflect/value.go) requires the [ownership/component acceptance cases](testing.md#required-matrix). It neither requires `unsafe` nor justifies excluding unexported components.

**Open component integration question:** the approved node-reference API supplies per-instance node pointers, but the component-reference factory and restricted-field ownership path remain undecided. Type/path metadata alone is not proof that reflection can extract a callable provider from an unexported field. The component API must address that gap without retaining instances in Plans, weakening ownership checks, or using `unsafe`; no component accessor or signature is selected here.

Each participating receiver has a directly declared Node. Node stores a scope value with the complete-object dispatch reference and a fixed level index. The receiver's `Node.Scope()` accessor makes the holder explicit and avoids a promoted ancestor holder.

## Public boundary

The wrapper stores internal state in a named unexported field. Each public forwarder performs a typed resolution and ordinary interface call. It does not embed Node or an implementation type, so promotion cannot add inheritance-only methods to its dynamic method set.

Resolving a whole aggregate interface would require one provider to implement every member. Per-operation capabilities avoid that requirement, while the wrapper combines the final public interface. The registration and visibility guarantees themselves are specified in S1, S3, and S4.

## Errors and copying

The error representation is an exported *Error with a stable Code and context fields for target type, level, provider path, contract category, capability, expected/actual signature, and optional cause. Error.Is compares codes; errors.As exposes context; Unwrap returns a cause when present. A code-only error is a matching target. Missing-method diagnostics are collected and sorted before returning.

Node contains a **named, unexported** noCopy field whose type has pointer Lock/Unlock methods. Vet recognizes the marker without promoting a fake lock API onto providers. It is an advisory static aid for the copying limitation in S8, not a runtime ownership check or a reason to add a global object registry.

## Design rationale

The [ADRs](decisions/) retain the rationale and rejected alternatives for nested embedding, explicit registrations, public/private separation, fixed dispatch scopes, receiver-free Plans, and fluent declaration. The [specification](specification.md) defines release scope and exclusions.
