#!/usr/bin/env bash

record_rss_sample() {
  local current_rss="$1" elapsed_seconds="$2"
  [[ "$current_rss" =~ ^[0-9]+$ ]] && (( current_rss > 0 )) || return 0
  if (( rss_start == 0 && elapsed_seconds >= 2 )); then
    rss_start="$current_rss"
  fi
  rss_end="$current_rss"
  (( current_rss > peak_rss )) && peak_rss="$current_rss"
}
