#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
sample_data="$repo_root/../data/SampleData.udbx"
henan_data="$repo_root/../data/henan.udbx"
output_dir="$repo_root/.benchmark-results/$(date +%Y%m%d-%H%M%S)"
skip_build=false

usage() {
  echo "Usage: $0 [--sample-data PATH] [--henan-data PATH] [--output-dir PATH] [--skip-build]"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sample-data)
      [[ $# -ge 2 ]] || { usage >&2; exit 2; }
      sample_data="$2"
      shift 2
      ;;
    --henan-data)
      [[ $# -ge 2 ]] || { usage >&2; exit 2; }
      henan_data="$2"
      shift 2
      ;;
    --output-dir)
      [[ $# -ge 2 ]] || { usage >&2; exit 2; }
      output_dir="$2"
      shift 2
      ;;
    --skip-build)
      skip_build=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

for command_name in wails jq node ps awk shasum stat sw_vers sysctl; do
  command -v "$command_name" >/dev/null || {
    echo "Missing required command: $command_name" >&2
    exit 2
  }
done

sample_data="$(cd "$(dirname "$sample_data")" && pwd)/$(basename "$sample_data")"
henan_data="$(cd "$(dirname "$henan_data")" && pwd)/$(basename "$henan_data")"
output_dir="$(mkdir -p "$output_dir" && cd "$output_dir" && pwd)"

for sample_path in "$sample_data" "$henan_data"; do
  [[ -f "$sample_path" ]] || {
    echo "Sample file not found: $sample_path" >&2
    exit 2
  }
done

config_dir="$output_dir/configs"
raw_dir="$output_dir/raw"
mkdir -p "$config_dir" "$raw_dir"

viewer_dir="$repo_root/cmd/udbx4go-viewer"
app_path="$viewer_dir/build/bin/udbx4go-viewer-wails.app"
executable="$app_path/Contents/MacOS/udbx4go-viewer-wails"

if [[ "$skip_build" != true ]]; then
  (
    cd "$viewer_dir"
    wails build -platform darwin/universal
  )
fi

[[ -x "$executable" ]] || {
  echo "Viewer executable not found after build: $executable" >&2
  exit 2
}

git_commit="$(git -C "$repo_root" rev-parse HEAD)"
macos_version="$(sw_vers -productVersion)"
cpu="$(sysctl -n machdep.cpu.brand_string 2>/dev/null || sysctl -n hw.model)"
memory_bytes="$(sysctl -n hw.memsize)"
sample_sha="$(shasum -a 256 "$sample_data" | awk '{print $1}')"
henan_sha="$(shasum -a 256 "$henan_data" | awk '{print $1}')"
sample_size="$(stat -f %z "$sample_data")"
henan_size="$(stat -f %z "$henan_data")"

process_tree_rss() {
  local root_pid="$1"
  ps -axo pid=,ppid=,rss= | awk -v root="$root_pid" '
    {
      pid[NR] = $1
      ppid[NR] = $2
      rss[NR] = $3
    }
    END {
      included[root] = 1
      changed = 1
      while (changed) {
        changed = 0
        for (i = 1; i <= NR; i++) {
          if (!included[pid[i]] && included[ppid[i]]) {
            included[pid[i]] = 1
            changed = 1
          }
        }
      }
      total = 0
      for (i = 1; i <= NR; i++) {
        if (included[pid[i]]) {
          total += rss[i]
        }
      }
      print total
    }
  '
}

write_failed_result() {
  local result_path="$1"
  local run_id="$2"
  local scenario_name="$3"
  local error_message="$4"
  jq -n \
    --arg runId "$run_id" \
    --arg scenario "$scenario_name" \
    --arg startedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg error "$error_message" \
    '{
      runId: $runId,
      status: "failed",
      startedAt: $startedAt,
      scenario: $scenario,
      metrics: {openFileMs: 0, loadLayersMs: 0, fitVisibleLayersMs: 0, selectAndFitMs: 0},
      error: $error
    }' > "$result_path"
}

run_iteration() {
  local scenario_name="$1"
  local file_path="$2"
  local layers_json="$3"
  local selection_dataset="$4"
  local selection_page="$5"
  local iteration="$6"
  local temperature="warm"
  [[ "$iteration" -eq 1 ]] && temperature="cold"

  local run_id="${scenario_name}-${iteration}"
  local config_path="$config_dir/${run_id}.json"
  local result_path="$raw_dir/${run_id}.json"

  jq -n \
    --arg runId "$run_id" \
    --arg outputPath "$result_path" \
    --arg name "$scenario_name" \
    --arg filePath "$file_path" \
    --argjson layers "$layers_json" \
    --arg datasetName "$selection_dataset" \
    --argjson page "$selection_page" \
    '{
      runId: $runId,
      outputPath: $outputPath,
      scenario: {
        name: $name,
        filePath: $filePath,
        layers: $layers,
        selection: {datasetName: $datasetName, page: $page, rowIndex: 0}
      }
    }' > "$config_path"

  "$executable" --benchmark-config "$config_path" > "$output_dir/${run_id}.log" 2>&1 &
  local app_pid=$!
  local peak_rss=0
  local polls=0

  while kill -0 "$app_pid" 2>/dev/null; do
    local current_rss
    current_rss="$(process_tree_rss "$app_pid")"
    if [[ "$current_rss" =~ ^[0-9]+$ ]] && (( current_rss > peak_rss )); then
      peak_rss="$current_rss"
    fi
    polls=$((polls + 1))
    if (( polls >= 600 )); then
      kill "$app_pid" 2>/dev/null || true
      wait "$app_pid" 2>/dev/null || true
      write_failed_result "$result_path" "$run_id" "$scenario_name" "benchmark timed out after 60 seconds"
      break
    fi
    sleep 0.1
  done

  local exit_code=0
  if wait "$app_pid" 2>/dev/null; then
    exit_code=0
  else
    exit_code=$?
  fi

  if [[ ! -f "$result_path" ]]; then
    write_failed_result "$result_path" "$run_id" "$scenario_name" "viewer exited with code $exit_code before writing a result"
  fi

  local input_sha="$sample_sha"
  local input_size="$sample_size"
  if [[ "$file_path" == "$henan_data" ]]; then
    input_sha="$henan_sha"
    input_size="$henan_size"
  fi
  local memory_error=""
  if (( peak_rss <= 0 )); then
    memory_error="no RSS sample captured"
  fi

  local enriched_path="$result_path.enriched"
  jq \
    --argjson iteration "$iteration" \
    --arg temperature "$temperature" \
    --argjson peakRssKiB "$peak_rss" \
    --arg memoryCaptureError "$memory_error" \
    --arg appPath "$app_path" \
    --arg gitCommit "$git_commit" \
    --arg macOSVersion "$macos_version" \
    --arg cpu "$cpu" \
    --argjson memoryBytes "$memory_bytes" \
    --arg samplePath "$file_path" \
    --arg sampleSha256 "$input_sha" \
    --argjson sampleSizeBytes "$input_size" \
    '. + {
      iteration: $iteration,
      temperature: $temperature,
      peakRssKiB: $peakRssKiB,
      memoryCaptureError: $memoryCaptureError,
      appPath: $appPath,
      environment: {
        gitCommit: $gitCommit,
        macOSVersion: $macOSVersion,
        cpu: $cpu,
        memoryBytes: $memoryBytes,
        samplePath: $samplePath,
        sampleSha256: $sampleSha256,
        sampleSizeBytes: $sampleSizeBytes
      }
    }' "$result_path" > "$enriched_path"
  mv "$enriched_path" "$result_path"

  echo "[$scenario_name] iteration $iteration: $(jq -r '.status' "$result_path"), peak RSS ${peak_rss} KiB"
}

for iteration in 1 2 3 4 5; do
  run_iteration \
    "sampledata-multilayer" \
    "$sample_data" \
    '["BaseMap_P", "BaseMap_L", "BaseMap_R", "CADDT"]' \
    "BaseMap_R" \
    1 \
    "$iteration"
done

for iteration in 1 2 3 4 5; do
  run_iteration \
    "henan-county-page-2" \
    "$henan_data" \
    '["县级行政区划"]' \
    "县级行政区划" \
    2 \
    "$iteration"
done

node "$script_dir/summarize-viewer-benchmark.mjs" \
  --input-dir "$raw_dir" \
  --json-out "$output_dir/summary.json" \
  --markdown-out "$output_dir/summary.md"

echo "Benchmark report: $output_dir/summary.md"
