# AGENTS.md — heiry

Implement heiry, a Go library for single inheritance with nested embedded state and typed method resolution.

## Authoritative documents

Follow these documents according to their distinct responsibilities:

- [docs/specification.md](docs/specification.md): **what** the library must do.
- [docs/architecture.md](docs/architecture.md): **how** the library is designed.
- [docs/testing.md](docs/testing.md): implementation order, tests, and acceptance criteria.

This file contains the rules Codex must follow. Put each requirement in its authoritative document and reference it elsewhere instead of duplicating it. Keep authoritative requirements in the four established documents. Record significant architectural decisions and rejected alternatives in `docs/decisions/`. ADRs explain decisions; they do not override the specification.

## Examples and project boundaries

`examples/` is only for code demonstrating usage of the implemented heiry package through its public API. It must import the actual package. Do not place prototype engines, alternative implementations, implementation guidance, or copies of package internals there, including under an example's internal directory.

Do not create an examples directory or placeholder examples before the implementation order permits them. Implementation guidance belongs in architecture.md; development tests belong with the actual package or its test fixtures. Do not create another directory to preserve a competing prototype implementation.

## Changes and reporting

Do not silently remove, weaken, reinterpret, or replace design decisions made by the project owner. Implementation details may evolve, but established architectural principles and functional requirements are not yours to discard. If there is a genuine technical obstacle or a proposed design change would be beneficial, explain the issue and request the project owner's approval **before making the change**. Do not present an unapproved proposal as an accepted decision.

Keep the four authoritative documents consistent without repeating their requirements. Update the document that owns an approved behavior or design change. Continue independent work only within the authorized scope.

Run the applicable gates in docs/testing.md. Report implemented behavior, tests added, observed Red/Green results, check results, and remaining limitations. Do not claim a milestone complete without its acceptance evidence or broaden the task's scope silently.

## Execution Protocol

### One milestone per task

- Work on **one roadmap milestone at a time**.
- Do not start the next milestone without explicit owner authorization.
- Do not implement features belonging to later milestones, even if convenient.
- Follow the TDD stages and acceptance criteria defined in `docs/testing.md`.

### Milestone completion

If acceptance criteria cannot be satisfied, mark the milestone as Blocked, 
document the reason, and stop. Do not mark it Completed, bypass the failing 
criteria, or proceed to another milestone.

Before declaring a milestone complete:

1. Run all applicable tests, static checks, and regression tests.
2. Verify every acceptance criterion for that milestone.
3. Update the milestone status in `docs/roadmap.md`.
4. Report briefly:
   - What was implemented.
   - Tests performed and their results.
   - Known limitations or unresolved issues.
   - Any architectural decisions requiring approval.

**Stop after reporting. Do not proceed automatically.**

### Architectural decisions

- Codex may choose internal implementation details consistent with the approved architecture.
- Changes to public APIs, inheritance semantics, or approved architectural decisions require explicit owner approval.
- If a technical obstacle requires such a change, stop and present alternatives before proceeding.
- Do not interpret permission to implement a milestone as permission to redesign it.

### Independent review

For milestones involving significant architectural choices, concurrency, reflection, or public API design, the owner may request an independent review.

Codex must address review findings before the milestone is considered complete.

Do not automatically change approved requirements in response to a reviewer's suggestions.

### Development discipline

Use strict **Red → Green → Refactor** for every feature or behavioral fix:

1. Identify the relevant requirement and current milestone in the authoritative documents.
2. Write a focused failing test and run it. Confirm that it fails for the expected reason.
3. Implement the simplest correct change that makes it pass.
4. Run the focused test and relevant regression tests.
5. Refactor while keeping tests green.

Follow the milestone order and acceptance gates in `docs/testing.md` and [docs/roadmap.md](docs/roadmap.md).

- Keep changes limited to the active milestone.
- Do not implement several major features before testing them.
- Do not weaken tests to accommodate incorrect implementations.
- Do not perform unrelated refactoring or introduce speculative abstractions for future milestones.
- Environment failures are not behavioral Red, and tests added around existing code are not retroactive TDD.
- Respect owner-approval gates. The roadmap never authorizes changes to the specification or approved architecture.

For documentation-only changes, verify consistency and links rather than inventing failing code tests.

**Never claim a test or check passed unless it was actually executed.**
