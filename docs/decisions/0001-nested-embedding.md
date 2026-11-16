# ADR 0001 — Nested embedded state

Status: accepted; rationale reconstructed from the design discussion.

Decision: inherited state lives in one leaf object containing value-embedded ancestors. Providers operate on its actual subobjects.

Alternatives considered:

- Separate ancestor allocations: rejected because mutations would target duplicated state and promoted fields would no longer describe one object.
- Pointer-embedded ancestors: deferred because nils and aliasing complicate ownership.
- Sibling bases grouped under one label: rejected for the initial single-inheritance model.

Consequence: initialized owner-bearing values must not be copied. A noCopy marker is advisory. This record explains [specification S5](../specification.md#s5-state-structure-and-providers), not an additional rule source.
