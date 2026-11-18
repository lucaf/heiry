# Testing and implementation plan

This document owns implementation order, required tests, and acceptance criteria. Apply the development process in [AGENTS.md](../AGENTS.md); do not treat the milestones below as an alternative process. Behavioral expectations come from the [specification](specification.md), representations/state transitions from [architecture](architecture.md), and milestone status and check results from the [roadmap](roadmap.md).

## Package before examples

Implement the actual root heiry package and pass the tests for its supported behavior before writing usage examples. Until then, use focused package tests and test fixtures to drive development. Do not build a separate engine as a demonstration or count tests of one as evidence that heiry is implemented.

Dynamic dispatch is the initial reference implementation. Prove it with the kernel tests before adding precomputation. The fluent builder follows the kernel and validation work; multiple providers follow the single-provider chain. Package-usage examples are the final milestone below, subject to the examples rules in AGENTS.md.

## Gated production milestones

| Stage | Add only after prior gate | Acceptance evidence |
| --- | --- | --- |
| 1. Typed dynamic kernel | Two-level Method[T]/ParentMethod[T] with one node provider each | Different signatures, variadics/multiple returns, ordinary errors, missing parent, unknown capability; simplest linear search |
| 2. Three-level semantics | A → B → C and selective overriding | Skip absent intermediate implementation; nested/different-method parent calls; body/provider scope determines parent position; finite recursion/reentry |
| 3. Embedded state/scopes | Bind actual leaf subobjects | Pointer identity and shared state; independent objects; zero/uninitialized scopes; race-safe concurrent calls; no mutable execution cursor |
| 4. Public/private contracts | Per-level contracts and wrapper | Both contracts extend independently; collisions/narrowing fail; abstract intermediate levels; final target coverage; public-only dynamic method set |
| 5. Structure/ownership | Type schema and physical checks | Definition validation independent of objects/constructors; value-only parent edges; duplicate/missing registrations; nils/foreign providers; deterministic typed errors; no partial writes |
| 6. Builder/lifecycle | Fluent adapter over tested kernel using an approved schema API | Compile and execute chained hierarchy/level/contract/binding configuration; eager and lazy preparation of one definition; validated Plan before instance binding; chained preparation/initialization; every lifecycle transition; stable handles; freeze; noCopy; individual and grouped registration with explicit type-checking guarantees; compile-only public API fixtures |
| 7. Shared plans/precomputation | Reuse and immutable conversion | Complete-definition cache identity; concurrent lazy first use and warm reuse; chained preparation/precomputation/initialization; dynamic oracle equivalence; no receiver leakage; concurrent reuse/conversion; no live mode change; no scans in precomputed path |
| 8. Multiple providers | Components A1/A2 at one level | Physical ownership excluding parents/pointers; multiple operations on one provider; distinct operations across providers; grouped/individual equivalence; duplicate and cross-level provider rejection; component scope anchoring; no sideways parent lookup; dynamic/precomputed equivalence |
| 9. Package usage examples | After the actual package and its acceptance tests work | Programs import heiry and demonstrate its accepted public API; compile and run in CI; contain no replacement engine |

Shared immutable schema metadata exists conceptually before stage 7; stage 7 proves reuse/concurrency and adds optimized resolution after the lifecycle is tested. Internal fixtures may assemble data for early tests, but must not masquerade as a stable public builder.

Milestones order the implementation; placing the builder after the kernel and component providers at stage 8 does not exclude them from the release. Stage 8 is required to satisfy specification S5. The declaration, dynamic Plan, grouped registration, precomputation, and component-reference decisions are recorded in [ADR 0007](decisions/0007-fluent-receiver-free-declaration.md), [ADR 0008](decisions/0008-dynamic-plan-lifecycle.md), [ADR 0009](decisions/0009-grouped-fluent-registration.md), [ADR 0010](decisions/0010-concurrent-preparation-and-precomputed-plans.md), and [ADR 0011](decisions/0011-component-provider-references.md); implementation fixtures use those approved signatures.

## Fluent configuration acceptance

