# How heiry was built

heiry explores a technique. It also explores a way of building a library: the project was conducted through Claude (Anthropic) and Codex (OpenAI). This page describes the method and where its records are. It states no rules; the documents it links do.

## The method

- **Specification first.** The required behavior and the design were written down before the first code, in separate documents.
- **One milestone at a time.** The work was split into milestones. The first nine were done in a fixed order, each with its own tests and acceptance criteria. Each later milestone was authorized by the owner on its own, as its roadmap entry records.
- **Test first.** A change in behavior started from a test that was run and seen to fail. Then the code changed until the test passed. The records say when a test only covered behavior that already existed.
- **Recorded evidence.** Each milestone's roadmap entry records what changed and how it was checked, with the results. Several entries also list what stayed open. For some later milestones, raw outputs and measurements are also kept as files.
- **Independent reviews.** Several milestones were reviewed independently. The records list the findings and how each was handled.
- **Owner approval.** Design decisions, changes to the public API and changes to approved requirements needed the project owner's approval. Significant decisions are written down, most with the alternatives that were rejected.

## Where the records are

| Record | What it holds |
| --- | --- |
| [AGENTS.md](../AGENTS.md) | The rules Codex must follow: which documents are authoritative, how a milestone is run and reported, and what needs the owner's approval. |
| [Specification](specification.md) | What the library and its optional generator must do. |
| [Architecture](architecture.md) | How they are designed. |
| [Testing plan](testing.md) | The implementation order, the required tests, the acceptance criteria and the quality gates. |
| [Roadmap](roadmap.md) | Each milestone's status, scope, checks and verification evidence. |
| [Decisions](decisions/) | Architecture decision records: approved choices, their reasons, rejected alternatives, and design history. |
| [Evidence](evidence/) | Raw outputs behind some later roadmap entries and the generator proposal: test runs, checks and measurements. |
| [Proposals](proposals/) | Investigations made before a decision, kept as history after they were superseded. |
