set -o pipefail
# Run through record.sh after the documentation is final. Nothing is staged:
# tracked changes are checked with git diff --check, untracked files one by
# one with git diff --no-index --check, whose exit status has bit 2 set
# when it finds a whitespace problem.
record=docs/evidence/r1-release-readiness/docs-checks.txt
failed=0
step() {
  name=$1; shift
  echo; echo "## $name"; echo "\$ $*"
  bash -eo pipefail -c "$*"; status=$?
  echo "exit $status"
  [ $status -eq 0 ] || failed=1
}
step "Documentation fingerprint, without this record" "git ls-files -z -co --exclude-standard -- docs ':!$record' | sort -z | xargs -0 shasum -a 256 | shasum -a 256 | cut -d' ' -f1"
step "Documentation checker" "python3 scripts/check_docs.py"
step "Documentation checker tests" "python3 -m unittest discover -s scripts -p 'test_*.py' 2>&1"
step "Whitespace in tracked changes" "git diff --check && echo 'no problems'"
step "Whitespace in untracked files" 'for f in $(git ls-files --others --exclude-standard); do set +e; git diff --no-index --check /dev/null "$f"; s=$?; set -e; if [ $((s & 2)) -ne 0 ]; then echo "problem: $f"; exit 1; fi; echo "no problems: $f"; done'
step "Workflow parses as YAML" "ruby -ryaml -e 'w = YAML.safe_load(File.read(%q(.github/workflows/test.yml))); j = w[%q(jobs)]; j[%q(go)][%q(strategy)][%q(matrix)][%q(include)].each { |e| puts %Q(go job: #{e[%q(os)]}, Go #{e[%q(go)]}) }; puts %Q(go job fail-fast: #{j[%q(go)][%q(strategy)][%q(fail-fast)]}, steps: #{j[%q(go)][%q(steps)].length}); puts %Q(documentation job: #{j[%q(documentation)][%q(runs-on)]})'"
echo; echo "docs checks exit status: $failed"
exit $failed
