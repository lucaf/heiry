# G1 evidence: optional code generator

Raw outputs behind the [G1 roadmap entry](../../roadmap.md#g1-optional-code-generator). Recorded on 2026-11-19 on the `generator-investigation` branch, uncommitted, on top of commit `a726da8b9eead9d1f032ec4eda39baef79c93db3`. The library's own source is unchanged by G1.

## Environment

Go 1.27.1, darwin/arm64, Apple M2 with 8 cores, macOS (Darwin 25.5.0), with other work running: the load average was between 5 and 8 during the measurement.

## Files

| File | Contents |
| --- | --- |
| `closure/` | The documentation checks of the closure, with the documentation fingerprint, on the unchanged source of `private-probe-alias/`. |
| `private-probe-alias/` | The current verification of the code, recorded after the [private-name probe fix](#private-name-probe-and-contract-names), with the scripts that produced it. Each file starts with the time, the toolchain, the system, the commit and state of the tree, and a fingerprint of every file outside `docs/` taken before and after the run, with the command that recomputes it. |
| `probe-alias/` | The verification after the [holder probe fix for leaf names](#holder-probe-and-leaf-names), on the tree with fingerprint `6ecb399b…`, with the scripts that produced it. |
| `final-findings/` | The verification after the [final findings](#final-findings), on the tree with fingerprint `1f448444…`, with the scripts that produced it. Each file starts with the time, the toolchain, the system, the commit and state of the tree, and a fingerprint of every file outside `docs/` taken before and after the run, with the command that recomputes it. |
| `wrapper-guards/` | The verification after the [wrapper guards](#wrapper-guards-and-record-reconciliation), on the tree with fingerprint `35d3dc5a…`. |
| `declarations/` | The verification after the fix of [existing declarations across files](#existing-declarations-across-files), on `1a5a8132…`. |
| `alias-guard/` | The verification after the [alias registration guard](#alias-registration-guard), on `307085a5…`. |
| `final-review/` | The verification after the [final review fixes](#final-review-fixes), on `c107b093…`, which introduced these provenance headers. |
| `gates.txt` | An earlier local replay of the CI workflow's Go job by a script, not by GitHub Actions: every step except the resolution fuzz step, with its command, output, and exit status; all exited 0. It was last written during the case-only rename rollback round, not after the review follow-up as this table said before. It records neither the Go version nor the tree it ran on, which was uncommitted and has changed since, so it cannot be tied to a source state; its first lines now say so. |
| `go-test-generator.txt` | `go test -count=1 -v` over the command, its internal package, its three fixtures, and the generated example after the rollback fix: 101 tests and 101 subtests passed and none failed. One test was skipped, `TestOutputsThatDifferOnlyInCase`, because this volume is case-insensitive; it ran on the case-sensitive image. Like `gates.txt`, its tree was not recorded. |
| `go-test-case-sensitive.txt` | The file-handling tests run with their temporary directories on a temporary case-sensitive APFS disk image after the rollback fix: all passed, including `TestOutputsThatDifferOnlyInCase`; `TestInterruptedCaseOnlyRename` and `TestCaseOnlyRenameKeepsAFileThatAppears` were skipped there, because a case-only rename is two files on that file system. Its tree was not recorded. |
| `check-control.txt` | Negative control for the freshness step, recorded with the command after the review follow-up: on a scratch copy of the generated example with an edited specification, `heirygen -check` reports the output and exits 1. Its tree was not recorded. |
| `bench-initplan.txt` | One run of `go test ./cmd/heirygen/internal/generator/testdata/basic -run '^$' -bench InitPlanReferences -benchmem -count=5`. |

## Measurement of the Plan helper

`HInitPlan` creates its references through the Definition, so that a Plan of another definition is rejected; each creation takes the Definition's lock. Five-run medians per initialized object, allocation of the object included:

| Case | Generated helper | Plan-created references written inline |
| --- | --- | --- |
| Sequential | 922 ns, 1,680 B, 14 allocations | 804 ns, 1,456 B, 13 allocations |
| 8 goroutines | 624 ns | 518 ns |

The helper's extra allocation is the reference slice it returns. The measurement does not separate the lock's cost from that allocation, and single runs varied widely (sequential helper 851 to 1,326 ns; parallel inline 451 to 1,074 ns). It was taken once and is not a threshold.

## Observed Red, then Green

Each failure below was observed before the change named, and the test passed after it.

| # | Failing observation | Change |
| --- | --- | --- |
| 1 | `TestOutputNamesKeepPlatformSelection`: `linux.go` and `amd64.go` were built everywhere but their outputs `heirygen_linux.go` and `heirygen_amd64.go` only on linux or amd64; the output of `linux_amd64.go` (amd64 only) was missing on darwin/amd64; the output of `darwin_test.go` was darwin-only. | Outputs named `<stem>.heirygen.go` and `<stem>.heirygen_test.go` |
| 2 | `TestHelperNamesInTestFiles`: a hierarchy named `TestShape` in a test file was accepted, generating `TestShapeRefs`, which go test treats as a test with the wrong signature. Found while running test 1. | Such names are refused for specifications in test files |
| 3 | `TestOutputOwnership`: a handwritten `main.heirygen.go` was overwritten; a handwritten `stray.heirygen.go` was deleted as obsolete; a marker naming another source, a marker on a file of another name, and a prototype output were accepted; a directory at an output name surfaced only as a write error. `TestIgnoredFiles`: `.editor.go` was parsed. | Classification by output name, first-line marker, and source; files the go command ignores are skipped |
| 4 | `TestFailedWriteChangesNoOutput`: in a read-only directory, generation failed but had already replaced `a.heirygen.go`. | Temporary files for every output first, then renames, then removals |
| 5 | `TestExitStatuses`: an invalid specification and a missing directory exited 1, like stale output. `TestVersion`: `-version` was an unknown flag. | Exit status 3 for failures; `-version` |
| 6 | `TestDiagnosticPositions`: errors pointed at the string literal, and type errors always said line 1. | Positions of the offending specification line |
| 7 | `TestSignatureUsingADotImport`, changed from the investigation's version: generation succeeded and wrote a file that did not compile. | Refusal of signatures that may use dot-imported names |
| 8 | `TestUndeclaredSpecificationTypes`: undeclared unqualified types in a specification were accepted. | Refusal at the specification line |
| 9 | `TestMisspelledMarker` and `TestRepeatedOption`: `// heiry:spec` and a repeated option were silently accepted. | Diagnostics for both |

Passed on their first run, as acceptance coverage: `TestObsoleteOwnedOutputs`, `TestOutputPermissions`, `TestInjectedWriteFailures`, `TestCheckModeWritesNothing`, `TestVerboseAndCheckAfterGeneration`, `TestGoGenerateWorkflow`, `TestUnsupportedContracts`, `TestOutputNames`, `TestCommittedOutputsAreFresh`, `TestFixturesBehaveLikeTheHandwrittenExamples`, and the tests of the three fixtures and of `examples/generated`. The injected rename and remove failures were implemented together with change 4 and are recorded here as coverage, not as Red.

## Migration from the investigation prototype

The prototype under `testdata/genprobe/` was reviewed and ported into `cmd/heirygen/internal/generator`. Its tests were migrated first and passed against the port; that is migration evidence, not new Red. During review some diagnostics were reworded, and the migrated expectations were updated to the new text.

| Prototype material | Now |
| --- | --- |
| Generator source | Ported into `spec.go`, `contracts.go`, `generate.go`, `emit.go`, and `files.go`, without the experimental comment-specification mode |
| 55 of its 65 distinct test names | Migrated with their names into the internal package and the fixtures |
| `TestCommentSpecNeedsFlag` | `TestCommentSpecIsRefused`: the mode does not exist |
| `TestStaleOutputWithCommentSpecIsSilent` | Dropped with the mode; the finding remains in the investigation evidence |
| `TestStaleWrapperExposesAnOperationThatBecamePrivate` | Dropped: it used a handwritten stand-in for a wrapper, superseded by `TestStaleOperationsAndWrapper` and the mechanism tests |
| `TestWorkloadOutputsAreFresh` | `TestCommittedOutputsAreFresh`, over the maintained fixtures and the example |
| `TestNilObject`, `TestStandalonePlansWithComponents` of the first components workload | Covered by `TestOwnershipAndNilObject` and `TestGeneratedOperationsInBothModes` of the components fixture |
| The four type-checker probes and the size measurement tool | Removed: the production design never type-checks and has no origin policy, and the measurement compared investigation workloads. Their outputs remain in the [investigation evidence](../generator-investigation/README.md). |
| Workloads `basicfull`, `componentsfull`, `matrix` | Fixtures `testdata/basic`, `testdata/components`, `testdata/matrix`, regenerated by the production command; the helper tests of the first basic workload moved into the basic fixture |

Migrated cases whose expectation changed with a deliberate behavior change, not Red evidence: the bootstrap cases now write broken output that carries the marker, since an unmarked file is refused; the "another generator version" case edits the body under an unchanged marker, since an edited marker is refused; and after a public operation is removed, regeneration is now refused at the specification line where the prototype wrote a file that failed with `undefined: AuditOp`.

Other test corrections, not Red evidence: in-process fixtures declared none of the types their specifications named, and were completed once change 8 refused that; two expected line numbers in `TestUndeclaredSpecificationTypes` were one too low; `TestGoGenerateWorkflow` first assumed that `go tool` prints the command's exit status as `go run` does.

## Not verified

- Remote CI. The workflow file was checked to parse as YAML, and its steps were run locally by a script, most recently as recorded in `private-probe-alias/gates.txt`. The owner designated G1's CI green under a local-gate exception at the [closure](#closure); no remote run was observed.
- The Go 1.27.x matrix entry: only Go 1.27.1 was run.
- Linux and Windows execution: those platforms were type-checked with `go vet` and `go list` in the selection tests, and executed only on darwin/arm64.
- The resolution fuzz campaign and the library benchmarks were not rerun: no library source changed.

## Review fixes

After G1's first delivery the owner reviewed the production generator and reported defects in six areas. Each fix below was preceded by a test that failed as recorded, except where the table says otherwise.

### Observed Red, then Green

| # | Failing observation | Change |
| --- | --- | --- |
| 10 | `TestForwarderMustResolveTheRegisteredInterface`: generation succeeded for a public contract embedding a handwritten `Namer`, for an extension listed on its own `operations` line, and for an imported `fmt.Stringer` registration. The built programs panicked on the first wrapper call with `UnknownCapability main.NameOp`, `main.WideDescribeOp`, and `main.StringOp`. | Each forwarder must resolve the interface the hierarchy registers for its method; a mismatch is refused with the `existing` line that fixes it, and an alias spelling gets a compile-time identity assertion |
| 11 | `TestLockstepRenameOfAnExistingPrivateOperation`: after renaming a private method together with its `existing` interface and adding a wrapper method that exposed it, `go build` succeeded. | The wrapper's output compares both contracts with the recorded methods; the private-name probe embeds the private contract; each `existing` interface is tied to its one method |
| 12 | `TestEveryBuildOfAWrapperChecksBothContracts`: with the `operations` line in a test file or a tagged file, and with a populated or an initially empty private contract, `go build` succeeded after a private operation and a wrapper method of its name were added. | The contract guards are in the wrapper's own output |
| 13 | `TestWrapperIsUnexported`: `wrapper View`, `field=State`, `wrapper _`, and `field=_` were accepted; the exported wrapper generated and ran. | Refused, as S8 requires |
| 14 | `TestForwarderUsesTheEmbeddedHolder`: `holder=Aux`, a named Node field, and the default `holder=Node` beside an embedded alias `Dispatch` were accepted; both programs panicked with `Uninitialized` on the first call. | The holder is checked against the leaf's directly embedded `heiry.Node` |
| 15 | `TestCaseOnlySourceRename`, on case-insensitive APFS: after renaming `Main.go` to `main.go`, the new output was removed as the old one's obsolete leftover, leaving only `main.go`. | A case-only rename replaces and renames the same file and does not remove it |
| 16 | `TestCaseEquivalentHandwrittenDestination`: the handwritten `main.HEIRYGEN.go` was overwritten with the generated output. | Output names that differ only in case from another entry are refused |
| 17 | `TestDestinationCreatedAfterPlanning` and `TestOutputReplacedAfterPlanning`: a file created at a new output's path, or put in place of an owned output, after inspection was replaced or removed. | New outputs are hard-linked into place, which never replaces a file; replacements and removals first check that the file is the one read |
| 18 | `TestCRLFOutputs`: an owned output with CRLF line endings was refused as unrecognized. | The marker is recognized with CRLF, and outputs are compared with CRLF read as LF |
| 19 | `TestOutputsFollowLegacyAndCgoConstraints`: across its 8 configurations, `CGO_ENABLED` 0 and 1 on darwin/arm64 and linux/amd64, each without a tag and with the tag `special`, 24 outputs of legacy `+build` and cgo sources were selected where their sources were not, in 7 of the 8. An earlier version of this row said 16 configurations; the test and its recorded failure have 8. | The output's `//go:build` line is computed from the source header, with `cgo` for files that import `"C"` |

The full suite after change 10 also exposed a regression of its own: `TestEqualMethodNamesInTwoHierarchies` reported the new dispatch diagnostic instead of the name collision it tests. The dispatch check was moved after the name checks, and the test passed again.

### Passed on first run

As acceptance coverage: the five dispatch controls, which initialize the object and call every public operation through the wrapper, one through an alias registration; `TestIdentityAssertionCatchesAChangedAlias`; `TestExistingInterfaceStillIdentifiesItsMethod`; the three holder controls, which also passed before the fix; `TestSymlinkAtOutputPath`, after the test's own snapshot helper learned to record links; `TestOutputsThatDifferOnlyInCase` on the case-sensitive image; `TestParentFieldChangesAreReportedAtValidation`; and the injected case of a file system without hard links. The controls of tests 11, 12, and 19 passed before and after their fixes.

Test corrections that are not Red evidence: the injected publication failure was retargeted from a second rename to the link that now publishes a new output, and made to fail the exclusive-create fallback too; the hand-edit case now removes every guard function, because the new wrapper guards would otherwise still stop its build.

### Platform coverage

- Executed on darwin/arm64, on case-insensitive APFS (the default volume) and, for the file-handling tests, on a temporary case-sensitive APFS disk image.
- Selection checked with `go list`, without compiling, in two matrices. The cgo and tag matrix of `TestOutputsFollowLegacyAndCgoConstraints` has 8 configurations: darwin/arm64 and linux/amd64, each with `CGO_ENABLED` 0 and 1, and each without a tag and with the tag `special`. The platform-name matrix of `TestOutputNamesKeepPlatformSelection` lists and type-checks with `go vet` the platform-name cases for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, and windows/amd64, with the default cgo setting and no tag. An earlier version of this line merged the two.
- Not exercised: Linux or Windows execution, Windows file-system semantics for links and renames, and a real file system without hard links, which was simulated through the writer seam.

### Remaining decision, since decided

Replacing or removing an existing output checks first that it is still the file read, and then renames or removes it; between the two a short window remains in which another program could replace the file. Go's standard library has no portable rename-if-unchanged or remove-if-unchanged. New outputs have no such window. The owner accepted this window on 2026-11-19, with the requirement that the package directory stay stable while the command runs; see the follow-up below.

## Review follow-up

On 2026-11-19 the owner approved the narrow S10, architecture, and testing updates for the review fixes and decided three points: refuse a wrapper whose holder the generator cannot establish; keep whole-file publication, failing on a file system that cannot provide it; and accept the residual window for replacing or removing owned outputs, requiring a stable package directory. The follow-up fixed the remaining findings test-first.

### Observed Red, then Green

| # | Failing observation | Change |
| --- | --- | --- |
| 20 | `TestHolderOfALeafDeclaredAsATypeDefinition`: for a leaf declared as `type node record`, whose struct embeds the holder as `Dispatch` and has a named `Node *heiry.Node` pointing at another initialized object's holder, `wrapper view` was accepted. The generated forwarder `return w.state.Node.Virtual[ReadOp]().Read()` printed `second`, the other object's state. | The leaf is read through aliases, type definitions, and instantiations; the default holder is refused and `holder=Dispatch` named |
| 21 | `TestUnestablishedHolderRefusesTheWrapper`: wrappers were accepted for a holder aliased in another package, a leaf declared in another package, and a leaf embedding a pointer. | Refused with the reason and the instruction to write the wrapper by hand |
| 22 | `TestInjectedWriteFailures/a_file_system_without_hard_links`: with links unsupported, the fallback wrote the output path directly and generation succeeded. | Hard-link support is checked before anything is put in place, and the command fails with no output changed; the direct-write fallback is removed |
| 23 | `TestInterruptedCaseOnlyRename`, on case-insensitive APFS: with the second step of a case-only rename failing, the directory kept `Main.heirygen.go` holding `main.go`'s marker, and the rerun was refused as an unrecognized file. | The old output is moved aside, the new one linked into place, and the old one moved back if the link fails; the rerun completes, and the exact ownership rule is unchanged |
| 24 | `TestRegisteredExistingInterfacesAreCheckedWhereTheyAreUsed`: with the `operations` line in a test file or a tagged file, swapping the methods of the registered `Sealer` and `Wiper` left every contract and the specification unchanged, and `go build` succeeded. | A hierarchy's output repeats the exact-method guard for each `existing` interface it uses from another file |
| 25 | `TestOutputsFollowLegacyAndCgoConstraints`: 12 outputs were selected where their sources were not, for an ``import `C` `` written as a raw string and for headers starting with a UTF-8 byte order mark. | The import path is unquoted before comparing it with `C`, and a leading byte order mark is removed before reading the header, as go/build does |

The subtest `publishing_the_second_output` was adapted, not shown failing for a behavior: it now targets the second link, because the first link is the support check.

### Passed on first run

As acceptance coverage: the demonstration step of test 20, a handwritten forwarder through the named pointer that printed the other object's `second`, and its `holder=Dispatch` control; the handwritten-wrapper control of test 21, which generates the hierarchy without its `wrapper` line; the controls of test 24 in `go vet`, `go test`, and the tagged build; the exact-ownership assertion of test 23; and `TestEveryBuildOfAWrapperChecksBothContracts` with its assertion tightened to require `does not implement Private (missing method Wipe)` on a line of the wrapper's generated file. The committed outputs of the fixtures and the example did not change.

### Platform coverage

Executed on darwin/arm64: on the default case-insensitive APFS volume, and for the file-handling tests on a temporary case-sensitive APFS disk image. Selection was checked with `go list` in the same two matrices: the 8 cgo and tag configurations on darwin/arm64 and linux/amd64, and the platform-name cases on five platforms with the default cgo setting and no tag. A file system without hard links was simulated through the writer seam, not run. Linux and Windows execution were not done.

## Case-only rename rollback

### Observed Red, then Green

| # | Failing observation | Change |
| --- | --- | --- |
| 26 | `TestCaseOnlyRenameKeepsAFileThatAppears`, on case-insensitive APFS: a handwritten `main.heirygen.go` created after the old output `Main.heirygen.go` was moved aside, and before the new output was linked, was overwritten with the old generated output when the old output was renamed back, and the error said the file "was not replaced". | The old output is linked back, which never replaces a file. If its name is taken, the old output stays under its hidden name, and the error reports that path and the recovery step; when it is put back, the error says it was restored. |

The existing control `TestInterruptedCaseOnlyRename`, where the link fails for another reason, still restores the old output, and the next run succeeds; it now also requires the diagnostic `the old output Main.heirygen.go was restored`, which the old message did not contain, so it failed with the regression and passed after the change. In the new test, generation fails, the handwritten file keeps its contents, the old output is byte-identical at the reported hidden path, and following the reported recovery step, then running again, completes the rename.

### Platform skips

On the default case-insensitive APFS volume all file-handling tests ran except `TestOutputsThatDifferOnlyInCase`, which needs a case-sensitive file system. On the temporary case-sensitive APFS image that test ran and passed, and the two case-only rename tests were skipped, because the rename is two files there.

## Final review fixes

Three findings of the final review, fixed test-first in the generator only; the library and its API are unchanged. The raw Red output of the new tests is in [final-review/red-shared.txt](final-review/red-shared.txt) and [final-review/red-publish.txt](final-review/red-publish.txt), and the verification after the fixes in the other files of [final-review/](final-review/).

### Observed Red, then Green

| # | Failing observation | Change |
| --- | --- | --- |
| 27 | `TestSharedExistingInterfaceIsDeterministic/hierarchy_file_names_the_interface`: `main.go`, which holds the hierarchy, and `labels.go` both name `fmt.Stringer` as an existing interface. Repeated in-process generation gave different bytes: `main.heirygen.go` had no repeated guard after the first run and had one after run 3, because the declaration chosen depended on map order. | The operations of a package are kept in file name and line order. A hierarchy's output omits the repeated guard when an operations line of its own file names the interface |
| 28 | `TestSharedExistingInterfaceIsDeterministic/declarations_in_two_other_files`: with the hierarchy's file naming no existing interface, and `labels.go` and `titles.go` naming `fmt.Stringer` as `String() string` and as `String() (text string)` under the import name `std`, check mode reported `main.heirygen.go: differs from the expected output` before run 15 | The first declaration in file and line order, that of `labels.go`, is the representative whose signature the repeated guard copies |
| 29 | `TestConflictingSharedExistingIsDeterministic`: when two operations lines name `fmt.Stringer` for different methods, the first run reported `wrapper view forwards String through StringOp, but hierarchy records registers String as fmt.Stringer`, and run 2 succeeded without a diagnostic | The dispatch check takes the interface's method from the same order, so every run reports the same diagnostic |
| 30 | `TestObsoleteRemovalFailureReportsProgress`: when removing the second of two obsolete outputs failed, the error was only `removing obsolete d.heirygen.go: injected failure`, although both new outputs were in place and the first obsolete output was removed | The error lists the outputs already put in place and those already removed |
| 31 | `TestCleanupFailuresNameThePathsLeftBehind`: when the temporary file of a linked output or the hard-link probe could not be removed, generation succeeded and left the file silently; when a temporary file discarded after a failure could not be removed, the error did not name it; a partial temporary file was left behind unreported | Each hidden file the run cannot remove is reported with its path and the cause. It does not stop the run, and a run that otherwise succeeds fails with `every output is up to date, but heirygen could not remove the hidden files …` |
| 32 | `TestCaseOnlyRenameCleanupFailure`, on case-insensitive APFS: when the hidden copy of the old output could not be removed after the case-only rename succeeded, the run stopped and did not put the later output `z.heirygen.go` in place | The hidden copy is reported as left behind, and the run puts the remaining outputs in place |
| 33 | `TestCaseOnlyRenameKeepsPermissions`, on case-insensitive APFS: after renaming `Main.go` to `main.go`, the output whose old file had mode 0600 had mode 0644 | The output of a case-only rename takes the old output's permission bits |

Red for test 32 was observed twice: first on the old wording alone, then, after a later output was added to the test, on the stopped run. Red for test 33 was observed only on the case-insensitive volume, where the rename is one file; on the case-sensitive image, where the old output is removed and a new one created, the test ran only after the change, and passed.

### Passed on first run

As acceptance coverage: the `builds` subtest of test 27, which builds both packages with `go vet` without a tag, with `labels`, with `titles`, and with both, and runs the program; the existing restore control `TestInterruptedCaseOnlyRename` and the no-overwrite rollback `TestCaseOnlyRenameKeepsAFileThatAppears`, unchanged; and the existing injected publication failures, whose messages still contain the text they require.

Test correction that is not Red evidence: the `partial temporary file` case of test 31 first compared the files left behind, in name order, with the refused removals, in call order. With random temporary names it failed in the full run and passed in the focused one. It now compares sorted lists, and the new tests then passed 50 repetitions.

### Remaining map walks

Every other walk of a map in the generator was checked: it fills a set, copies one, or sorts its keys before use, as in `checkCase` and `changes`. None reaches a diagnostic or the output in map order.

### Verification record

The earlier `gates.txt` and the other undated records above could not be tied to the source they ran on, so this round's verification is recorded separately in `final-review/`, each file with its own provenance header. All runs used Go 1.27.1 on darwin/arm64 (Darwin 25.5.0), on the uncommitted tree on top of `a726da8`, whose fingerprint outside `docs/`, `c107b09330d15918d3ead9ef66dda70b40f5725c54225fbbee8d097649edfe7a`, was the same before and after each run.

| File | Result |
| --- | --- |
| `final-review/gates.txt` | Every step of the CI workflow's Go job except the resolution fuzz step, replayed locally by a script, not by GitHub Actions: all 8 steps exited 0, including the race tests and the freshness loop. The fuzz step was not run, because no library source changed. |
| `final-review/go-test-generator.txt` | `go test -count=1 -v` over the command, its internal package, its three fixtures, and the generated example: 107 tests and 108 subtests passed and none failed. One test was skipped, `TestOutputsThatDifferOnlyInCase`, because this volume is case-insensitive; it ran on the case-sensitive image. |
| `final-review/go-test-case-sensitive.txt` | The file-handling tests on a temporary case-sensitive APFS disk image: all passed, including `TestOutputsThatDifferOnlyInCase` and `TestCaseOnlyRenameKeepsPermissions`. Skipped there, because a case-only rename is two files on that file system: `TestInterruptedCaseOnlyRename`, `TestCaseOnlyRenameKeepsAFileThatAppears`, and `TestCaseOnlyRenameCleanupFailure`. |
| `final-review/check-control.txt` | The freshness negative control: `heirygen -check` on a scratch copy of the example with an edited specification reports `main.heirygen.go` and exits 1. |

The maintained outputs, the three fixtures and the generated example, were regenerated and did not change. Not exercised: Linux or Windows execution, and a real file system without hard links.

## Alias registration guard

A registration in the hierarchy's own file skipped its repeated guard whenever that file's `operations` line named the interface it resolved to, even when the registration spelled the interface differently. The guard of the `operations` line checks only its own spelling, so a local alias could later name another interface unchecked.

### Observed Red, then Green

| # | Failing observation | Change |
| --- | --- | --- |
| 34 | `TestAliasRegistrationsAreGuardedInTheirOwnFile`: the hierarchy's file has `existing Seal Sealer` and `existing Wipe Wiper` and registers them through `Sealing = Sealer` and `Wiping = Wiper`. The control passed: `go vet`, and the program printed `read <nil> <nil>`. After the aliases were swapped, with the contracts, the original interfaces, and the specification unchanged, `go build` of the old output succeeded. | A use is covered by the same file's guard only when it has the same spelling as that file's `existing` line; a use spelled otherwise gets its own guard |

After the change, the same swap fails to compile in `main.heirygen.go`, on the guards of the two registrations: `Wiper does not implement interface{Seal() error} (missing method Seal)` for `Sealing`, and `Sealer does not implement interface{Wipe() error} (missing method Wipe)` for `Wiping`. The registration of `Reader`, spelled as its `existing` line, is still guarded once, by that line, so the guard against duplicates is kept. Which declaration a use reuses is still decided by type, in file and line order, and the shared-interface determinism tests pass unchanged. Conflicting `existing` lines are handled as before.

The raw Red output is in [alias-guard/red-alias.txt](alias-guard/red-alias.txt). The first draft of the test called the aliases in its handwritten code, so the swap broke that code instead of reaching the gap; the calls were changed to the original interfaces before the Red was recorded. The recorded Red cites `guards_test.go:304`; after it, the test gained six lines, the assertions of the alias guards and the log of the compiler output, so the same call is now at line 310.

### Verification record

Recorded in `alias-guard/` with Go 1.27.1 on darwin/arm64, on the uncommitted tree on top of `a726da8`, whose fingerprint outside `docs/`, `307085a5bc0d2d00584cef55023386cf32f7143206fa2111465b3af5408d99c1`, was the same before and after each run. Each file starts with its provenance.

| File | Result |
| --- | --- |
| `alias-guard/focused.txt` | The focused regression passed, with the compiler's output after the swap. |
| `alias-guard/go-test-generator.txt` | `go test -count=1 -v` over the command, its internal package, its three fixtures, and the generated example: 108 tests and 108 subtests passed and none failed. One test was skipped, `TestOutputsThatDifferOnlyInCase`, because this volume is case-insensitive. |
| `alias-guard/freshness.txt` | `heirygen -check` reported the generated example and the three fixtures up to date. |

The maintained outputs were regenerated and did not change: none registers an existing interface of its own file under another spelling. The CI-equivalent gates, the race tests, and the case-sensitive file-handling pass were not rerun this round; their last record is in `final-review/`, whose tree fingerprint differs from this one.

## Existing declarations across files

Two defects in how `existing` declarations from several files combine, found by reading the code after the alias guard. On 2026-11-19 the owner approved a narrow S10 rule: in the package's generation input, lines that resolve to the same interface must give it the same method name. Both were fixed test-first; the raw Red output is in [declarations/red-declarations.txt](declarations/red-declarations.txt).

### Observed Red, then Green

| # | Failing observation | Change |
| --- | --- | --- |
| 35 | `TestContradictoryExistingLinesAreRefused`, with `existing Name fmt.Stringer` in `aaa_test.go`, or in `aaa.go` under `//go:build legacy`, sorting before `main.go`, whose wrapper forwards `String` through a generated `StringOp` while the hierarchy registers `fmt.Stringer`: generation succeeded, because the dispatch check took `Name` as `fmt.Stringer`'s method from the first line, and the ordinary build ran and panicked with `UnknownCapability: capability main.StringOp at level 0` from `view.String` in `main.heirygen.go`. | Lines that give one interface different methods are refused before any output is computed, with both positions: `main.go:24:2: recordsSpec, line 5: existing String fmt.Stringer names fmt.Stringer for method String, but aaa_test.go:9:2: namedSpec, line 4: existing Name fmt.Stringer names it for method Name`. The directory is unchanged. |
| 36 | `TestRepeatedGuardUsesASignatureTheBuildCompiles/hierarchy's_own_declaration`: with `audit_test.go` declaring `type status = error` and `Audited interface{ Seal() status }` with `existing Seal Sealer`, and `main.go` naming `Sealer` itself and registering it through the alias `Sealing`, the repeated guard copied the first declaration, so the ordinary build failed with `main.heirygen.go:98:26: undefined: status`. | The hierarchy's own declaration is preferred |
| 37 | `TestRepeatedGuardUsesASignatureTheBuildCompiles/declaration_in_a_file_every_build_compiles`: with no declaration in `main.go`, and `ops.go` declaring `Sealer` with `Seal() error`, the guard still copied `audit_test.go`'s, and the ordinary build failed with `main.heirygen.go:64:26: undefined: status`. | Otherwise the first declaration in file and line order whose unqualified names every build of the output has |
| 38 | `TestRepeatedGuardUsesASignatureTheBuildCompiles/no_declaration_the_build_can_use`: with `audit_test.go` the only declaration of `Sealer`, generation wrote a guard using `status`. | The registration is refused at its line, before anything is written: `main.go:38:2: recordsSpec, line 4: registration Sealing needs a guard in the output of main.go …, which uses status, declared only in audit_test.go: declare status in a non-test file without build constraints or a platform suffix, or name the interface with an existing line in main.go` |

In tests 36 and 37, after the change the ordinary build, `go vet`, and the program succeed, and swapping the aliases still fails to compile on the guard of `Sealing` in `main.heirygen.go`, with `does not implement interface{Seal() error} (missing method Seal)`.

### Passed on first run

As acceptance coverage: the `valid shared declarations` control of test 35, with every line naming `fmt.Stringer` for `String` in the same tagged layout, which generates, passes `go vet` with and without the tag, and runs; the earlier existing-interface tests, unchanged, including `TestSharedExistingInterfaceIsDeterministic`, whose shared declarations remain valid reuse; and `TestFileBuildComparison`, written after the code, which checks the platform each file name selects against what go/build reads and the comparison of builds. `TestConflictingSharedExistingIsDeterministic` now meets the new refusal, the same in every run; only its comment changed.

### Availability of a signature's names

A name the guard's signature uses counts as available in the output of the hierarchy's file when it is predeclared and no file of the package declares it, or when a file that declares it is the hierarchy's file, a non-test file without a condition, or a file with exactly the output's conditions; a test file counts only for a test output. Conditions are compared as written: the `//go:build` line the file's output would need, cgo included, and, for a name from which go/build reads a platform, the raw last two underscore-separated parts of the name. Conditions that are equivalent but spelled differently, or that differ but overlap, count as unavailable, so the rule can refuse a usable signature in such a layout, never accept an unusable one. Only unqualified names are checked. There is no constraint solver. An earlier version of this paragraph said "the same test status" and "read as go/build reads it"; the code accepts a non-test file for a test output, and compares the name's parts as written.

### Verification record

Recorded in `declarations/` with Go 1.27.1 on darwin/arm64, on the uncommitted tree on top of `a726da8`, whose fingerprint outside `docs/`, `1a5a8132b70ecf6ef94f5b449e3a5137a43cfa99ab7279552ea9b71d254bd7a6`, was the same before and after each run. Each file starts with its provenance.

| File | Result |
| --- | --- |
| `declarations/focused.txt` | The new tests and the earlier existing-interface tests passed. |
| `declarations/gates.txt` | Every step of the CI workflow's Go job except the resolution fuzz step, replayed locally by a script, not by GitHub Actions, once on this fingerprint: discovery, formatting, `go vet`, the package tests, the race tests, running the examples, and the freshness loop. All 8 steps exited 0. The fuzz step was not run, because no library source changed. |
| `declarations/check-control.txt` | The freshness negative control: `heirygen -check` on a scratch copy of the example with an edited specification reports `main.heirygen.go` and exits 1. |
| `declarations/docs-checks.txt` | The documentation checker, its tests, and `git diff --check`, run after the documentation was final. |

The maintained outputs were regenerated and did not change. The case-sensitive file-handling pass was not rerun, because publication did not change; its last record is in `final-review/`.

## Wrapper guards and record reconciliation

On 2026-11-19 the owner approved the pending architecture and user-guide wording, and a narrow S10 update documenting the conservative refusal when the generator cannot establish that a signature is available. Two review findings were fixed test-first, one was documented, and the verification tooling was corrected. The raw Red output is in [wrapper-guards/red-wrapper.txt](wrapper-guards/red-wrapper.txt).

### Observed Red, then Green

| # | Failing observation | Change |
| --- | --- | --- |
| 39 | `TestStaleHolderEdit`: a wrapper generated for `type node struct{ heiry.Node; label string }`, with the leaf then edited alone to embed `Dispatch`, an alias of `heiry.Node`, and to hold `Node *heiry.Node` pointed at a second initialized object's `Dispatch`. The old output compiled, and `(&view{state: first}).Read()` printed `second`, the other object's label. | Each wrapper's output asserts `var _ *heiry.Node = &leaf.Node` for its holder. The same edit now fails with `main.heirygen.go:86:22: cannot use &leaf.Node (value of type **heiry.Node) as *heiry.Node value in variable declaration` |
| 40 | `TestWrapperCopiesItsOwnContracts`: `ops_test.go` declares `type status = error` and `Audited interface{ Wipe() status }`, and lists `operations Audited Public Private` with `existing Read Reader` and `existing Wipe Wiper`; `main.go` declares `Private interface{ Wipe() error }` and has the hierarchy and `wrapper view`. The wrapper's private contract guard copied `Audited`'s spelling, so the ordinary build failed with `main.heirygen.go:107:26: undefined: status`, while `go vet`, which compiles the test file, passed. | Each contract's operations keep the one interface that identifies each method, for dispatch, and the contract's own signature and imports, which the wrapper's forwarders and contract guards copy |

After the change, test 39 also shows the documented limit: with the field turned into a named `heiry.Node` value, the old output still compiles unchanged, and check mode reports `the default holder=Node names a field that node does not embed; its directly embedded heiry.Node is Dispatch`. Test 40's ordinary build, `go vet`, and program succeed, and adding `Seal` to `Private` without regenerating fails on the wrapper's guard with `does not implement Private (missing method Seal)` in `main.heirygen.go`.

### Passed on first run

As acceptance coverage: `TestHolderGuardControls`, in which the guard accepts the embedded holder under its own name and under the alias `Dispatch` with `holder=Dispatch`, and the wrapper reads its own object. The three maintained outputs with a wrapper, the example and the `basic` and `components` fixtures, were regenerated; the example's output differs from its previous version only by the holder guard.

### Documented, not changed

The availability check covers only unqualified names. A copied qualified type such as `syscall.Handle` may not exist on a platform where its package does; the user guide says how to supply a signature every target build compiles, by giving the hierarchy's own file the `existing` line, or by keeping the hierarchy in a file built only where the type exists. Broader platform analysis is outside G1.

### Verification tooling

- The freshness step of the local replay already stopped at a failing `-check`, because each step runs under `bash -eo pipefail`, but the record did not show that, and the replay as a whole exited 0 even when a step failed. The replay now states its shell mode, writes `|| exit 1` after each `-check`, and exits 1 when any step fails. [wrapper-guards/freshness-negative-replay.txt](wrapper-guards/freshness-negative-replay.txt) runs only that step on a scratch copy with an edited specification: the step and the replay exit 1.
- The recorder ran its command without `pipefail`, so for a command piped through `sed`, which most recordings are, its "Exit status of the recorded command" line was `sed`'s. The recorded results of earlier rounds rest on their test output, which shows no failure, not on that line. The recorder now runs the command with `pipefail`.
- Both scripts are kept beside the records as [replay-gates.sh](wrapper-guards/replay-gates.sh) and [record.sh](wrapper-guards/record.sh). They contain no machine-specific path; the recorder finds the repository with `git rev-parse --show-toplevel`.

### Verification record

Recorded in `wrapper-guards/` with Go 1.27.1 on darwin/arm64, on the uncommitted tree on top of `a726da8`, whose fingerprint outside `docs/` was the same before and after each run. Each file starts with its provenance.

| File | Result |
| --- | --- |
| `wrapper-guards/focused.txt` | The new tests and the earlier holder, wrapper, and existing-interface tests passed. |
| `wrapper-guards/gates.txt` | Every step of the CI workflow's Go job except the resolution fuzz step, replayed once by `replay-gates.sh`, not by GitHub Actions: discovery, formatting, `go vet`, the package tests, the race tests, running the examples, and the freshness loop. All 8 steps exited 0, and the replay exited 0. The fuzz step was not run, because no library source changed. |
| `wrapper-guards/go-test-generator.txt` | The test inventory: `go test -count=1 -v` over the command, its internal package, its three fixtures, and the generated example. 114 tests and 116 subtests passed and none failed; `TestOutputsThatDifferOnlyInCase` was skipped, because this volume is case-insensitive. |
| `wrapper-guards/go-test-case-sensitive.txt` | The file-handling tests on a temporary case-sensitive APFS disk image: all passed, including `TestOutputsThatDifferOnlyInCase`; the three case-only rename tests that need a case-insensitive file system were skipped. |
| `wrapper-guards/check-control.txt` | The freshness negative control: `heirygen -check` on a scratch copy of the example with an edited specification reports `main.heirygen.go` and exits 1. |
| `wrapper-guards/freshness-negative-replay.txt` | The replay's own negative control, described above: the step and the replay exit 1. |
| `wrapper-guards/docs-checks.txt` | The documentation checker, its tests, `git diff --check`, and a scan for trailing whitespace in the untracked G1 files, run after the documentation was final, with a fingerprint of `docs/`. |

The fingerprint was `35d3dc5a1f475a5647818d53e7073258b57e62311b7bf9e03e5b52b9cf317304` throughout.

## Final findings

The owner asked for the remaining review findings to be checked and fixed without regressions, keeping the current structure. One guard gap was fixed test-first, several acceptance tests were tightened and shown to detect their guard's removal, and the documentation was corrected. The raw outputs are in [final-findings/](final-findings/).

### Observed Red, then Green

| # | Failing observation | Change |
| --- | --- | --- |
| 41 | `TestStalePromotedHolder`: a wrapper generated with `holder=Dispatch` for a leaf embedding `Dispatch`, an alias of `heiry.Node`. The leaf alone was then edited to embed `heiry.Node` directly and a `peer` carrying `Dispatch`, whose objects a second hierarchy initializes. With `*peer`, pointed at an initialized peer, the old output compiled and the wrapper printed `peer`; with `peer` embedded by value and initialized, it printed `embedded peer`; with `peer` two levels down, through `outer`, it also printed `embedded peer`. [red-promoted.txt](final-findings/red-promoted.txt) records the first two before any change, and [red-promoted-previous-guard.txt](final-findings/red-promoted-previous-guard.txt) all three against the previous guard. | A probe struct embeds `*L` beside a decoy field of the holder's name two levels down: one level down the selector is ambiguous, deeper it selects the decoy. The three edits now fail with `ambiguous selector probe.Dispatch`, twice, and `cannot use &probe.Dispatch (value of type *struct{}) as *heiry.Node value`, in `main.heirygen.go` |

The review proposed a keyed literal, `_ = node{Dispatch: heiry.Node{}}`. Tried first, it caught the pointer case, with `invalid implicit pointer indirection to reach Dispatch`, but with Go 1.27.1 the value case still compiled and printed `embedded peer`, so it was replaced by the probe. The two-level case was added then; its Red was observed by running the test against the previous guard, with `emit.go` restored byte for byte afterwards. The controls pass: the holder under its own name and under an alias, beside a named Node field, and of a leaf declared as a type definition.

### Acceptance tests tightened

Tests written around existing code are not Red evidence. To show that each tightened test detects its guard's removal, the guard was removed from `emit.go` for one run and restored byte for byte:

| Test | Change | With the guard removed |
| --- | --- | --- |
| `TestExistingInterfaceStillIdentifiesItsMethod` | The provider implements the grown `Sealer`, so `Bind[Sealer]` compiles, and the diagnostic of the existing-interface guard is required on a line of `main.heirygen.go` | Fails: `go build` succeeds ([mutation-existing.txt](final-findings/mutation-existing.txt)). Before the change, the test passed without the guard, on `Bind`'s error |
| `TestEveryBuildOfAWrapperChecksBothContracts` | Both the public and the private contract are changed, each with a wrapper method so that only the contract guard can notice, and every build of the case, `go build`, `go vet`, and `go test` or the tagged build, must fail on the guard's line in `main.heirygen.go` | All 8 subtests fail ([mutation-wrapper.txt](final-findings/mutation-wrapper.txt)) |

The diagnostics of `TestStaleOutputWithSpecConstant`, in `go build`, `go vet`, and `go test`, of both cases of `TestAliasChanges`, of the redeclared `false` in `TestFreshnessAssertionCannotBeDisarmed`, and of `TestContractThatGrowsAtALaterLevel` are now required on a line of the generated file; for the contract alias, the exact guard line, `Narrow does not implement interface{ReadOp; AuditOp} (missing method Audit)`. The constant-turned-variable case first builds the fresh output as its control. `TestConflictingSharedExistingIsDeterministic` asserts the refusal itself and that nothing is written, and `TestForeignPlanIsRejected` compares the helper's result for another definition's Plan with the direct call, code and context.

### Documentation

- ADR 0013 records the later decisions: whole-file publication with a mandatory hard link, refusing a wrapper whose holder cannot be established, the accepted check-then-replace window, the same-method rule, and the conservative availability refusal, with the alternatives they rejected.
- S10 and the user guide say that a method without an `existing` line needs `prefix=` or `suffix=`.
- The user guide says that an alias holder must be exported: an unexported one passed generation, and `Init` reported `MissingNode` in a scratch run. It also lists `go run ./examples/generated`, says "local alias" where it means one, and says that a run that cannot remove its hidden files exits with status 3.
- The user guide's troubleshooting tells the holder probe's diagnostics, `ambiguous selector probe.X` for the holder and `&probe.X (value of type *struct{})`, from the private-name probe's, which share the name `probe`.
- The evidence index lists every record set, and its "Not verified" list points to the current replay.
- The recorder's count of uncommitted paths now excludes `.DS_Store`, as its fingerprint already did.

### Verification record

Recorded in `final-findings/` with Go 1.27.1 on darwin/arm64, on the uncommitted tree on top of `a726da8`, whose fingerprint outside `docs/` was the same before and after each run.

| File | Result |
| --- | --- |
| `final-findings/focused.txt` | The tests this round added or tightened, and the holder, wrapper, and existing-interface tests around them, passed. |
| `final-findings/gates.txt` | Every step of the CI workflow's Go job except the resolution fuzz step, replayed once by `replay-gates.sh`, not by GitHub Actions: discovery, formatting, `go vet`, the package tests, the race tests, running the examples, and the freshness loop. All 8 steps exited 0, and the replay exited 0. The fuzz step was not run, because no library source changed. |
| `final-findings/go-test-generator.txt` | The test inventory: 115 tests and 123 subtests passed and none failed; `TestOutputsThatDifferOnlyInCase` was skipped, because this volume is case-insensitive. |
| `final-findings/go-test-case-sensitive.txt` | The file-handling tests on a temporary case-sensitive APFS disk image: all 17 that can run there passed, including `TestOutputsThatDifferOnlyInCase`; the three case-only rename tests that need a case-insensitive file system were skipped. |
| `final-findings/check-control.txt` | The freshness negative control: `heirygen -check` on a scratch copy of the example with an edited specification reports `main.heirygen.go` and exits 1. |
| `final-findings/freshness-negative-replay.txt` | The replay's own negative control: its freshness step on that scratch copy exits 1, and so does the replay. |
| `final-findings/docs-checks.txt` | The documentation checker, its tests, `git diff --check`, and a scan for trailing whitespace in the untracked G1 files, run after the documentation was final, with a fingerprint of `docs/`. |

The fingerprint was `1f448444c914d2e578aad90707cf44c4812641cd49d0ce3c659fee7562d54aa3` throughout. The three maintained outputs with a wrapper were regenerated for the holder probe; the fixtures' and example's tests pass on them.

## Holder probe and leaf names

Review of the final findings found that the holder probe broke a valid input that compiled before. The probe embedded `*L`, and an embedded field is named after its type, so for a leaf type named like its holder, such as `Node` with the default `holder=Node`, `probe.Node` selected that embedded field. The generated wrapper then failed to compile, regeneration gave the same failure, and nothing dispatched wrongly. Fixed test-first; the raw Red output is in [probe-alias/red-nodeleaf.txt](probe-alias/red-nodeleaf.txt).

### Observed Red, then Green

| # | Failing observation | Change |
| --- | --- | --- |
| 42 | `TestHolderGuardForALeafNamedLikeItsHolder`: for a leaf `type Node struct{ heiry.Node; label string }`, a generic leaf `Node[int]`, and a leaf alias `type Node = record`, each with `wrapper view` and the default holder, generation succeeded and `go vet` failed with `main.heirygen.go:93:22: cannot use &probe.Node (value of type **Node) as *heiry.Node value`, `**Node[int]` for the generic leaf. | The probe embeds the leaf through a local alias, `type subject = L`, whose name is chosen like the decoy's to collide with nothing in the file. All three now compile and print their own label, and when the edit leaves the holder promoted from an embedded `inner`, each fails with `ambiguous selector` in `main.heirygen.go` |

The earlier holder tests pass unchanged: the stale pointer field, the three promoted holders, the controls under the holder's own name and an alias, beside a named Node field, and for a leaf declared as a type definition. The three maintained outputs with a wrapper were regenerated, and differ only by the alias line.

### Noted, not changed

By reading the code, not executed: the private-name probe embeds the wrapper type and the private contract by their own names, so a private operation named exactly like the wrapper type or the private contract would select that embedded field instead of being checked. Such a name is unusual, and the same alias would close it; it was left for the owner to decide, because it is outside this finding. Corrected in the [next fix](#private-name-probe-and-contract-names): such an operation made the generated file fail to compile rather than go unchecked, and a collision with the wrapper's name cannot occur, because wrappers are unexported and operations are exported.

### Verification record

Recorded in `probe-alias/` with Go 1.27.1 on darwin/arm64, on the uncommitted tree on top of `a726da8`, whose fingerprint outside `docs/` was the same before and after each run.

| File | Result |
| --- | --- |
| `probe-alias/focused.txt` | The new regression and every holder and wrapper guard test passed. |
| `probe-alias/gates.txt` | Every step of the CI workflow's Go job except the resolution fuzz step, replayed once by `replay-gates.sh`, not by GitHub Actions: discovery, formatting, `go vet`, the package tests, the race tests, running the examples, and the freshness loop. All 8 steps exited 0, and the replay exited 0. The fuzz step was not run, because no library source changed. |
| `probe-alias/go-test-generator.txt` | The test inventory: 116 tests and 126 subtests passed and none failed; `TestOutputsThatDifferOnlyInCase` was skipped, because this volume is case-insensitive. |
| `probe-alias/go-test-case-sensitive.txt` | The file-handling tests on a temporary case-sensitive APFS disk image: the 17 that can run there passed, including `TestOutputsThatDifferOnlyInCase`; the three case-only rename tests that need a case-insensitive file system were skipped. |
| `probe-alias/check-control.txt` | The freshness negative control: `heirygen -check` on a scratch copy of the example with an edited specification reports `main.heirygen.go` and exits 1. |
| `probe-alias/freshness-negative-replay.txt` | The replay's own negative control: its freshness step on that scratch copy exits 1, and so does the replay. |
| `probe-alias/docs-checks.txt` | The documentation checker, its tests, `git diff --check`, and a scan for trailing whitespace in the untracked G1 files, run after the documentation was final, with a fingerprint of `docs/`. |

The fingerprint was `6ecb399bb7e2c8f0b796e7cc02996ab02e41fd111e85d340ab77ad7d77ff4bbc` throughout.

## Private-name probe and contract names

The private-name probe embedded the private contract under its own name, and an embedded field is named after its type, while `probe.X` is a method expression on the probe type. For a private operation named like its contract, such as `type Reset interface{ Reset() }` used as `private=Reset`, the selector found the field, and the generated file never compiled; regeneration gave the same failure. It failed closed but refused a valid input. Fixed test-first.

### Observed Red, then Green

| # | Failing observation | Change |
| --- | --- | --- |
| 43 | `TestPrivateProbeForAnOperationNamedLikeItsContract`, for a contract `type Reset interface{ Reset() }` and for a local alias `type Reset = resetter`, each with `wrapper view`: generation succeeded, and `go vet` failed with `main.heirygen.go:126:6: operand for field selector Reset must be value of type probe`, the probe's line ([red.txt](private-probe-alias/red.txt), recorded with provenance on the tree before the change). | The probe declares `type contract = Private`, with the name chosen by `free()` like the holder probe's `subject`, and embeds that alias instead of the contract's name. Nothing else in the probe changed |

After the change ([green.txt](private-probe-alias/green.txt)), both variants vet and run, printing `read 0`; a handwritten `func (v *view) Reset()` fails with `ambiguous selector probe.Reset` on a line of `main.heirygen.go`; and with `Reset` removed from the contract without regenerating, the build fails with `probe.Reset undefined` in `main.heirygen.go`. Every existing private-name and wrapper-guard test passed unchanged in the same run: `TestPrivateNameAbsenceAssertion`, `TestStaleOperationsAndWrapper`, `TestContractThatGrowsAtALaterLevel`, `TestLockstepRenameOfAnExistingPrivateOperation`, `TestEveryBuildOfAWrapperChecksBothContracts`, and the holder, forwarder, and existing-interface guard tests.

The three maintained outputs with a wrapper, the example and the `basic` and `components` fixtures, were regenerated. Each differs from its previous version by the alias line and by the embedded field, now `contract` in place of `Private`, and by nothing else.

### Verification record

Recorded in `private-probe-alias/` with Go 1.27.1 on darwin/arm64, on the uncommitted tree on top of `a726da8`. `red.txt` ran on the tree before the change, with fingerprint `b610fdc2…`; every other file ran on the final tree, whose fingerprint outside `docs/` was the same before and after each run.

| File | Result |
| --- | --- |
| `private-probe-alias/red.txt` | The Red step, on the tree before the change: both variants of the new regression failed on the probe's line in `main.heirygen.go`. |
| `private-probe-alias/green.txt` | The Green step: the new regression and the 17 existing private-name and wrapper-guard tests passed. |
| `private-probe-alias/gates.txt` | Every step of the CI workflow's Go job except the resolution fuzz step, replayed once by `replay-gates.sh`, not by GitHub Actions: discovery, formatting, `go vet`, the package tests, the race tests, running the examples, and the freshness loop. All 8 steps exited 0, and the replay exited 0. The fuzz step was not run, because no library source changed. |
| `private-probe-alias/go-test-generator.txt` | The test inventory: 117 tests and 128 subtests passed and none failed; `TestOutputsThatDifferOnlyInCase` was skipped, because this volume is case-insensitive. |
| `private-probe-alias/go-test-case-sensitive.txt` | The file-handling tests on a temporary case-sensitive APFS disk image: the 17 that can run there passed, including `TestOutputsThatDifferOnlyInCase`; the three case-only rename tests that need a case-insensitive file system were skipped. |
| `private-probe-alias/check-control.txt` | The freshness negative control: `heirygen -check` on a scratch copy of the example with an edited specification reports `main.heirygen.go` and exits 1. |
| `private-probe-alias/freshness-negative-replay.txt` | The replay's own negative control: its freshness step on that scratch copy exits 1, and so does the replay. |
| `private-probe-alias/docs-checks.txt` | The documentation checker, its tests, `git diff --check`, and a scan for trailing whitespace in the untracked G1 files, run after the documentation was final, with a fingerprint of `docs/`. |

The final fingerprint was `f2d0e87c214589501618a69c1a91d00f137f42122fb9d1ecf3138703eefafaf6` for every file but `red.txt`. No environment failure occurred.

## Closure

On 2026-11-19 the owner approved the wording that the G1 roadmap entry listed as awaiting approval, accepted the fault-injected test `TestInjectedWriteFailures/a_file_system_without_hard_links` as the evidence that a run on a file system without hard links fails before any output changes, and directed G1's completion with a green CI designation from passing local code and documentation gates, if the tests and documentation checks pass.

The closure changed documentation only:

- S10 lists the holder guard among the compile-time guards, states its limit for a directly declared named `heiry.Node` value, states that a registration in the hierarchy's own file spelled differently from that file's `existing` line gets its own guard, and says that an interrupted run can leave hidden files.
- The generator acceptance criteria list the existing regression coverage of the holder guard, the alias spelling, the private-name probe for an operation named like its contract, the wrapper's own contract signatures, the refusal of contradictory lines, and the choice of a usable signature.
- The documented freshness loop propagates an early failure. Run as `sh -c` with the three fixtures, it exited 0 on the maintained directories; with a stale scratch copy of the example placed first, it exited 1, where the previous loop exited 0.
- The architecture and the user guide say that an interrupted run can leave hidden files, which later runs ignore, and that a rerun creates an output a case-only rename left missing.

### Fingerprints

The source fingerprint, recomputed with the command in `record.sh` after the documentation changes, is `f2d0e87c214589501618a69c1a91d00f137f42122fb9d1ecf3138703eefafaf6`, the fingerprint before and after every run in `private-probe-alias/`, whose replay of every CI Go step except the fuzz step exited 0 and whose test inventory passed 117 tests and 128 subtests with none failing. [closure/docs-checks.txt](closure/docs-checks.txt) records the documentation checks, run after the documentation was final, with the same source fingerprint and a fingerprint of `docs/` that leaves out only that file, so that the record can carry it.

