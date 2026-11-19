#!/bin/bash
# record.sh <output file> <description> <command...>
# Runs the command in the repository and writes its output to the file,
# preceded by what identifies the run: time, toolchain, system, the commit
# and state of the tree, and a fingerprint of every file outside docs/
# taken before and after the run.
set -o pipefail
repo=$(git rev-parse --show-toplevel)
out=$1; what=$2; shift 2
cd "$repo" || exit 1
fingerprint() { git ls-files -z -co --exclude-standard -- . ':!docs' ':(exclude,glob)**/.DS_Store' | sort -z | xargs -0 shasum -a 256 | shasum -a 256 | cut -d' ' -f1; }
before=$(fingerprint)
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
  cat "$body"
} > "$out"
rm -f "$body"
echo "$out: exit $status, fingerprint $before -> $after"
