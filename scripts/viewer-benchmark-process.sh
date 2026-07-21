#!/usr/bin/env bash

active_benchmark_pid=""
active_benchmark_pgid=""

start_benchmark_process_group() {
  local log_path="$1"
  shift
  local helper_dir ready_file tick
  helper_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  ready_file="${TMPDIR:-/tmp}/udbx-benchmark-pgid-$$-$RANDOM"
  python3 "$helper_dir/run-in-process-group.py" "$ready_file" "$@" > "$log_path" 2>&1 &
  active_benchmark_pid=$!
  active_benchmark_pgid=$active_benchmark_pid
  for ((tick = 0; tick < 100; tick += 1)); do
    [[ -s "$ready_file" ]] && break
    kill -0 "$active_benchmark_pid" 2>/dev/null || break
    sleep 0.01
  done
  rm -f "$ready_file"
}

benchmark_process_group_alive() {
  local pgid="$1"
  [[ -n "$pgid" ]] && /bin/kill -0 "-$pgid" 2>/dev/null
}

wait_benchmark_process_group() {
  local pid="$active_benchmark_pid" status=0
  if [[ -n "$pid" ]]; then
    if wait "$pid"; then
      status=0
    else
      status=$?
    fi
  fi
  active_benchmark_pid=""
  active_benchmark_pgid=""
  return "$status"
}

terminate_benchmark_process_group() {
  local pgid="${1:-$active_benchmark_pgid}" grace_ticks="${2:-20}"
  [[ -n "$pgid" ]] || return 0
  /bin/kill -TERM "-$pgid" 2>/dev/null || true
  local tick
  for ((tick = 0; tick < grace_ticks; tick += 1)); do
    benchmark_process_group_alive "$pgid" || break
    sleep 0.1
  done
  if benchmark_process_group_alive "$pgid"; then
    /bin/kill -KILL "-$pgid" 2>/dev/null || true
  fi
  for ((tick = 0; tick < 10; tick += 1)); do
    benchmark_process_group_alive "$pgid" || break
    sleep 0.1
  done
  if ! benchmark_process_group_alive "$pgid" && [[ -n "$active_benchmark_pid" ]]; then
    wait "$active_benchmark_pid" 2>/dev/null || true
  fi
  active_benchmark_pid=""
  active_benchmark_pgid=""
}

cleanup_benchmark_process_group() {
  terminate_benchmark_process_group "$active_benchmark_pgid" 10
}
