#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 --report <absolute-path>" >&2
}

report_path=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --report)
      report_path="${2:-}"
      shift 2
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$report_path" || "$report_path" != /* ]]; then
  usage
  exit 2
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
report_dir="$(dirname "$report_path")"
mkdir -p "$report_dir"

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/udbx-envelope-cache-poc.XXXXXX")"
trap 'rm -rf "$tmp_dir"' EXIT

samples_file="$tmp_dir/samples.tsv"
: > "$samples_file"

go_version="$(go version)"
macos_version="$(sw_vers -productVersion)"
cpu_model="$(sysctl -n machdep.cpu.brand_string)"
git_commit="$(git -C "$repo_root" rev-parse HEAD)"
generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

cd "$repo_root"

for sample in $(seq 1 20); do
  stdout_file="$tmp_dir/benchmark-$sample.out"
  stderr_file="$tmp_dir/benchmark-$sample.err"

  if ! /usr/bin/time -l go test ./internal/dataset \
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
    echo "failed to read maximum resident set size for sample $sample" >&2
    cat "$stderr_file" >&2
    exit 1
  fi

  awk -v sample="$sample" -v max_rss="$max_rss_bytes" \
    '/^BenchmarkEnvelopeCachePOC\// { print sample "\t" max_rss "\t" $0 }' \
    "$stdout_file" >> "$samples_file"
  printf 'Envelope cache PoC sample %d/20 complete\n' "$sample"
done

SAMPLES_FILE="$samples_file" \
REPORT_PATH="$report_path" \
GO_VERSION="$go_version" \
MACOS_VERSION="$macos_version" \
CPU_MODEL="$cpu_model" \
GIT_COMMIT="$git_commit" \
GENERATED_AT="$generated_at" \
node <<'NODE'
const fs = require('fs');

const samplesFile = process.env.SAMPLES_FILE;
const reportPath = process.env.REPORT_PATH;
const benchmarkText = fs.readFileSync(samplesFile, 'utf8').trim();
if (!benchmarkText) {
  throw new Error('benchmark output is empty');
}

const parseMetrics = (tokens) => {
  const metrics = {};
  for (let index = 4; index + 1 < tokens.length; index += 2) {
    metrics[tokens[index + 1]] = Number(tokens[index]);
  }
  return metrics;
};

const records = benchmarkText.split('\n').map((line) => {
  const fields = line.split('\t');
  const sampleText = fields.shift();
  const maxRssText = fields.shift();
  const benchmarkLine = fields.join(' ');
  const tokens = benchmarkLine.trim().split(/\s+/);
  const benchmarkName = tokens[0].replace(/-\d+$/, '');
  const match = benchmarkName.match(/^BenchmarkEnvelopeCachePOC\/(\d+)\/(build\/cold|build\/hot|filter|load|cancel)$/);
  if (!match) {
    throw new Error(`unexpected benchmark name: ${benchmarkName}`);
  }
  const metrics = parseMetrics(tokens);
  return {
    sample: Number(sampleText),
    maxRssBytes: Number(maxRssText),
    size: Number(match[1]),
    phase: match[2],
    nsOp: Number(tokens[2]),
    bytesOp: metrics['B/op'],
    allocsOp: metrics['allocs/op'],
    cacheMiB: metrics.cache_mib,
    rssDeltaMiB: metrics.rss_delta_mib,
    cancelReleased: metrics.cancel_released === 1,
  };
});

const sizes = [10000, 50000, 100000, 250000, 500000];
const phases = ['build/cold', 'build/hot', 'filter', 'load', 'cancel'];
for (const size of sizes) {
  for (const phase of phases) {
    const count = records.filter((record) => record.size === size && record.phase === phase).length;
    if (count !== 20) {
      throw new Error(`expected 20 samples for ${size}/${phase}, got ${count}`);
    }
  }
}

const percentile = (values, fraction) => {
  const sorted = [...values].sort((left, right) => left - right);
  const index = Math.max(0, Math.ceil(sorted.length * fraction) - 1);
  return sorted[index];
};

const stats = (values) => ({
  p50: percentile(values, 0.50),
  p95: percentile(values, 0.95),
});

const phaseRecords = (size, phase) => records.filter(
  (record) => record.size === size && record.phase === phase,
);

const round = (value, digits = 3) => Number(value.toFixed(digits));
const formatMs = (value) => (value / 1e6).toFixed(3);
const formatNumber = (value) => Math.round(value).toLocaleString('en-US');
const formatMiB = (value) => value.toFixed(3);

