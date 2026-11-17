# ADR 0005 — Receiver-free plans

Status: receiver-free ownership principle retained; the mandatory-startup policy is superseded by the owner's clarification permitting eager and lazy preparation.

Decision: hierarchy validation establishes definition validity; Plan preparation produces reusable, immutable, receiver-free metadata; initialization binds actual subobject pointers separately for each object. Eager startup and lazy first-use preparation are both supported. Precomputation remains optional and converts a Plan without mutating it or existing instances. Behavioral authority is [specification S6](../specification.md#s6-validation-plan-preparation-and-reuse).

Alternatives considered:

- Lazy first-use validation was previously rejected because errors could occur after startup. That timing restriction conflicts with the owner's clarification and is superseded: eager preparation remains available for startup failure reporting, while lazy preparation reports invalid definitions when first needed.
- Validating a dummy object: rejected because it couples schema description to instances and constructors.
- Sharing bound methods/provider pointers: rejected because independent objects would share state accidentally.
- Mutating live plans: rejected to reduce lifecycle and synchronization complexity. An earlier version also rejected offering conversion through both Plan and a prepared handle; that API restriction cannot exclude a fluent step delegating to the same immutable Plan conversion, as described in [architecture](../architecture.md#fluent-builder).

Consequence: preparation timing does not change inheritance semantics or instance ownership. This record did not select public signatures; the dynamic preparation and node-binding signatures were later approved in [ADR 0008](0008-dynamic-plan-lifecycle.md). Cache implementation remains open.
