#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
sample_data="$repo_root/../data/SampleData.udbx"
henan_data="$repo_root/../data/henan.udbx"
output_dir="$repo_root/.benchmark-results/$(date +%Y%m%d-%H%M%S)"
acceptance_report=""
max_concurrent=1
skip_build=false

usage() {
  echo "Usage: $0 [--sample-data PATH] [--henan-data PATH] [--output-dir PATH] [--max-concurrent 1|2|3] [--acceptance-report ABSOLUTE_PATH] [--skip-build]"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sample-data) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; sample_data="$2"; shift 2 ;;
    --henan-data) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; henan_data="$2"; shift 2 ;;
    --output-dir) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; output_dir="$2"; shift 2 ;;
    --max-concurrent) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; max_concurrent="$2"; shift 2 ;;
    --acceptance-report) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; acceptance_report="$2"; shift 2 ;;
    --skip-build) skip_build=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ "$max_concurrent" =~ ^[123]$ ]] || { echo "--max-concurrent must be 1, 2 or 3" >&2; exit 2; }
if [[ -n "$acceptance_report" && "$acceptance_report" != /* ]]; then
  echo "--acceptance-report must be an absolute path" >&2
  exit 2
fi

for command_name in wails jq node ps awk shasum stat sw_vers sysctl; do
  command -v "$command_name" >/dev/null || { echo "Missing required command: $command_name" >&2; exit 2; }
done

sample_data="$(cd "$(dirname "$sample_data")" && pwd)/$(basename "$sample_data")"
henan_data="$(cd "$(dirname "$henan_data")" && pwd)/$(basename "$henan_data")"
output_dir="$(mkdir -p "$output_dir" && cd "$output_dir" && pwd)"
for sample_path in "$sample_data" "$henan_data"; do
  [[ -f "$sample_path" ]] || { echo "Sample file not found: $sample_path" >&2; exit 2; }
done

config_dir="$output_dir/configs"
raw_dir="$output_dir/raw"
mkdir -p "$config_dir" "$raw_dir"
viewer_dir="$repo_root/cmd/udbx4go-viewer"
app_path="$viewer_dir/build/bin/udbx4go-viewer-wails.app"
executable="$app_path/Contents/MacOS/udbx4go-viewer-wails"

if [[ "$skip_build" != true ]]; then
  (cd "$viewer_dir" && wails build -platform darwin/universal)
fi
[[ -x "$executable" ]] || { echo "Viewer executable not found after build: $executable" >&2; exit 2; }

git_commit="$(git -C "$repo_root" rev-parse HEAD)"
app_sha="$(shasum -a 256 "$executable" | awk '{print $1}')"
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
    { pid[NR] = $1; ppid[NR] = $2; rss[NR] = $3 }
    END {
      included[root] = 1
      changed = 1
      while (changed) {
        changed = 0
        for (i = 1; i <= NR; i++) if (!included[pid[i]] && included[ppid[i]]) {
          included[pid[i]] = 1; changed = 1
        }
      }
      total = 0
      for (i = 1; i <= NR; i++) if (included[pid[i]]) total += rss[i]
      print total
    }'
}

write_failed_result() {
  local result_path="$1" run_id="$2" scenario_name="$3" error_message="$4"
  jq -n --arg runId "$run_id" --arg scenario "$scenario_name" \
    --arg startedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --arg error "$error_message" \
    '{runId:$runId,status:"failed",startedAt:$startedAt,scenario:$scenario,
      metrics:{openFileMs:0,loadLayersMs:0,fitVisibleLayersMs:0,selectAndFitMs:0,
      backendQueryMs:[],moveendToRenderMs:[],maxConcurrentQueries:0,pendingPeak:0,
      pendingFinal:0,staleResultsDiscarded:0,staleResultApplied:false,
      finalFeatureCount:0,blankRenderCount:0},error:$error}' > "$result_path"
}

run_iteration() {
  local scenario_name="$1" file_path="$2" layers_json="$3" selection_dataset="$4"
  local selection_page="$5" viewport_steps_json="$6" temperature="$7" iteration="$8"
  local run_id="${scenario_name}-${temperature}-${iteration}"
  local config_path="$config_dir/${run_id}.json" result_path="$raw_dir/${run_id}.json"

  jq -n --arg runId "$run_id" --arg outputPath "$result_path" --arg temperature "$temperature" \
    --argjson maxConcurrentQueries "$max_concurrent" --arg name "$scenario_name" \
    --arg filePath "$file_path" --argjson layers "$layers_json" \
    --arg datasetName "$selection_dataset" --argjson page "$selection_page" \
    --argjson viewportSteps "$viewport_steps_json" \
    '{runId:$runId,outputPath:$outputPath,temperature:$temperature,
      maxConcurrentQueries:$maxConcurrentQueries,scenario:{name:$name,filePath:$filePath,
      layers:$layers,selection:{datasetName:$datasetName,page:$page,rowIndex:0},viewportSteps:$viewportSteps}}' \
    > "$config_path"

  "$executable" --benchmark-config "$config_path" > "$output_dir/${run_id}.log" 2>&1 &
  local app_pid=$! peak_rss=0 rss_start=0 rss_end=0
  local started_seconds=$SECONDS
  while kill -0 "$app_pid" 2>/dev/null; do
    local current_rss
    current_rss="$(process_tree_rss "$app_pid")"
    if [[ "$current_rss" =~ ^[0-9]+$ ]] && (( current_rss > 0 )); then
      if (( SECONDS - started_seconds < 2 || rss_start == 0 )); then
        rss_start="$current_rss"
      fi
      rss_end="$current_rss"
      (( current_rss > peak_rss )) && peak_rss="$current_rss"
    fi
    if (( SECONDS - started_seconds >= 120 )); then
      kill "$app_pid" 2>/dev/null || true
      wait "$app_pid" 2>/dev/null || true
      write_failed_result "$result_path" "$run_id" "$scenario_name" "benchmark timed out after 120 seconds"
      break
    fi
    sleep 0.1
  done

  local exit_code=0
  wait "$app_pid" 2>/dev/null || exit_code=$?
  [[ -f "$result_path" ]] || write_failed_result "$result_path" "$run_id" "$scenario_name" "viewer exited with code $exit_code before writing a result"

  local input_sha="$sample_sha" input_size="$sample_size"
  if [[ "$file_path" == "$henan_data" ]]; then input_sha="$henan_sha"; input_size="$henan_size"; fi
  local memory_error=""
  (( peak_rss > 0 && rss_start > 0 && rss_end > 0 )) || memory_error="no RSS sample captured"

  jq --argjson iteration "$iteration" --arg temperature "$temperature" \
    --argjson peakRssKiB "$peak_rss" --argjson rssStartKiB "$rss_start" --argjson rssEndKiB "$rss_end" \
    --arg memoryCaptureError "$memory_error" --arg appPath "$app_path" \
    --argjson maxConcurrentQueries "$max_concurrent" --arg gitCommit "$git_commit" --arg appSha256 "$app_sha" \
    --arg macOSVersion "$macos_version" --arg cpu "$cpu" --argjson memoryBytes "$memory_bytes" \
    --arg samplePath "$file_path" --arg sampleSha256 "$input_sha" --argjson sampleSizeBytes "$input_size" \
    '.+{iteration:$iteration,temperature:$temperature,peakRssKiB:$peakRssKiB,
      rssStartKiB:$rssStartKiB,rssEndKiB:$rssEndKiB,memoryCaptureError:$memoryCaptureError,
      appPath:$appPath,maxConcurrentQueries:$maxConcurrentQueries,
      environment:{gitCommit:$gitCommit,appSha256:$appSha256,macOSVersion:$macOSVersion,cpu:$cpu,memoryBytes:$memoryBytes,
      samplePath:$samplePath,sampleSha256:$sampleSha256,sampleSizeBytes:$sampleSizeBytes}}' \
    "$result_path" > "$result_path.enriched"
  mv "$result_path.enriched" "$result_path"
  echo "[$scenario_name][$temperature] $iteration: $(jq -r '.status' "$result_path"), peak RSS ${peak_rss} KiB"
}

weibo_steps='[
 {"bounds":{"minX":113.50,"minY":34.50,"maxX":114.00,"maxY":35.00},"expectedStrategy":"rtree"},
 {"bounds":{"minX":113.55,"minY":34.52,"maxX":113.95,"maxY":34.92},"expectedStrategy":"rtree"},
 {"bounds":{"minX":113.60,"minY":34.55,"maxX":113.90,"maxY":34.85},"expectedStrategy":"rtree"},
 {"bounds":{"minX":113.65,"minY":34.58,"maxX":113.85,"maxY":34.78},"expectedStrategy":"rtree"},
 {"bounds":{"minX":113.70,"minY":34.60,"maxX":113.82,"maxY":34.72},"expectedStrategy":"rtree"},
 {"bounds":{"minX":113.75,"minY":34.65,"maxX":113.95,"maxY":34.85},"expectedStrategy":"rtree"},
 {"bounds":{"minX":113.80,"minY":34.70,"maxX":114.10,"maxY":35.00},"expectedStrategy":"rtree"},
 {"bounds":{"minX":113.90,"minY":34.80,"maxX":114.30,"maxY":35.20},"expectedStrategy":"rtree"}
]'
county_steps='[
 {"bounds":{"minX":110.35,"minY":31.38,"maxX":116.65,"maxY":36.37},"expectedStrategy":"envelope_cache"},
 {"bounds":{"minX":111.5,"minY":32.0,"maxX":114.5,"maxY":34.5},"expectedStrategy":"envelope_cache"},
 {"bounds":{"minX":113.0,"minY":33.0,"maxX":115.0,"maxY":35.0},"expectedStrategy":"envelope_cache"}
]'
sample_steps='[
 {"bounds":{"minX":115.43,"minY":38.56,"maxX":118.08,"maxY":41.05},"expectedStrategy":"envelope_cache"},
 {"bounds":{"minX":115.8,"minY":38.9,"maxX":117.5,"maxY":40.4},"expectedStrategy":"envelope_cache","hideLayers":["BaseMap_L"]},
 {"bounds":{"minX":116.1,"minY":39.1,"maxX":117.2,"maxY":40.1},"expectedStrategy":"envelope_cache","showLayers":["BaseMap_L"]},
 {"bounds":{"minX":116.4,"minY":39.4,"maxX":117.0,"maxY":39.9},"expectedStrategy":"envelope_cache","removeLayers":["BaseMap_P"]}
]'

for temperature in cold warm; do
  for iteration in 1 2 3 4 5; do
    run_iteration "henan-weibo-rtree-pan-zoom" "$henan_data" '["weibo"]' "weibo" 1 "$weibo_steps" "$temperature" "$iteration"
    run_iteration "henan-county-envelope-selection" "$henan_data" '["县级行政区划"]' "县级行政区划" 2 "$county_steps" "$temperature" "$iteration"
    run_iteration "sampledata-multilayer-viewport" "$sample_data" '["BaseMap_P","BaseMap_L","BaseMap_R","CADDT"]' "BaseMap_R" 1 "$sample_steps" "$temperature" "$iteration"
  done
done

summary_args=(--input-dir "$raw_dir" --json-out "$output_dir/summary.json" --markdown-out "$output_dir/summary.md")
[[ -z "$acceptance_report" ]] || summary_args+=(--acceptance-report "$acceptance_report")
node "$script_dir/summarize-viewer-benchmark.mjs" "${summary_args[@]}"
echo "Benchmark report: $output_dir/summary.md"
