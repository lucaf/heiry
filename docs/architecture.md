# Architecture

This document describes how the library implements the behavior in the [specification](specification.md). Development order and verification belong to [testing](testing.md). Significant decisions and rejected alternatives are recorded in [docs/decisions](decisions/). ADRs explain the design without overriding the specification.

## Components

The typed resolution kernel receives immutable level/capability/provider information. It does not know application method names or signatures. A builder describes bindings and supplies validated information to this kernel; it is not the inheritance mechanism itself.

| Component | Contents | Lifetime |
| --- | --- | --- |
| Schema definition | Target type; ordered level types; explicit parent/provider value-field paths; effective contracts at each level; canonical capability types; provider owner assertions | Mutable while being described |
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

Validation consumes a stable definition snapshot describing **types, paths, contracts, and explicit registrations**. Structural inspection checks parent/provider storage, Node declarations, contract composition, capability signatures, and registration coverage. Once the chain is structurally valid, invalid node-provider registrations are reported and omitted from the bindings passed to contract coverage analysis, so they cannot satisfy mandatory methods. Its result establishes definition validity or describes errors. It does not associate receivers with the hierarchy or install scopes, and it cannot replace physical checks on a later object.

### Plan preparation

Preparation turns a successfully validated snapshot into reusable metadata: level order, validated storage paths and types, effective contracts, and operation-to-provider ownership information. Dynamic resolution needs this metadata without precomputed destinations. Optional precomputation derives destinations from the same validated definition, as described under dispatch representations.

Validation and preparation may be coordinated within one internal flow, but a declaration alone is not proof of validity, and successful validation alone does not install an object. Publish a Plan only after its required metadata is complete. No application constructor or method is invoked to perform either responsibility.

### Eager and lazy lifecycle

One hierarchy definition supports either preparation schedule. The eager path starts validation and preparation at startup, independently of object allocation. The lazy path starts the same work when the definition is first needed; object setup then waits for a usable Plan before binding that object. Later objects reuse the completed Plan in either case. A cache miss on the lazy path is a preparation trigger, not an UnvalidatedSchema error by itself.

The runtime never needs two hierarchies or duplicate level/contract/binding declarations to provide these strategies. Definition-level metadata and object-specific references remain separate even when one fluent flow coordinates them. If a concrete object is temporarily available during preparation, only instance-independent type/path information may enter the reusable result; its pointers and scopes remain local to that object's setup.

### Receiver-free schema declaration API

The project owner approved M4's fluent receiver-free declaration design in [ADR 0007](decisions/0007-fluent-receiver-free-declaration.md). [ADR 0006](decisions/0006-receiver-free-schema.md) remains unapproved history. M4's `Definition[Leaf]` owns ordered levels; a concrete `LevelHandle[Leaf]` refers to one stable level. Generic `Level[Node, Public, Private](parentField)` and `Bind[Operation](typedNilProvider, providerPath)` methods keep typed individual registration fluent. `End()` returns the definition. `Validate()` examines the current snapshot without freezing or preparing a Plan. Invalid fluent calls retain only receiver-free structured diagnostics and add no binding; those diagnostics remain visible on later validation. Validation adds only checks that remain safe after an earlier error.

The final node type must equal `Leaf` exactly. Validation checks the complete direct anonymous value-parent chain before deriving leaf-relative node paths from its explicit parent field names. A provider path is relative to its owning level; `"."` names the node itself. M4 validates only node providers. A later edit is included in the next validation; M4 retains no reusable validated result. Preparation in a later milestone freezes a coherent configuration and must validate the current snapshot.

The approved M4 public declaration signatures are:

```go
func Define[Leaf any]() *Definition[Leaf]
func (d *Definition[Leaf]) Level[Node, Public, Private any](parentField string) *LevelHandle[Leaf]
func (l *LevelHandle[Leaf]) Bind[Operation any](provider Operation, providerPath string) *LevelHandle[Leaf]
func (l *LevelHandle[Leaf]) End() *Definition[Leaf]
func (d *Definition[Leaf]) Validate() error
```

The concrete level handle retains its original level after other levels are added. `Bind` reduces a typed-nil witness immediately to type metadata. Invalid capability, pointer witness, or malformed path calls record structured errors without adding a registration; subsequent calls cannot erase those errors. `Validate` reports them with whatever definition checks remain safe. The generic method parameter preserves the compiler's provider/interface check for individual bindings. The declaration surface has no Plan or instance pointers.

The receiver-free declaration design must describe types, storage paths, per-level contracts, and explicit typed registrations without an application object. Its output is a snapshot used by validation, not evidence that validation has already succeeded. Neither the reusable definition nor the resulting Plan may retain live receivers or object-capturing closures.

The design must connect this declaration to the fluent instance configuration without making authors repeat the full hierarchy. It must accommodate eager and lazy preparation, multiple providers at one level, and grouped registration. The exact grouped-registration syntax, preparation trigger, and way an object chain obtains a validated Plan remain to be resolved; they are not settled by the illustrative chain in the specification.

The alternatives considered for this approval were:

| Alternative | Fluent declaration approach | Advantages | Costs and unresolved checks |
| --- | --- | --- | --- |
| Type and field-path descriptors | A receiver-free hierarchy builder takes level/provider types, structural paths, contracts, and typed operation witnesses; object setup later uses the resulting Plan | Explicit metadata can be validated eagerly or lazily without receivers; typed witnesses can check interface satisfaction | Field paths need structural validation; M4 accepted typed-nil witnesses for individual binding; grouping and Plan handoff remain open |
| Typed field selectors with explicit schema metadata | A fluent builder associates levels/providers with typed selectors for their subobjects, alongside the types and paths needed for startup validation | Selectors express field access using Go's type system and can identify actual subobjects during instance setup | An arbitrary selector's signature cannot prove ownership, absence of captured objects, or consistency with declared paths; a proposal must resolve these limits without constructing sample objects or weakening validation |

