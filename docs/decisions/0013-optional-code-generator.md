# ADR 0013 — Optional code generator

Status: accepted by the project owner on 2026-11-19 for milestone G1, after the investigation recorded in the superseded [code-generation proposal](../proposals/code-generation.md).

Decision: the heiry module ships an optional command, `cmd/heirygen`, that reads specifications from marked Go string constants and writes ordinary Go source that uses only the existing public API. The command uses the standard library only, keeps its implementation in `cmd/heirygen/internal/generator`, and adds no library API. Its required behavior is [specification S10](../specification.md#s10-optional-code-generation); its design is the [generator section of the architecture](../architecture.md#optional-code-generator).

The input grammar is the investigation prototype's, corrected after the owner's review: `import`, `operations`, `existing`, `hierarchy`, `level`, `node`, `component`, and `wrapper`, with the prototype's defaults. A specification is only ever the value of a string constant documented by `//heiry:spec`; comment-only specifications are not supported. The output contains operation interfaces derived from listed handwritten contracts, receiver-free definitions with explicit `Level` parent fields and one compiler-checked `Bind[T]` per registration, reference lists and initialization helpers, public wrappers with forwarders, and the compile-time guards the investigation established.

The owner accepted this trust boundary: existing compiler and library checks remain mandatory; generated guards detect the demonstrated classes of stale input; correspondence between a specification and its emitted code also depends on the generator being correct; check mode detects output that differs from what the generator version being run would emit. Neither check mode nor tests are a compiler guarantee or a proof against generator defects. The acceptance does not permit dropping a guard or moving a compile-time check to run time.

One design point differs from the prototype. The prototype named its output `heirygen_<source>.go`. Go selects files for a platform by the part of the name before the first dot, after the first underscore, so that prefix changed which builds include the output for sources named like `linux.go` or `linux_amd64.go`. The output is named `<stem>.heirygen.go`, or `<stem>.heirygen_test.go` for a test file, which keeps the source's selection exactly. Ownership of an output is established by that name, a first-line marker, and the source file the marker names, not by a name pattern alone.

Later decisions, made by the owner on 2026-11-19 during G1 and recorded in the [roadmap](../roadmap.md#g1-optional-code-generator):

- Every output is put in place as a whole file. A new output is given its name with a hard link, which never replaces a file, so a file system without hard links makes the command fail before any output changes.
- A wrapper is generated only when the generator establishes the leaf's directly embedded holder; otherwise the wrapper is refused, and the hierarchy is still generated for a handwritten wrapper.
- The short window between checking an owned output and replacing or removing it is accepted. Nothing else may change the same generated files while the command runs.
- Lines that name the same interface must give it the same method name in the whole package's generation input, and the generator refuses a registration whose repeated guard it cannot show to compile in every build of its file.

Alternatives considered:

- Comment directives and a comment-block specification: rejected. Six of six stale-output cases compiled and kept their old behavior, because the compiler cannot see a comment.
- Specifications in external files: rejected. A file is not a constant, so no compile-time freshness assertion is possible.
- A type-checking generator: not adopted. Handwritten methods on a generated wrapper are declarations a type checker cannot resolve before the first generation. Reading contracts as syntax, with the compiler verifying the result through generated assertions, gives the same result without that restriction.
- Grouped registration in generated code: not adopted. `BindGroup` checks provider/interface satisfaction at validation; individual `Bind[T]` checks it at compilation.
- Typed virtual and parent accessors: excluded from this scope. A provider type can serve several slots, levels, and definitions with different operations.
- A version stamp in the output: not adopted. It would make the output depend on how the command was built. The command is in the library's module, so a consumer's module requirement selects the generator version together with the library version.
- A separate module for the command: not adopted. It would let the generator and library versions drift apart.
- Writing an output path directly when hard links are unavailable: rejected. A direct write can leave a partial output, or replace a file that appeared while the command ran.
- Closing the check-then-replace window with platform-specific calls, such as `renameat2` or `renamex_np`, or a lock every writer honors: not adopted. The standard library has no portable rename-if-unchanged or remove-if-unchanged.
- Generating a wrapper through a holder the generator cannot establish: rejected. A named or pointer `heiry.Node` field under the holder's name can dispatch on another object, or on a Node the library never installs.

Consequences: authors who use the command write a specification string and fewer declarations; authors who do not are unaffected. A limitation of the generator is never a restriction of the handwritten API. The generator, its fixtures, and freshness checks of committed generated output join the maintained gates in [testing](../testing.md).
