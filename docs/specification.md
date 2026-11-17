# Functional specification

This is the normative behavior for the first reusable heiry release. [Architecture](architecture.md) describes enforcement and lifecycle; [testing](testing.md) defines implementation gates. [Roadmap](roadmap.md) tracks implementation status and evidence.

## S1. Scope and authoring costs

Use one leaf object containing its ancestors through anonymous value embedding. Support single inheritance, typed virtual and parent dispatch, extensible public/private interfaces, and independent objects with shared immutable schemas. Application constructors retain arbitrary signatures; object allocation and inheritance setup are separate.

Hierarchy declarations are receiver-free: their reusable metadata contains types, paths, contracts, and explicit operation registrations, never an application object, bound method, or object-capturing closure. Individual `Bind[T]` registration takes a typed-nil provider witness and retains the compiler check that the provider satisfies T. Grouped registration remains required as a concise, chainable way to associate one provider with several explicitly selected operations. In a form such as `AddBinds(provider, Of[X](), Of[Y]())`, the tokens do not prove that the provider implements X and Y; validation must check each provider type against each selected capability interface. The grouped name and signature are illustrative. Instance-specific pointer and ownership checks apply to both styles.

Grouped and individual registration describe the same per-operation bindings and must not infer overrides from a provider's method set. **Validation-time grouped checks do not weaken invocation typing:** either style resolves to typed interfaces through `Method[T]` and `ParentMethod[T]`, preserving compile-time argument/result checks and ordinary calls under S2, without reflective invocation.

