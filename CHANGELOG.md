# Changelog

This file lists what each release of heiry contains. Versions follow [semantic versioning](https://semver.org/); before `v1.0.0`, any release may change the API.

## Unreleased

- **heirygen checks parent fields and component paths:** every generated file with a level after the root, or with a component, gains compile-time probes. The build fails when a level's parent field, or the first field of a component's path, is reached only through another embedded struct, for example because the specification leaves out a level. Regenerate them: until then, `heirygen -check` reports each of them that `v0.1.0` wrote as out of date, although it still compiles.
- **heirygen can leave out the Plan helper:** `omit=InitPlan` on the `hierarchy` line leaves out `HInitPlan`.
- **heirygen can share levels and components between specifications:** a `chain` line holds a named run of levels, and `from=` on a `hierarchy` or `chain` line starts its levels with those of a chain or another hierarchy of the package. A `part` line holds a named group of `component` lines, and `use` adds them to a level, where lines with one path merge into one slot. The output holds the hierarchy written out in full, with a freshness assertion for each constant it takes lines from, so an edit to a shared constant or part fails every output that takes its lines until it is regenerated. Each generated file now imports exactly what its code uses, and a name bound to two paths in one file is refused naming both bindings, also for specifications without the new lines. When a shared line has a mistake, the message names every hierarchy that takes the line, so it shows how far the fix reaches. Specifications without the new lines generate the same output as before, apart from the probes above. Two hierarchies with one name are now reported among the first checks, with the same message, so before problems that later checks find.
- **heirygen compares the leaf with its last level as types:** a leaf spelled otherwise than its last level, through two import names of one package, local aliases, other spacing in an alias, or parentheses, is accepted. A wrapper is generated for a parenthesized leaf too, also when the parentheses are on an alias's right side or around the struct the leaf is declared as. A leaf that is still refused gets the reason. Specifications that generated before generate the same output, apart from the probes above. For a specification with several problems, a leaf mismatch is now reported after the problems found while the files are read, so another problem may be reported first.
- **Clearer MissingMethods message:** the level that validation reports is followed by `(the target; no level has a valid registration)`.

## v0.1.0 — 2026-11-21

First release.

- **Typed inheritance:** single inheritance through value embedding, with virtual and parent calls made as ordinary typed method calls through `Method`, `ParentMethod`, and Node's `Virtual` and `Parent`.
- **Several providers per level:** a level's operations can come from components stored inside it.
- **Checked setup:** a `Definition` validates the whole hierarchy once, when it is prepared, and `Init` checks every object it binds.
- **Plans:** reusable declarations, immutable plans, concurrent lazy preparation, and precomputed resolution that costs one table probe per lookup.
- **Errors:** structured `*Error` values with a code, whose message is one line naming the target, level, path, and types involved.
- **heirygen:** an optional command that writes the operation interfaces, the declaration with its registrations, the references every object needs, and the public wrapper with its forwarders, from one specification string per hierarchy, through the same public API. It runs on Linux and macOS, including WSL2 on the Linux file system.
- **Documentation:** a user guide, a generation guide, a comparison with alternatives, and five runnable examples.

It needs Go 1.27.1 or later, as `go.mod` declares. The known limitations are listed in the [README](README.md#limitations).
