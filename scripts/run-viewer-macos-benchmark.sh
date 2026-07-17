#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
source "$script_dir/viewer-benchmark-rss.sh"
source "$script_dir/viewer-benchmark-process.sh"
source "$script_dir/viewer-benchmark-transaction.sh"
sample_data="$repo_root/../data/SampleData.udbx"
henan_data="$repo_root/../data/henan.udbx"
output_dir="$repo_root/.benchmark-results/$(date +%Y%m%d-%H%M%S)"
acceptance_report=""
max_concurrent=""
mock_fixtures=""
skip_build=false

usage() {
  echo "Usage: $0 [--sample-data PATH] [--henan-data PATH] [--output-dir PATH] [--max-concurrent 1|2|3] [--acceptance-report ABSOLUTE_PATH] [--skip-build] [--mock-fixtures DIR]"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sample-data) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; sample_data="$2"; shift 2 ;;
    --henan-data) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; henan_data="$2"; shift 2 ;;
    --output-dir) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; output_dir="$2"; shift 2 ;;
    --max-concurrent) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; max_concurrent="$2"; shift 2 ;;
    --acceptance-report) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; acceptance_report="$2"; shift 2 ;;
    --mock-fixtures) [[ $# -ge 2 ]] || { usage >&2; exit 2; }; mock_fixtures="$2"; shift 2 ;;
    --skip-build) skip_build=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ -z "$max_concurrent" || "$max_concurrent" =~ ^[123]$ ]] || { echo "--max-concurrent must be 1, 2 or 3" >&2; exit 2; }
