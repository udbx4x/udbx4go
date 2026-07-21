#!/usr/bin/env bash

rss_elapsed_seconds() {
  local sample_count="$1"
  echo $((sample_count / 10))
}

record_rss_sample() {
  local current_rss="$1" elapsed_seconds="$2"
  [[ "$current_rss" =~ ^[0-9]+$ ]] && (( current_rss > 0 )) || return 0
  if (( rss_first == 0 )); then
    rss_first="$current_rss"
  fi
  if (( rss_start == 0 && elapsed_seconds >= 1 )); then
    rss_start="$current_rss"
  fi
  rss_end="$current_rss"
  if (( current_rss > peak_rss )); then
    peak_rss="$current_rss"
  fi
}

finalize_rss_samples() {
  if (( rss_start == 0 )); then
    rss_start="$rss_end"
  fi
}
