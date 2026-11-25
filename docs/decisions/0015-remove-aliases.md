# ADR 0015 — Remove the aliases

Status: accepted by the project owner on 2026-11-25. It supersedes the parts of [ADR 0012](0012-additive-authoring-conveniences.md) that added `BindNode`, `BindNodeGroup`, and the level-handle forwarders.

Decision: the exported API drops the forms that only repeat another one:

- `BindNode` and `BindNodeGroup`, which are `Bind` and `BindGroup` with the path `"."`.
- The ten `LevelHandle` methods that forwarded `Level`, `Extend`, `ExtendWith`, `Validate`, `Prepare`, `MustPrepare`, `Init`, `MustInit`, `NodeProvider`, and `ComponentProvider` to the definition, which `End` returns.

Everything else stays, including `Extend`, `ExtendWith`, `Node.Virtual`, `Node.Parent`, `Method`, `ParentMethod`, `Scope`, and `Node.Scope`. A test lists the exported types, functions, methods, constants, variables, struct fields, and interface methods, by name, and fails when one is added or removed, or when an exported type embeds a type, is an alias of an unexported type, or is defined over one.

Each removed form called a form that stays, with the same checks. ADR 0012's acceptance tests showed that the node forms record the same metadata and diagnostics as `Bind` and `BindGroup` with `"."`, and the forwarders reached the same definition. No compile-time check, run-time validation, ownership check, reference provenance, or diagnostic is lost. The cost is brevity: a chain writes `End()` after each level, before the next level or any other Definition call, and `Bind(w, ".")` instead of `BindNode(w)`.

Why: an outside review of `v0.2.0` found the API large for one idea. The aliases existed only because ADR 0012 approved the shorter forms as additions, never as replacements. The removal is a breaking change, so the next release raises the middle number.

Alternatives: keeping the forwarders and removing `End`, which keeps ten methods to save one call per level, and keeping the aliases as deprecated for one release, which keeps the API as large until then. Neither is selected.
