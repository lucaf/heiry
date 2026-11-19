set -o pipefail
fixtures="./cmd/heirygen/internal/generator/testdata/basic ./cmd/heirygen/internal/generator/testdata/components ./cmd/heirygen/internal/generator/testdata/matrix"
examples="./examples/basic ./examples/components ./examples/lifecycle ./examples/comparison ./examples/generated"
# REPLAY_ONLY runs one step by name, and REPLAY_FRESH_DIRS replaces the
# directories of the freshness step; both exist for negative controls.
fresh=${REPLAY_FRESH_DIRS:-"./examples/generated $fixtures"}
only=${REPLAY_ONLY:-}
failed=0
echo "Each step runs as bash -eo pipefail -c '<command>', so it stops at the first failing command, and its exit status is printed after it. The replay exits 1 if any step fails."
step() {
  name=$1; shift
  if [ -n "$only" ] && [ "$name" != "$only" ]; then return; fi
  echo; echo "## $name"; echo "\$ $*"
  bash -eo pipefail -c "$*"; status=$?
  echo "exit $status"
  [ $status -eq 0 ] || failed=1
}
step "Discover root package" 'test "$(go list -f "{{len .GoFiles}}" .)" -gt 0 && test "$(go list -f "{{len .TestGoFiles}}" .)" -gt 0 && echo "root: $(go list -f "{{len .GoFiles}} source, {{len .TestGoFiles}} test, {{len .XTestGoFiles}} external test files" .)"'
step "Discover examples, generator packages, and fixtures" 'for p in '"$examples"' ./cmd/heirygen ./cmd/heirygen/internal/generator '"$fixtures"'; do g=$(go list -f "{{len .GoFiles}}" $p); t=$(go list -f "{{len .TestGoFiles}}" $p); test $g -gt 0; test $t -gt 0; echo "$p: $g source, $t test files"; done'
step "Formatting (workflow selection)" 'files="$(gofmt -l ./*.go testdata/stage4boundary/*.go testdata/stage4positive/*.go testdata/stage4negative/*.go testdata/stage5vet/*.go testdata/m4positive/*.go testdata/m4negative/*.go testdata/m6positive/*.go testdata/m6negativebind/*.go testdata/m6negativepointer/*.go testdata/m6negativetoken/*.go testdata/m8positive/*.go testdata/m8negativepointer/*.go testdata/m8negativebind/*.go testdata/e1positive/*.go testdata/e1negativeshadow/*.go testdata/e2negativebindnode/*.go testdata/e2negativegroup/*.go examples/basic/*.go examples/components/*.go examples/lifecycle/*.go examples/comparison/*.go examples/generated/*.go cmd/heirygen/*.go cmd/heirygen/internal/generator/*.go cmd/heirygen/internal/generator/testdata/*/*.go)"; if [ -n "$files" ]; then echo "$files"; exit 1; fi; echo "no files"'
step "Vet" "go vet . $examples ./cmd/heirygen/... $fixtures"
step "Package tests" "go test -count=1 . $examples ./cmd/heirygen/... $fixtures"
step "Race tests" "go test -race -count=1 . $examples $fixtures && go test -race -short -count=1 ./cmd/heirygen/..."
step "Run examples" "for p in $examples; do go run \$p > /dev/null; echo \"\$p ran\"; done"
step "Generated output is up to date" "for d in $fresh; do go run ./cmd/heirygen -check \"\$d\" || exit 1; echo \"\$d up to date\"; done"
echo; echo "replay exit status: $failed"
exit $failed