At stages 6 and 7, test the design goal in [specification S1](specification.md#fluent-configuration) through the actual public package API:

- Compile and execute a readable A → B → C chain configuring each level's contracts and typed bindings. Verify that levels retain their own configuration and that derived public/private contracts can extend their ancestors.
- Register several operations for one provider in a grouped fluent step and compare with equivalent individual registrations. Mixing grouped and individual calls must preserve semantics and reject duplicate operations across either form. At stage 8, extend these cases to A1/A2 at one level and verify that each selected body acts on its actual embedded provider.
- Verify continuation from level configuration back to hierarchy configuration and then into lifecycle operations. Saved level handles must still target their original level after subsequent levels are added.
- Exercise dynamic preparation/initialization and, at stage 7, preparation/precomputation/initialization as chains. Compare dispatch traces, results, and embedded-state identity against the kernel scenarios.
- Verify equivalence between a chain and the same sequence split across variables; assert handle identity only where the chosen API promises it. Stage-specific return types must permit the documented next operation.
- Exercise error-returning paths and Must variants. A failed stage must report the same underlying setup error and prevent partial initialization. Reject edits after preparation while preserving the prepared result.
- When a provider registration is invalid but the hierarchy and contracts remain analyzable, report its diagnostic together with every missing mandatory method. Test both orders of an invalid and valid registration for the same operation, including grouped and individual calls: the valid registration still covers the method, while the duplicate rejects the definition under [S5](specification.md#s5-state-structure-and-providers).
- When a later contract is invalid, retain earlier registration diagnostics alongside the contract error without continuing coverage analysis through the invalid contract.
- Exercise the same hierarchy definition through eager preparation and lazy first use, then reuse its Plan in fluent object setup. First use without an already prepared Plan must succeed through valid lazy preparation; attempting to bind without ever obtaining a valid Plan must fail. Ensure neither chaining nor precomputation captures receivers in shared metadata.
- Use negative compilation fixtures to prove typed binding remains enforced through the builder. For the approved grouped API, test every promised compiler check and any explicitly documented validation-time check; do not count runtime rejection as compile-time safety. Include successful controls so syntax or environment failures cannot count as type-safety evidence.

Use the agreed concrete signatures when these tests are implemented. The pseudocode in S1 is a style illustration, not a compilation fixture or a requirement to retain its exact names.

## Preparation and reuse acceptance

At stages 5–7, distinguish definition checks, Plan construction, and per-instance work as specified in [S6](specification.md#s6-validation-plan-preparation-and-reuse). Extend the applicable scenarios to multiple providers at stage 8:

- **Timing and equivalence:** configure one hierarchy and exercise eager preparation before any object allocation, and lazy preparation on first need. Compare dispatch traces, results, parent destinations, visibility, application errors, and state changes. Verify that neither strategy requires a second declaration or repeated configuration. Repeat with optional precomputation once available.
- **Definition-level work versus instance work:** observe validation/preparation and initialization separately. After a reusable Plan is ready, later objects in its reuse context must not repeat successful definition validation or metadata construction, while each object must still undergo instance checks and receive its own bindings/scopes. Do not require process-wide deduplication across separately managed contexts.
- **Invalid definitions:** eager preparation reports the error at startup; an unused lazy definition is not required to fail then. Lazy first use reports the equivalent structured error before any scopes are installed. Concurrent requests must not receive a partial Plan. Test the approved uncached failure/retry policy and a repair between attempts, including sticky frozen-edit interactions; do not assume which concurrent operation wins.
- **Complete definition identity:** use the same leaf Go type with definitions differing in intermediate public/private contracts, registrations, canonical capabilities/signatures, provider ownership, or parent/provider paths. Invalid variants must still fail and valid variants must retain their own dispatch behavior; leaf-type cache hits must not bypass these distinctions. Verify that requested precomputed resolution cannot silently use dynamic scanning.
- **Receiver-free reuse:** prepare once, stop using the first object, and initialize independent objects from the retained Plan. Check that shared metadata contains no receivers, instance pointers, Node holders, scopes, bound methods, or capturing closures. Include any retained preparation diagnostics in this check. Verify that later bindings point only into their own leaf objects and that mutations remain isolated.
- **Instance checks on reuse:** a warm Plan must still reject invalid object/provider identity, nils, foreign subobjects, missing holders, and already initialized objects. Failures must preserve existing scopes and leave the Plan and other objects usable.
- **Concurrency:** coordinate simultaneous lazy first requests for one definition from different objects, and concurrent initializations against an eagerly prepared or already cached Plan. Verify complete metadata publication, correct provider identities, independent scopes, and race-free library bookkeeping. Exercise concurrent Validate/reference creation and permitted configuration/preparation interleavings without treating a multi-call declaration as a transaction. Use synchronized or read-only application bodies so application-field races cannot be mistaken for dispatch failures. Same-object concurrent initialization remains outside the supported contract.
- **Immutable conversion:** precompute while other objects use the source Plan, then initialize new objects with the converted Plan. Both schedules and resolution modes must remain equivalent; original objects retain their selected Plan, and converted metadata must not capture an instance.

These criteria describe observable behavior and ownership. They require no global cache, particular synchronization primitive, eviction policy, or unapproved public method signature.

## Required matrix

Run applicable semantic scenarios in both modes once stage 7 exists:

- Virtual and parent lookup: every starting level; root-only implementation; gaps; override at leaf; parents of different methods; known capability with no parent versus unknown capability.
- Contracts: successful public/private validation; multiple missing mandatory methods in deterministic order; signature and variadic mismatch; duplicate/unknown registration; category collision/move; invalid extension; standalone ancestor versus complete descendant.
- Ownership: actual embedded receiver; foreign identical-type instance; nested/unexported fields; nonaddressable/nonpointer/nil values; interface aliases of the same pointer; unrelated pointer fields; parent-subtree exclusion; directly embedded versus promoted Node; repeated node/provider ownership.
- Trust boundary: register a provider that only promotes a method, show signature/physical checks can pass, and demonstrate the selected provider without entering unbounded recursion. Assertions must respect the trust boundary in specification S4.
- Lifecycle: every row of the architecture state tables, including frozen edits preserving the prepared result, repeated Init through aliases, attempts to bypass Plan preparation, mismatched per-level metadata, and failed setup preserving all preexisting cells. Eager/lazy timing, fluent configuration, and reuse cases are specified above.
- Visibility/type safety: an external-package consumer can use every public method; runtime private assertions fail; private calls and wrong typed Bind providers fail compilation for the expected reason; valid controls compile.
- Reentrancy/concurrency: recursion and callbacks preserve outer parent positions; same-object concurrent calls use race-safe bodies; different objects initialize against shared plans; conversion never mutates a source plan. Same-target concurrent initialization is unsupported, not a behavior to race and then promise.
- Copying: go vet diagnoses an ordinary Node-containing value copy; handle copies remain legal. Do not claim the marker catches reflection or every legal Go copy.

Negative compilation fixtures live in testdata and are invoked explicitly by a Go test harness. The harness must verify both nonzero exit and a diagnostic about the intended error, so missing modules, cache permissions, and syntax accidents cannot falsely pass. Use actual typed API calls in compile-only examples after acceptance. Library implementation may remain unexported; external-package tests must not reach into it.

For the reflection feasibility analysis in [architecture](architecture.md#state-binding-and-safe-reflection), add these acceptance cases at the ownership/component milestones; this list does not claim they have been implemented or run:

- Use exported and named unexported component fields, including nested value paths, with provider pointers supplied by their declaring package. Exercise the actual heiry package from an external-package fixture, without adding cross-package derivation or `unsafe`.
- Verify exact ownership for each supplied pointer; reject same-type foreign objects, providers under the designated parent subtree, and cross-level reuse. Matching a type alone must not pass. Include successful controls for multiple operations sharing one provider at one level.
- Verify scope installation through the supplied pointer's directly embedded Node, correct owning-level parent dispatch, and independent scopes for separate objects. A later failing provider check must leave every holder untouched.
- Characterize `CanAddr`, `CanInterface`, and `CanSet` separately. In isolated negative fixtures, confirm the expected panics from restricted-field `Interface()`, `Addr().Interface()`, and setters, including an exported Node reached through a named unexported field. Verify that normal initialization avoids these operations on restricted values and that nonaddressable/invalid inputs produce framework diagnostics rather than reflection panics. Run these cases on the supported Go toolchain matrix.

## Exhaustive/property/fuzz tests

Use an independently written backward-search model as the oracle. For small depths enumerate every nonempty registration subset and every parent/virtual starting point; compare selected provider identities and missing outcomes. Include operation introduction after the root and, at stage 8, multiple components. Compare precomputed tables/calls with the model, not with a second copy of the optimization.

For larger layouts use Go fuzzing. Bound generated depths and table sizes so inputs test semantics instead of exhausting memory. Seed root-only, leaf-only, alternating, dense, and missing-parent layouts. Persist minimized regressions in the fuzz corpus. Default go test runs seeds; a bounded fuzz campaign is separate evidence.

## Quality gates and CI

CI must discover a nonempty set of applicable maintained library packages, including actual non-test source in the root heiry package, before claiming Go verification. If no applicable packages exist, report Go verification as not run/no implementation; an empty successful tool invocation or unrelated package tests are not evidence. The existing root-package source and tests must be included. A no-tests run cannot establish a tested milestone.

For M2 only, the owner approved completion from successful scoped local formatting, vet, unit, race, and documentation checks on the M2 source. An M2 CI run is not a completion prerequisite under this exception. The authorization and actual evidence are recorded in the [M2 roadmap entry](roadmap.md#m2-nested-embedding-and-instance-scopes). Report local and CI evidence separately; do not describe an unrun CI job as successful.

For M4 only, the owner directed completion when the declaration milestone's scoped local code and documentation gates pass. The authorization and actual evidence are recorded in the [M4 roadmap entry](roadmap.md#m4-reusable-schema-declaration-api). Report this local-gate exception separately from remote CI evidence; do not describe an unrun remote job as successful.

For M5 only, the owner directed a green CI designation and milestone completion from passing scoped local code and documentation gates, as for previous milestones. The authorization and observed checks are recorded in the [M5 roadmap entry](roadmap.md#m5-immutable-dynamic-plans-and-instance-binding). This local-gate exception does not assert that remote CI ran.

For M6 only, the owner directed a green CI designation and milestone completion when its scoped local code and documentation gates pass. The authorization and observed checks are recorded in the [M6 roadmap entry](roadmap.md#m6-fluent-hierarchy-configuration). This local-gate exception does not assert that remote CI ran.

For M7 only, the owner directed a green CI designation and milestone completion when its scoped local code and documentation gates pass. The authorization and observed checks, including M7's fuzz and benchmark evidence, are recorded in the [M7 roadmap entry](roadmap.md#m7-shared-plans-precomputed-resolution-and-benchmarks). This local-gate exception does not assert that remote CI ran. Other milestones retain their CI gates.

Select formatting files and test packages explicitly from maintained project sources. Exclude `.ceiling`, archives, prototypes, and unrelated material from discovery, formatting, vet, tests, race checks, fuzzing, and benchmarks. Avoid recursive `gofmt -l .` or unfiltered filesystem globs. For the planned root package, after confirming source exists, the scoped gates are:

```sh
gofmt -l ./*.go
go vet .
go test .
go test -race .
```

Formatting passes only if the command succeeds and reports no files; an empty source selection or tool failure is not success. Extend the explicit selections when maintained library subpackages are added. The package test suite must explicitly run its maintained negative-compilation and negative-vet fixtures: testdata is skipped by ordinary package discovery. Include those fixture files in the maintained formatting selection without treating intentionally failing fixtures as ordinary packages. External consumer tests and compile-only API fixtures belong to the package suite. A milestone is complete only when its focused tests, relevant regressions, and applicable gates pass. These are CI requirements.

Run bounded fuzz campaigns in CI for the resolution fuzz target, and compile/run maintained usage examples when milestone 9 adds them. The root-package fuzz command is `go test . -run '^$' -fuzz '^FuzzResolution$' -fuzztime=5s -parallel=1`. Root-package benchmarks use `go test -run '^$' -bench . -benchmem -count=5 .`; apply the same source/package exclusions when extending these gates.

CI tests the go.mod minimum and the latest available patch in the same minor series. The current minimum is Go 1.27.1; the additional matrix entry is 1.27.x. Changing the minimum requires an explicit compatibility decision. Remote CI configuration is not proof that its jobs have run.

For documentation/layout-only changes, check local links, document ownership, obsolete paths, and consistency with the actual tree. Sandbox runs may set GOCACHE to a writable temporary directory. Report environment failures separately from implementation failures.

## Benchmark evidence

Compare ordinary interface dispatch, dynamic lookup, and precomputed lookup at depths 1/3/10 with the same bodies and consumed results. Include inherited root methods, sparse overrides, parent calls to another operation, and public-to-private delegation. Vary method count and eventually component count; use mixed receivers as well as monomorphic cases to expose devirtualization effects.

Measure hierarchy validation, Plan preparation, builder assembly, first lazy use, warm Plan reuse, optional conversion, and per-object checks/binding separately from steady-state invocation. Distinguish eager startup cost from lazy first-use cost; do not count revalidation on every object as warm reuse. Record ns/op, B/op, allocs/op, Go version, architecture, and setup; distinguish shared-plan memory from per-object storage. Benchmarks provide evidence, not arbitrary acceptance thresholds. No arbitrary speed or allocation thresholds until measured baselines justify them.
