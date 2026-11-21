# Changelog

This file lists what each release of heiry contains. Versions follow [semantic versioning](https://semver.org/); before `v1.0.0`, any release may change the API.

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
