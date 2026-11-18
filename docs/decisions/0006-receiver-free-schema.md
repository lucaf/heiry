# ADR 0006 — Minimal schema declarations

Status: unapproved proposal; withdrawn as the selected implementation design. Retained as design history.

Proposal: Define[Leaf] would combine Level[Owner, Public, Private] descriptors. At would name a value-field path relative to the leaf. Bind[Operation] would take a **typed nil pointer** as a provider-type witness and immediately reduce it to type metadata. No live provider would be stored or called; binding a nonnil object in this declaration API would be an error. These signatures were chosen without the project owner's approval and are not current API requirements.

Alternatives recorded by the proposal, not owner-approved decisions:

- Live instances or dummy objects: rejected because declarations must not require allocation or constructors.
- Untyped reflection descriptors alone: rejected because provider/interface compatibility should be checked by the compiler.
- Application closures selecting providers: deferred because closures may capture objects and expand the ownership trust boundary.
- A fluent schema builder: the proposal deferred it in favor of variadic descriptors. That deferral conflicted with the owner's design goal and has been removed from the authoritative documents.

The proposal also assumed one provider per level and selected Schema.Validate and Plan.Init as the subsequent lifecycle surface. The single-provider release restriction has been removed: multiple providers are required by [specification S5](../specification.md#s5-state-structure-and-providers). These proposed lifecycle signatures were not adopted; [ADR 0008](0008-dynamic-plan-lifecycle.md) records the later approved dynamic Plan and node-binding API. Later component references were approved in [ADR 0011](0011-component-provider-references.md); the current signatures are in [architecture](../architecture.md#receiver-free-schema-declaration-api).

The current status of the declaration API is in [architecture](../architecture.md#receiver-free-schema-declaration-api). This record does not override the specification or authorize restoring the deleted implementation.