const uniqueProcessSamples = new Map();
for (const record of records) {
  uniqueProcessSamples.set(record.sample, record.maxRssBytes);
}
if (uniqueProcessSamples.size !== 20) {
  throw new Error(`expected 20 process RSS samples, got ${uniqueProcessSamples.size}`);
}
const maxRssMiBStats = stats(
  [...uniqueProcessSamples.values()].map((bytes) => bytes / (1024 * 1024)),
);

const decisions = sizes.map((size) => {
  const cold = phaseRecords(size, 'build/cold');
  const hot = phaseRecords(size, 'build/hot');
  const filter = phaseRecords(size, 'filter');
  const load = phaseRecords(size, 'load');
  const cancel = phaseRecords(size, 'cancel');
  const buildP95Ms = percentile(cold.map((record) => record.nsOp), 0.95) / 1e6;
  const filterP95Ms = percentile(filter.map((record) => record.nsOp), 0.95) / 1e6;
  const candidateLoadP95Ms = percentile(load.map((record) => record.nsOp), 0.95) / 1e6;
  const rssDeltaMiB = percentile(cold.map((record) => record.rssDeltaMiB), 0.95);
  const cancelReleased = cancel.every((record) => record.cancelReleased);
  const accepted = buildP95Ms <= 500 && filterP95Ms <= 100 && rssDeltaMiB <= 32 && cancelReleased;

  return {
    size,
    buildP95Ms: round(buildP95Ms),
    buildHotP95Ms: round(percentile(hot.map((record) => record.nsOp), 0.95) / 1e6),
    filterP95Ms: round(filterP95Ms),
    candidateLoadP95Ms: round(candidateLoadP95Ms),
    rssDeltaMiB: round(rssDeltaMiB),
    cacheCapacityMiB: round(percentile(cold.map((record) => record.cacheMiB), 0.95)),
    cancelReleased,
    accepted,
  };
});

const phaseLabel = {
  'build/cold': 'build cold',
  'build/hot': 'build hot',
  filter: 'filter',
  load: 'candidate load',
  cancel: 'cancel',
};

const detailRows = [];
for (const size of sizes) {
  for (const phase of phases) {
    const group = phaseRecords(size, phase);
    const duration = stats(group.map((record) => record.nsOp));
    const bytes = stats(group.map((record) => record.bytesOp));
    const allocs = stats(group.map((record) => record.allocsOp));
    detailRows.push(
      `| ${formatNumber(size)} | ${phaseLabel[phase]} | ${formatMs(duration.p50)} | ${formatMs(duration.p95)} | ${formatNumber(bytes.p50)} | ${formatNumber(bytes.p95)} | ${formatNumber(allocs.p50)} | ${formatNumber(allocs.p95)} |`,
    );
  }
}

const decisionRows = decisions.map((decision) => {
  const cold = stats(phaseRecords(decision.size, 'build/cold').map((record) => record.nsOp));
  const hot = stats(phaseRecords(decision.size, 'build/hot').map((record) => record.nsOp));
  const filter = stats(phaseRecords(decision.size, 'filter').map((record) => record.nsOp));
  const load = stats(phaseRecords(decision.size, 'load').map((record) => record.nsOp));
  const rss = stats(phaseRecords(decision.size, 'build/cold').map((record) => record.rssDeltaMiB));
  return `| ${formatNumber(decision.size)} | ${formatMs(cold.p50)} / ${formatMs(cold.p95)} | ${formatMs(hot.p50)} / ${formatMs(hot.p95)} | ${formatMs(filter.p50)} / ${formatMs(filter.p95)} | ${formatMs(load.p50)} / ${formatMs(load.p95)} | ${formatMiB(rss.p50)} / ${formatMiB(rss.p95)} | ${decision.cancelReleased ? '是' : '否'} | ${decision.accepted ? '通过' : '不通过'} |`;
});

const acceptedSizes = decisions.filter((decision) => decision.accepted).map((decision) => formatNumber(decision.size));
const conclusion = acceptedSizes.length > 0
  ? `本机五档中通过的规模：${acceptedSizes.join('、')}。准入仍由内存预算和实测时延共同决定，不设置对象数硬阈值。`
  : '本机五档均未通过。首期无索引数据集全部采用有界降级，已有 RTree 查询路径继续启用。';