Level handles stay attached to the level where they were created, including after later levels are added. Invalid fluent calls add no binding, retain only receiver-free structured diagnostics, and keep those errors visible on later validation; a corrected definition is needed to clear them. `Validate()` examines the current configuration without freezing it or creating a Plan. Later edits are checked on the next validation and cannot reuse an earlier validation result. The final declared node type must equal `Leaf` exactly, and the complete direct value-parent chain must be valid before leaf-relative node paths are accepted. Validation reports recorded diagnostics together with further errors that can safely be established. Invalid registrations do not satisfy contract coverage; when contract analysis is possible, every missing mandatory public and private method is reported. [Architecture](architecture.md#receiver-free-schema-declaration-api) defines the declaration signatures and enforcement.

Authors supply canonical single-method interfaces, explicit registrations, bodies, directly embedded Node holders, and manually written public forwarders. Every public operation needs a forwarder on the public wrapper. Private operations need no public forwarder. This is an explicit cost, not a hidden promise to synthesize aggregate interfaces. Code generation is deferred.

The first reusable release must support multiple providers at one inheritance level, such as A1 and A2 supplying different operations. Fluent configuration must accommodate them.

### Fluent configuration

Hierarchy definition and behavior configuration must be intuitive, concise, readable, and naturally chainable, without unnecessary boilerplate. Make the hierarchy levels, their public/private contracts, and their explicit bindings clear at the point of configuration. Preparation, optional precomputation, and initialization should compose naturally with that style.

The following illustrates the intended programming style. It is pseudocode, not a mandatory API signature or an executable example; `...` stands for omitted configuration:

```go
NewHierarchy(c).
    NewLevel("A", &c.B.A).
        Contracts(...).
        Bind(...).
        End().
    NewLevel("B", &c.B).
        Contracts(...).
        Bind(...).
        End().
    NewLevel("C", c).
        Contracts(...).
        Bind(...).
        End().
    MustPrepare().MustPrecompute().MustInit()
```

The illustration does not replace the declaration signatures in [architecture](architecture.md#receiver-free-schema-declaration-api) or select later lifecycle and grouped-registration signatures. Return the same configuration handle where appropriate, or a handle for the next stage so the chain can continue. Fluent syntax must accommodate individual and grouped registration, multiple providers per level, per-level contract extension, ordinary error handling, and both dynamic and precomputed dispatch.

The illustration does not select how a reusable hierarchy definition supplies a Plan to the object chain. That Plan may be prepared eagerly or lazily under [S6](#s6-validation-plan-preparation-and-reuse), without requiring a second hierarchy definition.

## S2. Native calls and dispatch boundary

Consumers call ordinary methods, e.g. `obj.Process(data, retries)`. The public wrapper resolves the most-derived implementation. Implementations use `Method[T](receiver.Node.Scope())` for virtual calls and `ParentMethod[T](receiver.Node.Scope())` for parent calls, then invoke a method on the returned typed interface.

Preserve arbitrary arguments, variadics, multiple results, and ordinary application errors. No reflect.Value.Call, unsafe, method-name strings for invocation, source inspection, mutable execution cursor, constructor registry, or class registry is permitted. Structural reflection belongs in setup. Runtime type-identity lookup and type assertions are allowed.

Concrete calls follow Go selectors: `c.Process` selects C's body if declared there; `c.B.Process` may select A's promoted body. Converting an ancestor pointer to an interface does not change that. Direct calls bypass virtual resolution at entry; their bodies can still explicitly dispatch virtually. Calling `a.Validate` does not select B's override. Method/ParentMethod must not be advertised as interception of arbitrary Go calls.

## S3. Contracts and target validation

Public and Private are ordinary method-only interfaces. **Private means inheritance-internal API, not compiler-enforced protected access.** Each level supplies its effective public/private pair or inherits both unchanged; the root must supply both, including an explicit empty interface if needed.

Each child must be a structural superset of its parent's public contract and, independently, its private contract. Preserve exact parameter/result types and variadic status. Interface embedding is preferred. Reject narrowing, signature changes, or moving inherited operations between categories. Any public/private name collision is invalid, even with identical signatures. Identical repeated declarations within a single interface follow Go's native rules and denote one operation.

Use exported operation names and a single canonical single-method interface type for each operation across levels. Exact type aliases share identity; a separately defined but structurally identical interface is a different key and must not be registered as a second canonical type. Aggregate interfaces are contracts, not resolution requests. Reject unexported operations and unsupported constraint-only contracts.

A binding can implement only an operation declared in its own level's effective contracts. Descendants can introduce operations and override inherited ones. Every operation in the final target's effective contracts is mandatory. Intermediate levels may remain abstract; coverage is checked over the selected ancestry, not independently at every node. A standalone A checks only A; B checks A/B; C checks A/B/C. Descendants never satisfy a standalone ancestor target. There are no optional methods or try-parent API in this release.

For example, if B extends its private contract with Transform and only C implements it, standalone B is abstract while C can be complete. This is a behavioral illustration, not an existing example program.

Bodies should depend on their own effective contracts. Resolution keys come from the complete target: an ancestor can technically request a later capability on C but gets UnknownCapability on a smaller target without it. Validation does not inspect body dependencies or prove termination.

## S4. Implementation ownership: formal trust boundary

**A binding is an assertion by the programmer that the provider owns the intended implementation. The framework validates signature and physical ownership, but not method declaration origin.**

A typed `Bind[Validator]` registration for B can compile when B merely promotes A.Validate. Do not treat compiler satisfaction or reflection's method set as evidence of a new override. Empty registrations mean inherit; promoted bodies and forwarders do not create registrations automatically. Do not register wrappers or infrastructure as implementations.

Incorrect assertions may reenter the same body during a parent call and recurse indefinitely. Detection of declaration origin and proof of body termination are outside the guarantee.

## S5. State, structure, and providers

The hierarchy is a nonempty ordered chain of distinct named struct types. The final node is the target; each preceding node is exactly one direct anonymous value field of its child. Validate explicit parent field metadata, not declaration order or promoted methods. Reject pointer inheritance (including nonnil pointers), nil nodes/providers, repeated levels/types, missing intermediate nodes, and ambiguous or nonanonymous parent edges.

Bind the actual embedded addresses; never allocate independent ancestors or copy their state. Unrelated composition fields contribute no methods automatically.

A level may register its node, multiple component providers, or both. A provider is a pointer to the actual node or to a struct value physically contained within that node through value-field paths. Named and anonymous value fields may hold components. Exclude the designated inheritance-parent subtree when establishing component ownership; pointer, interface, slice, and map edges are not ownership paths. Reject foreign objects, even when their types match. A component provider's type need not equal the level node's type.

One provider may supply several operations at its owning level. Different providers at that level may supply different operations, such as A1.Process and A2.Validate. Each operation may be registered at most once per level, including across providers or registration groups; reject duplicates rather than using registration order as precedence. The same provider cannot belong to two inheritance levels. Unrelated fields contribute no implementations unless explicitly registered.

Components do not become additional inheritance levels. They share their owning level's contracts and dispatch position; the physical parent chain remains single inheritance.

Each participating node/provider directly embeds a value Node, not a pointer or merely a promoted ancestor Node. Bodies use their own `receiver.Node.Scope()` holder. Init installs that holder's fixed level and complete-object reference. Node has no application state. Its scope identifies the actual owning level, without relying on an ancestor's promoted holder. The holder and copying safeguards are designed in architecture.md.

## S6. Validation, plan preparation, and reuse

Keep three concerns distinct, even if a future fluent operation coordinates them:

| Concern | Required work | Frequency |
| --- | --- | --- |
| Hierarchy validation | Verify the definition's structure, public/private contracts and extensions, capability identities/signatures, explicit registrations, ownership paths, supported storage/receiver forms, duplicates, and final target's method coverage | Definition-level work performed when that immutable definition is first prepared for reuse; not repeated for each object using its Plan |
| Plan preparation | Produce complete, immutable metadata from a successfully validated definition; optionally include precomputed resolution destinations | Once for a reusable prepared definition; optional conversion produces an equivalent reusable resolution variant |
| Instance initialization | Associate a concrete leaf and its actual embedded nodes/providers with an existing validated Plan; check instance invariants and install their scopes | For every independently constructed object, including every object using a cached Plan |

Validation establishes that the definition is valid; preparation materializes reusable metadata. A validated definition alone is not an initialized object. A Plan is usable only after validation and preparation succeed. Invalid definitions must produce structured errors before any object is initialized from them, and failures must publish neither a partial Plan nor partial instance bindings.

### Eager and lazy preparation

Support both strategies for the same hierarchy definition and fluent configuration:

- **Eager:** validate and prepare during application startup, without constructing an application object, then reuse the Plan for subsequent objects.
- **Lazy:** validate and prepare when the hierarchy is first needed, including during first-object setup, then reuse the Plan for subsequent objects.

Eager validation is optional. Lazy preparation is not an invalid or unvalidated initialization path: it must obtain a successfully validated, prepared Plan before the instance-binding stage proceeds. No separate hierarchy declaration or duplicated configuration may be required to switch strategies. Neither validation nor plan preparation invokes application constructors or methods.

Both strategies have identical inheritance semantics and initialized-object behavior, including dispatch, parent calls, state ownership, visibility, and application errors. Preparation timing and cost, and therefore the time at which an invalid definition is reported, differ. Eager preparation can report invalid definitions at startup even if no object is constructed; lazy preparation reports them on first need. An unused lazy definition need not be validated at startup.

### Plan ownership and caching

Reuse successful validation and prepared metadata across objects rather than repeating definition-level work on every initialization. This is reuse within a retained definition/Plan context, not a requirement for process-wide deduplication: separately managed contexts may prepare equivalent definitions. A changed definition needs its own validation and prepared metadata; it cannot mutate an existing Plan or silently reuse incompatible cached results.

Reusable plans and any cached preparation metadata must never retain application instances, receivers, Node holders, dispatch scopes, bound methods, or object-capturing closures. This includes references to any concrete object temporarily available during configuration or preparation. Only per-instance bindings retain that instance's actual subobjects. Independent objects may share immutable Plans and resolution tables, but must not share instance-specific receivers, pointers, or scopes.

Cache identity must account for the complete definition: selected target, ordered levels and node types, parent paths, every level's effective public/private contracts, canonical capability identities and signatures, explicit operation registrations, provider types, and provider ownership paths/levels. The leaf Go type alone is insufficient. Optional resolution variants must also be distinguished so a precomputed request cannot receive a dynamic lookup path. Cache keys and representation are implementation choices, not public API commitments.

Cached Plans must be safe for concurrent reuse. Concurrent first use must coordinate preparation/publication so no caller sees incomplete metadata or another object's bindings. A cache hit never bypasses per-instance validation of actual type/address ownership, nils, directly embedded holders, or prior initialization. A failure for one instance must not invalidate a valid shared Plan or alter another object.

Do not mandate a global cache, a particular synchronization mechanism, or a cache lifetime/eviction policy. Precomputation remains optional under S7. The lifecycle and ownership responsibilities are described in [architecture](architecture.md#validation-plan-preparation-and-instance-initialization).

## S7. Resolution and precomputation

Both dynamic and precomputed resolution must implement the following behavior. Number levels root-first. Virtual resolution returns the greatest level explicitly registering T. Parent resolution from L returns the greatest registered level strictly below L, skipping absent implementations. Invoke that level's actual provider, never a leaf pointer that happens to promote it.

The body/provider's own scope determines subsequent parent calls. C.Process → A.Process still lets A virtually select B.Validate. Parent calls inside A start below A, not below C. A parent call to another operation also starts below the executing level. For component providers, this is the owning inheritance level: ParentMethod from A1 does not search a sibling A2 at the same level. Recursive virtual calls restart at the leaf; parent delegation descends; callbacks never change scopes.

Unknown capability requests panic with UnknownCapability; a known capability with no eligible ancestor panics with NoParentImplementation. Application signatures gain no framework error result. These must be distinguishable typed diagnostics. Calls through an uninitialized scope panic with Uninitialized.

A precomputed Plan must select the same provider as dynamic resolution for every operation and virtual/parent starting position. Missing ancestors are valid metadata, not conversion errors. This includes operations absent at intermediate levels and operations supplied by different components at one level. The table representation belongs to architecture.md.

Precompute is pure: it changes no source plan, scopes, or objects; invokes no application methods; publishes no partial result on failure; and is semantically idempotent. Existing objects retain their plan. It is safe to convert an immutable plan while other objects use it. Precomputed invocation must not scan the hierarchy or silently fall back to dynamic lookup. Results, traces, side effects, errors, and ownership must match the dynamic oracle.

Production precomputation operates on Plan, before binding an instance. A fluent lifecycle operation may select the converted Plan for subsequent initialization while preserving the source Plan. This does not require a second resolution engine or mutation of a live object. Live mode switching and mixing modes among one object's scopes are unsupported.

## S8. Visibility, lifecycle, and concurrency

Constructors publish a dedicated unexported public wrapper with named unexported storage, never an embedded implementation or Node. Its dynamic method set contains only final public operations. Private type assertions must fail. Internal leaked implementation/provider pointers retain their exported methods. External consumers are supported; cross-package derivation is deferred.

The lifecycle separates hierarchy validation, reusable Plan preparation, and initialization of an allocated leaf. Preparation may be eager or lazy under S6. Fluent configuration presents these concerns coherently. Invalid setup changes no existing scopes, including those already initialized and causing rejection. See the state transitions in architecture.md.

Install all scopes exactly once under exclusive instance initialization access, then publish. A second Init, even through another alias of the same object, returns AlreadyInitialized without replacement. Concurrent setup of the same target is unsupported; concurrent setup of different targets sharing one Plan or triggering lazy preparation of the same definition is supported.

Published plans, tables, object bindings, and scopes are immutable. Concurrent/reentrant dispatch is supported; application state, callbacks, and input data need their own synchronization. No global, goroutine-local, or object-local mutable current-level cursor is allowed.

Initialized nodes, ancestor values, and component providers must not be copied. Copies would retain references into the original object. Interface/plan/scope handle copying aliases the original and is allowed; it does not clone state. noCopy is advisory; reflection and all legal Go copies cannot be universally prevented or detected. Rebinding and cloning remain deferred.

## S9. Errors, verification, and excluded scope

Use deterministic exported typed errors compatible with errors.Is/errors.As, with stable codes and structured context. The error representation is designed in architecture.md. Must variants panic with the same error object returned by error-returning APIs. Deterministically report all missing mandatory methods. Do not match errors solely by text.

Distinguish at least InvalidDeclaration, InvalidHierarchy, InvalidContract, ContractExtension, ContractCollision, InvalidCapability, CapabilityIdentity, SignatureMismatch, UndeclaredOperation, DuplicateBinding, MissingMethods, InvalidProvider, MissingNode, ProviderOwnership, InvalidPlan, UnvalidatedSchema, BindingMismatch, AlreadyInitialized, Uninitialized, UnknownCapability, and NoParentImplementation. Diagnostic context must identify the target, level, provider path, contract/capability, and expected/actual signatures where relevant. Exact prose is not an API; codes and structured meaning are.

Acceptance evidence is defined in testing.md. Code generation, arbitrary external providers, pointer inheritance, cross-package derivation, live mode switching, and cloning are deferred. Multiple providers are required by S5; fluent configuration and grouped registration are design goals under S1, not excluded features.

The minimum supported Go version is the version declared in go.mod. Toolchain coverage and performance verification belong to testing.md.