The type-and-path alternative was selected for M4. The exact grouped-registration and lifecycle/provider-pointer signatures remain unapproved. M6 extends these same handles rather than introducing another builder; component ownership and binding remain M8 work. The selector alternative cannot establish capture freedom or physical ownership from its signature alone.

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

A hierarchy handle owns the ordered configuration. A level handle identifies one particular level and attaches its contracts and typed bindings there. In the illustrated style, `NewLevel` enters that context, configuration operations return it for chaining, and `End` returns the hierarchy handle. A saved level handle stays attached to its original level even after another level is added; a mutable last-level pointer must not redirect it. Names are illustrative, while the context relationships explain how the chain remains readable and unambiguous.

Public and private contracts remain per-level metadata, so the chain can describe their independent extension through the hierarchy. A level can hold several provider descriptions, each associated with its explicitly selected operations. Grouped registration, in the spirit of `AddBinds`, expands into the same per-operation entries as individual registration; it does not create a different resolver or change override precedence. Both paths use the same duplicate, contract, and ownership validation.

Typed binding helpers retain the compiler's provider/interface check. The exact grouped syntax and its type-checking boundary remain part of API design under specification S1. An illustrative `.Bind(...)` or `.AddBinds(...)` does not prescribe how type parameters appear in the final Go API or imply that arbitrary type tokens prove interface satisfaction.

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

**Feasibility finding from documentation and Go source inspection, not an executed prototype:** code in the component's package can explicitly supply its provider pointer, for example `&object.hiddenComponent`. Ownership inspection can compare the expected field's exact type and `Addr().Pointer()` with that supplied pointer, without extracting the restricted field through `Interface()`. Keep the owning object and provider strongly reachable during address comparisons; never convert numeric addresses back into pointers or retain them as reusable Plan metadata. Address/type checks must still enforce the declared path and parent-subtree exclusion. See [Value.Pointer](https://pkg.go.dev/reflect#Value.Pointer).

Scope installation can then start from the independently supplied pointer through `reflect.ValueOf(provider).Elem()`, rather than the restricted value obtained by traversing the containing object. The directly declared exported Node can provide a checked `*Node` for ordinary library-side scope assignment; its unexported internals must not be set through reflection. This is an inference from the [ValueOf and Field implementation](https://go.dev/src/reflect/value.go), to be verified by the acceptance cases in testing.md. It neither requires `unsafe` nor justifies excluding unexported components.

**Open integration question:** how the eventual initialization API supplies those legitimate per-instance pointers alongside a receiver-free Plan remains undecided. Type/path metadata alone is not proof that reflection can extract a callable provider from an unexported field. The future API must address that gap without retaining instances in Plans, weakening ownership checks, or using `unsafe`; no new accessor or signature is selected here.

Each participating receiver has a directly declared Node. Node stores a scope value with the complete-object dispatch reference and a fixed level index. Its provisional accessor is `receiver.Node.Scope()`, making the infrastructure holder explicit and avoiding a promoted ancestor holder.

## Public boundary

The wrapper stores internal state in a named unexported field. Each public forwarder performs a typed resolution and ordinary interface call. It does not embed Node or an implementation type, so promotion cannot add inheritance-only methods to its dynamic method set.

Resolving a whole aggregate interface would require one provider to implement every member. Per-operation capabilities avoid that requirement, while the wrapper combines the final public interface. The registration and visibility guarantees themselves are specified in S1, S3, and S4.

## Errors and copying

The error representation is an exported *Error with a stable Code and context fields for target type, level, provider path, contract category, capability, expected/actual signature, and optional cause. Error.Is compares codes; errors.As exposes context; Unwrap returns a cause when present. A code-only error is a matching target. Missing-method diagnostics are collected and sorted before returning.

Node contains a **named, unexported** noCopy field whose type has pointer Lock/Unlock methods. Vet recognizes the marker without promoting a fake lock API onto providers. It is an advisory static aid for the copying limitation in S8, not a runtime ownership check or a reason to add a global object registry.

## Design rationale

| Choice | Reason |
| --- | --- |
| Nested value embedding | Separate ancestors would duplicate state and disconnect inherited methods from the leaf's fields |
| Explicit capability registrations | A reflected method set cannot reliably distinguish direct bodies from promoted methods or forwarders |
| Fixed per-receiver scopes | A mutable current-level cursor would confuse recursion, callbacks, or overlapping calls |
| Public-only wrapper | Returning a concrete implementation through a smaller interface would still allow private capability assertions |
| Receiver-free Plan plus per-object bindings | Metadata can be shared without accidentally sharing application state |
| Plan-based conversion behind the fluent lifecycle | One conversion mechanism selects the mode before binding; chainable configuration does not require duplicated resolution logic |
| Stable level handles | A saved handle to A cannot silently start registering methods on B |
| Fluent configuration over the kernel | Keeps levels, contracts, bindings, and lifecycle steps readable without entangling dispatch with builder syntax |

Pointer inheritance, generation, and other excluded features remain outside the release scope defined by the specification. This rationale explains the current design; it does not introduce another source of requirements.
