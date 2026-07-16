#!/usr/bin/env bash

benchmark_policy_file=""
benchmark_policy_backup=""
benchmark_policy_committed=false

begin_benchmark_policy_transaction() {
  benchmark_policy_file="$1"
  benchmark_policy_backup="$2"
  benchmark_policy_committed=false
  cp "$benchmark_policy_file" "$benchmark_policy_backup"
}

commit_benchmark_policy_transaction() {
  benchmark_policy_committed=true
}

restore_benchmark_policy_transaction() {
  [[ "$benchmark_policy_committed" == true ]] && return 0
  [[ -n "$benchmark_policy_file" && -f "$benchmark_policy_backup" ]] || return 0
  cp "$benchmark_policy_backup" "$benchmark_policy_file"
}
