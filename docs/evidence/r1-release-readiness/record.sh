#!/bin/bash
# record.sh <output file> <description> <command...>
# Runs the command in the repository and writes its output to the file,
# preceded by what identifies the run: time, toolchain, system, the commit
# and state of the tree, a fingerprint of every file outside docs/ taken
# before and after the run, the command, and each uncommitted path outside
# docs/ with its SHA-256 at the start of the run.
# R1 extended the G1 copy with the last two items.
set -o pipefail
repo=$(git rev-parse --show-toplevel)
out=$1; what=$2; shift 2
cd "$repo" || exit 1
fingerprint() { git ls-files -z -co --exclude-standard -- . ':!docs' ':(exclude,glob)**/.DS_Store' | sort -z | xargs -0 shasum -a 256 | shasum -a 256 | cut -d' ' -f1; }
# One line per uncommitted path, untracked files listed one by one: its
# git status code, its SHA-256 (or "no file" when deleted), and its path.
uncommitted() {
  git status --porcelain -z --untracked-files=all -- . ':!docs' ':(exclude,glob)**/.DS_Store' | while IFS= read -r -d '' entry; do
    state=${entry:0:2}; path=${entry:3}
    case $state in R*|C*) IFS= read -r -d '' _ ;; esac # skip the old name of a rename or copy
    if [ -f "$path" ]; then
      echo "#   $state $(shasum -a 256 "$path" | cut -d' ' -f1) $path"
    else
      echo "#   $state no file $path"
    fi
  done
}
before=$(fingerprint)
paths=$(uncommitted)
started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
body=$(mktemp)
bash -o pipefail -c "$*" > "$body" 2>&1
status=$?
after=$(fingerprint)
dirty=$(git status --porcelain -- . ':!docs' ':(exclude,glob)**/.DS_Store' | wc -l | tr -d ' ')
{
  echo "# $what"
  echo "# Recorded $started on this machine by a local script, not by GitHub Actions."
  echo "# Toolchain: $(go version)"
  echo "# System: $(uname -srm)"
  echo "# Tree: branch $(git branch --show-current) at $(git rev-parse HEAD), with $dirty uncommitted paths outside docs/"
  echo "# Tree fingerprint before the run: $before"
  echo "# Tree fingerprint after the run:  $after"
  echo "# Fingerprint command: git ls-files -z -co --exclude-standard -- . ':!docs' ':(exclude,glob)**/.DS_Store' | sort -z | xargs -0 shasum -a 256 | shasum -a 256"
  echo "# Exit status of the recorded command: $status"
  echo "# Recorded command: $*"
  echo "# Uncommitted paths outside docs/ at the start of the run (git status, SHA-256, path):"
  if [ -n "$paths" ]; then echo "$paths"; else echo "#   none"; fi
  cat "$body"
} > "$out"
rm -f "$body"
echo "$out: exit $status, fingerprint $before -> $after"