const report = `# udbx-viewer 无索引包络缓存 PoC 报告

日期：${process.env.GENERATED_AT.slice(0, 10)}

## 1. 运行环境

- 生成时间：${process.env.GENERATED_AT}
- Go：\`${process.env.GO_VERSION}\`
- macOS：\`${process.env.MACOS_VERSION}\`
- CPU：\`${process.env.CPU_MODEL}\`
- Git commit：\`${process.env.GIT_COMMIT}\`
- 测量次数：每个规模、每个阶段固定 20 次，\`-benchtime=1x\`，每次由独立 \`go test\` 进程执行。
- 规模：10,000、50,000、100,000、250,000、500,000。

## 2. 测量方法

- 夹具是临时 SQLite 数据库，表名为 \`poc_points\`，不创建 \`idx_poc_points_SmGeometry\` 或任何 RTree。
- 每条 \`SmGeometry\` 使用真实 GAIA Point 编码。构建查询只读取 \`SmID\` 与 \`substr(SmGeometry, 1, 43)\`，再通过真实 GAIA header codec 解析 MBR。
- 夹具写入完成后关闭并重开 SQLite 连接。\`build cold\` 是新连接上的首次完整扫描；\`build hot\` 紧随其后复扫同一完整表。macOS 文件系统页缓存不做破坏性清理，因此 cold 表示“冷包络缓存/新 SQLite 连接”，不表示物理磁盘冷启动。
- \`filter\` 复用完整连续缓存，固定视口 \`[0, -1, 9, 1000000]\`，命中正好 1%。
- \`candidate load\` 按 500 个 ID 一批读取完整 geometry，并通过真实 \`GaiaGeometryCodec\` 解码。
- \`cancel\` 在扫描达到约 10% 时取消 context，要求返回 \`context.Canceled\`、不返回残缺缓存，并在 1 秒内复用同一单连接数据库执行 \`SELECT 1\`。
- 缓存容量按 \`cap(slice) * unsafe.Sizeof(pocEnvelopeEntry{})\` 计算；对象数只用于预估容量，不作为准入阈值。
- \`rssDeltaMiB\` 是 PoC 进程在每次 cold build 前通过 \`ps\` 采样的当前 RSS，与构建完成后当前 RSS 的差值。该增量用于 32 MiB 准入判定。
- 最大 RSS 由 macOS \`/usr/bin/time -l\` 记录，覆盖该独立 PoC 进程的夹具生成和全部五档 benchmark；20 次进程最大 RSS 的 P50/P95 为 ${formatMiB(maxRssMiBStats.p50)} / ${formatMiB(maxRssMiBStats.p95)} MiB。它用于记录进程峰值，不代替构建前后增量。

## 3. 资源策略判定

准入条件：\`build cold P95 <= 500 ms\`、\`filter P95 <= 100 ms\`、\`rssDelta P95 <= 32 MiB\`、20 次取消均释放资源。当前文件全部包络缓存预算上限保持 64 MiB。

| 规模 | Build cold P50/P95 ms | Build hot P50/P95 ms | Filter P50/P95 ms | Candidate load P50/P95 ms | RSS 增量 P50/P95 MiB | Cancel released | 判定 |
|---:|---:|---:|---:|---:|---:|:---:|:---:|
${decisionRows.join('\n')}

${conclusion}

候选 geometry 读取保留 500 ID 分批策略。原因是 SQLite 参数上限、rows 生命周期和峰值分配都需要有界控制；该批大小是实现参数，可在后续真实复杂线面样本中继续校准。

默认资源策略锁定为：单数据集 32 MiB，当前文件全部包络缓存 64 MiB。即使 500,000 档通过，也必须先按缓存实际容量申请预算，不能因对象数处于已测档位而绕过预算门。

## 4. 机器判定 JSON

\`accepted\` 只由本节列出的四项条件计算；\`candidateLoadP95Ms\` 记录但不参与本轮准入。

\`\`\`json
${JSON.stringify(decisions, null, 2)}
\`\`\`

## 5. 原始 benchmark 汇总

| 规模 | 阶段 | ns/op P50 (ms) | ns/op P95 (ms) | B/op P50 | B/op P95 | allocs/op P50 | allocs/op P95 |
|---:|---|---:|---:|---:|---:|---:|---:|
${detailRows.join('\n')}

原始逐次输出位于脚本临时目录，报告生成后自动删除，不进入仓库。
`;

fs.writeFileSync(reportPath, report);
process.stdout.write(`${JSON.stringify(decisions)}\n`);
NODE

echo "Envelope cache PoC report: $report_path"
