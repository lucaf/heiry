# ADR 0003 — Contracts and public wrappers

Status: accepted; rationale reconstructed from the design discussion.

Decision: public and inheritance-internal contracts extend independently. The public object is a dedicated wrapper whose dynamic method set contains only public operations.

Alternatives considered:

- Returning a leaf under a smaller interface: rejected because callers can still assert its private capabilities.
- Package-private methods as protected access: rejected because Go package visibility is not inheritance visibility.
- Reflection-based private invocation: rejected because it loses ordinary typed calls.
- Automatic forwarder generation: deferred; the initial wrapper is handwritten.

Consequence: public forwarding is a real authoring cost, and leaked implementation pointers remain accessible. [Specification S3](../specification.md#s3-contracts-and-target-validation) and S8 govern behavior.
