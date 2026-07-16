#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$script_dir/viewer-benchmark-process.sh"

if [[ "${1:-}" == "trap-child" ]]; then
  pgid_file="$2"
  trap 'cleanup_benchmark_process_group' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  start_benchmark_process_group /dev/null bash -c 'sleep 60 & wait'
  printf '%s\n' "$active_benchmark_pgid" > "$pgid_file"
  wait "$active_benchmark_pid"
  exit 0
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
pgid_file="$tmp_dir/pgid"

bash "$0" trap-child "$pgid_file" &
harness_pid=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do
  [[ -s "$pgid_file" ]] && break
  sleep 0.1
done
[[ -s "$pgid_file" ]]
pgid="$(<"$pgid_file")"
kill -TERM "$harness_pid"
wait "$harness_pid" 2>/dev/null || true

for _ in 1 2 3 4 5 6 7 8 9 10; do
  if ! ps -axo pgid= | awk -v pgid="$pgid" '$1 == pgid { found=1 } END { exit found ? 0 : 1 }'; then
    exit 0
  fi
  sleep 0.1
done

echo "process group $pgid survived cleanup" >&2
exit 1
