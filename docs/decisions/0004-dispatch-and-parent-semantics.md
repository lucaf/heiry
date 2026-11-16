# ADR 0004 — Explicit virtual and parent resolution

Status: accepted; rationale reconstructed from the design discussion.

Decision: virtual calls resolve from the leaf; parent calls resolve strictly below the executing provider's fixed level. Ordinary concrete calls retain Go selector behavior.

Alternatives considered:

- A mutable global, goroutine-local, or object-local current level: rejected because nested and overlapping calls can corrupt it.
- Registering promoted methods automatically: rejected because presence does not establish ownership.
- Sideways delegation between providers: rejected as a definition of parent dispatch.

Consequence: the dynamic backward search provides a simple oracle for later optimizations. Missing parents and unknown capabilities remain different errors. See [specification S7](../specification.md#s7-resolution-and-precomputation).
