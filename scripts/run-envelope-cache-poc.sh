#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 --report <absolute-path>" >&2
}

report_path=""
summary_self_test=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --report)
      report_path="${2:-}"
      shift 2
      ;;
    --summary-self-test)
      summary_self_test=true
      shift
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [[ "$summary_self_test" == false && ( -z "$report_path" || "$report_path" != /* ) ]]; then
  usage
  exit 2
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/udbx-envelope-cache-poc.XXXXXX")"
run_succeeded=false

cleanup() {
  rm -rf "$tmp_dir"
  if [[ "$run_succeeded" == false && -n "$report_path" ]]; then
    rm -f "$report_path"
  fi
}
trap cleanup EXIT

run_summary() {
  SAMPLES_FILE="$1" \
  REPORT_PATH="$2" \
  GO_VERSION="${3:-go version self-test}" \
  MACOS_VERSION="${4:-self-test}" \
  CPU_MODEL="${5:-self-test}" \
  GIT_COMMIT="${6:-0000000000000000000000000000000000000000}" \
  RUN_STARTED_AT="${7:-2026-01-01T00:00:00Z}" \
  SUMMARY_SELF_TEST="${8:-0}" \
  node <<'NODE'
const fs = require('fs');
const path = require('path');

const sizes = [10000, 50000, 100000, 250000, 500000];
const phases = ['build/cold', 'build/hot', 'filter', 'load', 'cancel'];
const requiredBaseFields = ['sample', 'maxRssBytes', 'size', 'phase', 'nsOp', 'bytesOp', 'allocsOp'];

function finiteNonNegative(value, label) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) {
    throw new Error(`${label} must be finite and non-negative, got ${String(value)}`);
  }
}

function parseMetrics(tokens, lineNumber) {
  if ((tokens.length - 4) % 2 !== 0) {
    throw new Error(`line ${lineNumber}: metric tokens are not value/unit pairs`);
  }
  const metrics = {};
  for (let index = 4; index < tokens.length; index += 2) {
    const value = Number(tokens[index]);
    const unit = tokens[index + 1];
    if (!unit || Object.hasOwn(metrics, unit)) {
      throw new Error(`line ${lineNumber}: invalid or duplicate metric unit ${String(unit)}`);
    }
    finiteNonNegative(value, `line ${lineNumber} metric ${unit}`);
    metrics[unit] = value;
  }
  return metrics;
}

function parseRecord(line, lineNumber) {
  const fields = line.split('\t');
  if (fields.length < 3) {
    throw new Error(`line ${lineNumber}: expected sample, max RSS, and benchmark output`);
  }
  const sampleText = fields.shift();
  const maxRssText = fields.shift();
  const benchmarkLine = fields.join(' ');
  const tokens = benchmarkLine.trim().split(/\s+/);
  if (tokens.length < 8 || tokens[3] !== 'ns/op') {
    throw new Error(`line ${lineNumber}: unexpected benchmark format`);
  }
  const benchmarkName = tokens[0].replace(/-\d+$/, '');
  const match = benchmarkName.match(/^BenchmarkEnvelopeCachePOC\/(\d+)\/(build\/cold|build\/hot|filter|load|cancel)$/);
  if (!match) {
    throw new Error(`line ${lineNumber}: unexpected benchmark name ${benchmarkName}`);
  }
  const metrics = parseMetrics(tokens, lineNumber);
  const record = {
    sample: Number(sampleText),
    maxRssBytes: Number(maxRssText),
    size: Number(match[1]),
    phase: match[2],
    nsOp: Number(tokens[2]),
    bytesOp: metrics['B/op'],
    allocsOp: metrics['allocs/op'],
    cacheMiB: metrics.cache_mib,
    stableRssDeltaMiB: metrics.stable_rss_delta_mib,
    cancelReleased: metrics.cancel_released,
  };
  return record;
}

function validateRecords(records) {
  if (records.length !== 20 * sizes.length * phases.length) {
    throw new Error(`expected 500 records, got ${records.length}`);
  }
  const seen = new Set();
  for (const [index, record] of records.entries()) {
    for (const field of requiredBaseFields) {
      if (!Object.hasOwn(record, field)) {
        throw new Error(`record ${index + 1}: missing ${field}`);
      }
    }
    if (!Number.isInteger(record.sample) || record.sample < 1 || record.sample > 20) {
      throw new Error(`record ${index + 1}: sample must be an integer from 1 through 20`);
    }
    if (!sizes.includes(record.size) || !phases.includes(record.phase)) {
      throw new Error(`record ${index + 1}: unsupported size or phase`);
    }
    for (const field of ['maxRssBytes', 'size', 'nsOp', 'bytesOp', 'allocsOp']) {
      finiteNonNegative(record[field], `record ${index + 1} ${field}`);
    }
    if (record.phase === 'build/cold') {
      finiteNonNegative(record.cacheMiB, `record ${index + 1} cacheMiB`);
      finiteNonNegative(record.stableRssDeltaMiB, `record ${index + 1} stableRssDeltaMiB`);
    }
    if (record.phase === 'build/hot') {
      finiteNonNegative(record.cacheMiB, `record ${index + 1} cacheMiB`);
    }
    if (record.phase === 'cancel' && record.cancelReleased !== 1) {
      throw new Error(`record ${index + 1}: cancel_released must equal 1`);
    }
    const key = `${record.sample}/${record.size}/${record.phase}`;
    if (seen.has(key)) {
      throw new Error(`duplicate record ${key}`);
    }
    seen.add(key);
  }
  for (const size of sizes) {
    for (const phase of phases) {
      const group = records.filter((record) => record.size === size && record.phase === phase);
      if (group.length !== 20) {
        throw new Error(`expected 20 samples for ${size}/${phase}, got ${group.length}`);
      }
      const sampleSet = new Set(group.map((record) => record.sample));
      if (sampleSet.size !== 20) {
        throw new Error(`samples for ${size}/${phase} are not exactly 1 through 20`);
      }
    }
  }
  return records;
}

function parseAndValidate(text) {
  if (typeof text !== 'string' || text.trim() === '') {
    throw new Error('benchmark output is empty');
  }
  return validateRecords(text.trim().split('\n').map(parseRecord));
}

function percentile(values, fraction) {
  const sorted = [...values].sort((left, right) => left - right);
  return sorted[Math.max(0, Math.ceil(sorted.length * fraction) - 1)];
}

function stats(values) {
  return {p50: percentile(values, 0.50), p95: percentile(values, 0.95)};
}

function phaseRecords(records, size, phase) {
  return records.filter((record) => record.size === size && record.phase === phase);
}

function atomicWrite(reportPath, content) {
  const directory = path.dirname(reportPath);
  const temporaryPath = path.join(directory, `.${path.basename(reportPath)}.${process.pid}.tmp`);
  let descriptor;
  try {
    descriptor = fs.openSync(temporaryPath, 'wx', 0o644);
    fs.writeFileSync(descriptor, content, 'utf8');
    fs.fsyncSync(descriptor);
    fs.closeSync(descriptor);
    descriptor = undefined;
    fs.renameSync(temporaryPath, reportPath);
  } catch (error) {
    if (descriptor !== undefined) {
      fs.closeSync(descriptor);
    }
    fs.rmSync(temporaryPath, {force: true});
    throw error;
  }
}

function syntheticTSV() {
  const lines = [];
  for (let sample = 1; sample <= 20; sample++) {
    for (const size of sizes) {
      const common = `${sample}\t104857600\t`;
      lines.push(`${common}BenchmarkEnvelopeCachePOC/${size}/build/cold-12\t1\t1000000 ns/op\t1 cache_mib\t2 stable_rss_delta_mib\t100 B/op\t10 allocs/op`);
      lines.push(`${common}BenchmarkEnvelopeCachePOC/${size}/build/hot-12\t1\t900000 ns/op\t1 cache_mib\t100 B/op\t10 allocs/op`);
      lines.push(`${common}BenchmarkEnvelopeCachePOC/${size}/filter-12\t1\t1000 ns/op\t10 B/op\t1 allocs/op`);
      lines.push(`${common}BenchmarkEnvelopeCachePOC/${size}/load-12\t1\t2000 ns/op\t20 B/op\t2 allocs/op`);
      lines.push(`${common}BenchmarkEnvelopeCachePOC/${size}/cancel-12\t1\t3000 ns/op\t1 cancel_released\t30 B/op\t3 allocs/op`);
    }
  }
  return lines.join('\n');
}

function runSelfTest(reportPath) {
  const valid = syntheticTSV();
  const validLines = valid.split('\n');
  const duplicateLines = [...validLines];
  duplicateLines[1] = duplicateLines[0];
  const cases = [
    ['duplicate sample', duplicateLines.join('\n')],
    ['NaN metric', valid.replace('1000000 ns/op', 'NaN ns/op')],
    ['missing field', valid.replace('\t2 stable_rss_delta_mib', '')],
  ];
  const records = parseAndValidate(valid);
  atomicWrite(reportPath, JSON.stringify({records: records.length}));
  if (!fs.existsSync(reportPath)) {
    throw new Error('valid input did not generate output');
  }
  fs.rmSync(reportPath);
  for (const [name, input] of cases) {
    let rejected = false;
    try {
      parseAndValidate(input);
    } catch (_error) {
      rejected = true;
    }
    if (!rejected || fs.existsSync(reportPath)) {
      throw new Error(`${name} input was not rejected safely`);
    }
  }
  process.stdout.write('summary self-test passed: valid accepted; duplicate/NaN/missing rejected\n');
}

function buildReport(records) {
  const round = (value, digits = 3) => Number(value.toFixed(digits));
  const formatMs = (value) => (value / 1e6).toFixed(3);
  const formatNumber = (value) => Math.round(value).toLocaleString('en-US');
  const formatMiB = (value) => value.toFixed(3);
  const generatedAt = new Date().toISOString().replace(/\.\d{3}Z$/, 'Z');

  const decisions = sizes.map((size) => {
    const cold = phaseRecords(records, size, 'build/cold');
    const hot = phaseRecords(records, size, 'build/hot');
    const filter = phaseRecords(records, size, 'filter');
    const load = phaseRecords(records, size, 'load');
    const cancel = phaseRecords(records, size, 'cancel');
    const buildP95Ms = percentile(cold.map((record) => record.nsOp), 0.95) / 1e6;
    const filterP95Ms = percentile(filter.map((record) => record.nsOp), 0.95) / 1e6;
    const stableRssDeltaMiB = percentile(cold.map((record) => record.stableRssDeltaMiB), 0.95);
    const cancelReleased = cancel.every((record) => record.cancelReleased === 1);
    return {
      size,
      buildP95Ms: round(buildP95Ms),
      buildHotP95Ms: round(percentile(hot.map((record) => record.nsOp), 0.95) / 1e6),
      filterP95Ms: round(filterP95Ms),
      candidateLoadP95Ms: round(percentile(load.map((record) => record.nsOp), 0.95) / 1e6),
      cacheCapacityMiB: round(percentile(cold.map((record) => record.cacheMiB), 0.95)),
      rssDeltaMiB: round(stableRssDeltaMiB),
      stableRssDeltaMiB: round(stableRssDeltaMiB),
      externalMaxRssMiB: round(percentile(cold.map((record) => record.maxRssBytes / (1024 * 1024)), 0.95)),
      cancelReleased,
      accepted: buildP95Ms <= 500 && filterP95Ms <= 100 && stableRssDeltaMiB <= 32 && cancelReleased,
    };
  });

  const decisionRows = decisions.map((decision) => {
    const cold = stats(phaseRecords(records, decision.size, 'build/cold').map((record) => record.nsOp));
    const hot = stats(phaseRecords(records, decision.size, 'build/hot').map((record) => record.nsOp));
    const filter = stats(phaseRecords(records, decision.size, 'filter').map((record) => record.nsOp));
    const load = stats(phaseRecords(records, decision.size, 'load').map((record) => record.nsOp));
    const capacity = stats(phaseRecords(records, decision.size, 'build/cold').map((record) => record.cacheMiB));
    const rss = stats(phaseRecords(records, decision.size, 'build/cold').map((record) => record.stableRssDeltaMiB));
    const maxRss = stats(phaseRecords(records, decision.size, 'build/cold').map((record) => record.maxRssBytes / (1024 * 1024)));
    return `| ${formatNumber(decision.size)} | ${formatMs(cold.p50)} / ${formatMs(cold.p95)} | ${formatMs(hot.p50)} / ${formatMs(hot.p95)} | ${formatMs(filter.p50)} / ${formatMs(filter.p95)} | ${formatMs(load.p50)} / ${formatMs(load.p95)} | ${formatMiB(capacity.p50)} / ${formatMiB(capacity.p95)} | ${formatMiB(rss.p50)} / ${formatMiB(rss.p95)} | ${formatMiB(maxRss.p50)} / ${formatMiB(maxRss.p95)} | ${decision.cancelReleased ? '是' : '否'} | ${decision.accepted ? '通过' : '不通过'} |`;
  });

  const phaseLabel = {'build/cold': 'build cold', 'build/hot': 'build hot', filter: 'filter', load: 'candidate load', cancel: 'cancel'};
  const detailRows = [];
  for (const size of sizes) {
    for (const phase of phases) {
      const group = phaseRecords(records, size, phase);
      const duration = stats(group.map((record) => record.nsOp));
      const bytes = stats(group.map((record) => record.bytesOp));
      const allocs = stats(group.map((record) => record.allocsOp));
      detailRows.push(`| ${formatNumber(size)} | ${phaseLabel[phase]} | ${formatMs(duration.p50)} | ${formatMs(duration.p95)} | ${formatNumber(bytes.p50)} | ${formatNumber(bytes.p95)} | ${formatNumber(allocs.p50)} | ${formatNumber(allocs.p95)} |`);
    }
  }

  const acceptedSizes = decisions.filter((decision) => decision.accepted).map((decision) => formatNumber(decision.size));
  const conclusion = acceptedSizes.length > 0
    ? `本机五档中通过的规模：${acceptedSizes.join('、')}。准入仍由内存预算和实测时延共同决定，不设置对象数硬阈值。`
    : '本机五档均未通过。首期无索引数据集全部采用有界降级，已有 RTree 查询路径继续启用。';

  return `# udbx-viewer 无索引包络缓存 PoC 报告

日期：${generatedAt.slice(0, 10)}

## 1. 运行环境

- 运行开始时间：${process.env.RUN_STARTED_AT}
- 报告生成时间：${generatedAt}
- Go：\`${process.env.GO_VERSION}\`
- macOS：\`${process.env.MACOS_VERSION}\`
- CPU：\`${process.env.CPU_MODEL}\`
- Git commit：\`${process.env.GIT_COMMIT}\`
- 测量次数：每个规模固定 20 次；每个 \`(sample,size)\` 使用独立 \`go test\` 进程，\`-benchtime=1x\`，每个进程只运行一个规模。
- 规模：10,000、50,000、100,000、250,000、500,000。

## 2. 测量方法

- 脚本先在测量进程外生成五个临时 SQLite 夹具。benchmark 进程以 \`mode=ro&immutable=1\` 只读打开对应规模数据库，不执行夹具写入。
- 夹具表为 \`poc_points\`，不创建 \`idx_poc_points_SmGeometry\` 或任何 RTree。每条 \`SmGeometry\` 使用真实 GAIA Point 编码。
- 每个 benchmark 进程只测量一个规模。\`build cold\` 前执行 GC/释放空闲页并采集三次当前 RSS 中位数；构建后保持 cache 存活，再以相同方式采集稳定态 RSS。
- \`stableRssDeltaMiB\` 是构建后稳定态 RSS 减构建前基线，不是进程绝对 RSS；负抖动按 0 计。\`externalMaxRssMiB\` 是 macOS \`/usr/bin/time -l\` 记录的该隔离进程绝对峰值，单独报告且不作为增量。
- 缓存容量按 \`cap(slice) * unsafe.Sizeof(pocEnvelopeEntry{})\` 单独报告。正式预算按经验稳定 RSS charge 计费：每数据集固定约 4 MiB，加每个 capacity entry 约 80 bytes；80 bytes 来自 250k/500k 稳定 RSS P95 斜率拟合。对象数只用于预估 capacity，不作为准入阈值。
- \`filter\` 复用完整连续缓存，固定视口命中正好 1%。\`candidate load\` 按 500 个 ID 一批读取完整 geometry，并通过真实 \`GaiaGeometryCodec\` 解码。
- \`cancel\` 在扫描达到约 10% 时取消 context，要求返回 \`context.Canceled\`、不发布残缺缓存，并可继续复用同一单连接数据库。

## 3. 资源策略判定

准入条件：\`build cold P95 <= 500 ms\`、\`filter P95 <= 100 ms\`、稳定态 RSS 增量 \`P95 <= 32 MiB\`、20 次取消均释放资源。当前文件全部包络缓存预算上限保持 64 MiB。

| 规模 | Build cold P50/P95 ms | Build hot P50/P95 ms | Filter P50/P95 ms | Candidate load P50/P95 ms | Cache capacity P50/P95 MiB | 稳定态 RSS 增量 P50/P95 MiB | 外部最大 RSS P50/P95 MiB | Cancel released | 判定 |
|---:|---:|---:|---:|---:|---:|---:|---:|:---:|:---:|
${decisionRows.join('\n')}

${conclusion}

候选 geometry 读取保留 500 ID 分批策略。默认资源策略保持单数据集 32 MiB、当前文件全部包络缓存 64 MiB，并按经验稳定 RSS charge 计费。500,000 档若只看 19.070 MiB raw capacity 可能通过，但约 42.15 MiB 的 RSS charge 与 42.340 MiB 实测 P95 都不能通过 32 MiB 门；拒绝来自资源公式，不是硬编码对象数。

## 4. 机器判定 JSON

\`accepted\` 只由首次构建、过滤、稳定态 RSS 增量和取消释放四项条件计算；外部最大 RSS 与候选加载时延单独记录。

\`\`\`json
${JSON.stringify(decisions, null, 2)}
\`\`\`

## 5. 原始 benchmark 汇总

| 规模 | 阶段 | ns/op P50 (ms) | ns/op P95 (ms) | B/op P50 | B/op P95 | allocs/op P50 | allocs/op P95 |
|---:|---|---:|---:|---:|---:|---:|---:|
${detailRows.join('\n')}

原始逐次输出和临时夹具在报告原子发布后删除，不进入仓库。
`;
}

function main() {
  if (process.env.SUMMARY_SELF_TEST === '1') {
    runSelfTest(process.env.REPORT_PATH);
    return;
  }
  const records = parseAndValidate(fs.readFileSync(process.env.SAMPLES_FILE, 'utf8'));
  atomicWrite(process.env.REPORT_PATH, buildReport(records));
  const decisions = sizes.map((size) => {
    const cold = phaseRecords(records, size, 'build/cold');
    return {size, stableRssDeltaP95MiB: percentile(cold.map((record) => record.stableRssDeltaMiB), 0.95)};
  });
  process.stdout.write(`${JSON.stringify(decisions)}\n`);
}

main();
NODE
}

if [[ "$summary_self_test" == true ]]; then
  self_test_report="$tmp_dir/summary-self-test.md"
  run_summary "$tmp_dir/unused.tsv" "$self_test_report" "" "" "" "" "" 1
  [[ ! -e "$self_test_report" ]]
  run_succeeded=true
  exit 0
fi

report_dir="$(dirname "$report_path")"
mkdir -p "$report_dir"
rm -f "$report_path"

samples_file="$tmp_dir/samples.tsv"
: > "$samples_file"
go_version="$(go version)"
macos_version="$(sw_vers -productVersion)"
cpu_model="$(sysctl -n machdep.cpu.brand_string)"
git_commit="$(git -C "$repo_root" rev-parse HEAD)"
run_started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
sizes=(10000 50000 100000 250000 500000)

cd "$repo_root"
for size in "${sizes[@]}"; do
  fixture_path="$tmp_dir/envelope-cache-poc-$size.db"
  UDBX_ENVELOPE_CACHE_POC_GENERATE_PATH="$fixture_path" \
  UDBX_ENVELOPE_CACHE_POC_SIZE="$size" \
    go test ./internal/dataset -run '^TestEnvelopeCachePOCGenerateFixture$' -count=1 >/dev/null
done

for sample in $(seq 1 20); do
  for size in "${sizes[@]}"; do
    stdout_file="$tmp_dir/benchmark-$sample-$size.out"
    stderr_file="$tmp_dir/benchmark-$sample-$size.err"
    fixture_path="$tmp_dir/envelope-cache-poc-$size.db"

    if ! UDBX_ENVELOPE_CACHE_POC_FIXTURE_PATH="$fixture_path" \
      UDBX_ENVELOPE_CACHE_POC_SIZE="$size" \
      /usr/bin/time -l go test ./internal/dataset \
        -run '^$' \
        -bench '^BenchmarkEnvelopeCachePOC$' \
        -benchmem \
        -benchtime=1x \
        -count=1 \
        >"$stdout_file" 2>"$stderr_file"; then
      cat "$stdout_file" >&2
      cat "$stderr_file" >&2
      exit 1
    fi

    max_rss_bytes="$(awk '/maximum resident set size/ { value = $1 } END { print value }' "$stderr_file")"
    if [[ -z "$max_rss_bytes" ]]; then
      echo "failed to read maximum resident set size for sample $sample size $size" >&2
      exit 1
    fi
    awk -v sample="$sample" -v max_rss="$max_rss_bytes" \
      '/^BenchmarkEnvelopeCachePOC\// { print sample "\t" max_rss "\t" $0 }' \
      "$stdout_file" >> "$samples_file"
  done
  printf 'Envelope cache PoC isolated sample %d/20 complete\n' "$sample"
done

run_summary "$samples_file" "$report_path" "$go_version" "$macos_version" "$cpu_model" "$git_commit" "$run_started_at" 0
run_succeeded=true
echo "Envelope cache PoC report: $report_path"
