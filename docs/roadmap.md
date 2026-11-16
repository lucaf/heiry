# Implementation roadmap

This document tracks implementation of the approved design. [AGENTS.md](../AGENTS.md) governs development; the [specification](specification.md) defines behavior, [architecture](architecture.md) defines responsibilities, and [testing](testing.md) owns implementation order and acceptance criteria. [ADRs](decisions/) preserve rationale. The roadmap does not replace any of them or authorize a new API.

Implementation has not started. No milestone below claims working library code or completed verification.

## Order and dependencies

Work through M1–M9 sequentially, completing each gate before starting the next milestone. This mapping preserves the [existing TDD sequence](testing.md#gated-production-milestones):

| Roadmap | Testing stages | Relationship |
| --- | --- | --- |
| M1 | 1, then 2 | Prove two-level dispatch before three-level semantics |
| M2 | 3 | Bind actual embedded state and scopes |
| M3 | 4, then 5 | Establish contracts before structural/ownership validation |
| M4–M6 | 6 | Resolve the schema API, then implement dynamic Plan/lifecycle foundations and fluent configuration |
| M7 | 7 | Add concurrent lazy preparation, caching behavior, and precomputation |
| M8 | 8 | Extend the proven mechanism to multiple providers |
| M9 | 9 | Integrate, document, and demonstrate the implemented package |

Precomputation precedes multiple providers because testing.md already requires that order. M8 extends the equivalence tests and benchmarks to components. M5 delivers explicit reuse of an already prepared dynamic Plan; M7 extends that machinery with concurrent lazy preparation, caching behavior, and optimization. These subdivisions do not allow skipping the testing plan's internal gates or dropping later release requirements.

## Approval and completion gates

The receiver-free schema API in [architecture](architecture.md#receiver-free-schema-declaration-api) requires explicit owner approval before implementation in M4. Analysis and recommendations are allowed; adopting proposed signatures is not. [ADR 0006](decisions/0006-receiver-free-schema.md) is an unapproved proposal, not authorization.

Approval also precedes implementation of other unresolved architectural choices: concrete grouped/fluent API decisions affecting M4/M6, the per-instance provider-pointer integration described under [safe reflection](architecture.md#state-binding-and-safe-reflection) affecting M5/M8, and any observable cache failure/retry policy that requires a decision in M7. Do not infer approval from illustrative syntax, feasibility analysis, or this roadmap. Unspecified internal mechanisms are not additional roadmap requirements.

Every implementation milestone follows **Red → Green → Refactor** as required by AGENTS.md. Mark it Completed only when all of these are evidenced:

- [ ] Focused acceptance tests pass after an observed, meaningful Red result.
- [ ] Existing regression tests remain green.
- [ ] Applicable [static, test, race, and CI gates](testing.md#quality-gates-and-ci) pass on actual maintained source.
- [ ] Documentation accurately describes the delivered behavior and remaining later milestones.
- [ ] Required owner decisions are recorded and no unresolved blocker affects the delivered functionality.

Code existence, documentation alone, empty package runs, and tests of a separate engine do not satisfy completion. Examples wait until M9; earlier development uses the actual package and its test fixtures, without an alternative implementation or unapproved public API.

## M1. Minimal inheritance and typed dispatch kernel

- **Status:** Not started.
- **Objective:** Establish the dynamic inheritance mechanism with ordinary typed calls.
- **Scope:** Testing stages 1 → 2; [S2](specification.md#s2-native-calls-and-dispatch-boundary), [S7](specification.md#s7-resolution-and-precomputation), and relevant S9 diagnostics.
- **Deliverables:** Root-package dynamic resolver, focused two-level tests, then A → B → C semantic tests using internal fixtures.
- **Dependencies:** None; no builder or schema API is needed to begin.
- **Acceptance criteria:** Stage 1 passes before adding stage 2. Cover signatures, variadics/results, application errors, missing parent versus unknown capability, skipped levels, nested/different-method parent calls, and finite recursion/reentry. Apply the relevant [required matrix](testing.md#required-matrix) and [dispatch rationale](decisions/0004-dispatch-and-parent-semantics.md).

## M2. Nested embedding and instance scopes

- **Status:** Not started.
- **Objective:** Make inherited bodies operate on the actual subobjects of one leaf.
- **Scope:** Testing stage 3; [S5](specification.md#s5-state-structure-and-providers) and [S8](specification.md#s8-visibility-lifecycle-and-concurrency), initially with one node provider per level.
- **Deliverables:** Actual embedded receiver bindings and fixed per-receiver scopes exercised through package tests.
- **Dependencies:** M1.
- **Acceptance criteria:** Stage 3 and the matrix's ownership/reentrancy cases prove pointer identity, shared embedded state, independent objects, uninitialized-scope errors, and concurrent invocation with suitable application bodies. Preserve [nested state](decisions/0001-nested-embedding.md); no public initialization API is selected by these internal fixtures.

## M3. Public/private contracts and hierarchy validation

- **Status:** Not started.
- **Objective:** Validate both contracts and the physical hierarchy before public configuration is introduced.
- **Scope:** Testing stages 4 → 5; [S3](specification.md#s3-contracts-and-target-validation), [S4](specification.md#s4-implementation-ownership-formal-trust-boundary), S5, S6 definition checks, S8 visibility, and S9 errors.
- **Deliverables:** Contract and structural validators, deterministic diagnostics, public-wrapper fixtures, and trust-boundary characterization tests. Use internal definition metadata while the public declaration API remains open.
- **Dependencies:** M2; stage 4 acceptance precedes stage 5 work.
- **Acceptance criteria:** The [required matrix](testing.md#required-matrix) covers contract extension/collisions, target-specific coverage, abstract intermediate levels, signature mismatches, duplicates, invalid structure/ownership, and no partial writes. External consumer and negative compilation cases verify the [public boundary](decisions/0003-public-private-contracts.md). Apply relevant reflection cases now and extend them to components in M8.

## M4. Reusable schema declaration API — owner approval required

- **Status:** Blocked — public API implementation awaits owner approval; no implementation has started.
- **Objective:** Obtain an approved way to declare a reusable hierarchy without a concrete receiver, then implement only that approved declaration surface.
- **Scope:** The API gate within testing stage 6; [S1](specification.md#s1-scope-and-authoring-costs), [S6](specification.md#s6-validation-plan-preparation-and-reuse), and the [open architecture decision](architecture.md#receiver-free-schema-declaration-api).
- **Deliverables:** A reviewable comparison and owner decision reference; after approval, tested declaration support and external-package compilation fixtures. No success-returning lifecycle placeholders.
- **Dependencies:** M3 and explicit approval of the proposed API/architecture before its code is written.
- **Acceptance criteria:** The approved design demonstrates one definition for eager/lazy preparation, grouped and individual registrations, independent objects, and the intended fluent lifecycle. Multiple-provider compatibility is a design review requirement here: show how the API accommodates several providers at one level. Implementing component ownership validation, binding, and dispatch belongs to M8, not M4. M4's implemented single-provider declaration subset passes the stage 6 compilation/validation cases without binding receivers. Later lifecycle work remains explicitly assigned to M5–M7.

## M5. Immutable dynamic plans and instance binding

- **Status:** Not started.
- **Objective:** Separate validation, reusable metadata, and each object's initialization.
- **Scope:** Dynamic lifecycle foundations of testing stage 6; [S6](specification.md#s6-validation-plan-preparation-and-reuse), S8, S9, and the [lifecycle states](architecture.md#state-machines).
- **Deliverables:** Immutable dynamic Plans, sequential eager/first-use preparation from the approved declaration, explicit reuse of an already prepared Plan, per-object checks, and scope installation after all checks succeed. Application constructors retain native signatures. Concurrent lazy preparation and cache behavior are M7 deliverables.
- **Dependencies:** M4; approval of any unresolved initialization API choice before implementing the affected path.
- **Acceptance criteria:** Explicitly reuse the same already prepared Plan to initialize independent objects without repeating definition validation or metadata construction; each object still receives its own checks and bindings. Applicable [preparation/reuse cases](testing.md#preparation-and-reuse-acceptance) and lifecycle rows also prove no receiver retention, failed setup preserving holders, repeat initialization rejection, and copying diagnostics. These tests establish the reusable foundation that M7 extends, not a separate cache implementation.

## M6. Fluent hierarchy configuration

- **Status:** Not started.
- **Objective:** Present the tested declaration and dynamic lifecycle through readable, chainable configuration.
- **Scope:** Complete testing stage 6; [S1 fluent configuration](specification.md#fluent-configuration), S3 per-level contracts, S6/S8 lifecycle, and [builder responsibilities](architecture.md#fluent-builder).
- **Deliverables:** The approved fluent adapter, individual/grouped registration support, stable level handles, and public API fixtures. Exact names and signatures follow approved decisions, not the illustrative chain.
- **Dependencies:** M5 and approval of any still-open grouped/fluent API choice.
- **Acceptance criteria:** The dynamic subset of [fluent acceptance](testing.md#fluent-configuration-acceptance) passes: chain/split-call equivalence, contract extension, correct level attachment, freezing, error/Must behavior, and eager/lazy reuse. Distinguish compiler-checked typed registration from validation-time grouped checks while preserving typed invocation. Precomputed chaining is completed in M7.

## M7. Shared plans, precomputed resolution, and benchmarks

- **Status:** Not started.
- **Objective:** Extend M5's reusable Plan and instance-binding machinery with concurrent lazy preparation, caching behavior, and the optional equivalent lookup optimization.
- **Scope:** Testing stage 7; [S6](specification.md#s6-validation-plan-preparation-and-reuse), [S7](specification.md#s7-resolution-and-precomputation), and S8 concurrency.
- **Deliverables:** Cache behavior with verified complete-definition identity, concurrent lazy preparation/publication, immutable precomputed Plans, precomputed fluent lifecycle coverage, and benchmark results. Reuse M5's preparation and binding machinery rather than implementing a second Plan-reuse or initialization path.
- **Dependencies:** M6 and a proven dynamic oracle; owner resolution of any architectural policy needed by the delivered behavior.
- **Acceptance criteria:** Complete [preparation/reuse acceptance](testing.md#preparation-and-reuse-acceptance), the applicable two-mode matrix, [exhaustive/fuzz checks](testing.md#exhaustivepropertyfuzz-tests), and [benchmark evidence](testing.md#benchmark-evidence). No receiver leakage, partial publication, live mode switching, or dynamic scan fallback. Record measured results without inventing performance thresholds or requiring a global cache.

## M8. Multiple implementation providers per level

- **Status:** Not started.
- **Objective:** Extend the validated mechanism to A1/A2 components at one inheritance level.
- **Scope:** Testing stage 8; [S5](specification.md#s5-state-structure-and-providers), S1 grouped registration, S7 parent semantics, and [safe-reflection analysis](architecture.md#state-binding-and-safe-reflection).
- **Deliverables:** Owned component bindings, same-level registration validation, component scope installation, and extensions to both-mode tests and benchmarks.
- **Dependencies:** M7 and approval of unresolved provider-pointer integration before affected implementation. The documented feasibility analysis is not experimental proof or permission to invent an accessor.
- **Acceptance criteria:** Stage 8 and [ownership/reflection cases](testing.md#required-matrix) cover exported/unexported components, foreign pointers, parent-subtree exclusion, duplicates across registration styles, shared providers for distinct operations, and forbidden cross-level reuse. Parent calls must not search sideways. Both modes preserve actual state and owning-level scopes without `unsafe` or reflective invocation.

## M9. Integration, documentation, usage examples, and release readiness

- **Status:** Not started.
- **Objective:** Demonstrate and verify the complete supported package for consumers.
- **Scope:** Testing stage 9 and all first-release requirements in S1–S9.
- **Deliverables:** End-to-end/external-consumer coverage, maintained package-usage examples, accurate API/limitation documentation, integrated CI gates, and a release-readiness evidence record.
- **Dependencies:** M1–M8 Completed; no unresolved decision affecting supported functionality.
- **Acceptance criteria:** The complete [test matrix](testing.md#required-matrix), negative fixtures, race checks, fuzz campaign, and [CI gates](testing.md#quality-gates-and-ci) pass on the prescribed toolchains. Examples import the actual package and compile/run in CI. Documentation covers approved registration, both preparation strategies and resolution modes, ownership, visibility, and copying limitations; benchmark evidence includes components. Empty package runs and archived material are excluded from verification.

## Maintenance and unresolved items

Use only **Not started**, **In progress**, **Blocked**, or **Completed** for milestone status. When work advances, update that milestone with completed deliverables, observed Red/Green and check evidence, and remaining blockers. Link the owner decision that clears an approval gate. Keep only one milestone In progress; a blocker does not authorize skipping a dependency or changing scope.

Scope, approved requirements, and architectural decisions change only with owner approval. Record approved changes in their authoritative documents and then align this roadmap. Routine status/evidence updates do not change requirements. Follow AGENTS.md for new conflicts rather than deciding them through roadmap edits.

No contradiction requiring a scope change was found in the authoritative documents. The schema API, concrete grouped/fluent surface, and unexported-provider integration remain open decisions, already identified above. Error caching/retry behavior is unspecified; the roadmap selects no policy. ADR 0005's former mandatory-startup policy is explicitly superseded, and ADR 0006 remains unapproved history.
