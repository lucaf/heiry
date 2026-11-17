# ADR 0002 — Explicit operation registrations

Status: accepted principle; updated to restore the owner's grouped-registration design goal. The exact grouped syntax was later approved in [ADR 0009](0009-grouped-fluent-registration.md).

Decision: a canonical single-method interface identifies an operation. Bind[T] preserves compiler verification that the declared provider satisfies it. Grouped registration, in the spirit of AddBinds(), remains a design goal alongside individual typed registration. Both describe explicit per-operation bindings. Registration asserts ownership; it does not prove a directly declared body. The functional requirements and type-checking boundary belong to [specification S1](../specification.md#s1-scope-and-authoring-costs) and S4.

Alternatives considered:

- Inferring overrides from reflection method sets: rejected because promoted methods appear in those sets.
- Source/symbol heuristics: rejected as unreliable and unnecessary.
- An earlier revision rejected token-only registration and removed grouped registration with it. That exclusion was too broad: grouping is an authoring goal, while compile-time versus validation-time checking is a separate API design question. At that stage, the name and signature were not fixed and no token-based replacement had been approved; [ADR 0009](0009-grouped-fluent-registration.md) later approved the `OperationToken`/`Of[T]`/`BindGroup` form.
- Resolving an aggregate interface: rejected because different methods can be supplied by different levels.

Consequence: grouped calls must feed the same validation and resolution machinery as individual calls. Bad ownership assertions, especially registered forwarders, can recurse. [ADR 0007](0007-fluent-receiver-free-declaration.md) approves typed-nil witnesses for individual declaration; [ADR 0009](0009-grouped-fluent-registration.md) approves the grouped form. The earlier design in [ADR 0006](0006-receiver-free-schema.md) remains unapproved.
