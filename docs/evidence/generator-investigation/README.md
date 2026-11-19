# Generator investigation evidence

Commands and raw outputs behind the [code-generation proposal](../../proposals/code-generation.md) and the [roadmap record](../../roadmap.md#deferred-investigations). The first pass was recorded on 2026-11-18. The second, which extended the prototype to operation interfaces and public wrappers, and the fixes that followed the owner's review of the prototype were recorded on 2026-11-19. All of it was recorded on the `generator-investigation` branch. Nothing in this directory is an approved design or a maintained gate.

**Historical.** On 2026-11-19 the owner approved production implementation as milestone [G1](../../roadmap.md#g1-optional-code-generator) under [ADR 0013](../../decisions/0013-optional-code-generator.md). The maintained command in `cmd/heirygen` superseded the prototype, and the probe source under `testdata/genprobe/` was removed. That source was never committed, so the blob ids below identify it but the repository cannot restore it. Its reviewed regressions, freshness cases, and workloads were migrated into `cmd/heirygen/internal/generator` and its fixtures; the [G1 evidence](../g1-generator/README.md) lists what was migrated, changed, and dropped. The paths and commands below describe the investigation as it was run and no longer work. The outputs recorded here remain as the investigation's evidence.

## Environment and source identities

- Go 1.27.1, darwin/arm64 (Darwin 25.5.0).
- Library source: commit `a726da8b9eead9d1f032ec4eda39baef79c93db3`, unchanged by the investigation. The probe files were not committed when the outputs were recorded, so they are identified by Git blob ids (`git hash-object`) as of the final run:

| File | Blob |
| --- | --- |
| `testdata/genprobe/heirygen/main.go` | `518eb52e6aa0` |
| `testdata/genprobe/heirygen/spec.go` | `0a19672064f0` |
| `testdata/genprobe/heirygen/emit.go` | `d5cd45845f76` |
| `testdata/genprobe/heirygen/contracts.go` | `3e991dfd39cc` |
| `testdata/genprobe/typeprobe/typeprobe.go` | `2c98bb6fdc4b` |
| `testdata/genprobe/basic/heirygen_main.go` | `cdbb12e279e7` |
| `testdata/genprobe/components/heirygen_main.go` | `f16ef0831b07` |
| `testdata/genprobe/matrix/heirygen_matrix.go` | `b7b391ac5087` |
| `testdata/genprobe/basicfull/heirygen_main.go` | `3b1d1629820a` |
| `testdata/genprobe/componentsfull/heirygen_main.go` | `3e455717815d` |

## Where the probe source lived

All probe source was under `testdata/genprobe/` in the repository, so the go tool does not pick it up through `./...` and it is not part of the library's gates.

| Directory | Contents |
| --- | --- |
| `heirygen/` | The prototype generator (1,554 lines, standard library only) and its tests (2,580 lines). The tests build throwaway modules outside the repository that import the actual heiry package through a `replace` directive, then edit and break them. |
| `basic/`, `components/` | The two shipped examples as input of the first prototype: declaration and initialization generated; operation interfaces and wrapper handwritten. |
| `basicfull/`, `componentsfull/` | The same examples as input of the extended prototype: operation interfaces and the public wrapper generated as well. |
| `matrix/` | A first-prototype fixture for the parts of the supported model a generator could narrow: several definitions over one leaf, a standalone ancestor, same-type slots, a provider type reused at two levels, nested, anonymous, unexported and generic components, an alias-named parent, a contract that grows at a later level, and an imported operation interface. |
| `typeprobe/` | Probes of the standard-library type checker: information available while generated names are missing, and declared versus promoted methods. Its fixtures under `typeprobe/testdata/` do not compile on purpose. |
| `measure/` | The size measurement behind the proposal's figures. Its tables name every declaration it counts as boilerplate. |

## Reproducing, at the time

These commands ran from the repository root before the prototype was removed. They no longer run.

```sh
go vet ./testdata/genprobe/...
go test ./testdata/genprobe/... -count=1 -v
go test -race ./testdata/genprobe/basic ./testdata/genprobe/components ./testdata/genprobe/matrix ./testdata/genprobe/basicfull ./testdata/genprobe/componentsfull -count=1
go run ./testdata/genprobe/measure
for w in basic components basicfull componentsfull; do diff <(go run ./examples/${w%full}) <(go run ./testdata/genprobe/$w); done
go run ./testdata/genprobe/heirygen -check testdata/genprobe/basicfull
(cd testdata/genprobe/basicfull && go generate .)
```

No normal project gate runs these. The go tool skips `testdata` directories, so the root package commands and any `./...` pattern leave the prototype out, and the CI workflow names its packages and formatting files explicitly without naming `testdata/genprobe`.

## Recorded outputs

| File | Command | Result |
| --- | --- | --- |
| `go-test-genprobe.txt` | `go test ./testdata/genprobe/... -count=1 -v` | Exit 0. Seven test packages passed; 69 tests and 65 subtests, none failed. The log also holds the exact diagnostics compared in the initialization tests. |
| `go-test-race-workloads.txt` | The race command above | Exit 0 for the five workload and fixture packages. |
| `measure.txt` | `go run ./testdata/genprobe/measure` | Whole-program and boilerplate sizes of the three versions of each example, generated sizes, and every line counted as boilerplate. |
| `dependencies-and-binary.txt` | Shown in the file; first pass | The prototype packages depend on the standard library only, which was confirmed again for the extended generator. The built basic workload of the first pass contains neither the spec text nor the freshness function. |
| `library-gates.txt` | Shown in the file | Refreshed after the review fixes: the workflow's formatting selection and the probe sources are clean; `go vet` and `go test -count=1` pass for the root package and the four examples; the documentation checker and its seven tests pass; `git diff --check` and a trailing-whitespace search of the new files report nothing. The race, fuzz, and benchmark gates of the library were not rerun: no library source changed. |
| `gofmt-comment-spec.txt` | `gofmt` on a two-comment sample; first pass | gofmt rewrites a comment-block spec that is the doc comment of a declaration: the directive line moves to the end and indented lines are reformatted. A detached block is left alone. |

All four `diff` commands printed nothing: each rewritten workload prints exactly what its shipped example prints.

## What was observed failing first

These are the prototype's Red results, in order. Each was followed by the change named and a passing run. Everything else in `go-test-genprobe.txt` passed on its first run and is acceptance coverage or a recorded observation, not Red evidence.

First pass:

| # | Failing observation | Change that made it pass |
| --- | --- | --- |
| 1 | `go vet ./testdata/genprobe/basic`: `undefined: hierarchyInit`, with no generated file and no generator | A minimal generator for node registrations and a lazy helper |
| 2 | Helper tests: `undefined: hierarchyInitPlan`, `undefined: hierarchyPlanRefs` | Plan-based helpers |
| 3 | `TestNilLeafIsReportedByTheLibrary`: nil pointer dereference inside the generated `hierarchyRefs` | References built from typed nil pointers when the leaf is nil; superseded by 10 |
| 4 | Components workload: `unknown keyword "component"`, then undefined generated names | Component lines and `ComponentProvider` references |
| 5 | `TestEditedSpecConstantFailsCompilation`: `go build succeeded` on an edited spec | The compile-time freshness assertion |
| 6 | `TestCheckMode`: exit 2, flag unknown | Check mode and orphan handling |
| 7 | `TestGeneratedNamesDoNotCaptureApplicationIdentifiers`: `heiry already declared through import of package` | Selection of import, parameter, local, and constant names |
| 8 | `TestGeneratedNameCollisionsAreReported`: exit 0 and a written file | Collision reporting before writing |
| 9 | `TestGeneratedNamesAcrossFiles`: another file's generated variable clashed with this file's import name | Names chosen against every generated name of the package |

Second pass:

| # | Failing observation | Change that made it pass |
| --- | --- | --- |
| 10 | `TestNilObjectIsReportedAsInitReportsIt`: the helper returned `InvalidProvider` with level 0, path `limited.base`, expected and actual `*base`, where `Init` with a nil object and well-formed references returns `InvalidProvider` with the target only | References into a scratch value for a nil object, so `Init` reaches its own check of the object |
| 11 | `TestForeignPlanIsRejected`: `hierarchyInitPlan` with a Plan of another definition that has the same slots returned no error | References created by the helper's own definition; `hierarchyPlanRefs` removed |
| 12 | Extended workloads: `spec line 2: the first line must be: hierarchy <Name> <LeafType>` for a spec that starts with `operations` | The `operations`, `existing`, and `wrapper` lines, contract reading, and their emission |
| 13 | `TestStaleOperationsAndWrapper`: `go build succeeded` for five of six cases: an operation moved to the private contract, a public operation removed, a private operation added, a signature changed, and a wrapper method named after a private operation | The contract equality assertion and the private-name absence guard |

After the owner's review of 2026-11-19. The owner read the prototype without running it; each finding was first reproduced:

| # | Failing observation | Change that made it pass |
| --- | --- | --- |
| 14 | `TestWrapperNeedsBothContractsListed`: the generator exited 0 for a wrapper whose private contract no `operations` line lists. A separate one-off reproduction confirmed the hole: after adding a private operation and a wrapper method of its name, `go build` succeeded with no output. | A wrapper requires both leaf contracts to be listed; its guard takes the private names from the listed contract |
| 15 | `TestFreshnessAssertionCannotBeDisarmed`: with `const` changed to `var`, `go build succeeded` on a stale spec. Found in the re-audit and observed in the same test: with `false` redeclared by the package, fresh output failed with `duplicate key true`, so stale output would have passed. | The comparison is bound to a generated constant; the map literal names neither `false` nor `bool` |
| 16 | `TestSignaturesThatDifferOnlyInSpelling`: `method Read is written as (y text) string here and as (x string) string` for two identical signatures | The generator compares only the number of parameters and results and leaves type identity to the compiler's equality assertion |
| 17 | `TestForwardersWithNamedResults`: `w redeclared in this block` in a generated forwarder | Forwarders drop result names |

Passed on first run after those fixes, as acceptance coverage of the re-audit: a package that redeclares `nil` makes the generated file stop compiling, so it cannot disarm the other guards; a signature that uses a dot-imported name is copied and fails to compile, which documents an unsupported form.

Corrections to tests that are not Red evidence:

- First pass: one test named a hierarchy after an import of the handwritten file, which Go itself forbids; two expected values in the type-checker probe were wrong as first written.
- Second pass: after the generated helpers changed form, the expected generated names, two spec messages, and three compiler messages in existing tests were updated to the new text; one expected compiler line was too specific, because the compiler listed other undefined names first; one check counted an interface declaration by text that gofmt aligns differently.
- After the review: one expected program output had the wrong spacing; the expected forwarder text in one existing test was updated because result names are no longer copied.

## Not verified at the time

- The Go 1.27.x matrix entry and remote CI were not run.
- Cross-platform cases were type-checked with `go vet` under `GOOS=linux`, `GOOS=windows`, and `GOOS=darwin`, and executed only on darwin/arm64.
- No hierarchy larger than the fixtures was measured.
- The cost of the definition's lock in the Plan helper was not measured.
- No independent review had been done since the review fixes. The re-audit for further guard gaps was done by the prototype's author.
