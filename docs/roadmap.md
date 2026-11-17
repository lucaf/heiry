# Implementation roadmap

This document tracks implementation of the approved design. [AGENTS.md](../AGENTS.md) governs development; the [specification](specification.md) defines behavior, [architecture](architecture.md) defines responsibilities, and [testing](testing.md) owns implementation order and acceptance criteria. [ADRs](decisions/) preserve rationale. The roadmap does not replace any of them or authorize a new API.

Each milestone's status and evidence are recorded below.

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

The owner approved M4's receiver-free declaration in [ADR 0007](decisions/0007-fluent-receiver-free-declaration.md) and M5's dynamic preparation and node binding in [ADR 0008](decisions/0008-dynamic-plan-lifecycle.md). [ADR 0006](decisions/0006-receiver-free-schema.md) remains unapproved history. Exact grouped, complete fluent-lifecycle, component-provider, and precomputation signatures remain open.

Approval also precedes implementation of other unresolved architectural choices: the concrete grouped-registration and complete lifecycle API decisions affecting M6, component-provider pointer integration described under [safe reflection](architecture.md#state-binding-and-safe-reflection) affecting M8, and any observable cache failure/retry policy that requires a decision in M7. The M5 node-reference API is approved in [ADR 0008](decisions/0008-dynamic-plan-lifecycle.md). Do not infer approval from illustrative syntax, feasibility analysis, or this roadmap. Unspecified internal mechanisms are not additional roadmap requirements.

Every implementation milestone follows **Red → Green → Refactor** as required by AGENTS.md. Mark it Completed only when all of these are evidenced:

- [ ] Focused acceptance tests pass after an observed, meaningful Red result.
- [ ] Existing regression tests remain green.
- [ ] Applicable [static, test, race, and CI gates](testing.md#quality-gates-and-ci) pass on actual maintained source, with the explicit owner-approved M2 and M4 local-gate exceptions recorded there.
- [ ] Documentation accurately describes the delivered behavior and remaining later milestones.
- [ ] Required owner decisions are recorded and no unresolved blocker affects the delivered functionality.

Code existence, documentation alone, empty package runs, and tests of a separate engine do not satisfy completion. Examples wait until M9; earlier development uses the actual package and its test fixtures, without an alternative implementation or unapproved public API.

## M1. Minimal inheritance and typed dispatch kernel

- **Status:** Completed.
- **Objective:** Establish the dynamic inheritance mechanism with ordinary typed calls.
- **Scope:** Testing stages 1 → 2; [S2](specification.md#s2-native-calls-and-dispatch-boundary), [S7](specification.md#s7-resolution-and-precomputation), and relevant S9 diagnostics.
- **Deliverables:** Root-package dynamic resolver, focused two-level tests, then A → B → C semantic tests using internal fixtures.
- **Dependencies:** None; no builder or schema API is needed to begin.
- **Acceptance criteria:** Stage 1 passes before adding stage 2. Cover signatures, variadics/results, application errors, missing parent versus unknown capability, skipped levels, nested/different-method parent calls, and finite recursion/reentry. Apply the relevant [required matrix](testing.md#required-matrix) and [dispatch rationale](decisions/0004-dispatch-and-parent-semantics.md).
- **Red/Green and acceptance:** Stage 1 tests failed to compile for the absent `Scope`/`Method` resolver, then passed after the dynamic backward-search kernel was added. Stage 1 passed before Stage 2 tests were added. Stage 2 passed immediately against that kernel and counts as acceptance coverage, not retroactive TDD. A later fixture-only Red exposed a missing different-operation parent call in the fixture; its correction is not resolver TDD evidence.
- **Local checks:** On Go 1.27.1, `gofmt -l ./*.go` produced no files; `go vet .`, `go test . -count=1`, and `go test -race . -count=1` passed with a writable `GOCACHE` after the default cache was denied by the sandbox.
- **CI evidence:** The owner reported on 2026-11-17 that CI ran without issue; no remote run was independently observed. The original completion gates remain in force.

## M2. Nested embedding and instance scopes

- **Status:** Completed — owner approved completion from passing local gates for M2 specifically.
- **Objective:** Make inherited bodies operate on the actual subobjects of one leaf.
- **Scope:** Testing stage 3; [S5](specification.md#s5-state-structure-and-providers) and [S8](specification.md#s8-visibility-lifecycle-and-concurrency), initially with one node provider per level.
- **Deliverables:** Actual embedded receiver bindings and fixed per-receiver scopes exercised through package tests.
- **Dependencies:** M1.
- **Acceptance criteria:** Stage 3 and the matrix's ownership/reentrancy cases prove pointer identity, shared embedded state, independent objects, uninitialized-scope errors, and concurrent invocation with suitable application bodies. Preserve [nested state](decisions/0001-nested-embedding.md); no public initialization API is selected by these internal fixtures.
- **Red/Green and acceptance:** `Node` gained a fixed scope and named noCopy marker; an internal binding path installs scopes on the actual A → B → C subobjects. Tests verify selected pointer identity, leaf-visible mutation, independent objects, typed `Uninitialized` panics for virtual/parent calls, recursive reentry, and concurrent read-only dispatch. Focused tests failed to compile first for the absent `Node`, then for absent `bindNodeLevels`/`nodeBinding`; each passed after the corresponding capability was added. These were library Red results, not fixture-only failures.
- **Local checks:** On Go 1.27.1, scoped formatting, `go vet .`, `go test . -count=1`, `go test -race . -count=1`, documentation-checker tests, and `scripts/check_docs.py` passed with writable `/tmp/heiry-go-cache` after the default cache was denied. Package tests include M1 regressions.
- **CI evidence and boundary:** The owner reported a successful M2 CI run on 2026-11-17; no remote run was independently observed. The [M2 local-gate exception](testing.md#quality-gates-and-ci) remains in place. The internal fixture path has no later structural/lifecycle validation or public initializer.

## M3. Public/private contracts and hierarchy validation

- **Status:** Completed.
- **Objective:** Validate both contracts and the physical hierarchy before public configuration is introduced.
- **Scope:** Testing stages 4 → 5; [S3](specification.md#s3-contracts-and-target-validation), [S4](specification.md#s4-implementation-ownership-formal-trust-boundary), S5, S6 definition checks, S8 visibility, and S9 errors.
- **Deliverables:** Contract and structural validators, deterministic diagnostics, public-wrapper fixtures, and trust-boundary characterization tests. Use internal definition metadata before the M4 public declaration layer.
- **Dependencies:** M2; stage 4 acceptance precedes stage 5 work.
- **Acceptance criteria:** The [required matrix](testing.md#required-matrix) covers contract extension/collisions, target-specific coverage, abstract intermediate levels, signature mismatches, duplicates, invalid structure/ownership, and no partial writes. External consumer and negative compilation cases verify the [public boundary](decisions/0003-public-private-contracts.md). Apply relevant reflection cases now and extend them to components in M8.
- **Local evidence:** Stage 4 introduced internal contract metadata and validation, including inherited/extended public and private interfaces, capability identity, declaration/signature checks, target-specific coverage, and sorted missing-method diagnostics. An internal wrapper fixture exercises public forwarding and typed private virtual/parent delegation; an external consumer and a negative compilation fixture check the public boundary. A promoted-method fixture confirms the S4 ownership trust boundary without invoking a potentially recursive body. The first focused Stage 4 Red failed to compile because the contract validator did not exist; focused tests passed after implementation. A later diagnostic-order test failed because map iteration selected `Write` before `Read`; sorting made it pass. Other Stage 4 cases added after that implementation passed immediately as acceptance coverage. Stage 5 added receiver-free internal structural checks for a named value-embedded chain, direct Node holders, and single-node registration types. Its checked per-object binding verifies exact embedded addresses and every holder before installing scopes. Tests cover invalid chains, nil/nonpointer/foreign providers, no partial writes, stable validated metadata, and restricted reflection values. The first focused Stage 5 Red failed to compile for the absent validators; a later test exposed a mutable binding slice in validated metadata, which was copied and then passed. The negative vet fixture confirms that an ordinary Node-containing copy is diagnosed. Internal metadata defines no public declaration API or reusable Plan.
- **Local gates:** On Go 1.27.1, scoped formatting (including testdata fixtures), `go vet .`, `go test . -count=1`, `go test -race . -count=1`, documentation-checker unit tests, and `scripts/check_docs.py` passed with writable `GOCACHE`; package tests run the positive/negative compilation and negative-vet fixtures. M1/M2 regressions remain green.
- **CI evidence:** The owner reported that CI ran successfully for M3; this is owner-reported evidence, not a run observed by Codex. M2's local-gate exception does not extend to M3.
- **Review follow-up:** Contract diagnostics returned through hierarchy validation now carry the leaf target and level path; each missing-method diagnostic carries its expected signature. Focused tests failed on the absent context and passed after the fix, including `errors.As` inspection of individual joined errors. An external consumer package now calls the initialized, unexported wrapper through a public interface, entering its typed resolver forwarder and checking that private methods remain absent; its negative compilation fixture still checks the intended diagnostic. The CI formatting command now includes every maintained `testdata` Go fixture. The exact scoped formatting, vet, unit, race, documentation, and documentation-checker commands passed locally after these changes. A remote CI run on this review follow-up was not independently observed.

## M4. Reusable schema declaration API

- **Status:** Completed — core declaration API approved in [ADR 0007](decisions/0007-fluent-receiver-free-declaration.md) and local acceptance gates passed under the owner's M4 local-gate direction.
- **Objective:** Obtain an approved way to declare a reusable hierarchy without a concrete receiver, then implement only that approved declaration surface.
- **Scope:** The API gate within testing stage 6; [S1](specification.md#s1-scope-and-authoring-costs), [S6](specification.md#s6-validation-plan-preparation-and-reuse), and the [declaration architecture](architecture.md#receiver-free-schema-declaration-api).
- **Deliverables:** A reviewable comparison and owner decision reference; after approval, tested declaration support and external-package compilation fixtures. No success-returning lifecycle placeholders.
- **Dependencies:** M3 and explicit approval of the proposed API/architecture before its code is written.
- **Acceptance criteria:** The approved design demonstrates one definition for eager/lazy preparation, grouped and individual registrations, independent objects, and the intended fluent lifecycle. Multiple-provider compatibility is a design review requirement here: show how the API accommodates several providers at one level. Implementing component ownership validation, binding, and dispatch belongs to M8, not M4. M4's implemented single-provider declaration subset passes the stage 6 compilation/validation cases without binding receivers. Later lifecycle work remains explicitly assigned to M5–M7.
- **Evidence:** `Definition[Leaf]` and stable concrete `LevelHandle[Leaf]` support chained and separate-statement declarations with generic typed `Bind[Operation]`. Definition validation checks exact leaf identity and delegates parent, Node, contract, registration, and coverage checks to the M3 validators. It does not freeze, prepare a Plan, or bind an object. Invalid fluent calls add no binding and retain only receiver-free structured diagnostics; validation combines them with safe further checks, including all missing methods when possible. Focused Red runs first failed to compile for absent declaration and binding methods, then passed after their implementations. A nonnil-witness test failed because a binding was added, an invalid-capability test failed for the same reason, and a malformed-path test failed for the same reason; each passed after the corresponding rejection was added. Chained/separate equivalence, alias identity, independent contract extension, safe validation, sticky errors, and external positive/negative compilation cases passed when added as acceptance coverage, not retroactive TDD. On Go 1.27.1, scoped formatting including all maintained fixture files, `go vet .`, `go test . -count=1`, `go test -race . -count=1`, six documentation-checker tests, and `scripts/check_docs.py` passed locally with a writable `GOCACHE`; the package suite includes maintained negative-compilation and negative-vet fixtures. M1–M3 regressions remain green.
- **CI status and evidence:** Green for the current M4 tree under the owner's direction to use passing scoped local code and documentation gates. This designation does not assert that remote CI ran on the current tree. The owner previously reported M4 CI green after the invalid-provider follow-up, before the latest validation fixes; no remote run of the current changes was independently observed. The [M4 local-gate exception](testing.md#quality-gates-and-ci) remains in place. Grouped registration, Plans, lifecycle, precomputation, and component providers remain unimplemented under their later milestones. Their public signatures remain unapproved.
- **Review follow-up:** A public-API regression exposed that a wrong node-provider type returned `InvalidProvider` before contract coverage could report missing methods. Definition validation now collects provider-type errors, excludes those registrations from coverage, and runs contract analysis when the structure is valid. The focused test failed with only `InvalidProvider`, then passed with that diagnostic plus typed `MissingMethods` entries for both uncovered operations. The owner-specified local gates were rerun after the fix; the subsequent CI-green report is recorded above.
- **Further review follow-ups:** Public-API tests reproduced duplicate Read and undeclared Extra registrations suppressing otherwise discoverable `MissingMethods` diagnostics. Each focused case failed with only its registration error, then passed after contract validation began collecting registration errors and completing safe coverage analysis. A subsequent A-duplicate/B-narrowing test failed because the later `ContractExtension` discarded the earlier `DuplicateBinding`; after contract-validation exits began joining accumulated diagnostics, both errors were reported without attempting coverage through the invalid contract. On Go 1.27.1, scoped formatting including fixtures, `go vet .`, `go test . -count=1`, `go test -race . -count=1`, six documentation-checker tests, and `scripts/check_docs.py` passed locally with writable `GOCACHE`; package tests include the compilation and vet fixtures. These local gates were rerun successfully on the current tree for the owner-directed green designation above.

## M5. Immutable dynamic plans and instance binding

- **Status:** Completed — the owner approved the public dynamic lifecycle and node-reference API in [ADR 0008](decisions/0008-dynamic-plan-lifecycle.md) and directed a green designation from passing local code and documentation gates for M5 specifically.
- **Objective:** Separate validation, reusable metadata, and each object's initialization.
- **Scope:** Dynamic lifecycle foundations of testing stage 6; [S6](specification.md#s6-validation-plan-preparation-and-reuse), S8, S9, and the [lifecycle states](architecture.md#state-machines).
- **Deliverables:** Immutable dynamic Plans, sequential eager/first-use preparation from the approved declaration, explicit reuse of an already prepared Plan, per-object checks, and scope installation after all checks succeed. Application constructors retain native signatures. Concurrent lazy preparation and cache behavior are M7 deliverables.
- **Dependencies:** M4; approval of any unresolved initialization API choice before implementing the affected path.
- **Acceptance criteria:** Explicitly reuse the same already prepared Plan to initialize independent objects without repeating definition validation or metadata construction; each object still receives its own checks and bindings. Applicable [preparation/reuse cases](testing.md#preparation-and-reuse-acceptance) and lifecycle rows also prove no receiver retention, failed setup preserving holders, repeat initialization rejection, and copying diagnostics. These tests establish the reusable foundation that M7 extends, not a separate cache implementation.
- **Implementation and Red/Green evidence:** The approved public `Prepare`/`MustPrepare`, `Init`/`MustInit`, and generic `NodeProvider` operations are implemented on the existing Definition/Plan surface. One validated snapshot supplies immutable receiver-free Plan metadata; each Init checks the leaf, reference provenance and slots, actual embedded addresses, and all Node holders before installing fixed per-object scopes. Saved handles freeze after preparation, rejected edits remain sticky on the definition, and the retained Plan remains independently usable. The focused preparation test first failed to compile for absent `Prepare`; freeze, binding, lazy Init, and typed nil-definition tests likewise observed focused Red failures before their fixes. A missing-reference test exposed absent slot context, and a post-freeze diagnostic test exposed missing capability context; both passed after correction. Other M5 tests passed on first run as acceptance coverage, including eager/lazy semantic equivalence, Plan reuse across independent objects, public forwarding from an external package, instance-failure isolation, concurrent use of one prepared Plan, Must error behavior, and absence of instance-bearing Plan fields.
- **Review follow-up:** A focused public-Plan test initialized only embedded B, then attempted complete C initialization. It failed because the typed `AlreadyInitialized` diagnostic identified level 1 but omitted B's provider path; after adding that path, it passed and confirmed all prior scopes were preserved. The nearby `MissingNode` exits also now include their Node paths. Additional integration tests cover eager/lazy private dispatch and visibility, application errors, invalid-definition diagnostics, shuffled references, and two valid definitions of one leaf type with different dispatch. These cases passed against existing behavior after a mistaken expected result in the last test was corrected; that fixture correction is not library Red evidence. The eager/lazy invalid-definition test now compares individual structured diagnostics instead of error text and passed on its first run after that improvement. The M5/M8 approval wording was narrowed to the still-open M8 component integration.
- **Local verification:** On Go 1.27.1, scoped formatting including maintained fixtures, `go vet .`, `go test . -count=1`, `go test -race . -count=1`, six documentation-checker tests, `scripts/check_docs.py`, and `git diff --check` passed after the review follow-up. The package suite exercises maintained negative-compilation and negative-vet fixtures. The focused eager/lazy equivalence test passed on first run as acceptance coverage; the full gates were rerun afterward and again after the review changes.
- **CI status and evidence:** Green for the current M5 tree under the owner's explicit [M5 local-gate direction](testing.md#quality-gates-and-ci), based on the passing scoped local code and documentation checks above. This designation does not assert that remote CI ran on this tree. No remote run was independently observed: the configured GitHub CLI token was invalid, `gh run list` could not reach `api.github.com`, and the SSH remote could not resolve `github.com`. The M5 exception does not extend to later milestones.

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

Open decisions: grouped and complete fluent-lifecycle signatures, component-provider integration, precomputation signatures, and concurrent failure caching/retry behavior. [ADR 0005](decisions/0005-immutable-plans.md) records the superseded mandatory-startup policy, and [ADR 0006](decisions/0006-receiver-free-schema.md) remains unapproved history.
