#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$script_dir/viewer-benchmark-rss.sh"

peak_rss=0
rss_first=0
rss_start=0
rss_end=0

record_rss_sample 100 0
[[ "$peak_rss" == 100 && "$rss_start" == 0 && "$rss_end" == 100 ]]
record_rss_sample 110 1
[[ "$peak_rss" == 110 && "$rss_start" == 0 && "$rss_end" == 110 ]]
record_rss_sample 120 2
[[ "$peak_rss" == 120 && "$rss_start" == 120 && "$rss_end" == 120 ]]
record_rss_sample 130 3
[[ "$peak_rss" == 130 && "$rss_start" == 120 && "$rss_end" == 130 ]]

# Repeated or lower samples are valid and must not trip callers using set -e.
record_rss_sample 130 4
record_rss_sample 125 5
[[ "$peak_rss" == 130 && "$rss_start" == 120 && "$rss_end" == 125 ]]

# Runs shorter than two seconds use the first valid sample as their baseline.
peak_rss=0
rss_first=0
rss_start=0
rss_end=0
record_rss_sample 90 0
record_rss_sample 100 1
finalize_rss_samples
[[ "$peak_rss" == 100 && "$rss_start" == 90 && "$rss_end" == 100 ]]