if [[ -n "$acceptance_report" && "$acceptance_report" != /* ]]; then
  echo "--acceptance-report must be an absolute path" >&2
  exit 2
fi
if [[ -z "$max_concurrent" && "$skip_build" == true ]]; then
  echo "--skip-build is only valid with --max-concurrent single-suite mode" >&2
  exit 2
fi

for command_name in jq node shasum awk python3; do
  command -v "$command_name" >/dev/null || { echo "Missing required command: $command_name" >&2; exit 2; }
done

output_dir="$(mkdir -p "$output_dir" && cd "$output_dir" && pwd)"
viewer_dir="$repo_root/cmd/udbx4go-viewer"
policy_file="$viewer_dir/frontend/src/spatial/viewportQueryPolicy.ts"
mock_runner=""

if [[ -n "$mock_fixtures" ]]; then
  mock_fixtures="$(cd "$mock_fixtures" && pwd)"
  mock_runner="$mock_fixtures/mock-runner.mjs"
  sample_data="$mock_fixtures/SampleData.udbx"
  henan_data="$mock_fixtures/henan.udbx"
  [[ -f "$mock_runner" ]] || { echo "Mock runner not found: $mock_runner" >&2; exit 2; }
  mock_workspace="$output_dir/mock-workspace"
  policy_file="$mock_workspace/frontend/src/spatial/viewportQueryPolicy.ts"
  mkdir -p "$(dirname "$policy_file")"
  cp "$viewer_dir/frontend/src/spatial/viewportQueryPolicy.ts" "$policy_file"
  app_path="$mock_workspace/build/bin/udbx4go-viewer-wails.app"
else
  for command_name in wails ps sw_vers sysctl; do
    command -v "$command_name" >/dev/null || { echo "Missing required command: $command_name" >&2; exit 2; }
  done
  app_path="$viewer_dir/build/bin/udbx4go-viewer-wails.app"
fi
executable="$app_path/Contents/MacOS/udbx4go-viewer-wails"
policy_backup="$output_dir/.viewport-query-policy.original"
begin_benchmark_policy_transaction "$policy_file" "$policy_backup"
workflow_phase="setup"

handle_workflow_exit() {
  local exit_status="$1"
  trap - EXIT
  cleanup_benchmark_process_group
  restore_benchmark_policy_transaction
  exit "$exit_status"
}

trap 'exit 130' INT
trap 'exit 143' TERM
trap 'handle_workflow_exit $?' EXIT

sample_data="$(cd "$(dirname "$sample_data")" && pwd)/$(basename "$sample_data")"
henan_data="$(cd "$(dirname "$henan_data")" && pwd)/$(basename "$henan_data")"
for sample_path in "$sample_data" "$henan_data"; do
  [[ -f "$sample_path" ]] || { echo "Sample file not found: $sample_path" >&2; exit 2; }
done

file_size() {
  stat -f %z "$1" 2>/dev/null || stat -c %s "$1"
}

build_viewer() {
  if [[ -n "$mock_fixtures" ]]; then
    local build_count_file="$output_dir/mock-build-count"
    local build_count=0 concurrency
    [[ ! -f "$build_count_file" ]] || build_count="$(<"$build_count_file")"
    build_count=$((build_count + 1))
    printf '%s\n' "$build_count" > "$build_count_file"
    if [[ "${UDBX_BENCHMARK_MOCK_FAIL_STAGE:-}" == "build" && "$build_count" == 1 ]]; then
      echo "mock build failure" >&2
      return 91
    fi
    concurrency="$(awk '/VIEWPORT_QUERY_MAX_CONCURRENCY/ { print $NF }' "$policy_file")"
    mkdir -p "$(dirname "$executable")"
    printf '#!/usr/bin/env bash\n# mock app build %s policy %s\nexit 0\n' "$build_count" "$concurrency" > "$executable"
    chmod +x "$executable"
    return
  fi
  (cd "$viewer_dir" && wails build -platform darwin/universal)
}

if [[ -n "$mock_fixtures" ]]; then
  macos_version="mock-macos-1"
  cpu="mock-cpu"
  memory_bytes=17179869184
else
  macos_version="$(sw_vers -productVersion)"
  cpu="$(sysctl -n machdep.cpu.brand_string 2>/dev/null || sysctl -n hw.model)"
  memory_bytes="$(sysctl -n hw.memsize)"
fi
git_commit="$(git -C "$repo_root" rev-parse HEAD)"
sample_sha="$(shasum -a 256 "$sample_data" | awk '{print $1}')"
henan_sha="$(shasum -a 256 "$henan_data" | awk '{print $1}')"
sample_size="$(file_size "$sample_data")"
henan_size="$(file_size "$henan_data")"
app_sha=""

refresh_app_identity() {
  [[ -x "$executable" ]] || { echo "Viewer executable not found after build: $executable" >&2; exit 2; }
  app_sha="$(shasum -a 256 "$executable" | awk '{print $1}')"
}

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
  local result_path="$1" run_id="$2" scenario_name="$3" error_message="$4" exit_code="$5"
  jq -n --arg runId "$run_id" --arg scenario "$scenario_name" \
    --arg startedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --arg error "$error_message" --argjson processExitCode "$exit_code" \
    '{runId:$runId,status:"failed",startedAt:$startedAt,scenario:$scenario,
      metrics:{openFileMs:0,loadLayersMs:0,fitVisibleLayersMs:0,selectAndFitMs:0,
      backendQueryMs:[],moveendToRenderMs:[],maxConcurrentQueries:0,pendingPeak:0,
      pendingFinal:0,staleResultsDiscarded:0,staleResultApplied:false,
      finalFeatureCount:0,blankRenderCount:0},error:$error,processExitCode:$processExitCode}' > "$result_path"
}

run_iteration() {
  local suite_dir="$1" concurrency="$2" scenario_name="$3" file_path="$4" layers_json="$5"
  local selection_dataset="$6" selection_page="$7" viewport_steps_json="$8" temperature="$9" iteration="${10}"
  local config_dir="$suite_dir/configs" raw_dir="$suite_dir/raw"
  local run_id="${scenario_name}-${temperature}-${iteration}"
  local config_path="$config_dir/${run_id}.json" result_path="$raw_dir/${run_id}.json"

  jq -n --arg runId "$run_id" --arg outputPath "$result_path" --arg temperature "$temperature" \
    --argjson maxConcurrentQueries "$concurrency" --arg name "$scenario_name" \
    --arg filePath "$file_path" --argjson layers "$layers_json" \
    --arg datasetName "$selection_dataset" --argjson page "$selection_page" \
    --argjson viewportSteps "$viewport_steps_json" \
    '{runId:$runId,outputPath:$outputPath,temperature:$temperature,
      maxConcurrentQueries:$maxConcurrentQueries,scenario:{name:$name,filePath:$filePath,
      layers:$layers,selection:{datasetName:$datasetName,page:$page,rowIndex:0},viewportSteps:$viewportSteps}}' \
    > "$config_path"

  local peak_rss=0 rss_first=0 rss_start=0 rss_end=0 process_exit_code=0 timed_out=false
  local log_path="$suite_dir/${run_id}.log"
  if [[ -n "$mock_fixtures" ]]; then
    start_benchmark_process_group "$log_path" node "$mock_runner" "$config_path"
    if wait_benchmark_process_group; then process_exit_code=0; else process_exit_code=$?; fi
    case "$concurrency" in
      1) peak_rss=200000 ;;
      2) peak_rss=205000 ;;
      3) peak_rss=230000 ;;
    esac
    rss_start=180000
    rss_end=185000
  else
    start_benchmark_process_group "$log_path" "$executable" --benchmark-config "$config_path"
    local app_pid="$active_benchmark_pid" app_pgid="$active_benchmark_pgid" started_seconds=$SECONDS rss_sample_count=0
    while benchmark_process_group_alive "$app_pgid"; do
      local current_rss elapsed
      current_rss="$(process_tree_rss "$app_pid")"
      elapsed="$(rss_elapsed_seconds "$rss_sample_count")"
      record_rss_sample "$current_rss" "$elapsed"
      rss_sample_count=$((rss_sample_count + 1))
      if (( elapsed >= 120 )); then
        terminate_benchmark_process_group "$app_pgid" 20
        process_exit_code=124
        timed_out=true
        break
      fi
      sleep 0.1
    done
    if [[ "$timed_out" != true ]]; then
      if wait_benchmark_process_group; then process_exit_code=0; else process_exit_code=$?; fi
    fi
  fi

  local exit_error=""
  if [[ "$timed_out" == true ]]; then
    exit_error="benchmark timed out after 120 seconds"
  elif (( process_exit_code != 0 )); then
    exit_error="viewer exited with exit code $process_exit_code"
  fi
  if [[ ! -f "$result_path" ]]; then
    [[ -n "$exit_error" ]] || exit_error="viewer exited before writing a result"
    write_failed_result "$result_path" "$run_id" "$scenario_name" "$exit_error" "$process_exit_code"
  elif (( process_exit_code != 0 )); then
    jq --arg error "$exit_error" --argjson processExitCode "$process_exit_code" \
      '.status="failed" | .error=$error | .processExitCode=$processExitCode' \
      "$result_path" > "$result_path.exit-failed"
    mv "$result_path.exit-failed" "$result_path"
  fi

  finalize_rss_samples

  local input_sha="$sample_sha" input_size="$sample_size"
  if [[ "$file_path" == "$henan_data" ]]; then input_sha="$henan_sha"; input_size="$henan_size"; fi
  local memory_error=""
  (( peak_rss > 0 && rss_start > 0 && rss_end > 0 )) || memory_error="no RSS sample captured"

  jq --argjson iteration "$iteration" --arg temperature "$temperature" \
    --argjson peakRssKiB "$peak_rss" --argjson rssStartKiB "$rss_start" --argjson rssEndKiB "$rss_end" \
    --arg memoryCaptureError "$memory_error" --arg appPath "$app_path" \
    --argjson processExitCode "$process_exit_code" \
    --argjson maxConcurrentQueries "$concurrency" --arg gitCommit "$git_commit" --arg appSha256 "$app_sha" \
    --arg macOSVersion "$macos_version" --arg cpu "$cpu" --argjson memoryBytes "$memory_bytes" \
    --arg samplePath "$file_path" --arg sampleSha256 "$input_sha" --argjson sampleSizeBytes "$input_size" \
    '.+{iteration:$iteration,temperature:$temperature,peakRssKiB:$peakRssKiB,
      rssStartKiB:$rssStartKiB,rssEndKiB:$rssEndKiB,memoryCaptureError:$memoryCaptureError,
      appPath:$appPath,maxConcurrentQueries:$maxConcurrentQueries,processExitCode:$processExitCode,
      environment:{gitCommit:$gitCommit,appSha256:$appSha256,macOSVersion:$macOSVersion,cpu:$cpu,memoryBytes:$memoryBytes,
      samplePath:$samplePath,sampleSha256:$sampleSha256,sampleSizeBytes:$sampleSizeBytes}}' \
    "$result_path" > "$result_path.enriched"
  mv "$result_path.enriched" "$result_path"
  (( process_exit_code == 0 ))
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

run_suite() {
  local concurrency="$1" suite_dir="$2"
  shift 2
  mkdir -p "$suite_dir/configs" "$suite_dir/raw"
  if [[ "$workflow_phase" == "final" && "${UDBX_BENCHMARK_MOCK_FAIL_STAGE:-}" == "final" ]]; then
    echo "mock final run failure" >&2
    return 92
  fi
  for temperature in cold warm; do
    for iteration in 1 2 3 4 5; do
      run_iteration "$suite_dir" "$concurrency" "henan-weibo-rtree-pan-zoom" "$henan_data" '["weibo"]' "weibo" 1 "$weibo_steps" "$temperature" "$iteration"
      run_iteration "$suite_dir" "$concurrency" "henan-county-envelope-selection" "$henan_data" '["县级行政区划"]' "县级行政区划" 2 "$county_steps" "$temperature" "$iteration"
      run_iteration "$suite_dir" "$concurrency" "sampledata-multilayer-viewport" "$sample_data" '["BaseMap_P","BaseMap_L","BaseMap_R","CADDT"]' "BaseMap_R" 1 "$sample_steps" "$temperature" "$iteration"
    done
  done
  node "$script_dir/summarize-viewer-benchmark.mjs" \
    --input-dir "$suite_dir/raw" --json-out "$suite_dir/summary.json" --markdown-out "$suite_dir/summary.md" "$@"
}

candidate_suite_completed() {
  local summary_path="$1/summary.json"
  [[ -f "$summary_path" ]] && jq -e '
    .completeTenRunGate == true and
    ([.scenarios[].runs[].status] | all(. == "passed"))
  ' "$summary_path" >/dev/null
}

if [[ -n "$max_concurrent" ]]; then
  workflow_phase="single"
  if [[ "$skip_build" != true ]]; then build_viewer; fi
  refresh_app_identity
  if [[ -n "$acceptance_report" ]]; then
    run_suite "$max_concurrent" "$output_dir" --acceptance-report "$acceptance_report"
  else
    run_suite "$max_concurrent" "$output_dir"
  fi
  echo "Benchmark report: $output_dir/summary.md"
  exit 0
fi

node "$script_dir/set-viewer-concurrency.mjs" --file "$policy_file" --value 1
workflow_phase="candidate-build"
build_viewer
refresh_app_identity
mkdir -p "$output_dir/candidates"
for concurrency in 1 2 3; do
  workflow_phase="candidate"
  candidate_dir="$output_dir/candidates/concurrency-$concurrency"
  if ! run_suite "$concurrency" "$candidate_dir"; then
    if candidate_suite_completed "$candidate_dir"; then
      echo "Candidate concurrency $concurrency did not meet performance gates; continuing calibration." >&2
    else
      exit 1
    fi
  fi
done

node "$script_dir/summarize-viewer-benchmark.mjs" \
  --select-baseline "$output_dir/candidates/concurrency-1/summary.json" \
  --select-candidate-2 "$output_dir/candidates/concurrency-2/summary.json" \
  --select-candidate-3 "$output_dir/candidates/concurrency-3/summary.json" \
  --json-out "$output_dir/selection.json" --markdown-out "$output_dir/selection.md"
selected_concurrency="$(jq -r '.selected' "$output_dir/selection.json")"

node "$script_dir/set-viewer-concurrency.mjs" --file "$policy_file" --value "$selected_concurrency"
workflow_phase="final-build"
build_viewer
refresh_app_identity
final_args=(
  --selection-json "$output_dir/selection.json"
  --candidate-summary-1 "$output_dir/candidates/concurrency-1/summary.json"
  --candidate-summary-2 "$output_dir/candidates/concurrency-2/summary.json"
  --candidate-summary-3 "$output_dir/candidates/concurrency-3/summary.json"
)
[[ -z "$acceptance_report" ]] || final_args+=(--acceptance-report "$acceptance_report")
workflow_phase="final"
run_suite "$selected_concurrency" "$output_dir/final" "${final_args[@]}"
commit_benchmark_policy_transaction
echo "Benchmark report: $output_dir/final/summary.md"
