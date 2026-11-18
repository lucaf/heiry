# P1 measurement evidence

Raw outputs behind the figures in the [P1 roadmap entry](../../roadmap.md#p1-precomputed-resolution-performance). Everything here was recorded on 2026-11-18. The files are verbatim tool output, except that the local checkout path and trailing whitespace were removed from `profile-new-pre.txt`.

## Environment

- Go 1.27.1, darwin/arm64, Apple M2 (4 performance and 4 efficiency cores, 16 GB), macOS 26.5.
- On battery, with Spotlight indexing active. The `*.timing.txt` files record load averages around the runs, between 6.98 and 21.64.
- Every benchmark ran from a test binary built beforehand with `go test -c`, so no figure includes compilation.

## Source identities

"Base" is commit `0e7b51936c0868a343c32a51207e86e1d669b033` plus the new benchmark file. "New" is the same commit plus the working-tree changes of this task. Neither was committed when measured, so the changed files are identified by their Git blob ids (`git hash-object`):

| File | Base | New |
| --- | --- | --- |
| `dispatch.go` | `bc077767fa99` | `93660a1bb0b9` |
| `plan.go` | `f60ea6240011` | `0fb5bbd984e2` |
| `hierarchy.go` | `d677d65befbe` | `062cdaccd08d` |
| `benchmark_test.go` | `9a25882801b6` | `c73f67760628` |
| `precompute_test.go` | `2fc7f26a318a` | `0c3a868a77bf` |
| `resolution_oracle_test.go` | `5502f6b542fc` | `6bea4b5b76cf` |
| `benchmark_precompute_test.go` | `c08467fba61d` | `c08467fba61d` |

SHA-256 of the test binaries that produced the outputs. The binaries were not kept.

| Binary | Built with | SHA-256 |
| --- | --- | --- |
| root package, base | `go test -c .` | `3574b26c0896d1b23a65f4c1ac65d9e95e21791ffbaddb8dfb0ea465c6c64d74` |
| root package, new | `go test -c .` | `1dec86e509c9af29c807394097a37e3a47ba2f6895f07a478d3d7ad9cb46c19d` |
| comparison example, base | `go test -c ./examples/comparison` | `e2714ae3ee4bebb5379b0ca0f6931d0fabd77217bbe2846754325468d0bfcb31` |
| comparison example, new | `go test -c ./examples/comparison` | `651b4812a18c556c5a7bd1be6ab7fef152181eb52b409afb6d34cd7e58478cee` |
| root package with the memory script, base | `go test -c .` | `9c7e9363b2eb73a1c2fe9bea54c289e74640f24ac8baa05b08e22d7d09314d79` |
| root package with the memory script, new | `go test -c .` | `d863b3cf9c7ff8c4bdcf3ce527ad9f559622be65026e7fb9e93a5659101b56ac` |

The base binaries for the comparison example and the memory script were built while the six changed Go files were set aside with `git stash push -- <files>` and restored with `git stash pop`; the diff of the working tree had the same hash before and after.

## Files and commands

| File | Command | Content |
| --- | --- | --- |
| `full-base.txt`, `full-new.txt` | `./heiry.test -test.run '^$' -test.bench . -test.benchmem -test.count=5` | The complete benchmark suite, 151 cases and 755 measurements each. This is the documented benchmark gate without the build. |
| `full-base.timing.txt`, `full-new.timing.txt` | `date`, `/usr/bin/time -p`, `uptime` around the run | Start and end time, elapsed seconds, load average |
| `alternating-base.txt`, `alternating-new.txt` | Six rounds; in each round the base binary and then the new binary ran `-test.bench 'PrecomputeLookup' -test.benchmem -test.count=1` and then `-test.bench 'Resolution/depth1/(root\|dense)/methods(1\|5)/(dynamic\|precomputed)' -test.benchmem -test.count=1` | 50 lookup cases, six measurements each per binary. The second pattern is unanchored and also selected the depth-10 cases. |
| `alternating.timing.txt` | `date` and `uptime` around the six rounds | Start and end time, load average |
| `comparison-example-base.txt`, `comparison-example-new.txt` | Six alternating rounds of `./comparison.test -test.run '^$' -test.bench . -test.benchmem -test.count=1` | `BenchmarkRender` of `examples/comparison` |
| `profile-base-pre.txt`, `profile-base-dyn.txt`, `profile-base-dynbusy.txt` | `./heiry.test -test.run '^$' -test.bench 'PrecomputeLookup/<case>$' -test.benchtime=3s -test.cpuprofile cpu.prof`, then `go tool pprof -top -nodecount=25` | Base CPU profiles of `depth3/ops4/dense/virtual/precomputed`, `depth3/ops4/dense/virtual/dynamic`, and `depth10/ops4/busy/virtual/dynamic` |
| `profile-new-pre.txt` | The same for the new binary, followed by `go tool pprof -list 'heiry.resolve$'` | New CPU profile of `depth3/ops4/dense/virtual/precomputed` |
| `retained-memory-base.txt`, `retained-memory-new.txt` | `./heiry.test -test.run '^TestZZRetained$' -test.v` | Output of the memory script below |
| `retained_memory_test.go.txt` | — | The memory script |
| `summarize.py` | `python3 summarize.py BASE NEW` | Prints median and lowest–highest run per case, and the change of the median |
| `summary-full.txt`, `summary-alternating.txt`, `summary-comparison-example.txt` | `summarize.py` on the three pairs above | The medians and ranges quoted in the roadmap |

## Retained-memory script

`retained_memory_test.go.txt` is not part of the package. To run it, copy it to the repository root as `zz_retained_test.go`, run `go test . -run '^TestZZRetained$' -v`, and delete the copy. It uses the fixtures in `benchmark_precompute_test.go`.

For each case it initializes 20,000 objects and reports the growth of `runtime.MemStats.HeapAlloc` after two garbage collections, divided by the object count. The leaf objects and their references are allocated before the first reading, so the "binding" figures are what initialization leaves reachable. Those figures were the same within 1 B in two runs of each binary. The "table" figure is one heap difference around a single conversion; it varied between runs by several kilobytes and is not used. The roadmap quotes the bytes allocated by the conversion benchmarks instead.

## Limits of this evidence

- One machine, busy and on battery. Ratios between cases measured in the same run are more reliable than absolute values.
- The two full runs followed one another; only the alternating runs compare the two binaries under closely matched conditions.
- The profiles are sampled over three seconds of one benchmark case each.
- The lookup benchmarks keep the selected provider without calling it, so they say nothing about the cost of the call that follows.
