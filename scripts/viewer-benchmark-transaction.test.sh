#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$script_dir/viewer-benchmark-transaction.sh"

if [[ "${1:-}" == "signal-child" ]]; then
  policy_file="$2"
  ready_file="$3"
  signal_name="$4"
  begin_benchmark_policy_transaction "$policy_file" "$policy_file.backup"
  printf 'export const VIEWPORT_QUERY_MAX_CONCURRENCY = 3\n' > "$policy_file"
  trap 'restore_benchmark_policy_transaction' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  printf 'ready\n' > "$ready_file"
  kill -"$signal_name" "$$"
  while true; do sleep 1; done
fi

for signal_name in INT TERM; do
  tmp_dir="$(mktemp -d)"
  policy_file="$tmp_dir/policy.ts"
  ready_file="$tmp_dir/ready"
  group_ready_file="$tmp_dir/group-ready"
  printf 'export const VIEWPORT_QUERY_MAX_CONCURRENCY = 1\n' > "$policy_file"
  python3 "$script_dir/run-in-process-group.py" "$group_ready_file" bash "$0" signal-child "$policy_file" "$ready_file" "$signal_name" &
  child_pid=$!
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    [[ -s "$ready_file" ]] && break
    sleep 0.1
  done
  [[ -s "$ready_file" ]]
  for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
    kill -0 "$child_pid" 2>/dev/null || break
    sleep 0.1
  done
  if kill -0 "$child_pid" 2>/dev/null; then
    kill -KILL "-$child_pid" 2>/dev/null || true
    echo "$signal_name transaction child did not exit" >&2
    exit 1
  fi
  wait "$child_pid" 2>/dev/null || true
  [[ "$(<"$policy_file")" == 'export const VIEWPORT_QUERY_MAX_CONCURRENCY = 1' ]]
  rm -rf "$tmp_dir"
done
