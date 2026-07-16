#!/usr/bin/env node

import fs from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

const requiredScenarios = [
  'henan-weibo-rtree-pan-zoom',
  'henan-county-envelope-selection',
  'sampledata-multilayer-viewport',
]
const scalarMetricNames = ['openFileMs', 'loadLayersMs', 'fitVisibleLayersMs', 'selectAndFitMs']
const maxRssGrowthKiB = 64 * 1024
const expectedViewportStepCounts = new Map([
  ['henan-weibo-rtree-pan-zoom', 8],
  ['henan-county-envelope-selection', 3],
  ['sampledata-multilayer-viewport', 4],
])

function percentile(values, fraction) {
  if (values.length === 0) return null
  const sorted = [...values].sort((a, b) => a - b)
  return sorted[Math.max(0, Math.ceil(sorted.length * fraction) - 1)]
}

function distribution(values) {
  const finite = values.map(Number).filter(Number.isFinite)
  return { p50: percentile(finite, 0.5), p95: percentile(finite, 0.95) }
}

function validateRuns(runs) {
  if (!Array.isArray(runs) || runs.length !== 30) throw new Error('benchmark results must contain exactly 30 runs')
  const runIDs = new Set()
  const identity = batchIdentity(runs[0])
  const configuredConcurrency = positiveInteger(runs[0].maxConcurrentQueries, 'maxConcurrentQueries')
  if (configuredConcurrency > 3) throw new Error('maxConcurrentQueries must be between 1 and 3')
  const sampleIdentities = new Map()

  for (const run of runs) {
    if (!run || typeof run !== 'object') throw new Error('every benchmark run must be an object')
    if (typeof run.runId !== 'string' || run.runId.trim() === '') throw new Error('every benchmark run must contain runId')
    if (runIDs.has(run.runId)) throw new Error(`benchmark runId must be unique: ${run.runId}`)
    runIDs.add(run.runId)
    if (run.status !== 'passed') throw new Error(`benchmark run ${run.runId} status must be passed`)
    if (typeof run.error !== 'string' || run.error !== '') throw new Error(`benchmark run ${run.runId} must not contain an error`)
    assertSameBatchIdentity(identity, batchIdentity(run))
    if (positiveInteger(run.maxConcurrentQueries, 'maxConcurrentQueries') !== configuredConcurrency) {
      throw new Error('all benchmark runs must use the same maxConcurrentQueries')
    }
    validateRunMetrics(run, configuredConcurrency)
    validateRunRSS(run)

    const sample = sampleIdentity(run)
    const existingSample = sampleIdentities.get(run.scenario)
    if (existingSample && existingSample !== sample) {
      throw new Error(`scenario ${run.scenario} must use the same sample SHA256 and path in every run`)
    }
    sampleIdentities.set(run.scenario, sample)
  }

  for (const name of requiredScenarios) {
    const scenarioRuns = runs.filter((run) => run.scenario === name)
    if (scenarioRuns.length === 0) throw new Error(`required scenario ${name} is missing`)
    if (scenarioRuns.length !== 10) throw new Error(`scenario ${name} must contain exactly 10 runs`)
    for (const temperature of ['cold', 'warm']) {
      const temperatureRuns = scenarioRuns.filter((run) => run.temperature === temperature)
      if (temperatureRuns.length !== 5) throw new Error(`scenario ${name} must contain exactly 5 ${temperature} runs`)
      const iterations = temperatureRuns.map((run) => run.iteration).sort((a, b) => a - b)
      if (iterations.some((iteration, index) => iteration !== index + 1)) {
        throw new Error(`scenario ${name} ${temperature} runs must contain iterations 1 through 5`)
      }
    }
  }
  if (sampleIdentities.get(requiredScenarios[0]) !== sampleIdentities.get(requiredScenarios[1])) {
    throw new Error('henan scenarios must use the same sample SHA256 and path')
  }
}

function batchIdentity(run) {
  const environment = run?.environment
  if (!environment || typeof environment !== 'object') throw new Error('every benchmark run must contain environment')
  const fields = {
    gitCommit: requiredString(environment.gitCommit, 'Git commit'),
    appPath: requiredString(run.appPath, 'app path'),
    appSha256: requiredString(environment.appSha256, 'app SHA256'),
    macOSVersion: requiredString(environment.macOSVersion, 'macOS'),
    cpu: requiredString(environment.cpu, 'CPU'),
    memoryBytes: positiveFinite(environment.memoryBytes, 'hardware memory'),
  }
  return JSON.stringify(fields)
}

function assertSameBatchIdentity(expected, actual) {
  if (expected === actual) return
  const expectedFields = JSON.parse(expected)
  const actualFields = JSON.parse(actual)
  const labels = {
    gitCommit: 'Git commit', appPath: 'app path', appSha256: 'app SHA256',
    macOSVersion: 'macOS', cpu: 'CPU', memoryBytes: 'hardware memory',
  }
  const field = Object.keys(expectedFields).find((name) => expectedFields[name] !== actualFields[name])
  throw new Error(`all benchmark runs must use the same ${labels[field] ?? field}`)
}

function validateRunMetrics(run, configuredConcurrency) {
  const metrics = run.metrics
  if (!metrics || typeof metrics !== 'object') throw new Error(`benchmark run ${run.runId} must contain metrics`)
  for (const metricName of scalarMetricNames) nonNegativeFinite(metrics[metricName], metricName)
  const expectedStepCount = expectedViewportStepCounts.get(run.scenario)
  if (!expectedStepCount) throw new Error(`unexpected scenario: ${run.scenario}`)
  finiteArray(metrics.backendQueryMs, 'backendQueryMs', expectedStepCount, false)
  finiteArray(metrics.moveendToRenderMs, 'moveendToRenderMs', expectedStepCount, true)
  const observedConcurrency = positiveInteger(metrics.maxConcurrentQueries, 'metrics.maxConcurrentQueries')
  if (observedConcurrency > configuredConcurrency) {
    throw new Error('metrics.maxConcurrentQueries must not exceed configured maxConcurrentQueries')
  }
  nonNegativeInteger(metrics.pendingPeak, 'pendingPeak')
  nonNegativeInteger(metrics.pendingFinal, 'pendingFinal')
  nonNegativeInteger(metrics.staleResultsDiscarded, 'staleResultsDiscarded')
  if (typeof metrics.staleResultApplied !== 'boolean') throw new Error('staleResultApplied must be boolean')
  nonNegativeInteger(metrics.finalFeatureCount, 'finalFeatureCount')
  nonNegativeInteger(metrics.blankRenderCount, 'blankRenderCount')
}

function validateRunRSS(run) {
  positiveFinite(run.peakRssKiB, 'RSS peak')
  positiveFinite(run.rssStartKiB, 'RSS start')
  positiveFinite(run.rssEndKiB, 'RSS end')
  if (typeof run.memoryCaptureError !== 'string' || run.memoryCaptureError !== '') {
    throw new Error('every benchmark run must contain complete RSS samples without memoryCaptureError')
  }
}

function finiteArray(value, name, expectedStepCount, exact) {
  if (!Array.isArray(value) || (exact ? value.length !== expectedStepCount : value.length < expectedStepCount)) {
    const qualifier = exact ? 'exactly' : 'at least'
    throw new Error(`${name} must contain ${qualifier} ${expectedStepCount} values`)
  }
  value.forEach((item) => nonNegativeFinite(item, name))
}

function sampleIdentity(run) {
  const environment = run.environment
  return JSON.stringify({
    path: requiredString(environment.samplePath, 'sample path'),
    sha256: requiredString(environment.sampleSha256, 'sample SHA256'),
    sizeBytes: positiveFinite(environment.sampleSizeBytes, 'sample size'),
  })
}

function requiredString(value, name) {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} is required`)
  return value
}

function positiveFinite(value, name) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    throw new Error(`${name} must be finite and positive`)
  }
  return value
}

function nonNegativeFinite(value, name) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) {
    throw new Error(`${name} must be finite and non-negative`)
  }
  return value
}

function positiveInteger(value, name) {
  if (typeof value !== 'number' || !Number.isInteger(value) || value <= 0) {
    throw new Error(`${name} must be a positive integer`)
  }
  return value
}

function nonNegativeInteger(value, name) {
  if (typeof value !== 'number' || !Number.isInteger(value) || value < 0) {
    throw new Error(`${name} must be a non-negative integer`)
  }
  return value
}

function summarizeTemperature(runs) {
  const result = {}
  for (const metricName of scalarMetricNames) {
    result[metricName] = distribution(runs.map((run) => run.metrics[metricName]))
  }
  result.backendQueryMs = distribution(runs.flatMap((run) => run.metrics.backendQueryMs ?? []))
  result.moveendToRenderMs = distribution(runs.flatMap((run) => run.metrics.moveendToRenderMs ?? []))
  result.peakRssKiB = distribution(runs.map((run) => run.peakRssKiB))
  result.rssGrowthKiB = distribution(runs.map((run) => Math.max(0, Number(run.rssEndKiB) - Number(run.rssStartKiB))))
  return result
}

export function summarizeRuns(runs) {
  validateRuns(runs)
  const unexpected = [...new Set(runs.map((run) => run.scenario))].filter((name) => !requiredScenarios.includes(name))
  if (unexpected.length > 0) throw new Error(`unexpected scenario: ${unexpected.join(', ')}`)

  const scenarios = requiredScenarios.map((name) => {
    const scenarioRuns = runs.filter((run) => run.scenario === name).sort((a, b) =>
      a.temperature.localeCompare(b.temperature) || a.iteration - b.iteration,
    )
    const coldRuns = scenarioRuns.filter((run) => run.temperature === 'cold')
    const warmRuns = scenarioRuns.filter((run) => run.temperature === 'warm')
    const staleApplied = scenarioRuns.some((run) => Boolean(run.metrics.staleResultApplied))
    const blankRendered = scenarioRuns.some((run) => Number(run.metrics.blankRenderCount) > 0)
    const pendingDrained = scenarioRuns.every((run) => Number(run.metrics.pendingFinal) === 0)
    const noSustainedRssGrowth = scenarioRuns.every((run) =>
      Number(run.rssEndKiB) - Number(run.rssStartKiB) <= maxRssGrowthKiB,
    )
    return {
      name,
      runs: scenarioRuns,
      cold: summarizeTemperature(coldRuns),
      warm: summarizeTemperature(warmRuns),
      peakRssKiB: Math.max(...scenarioRuns.map((run) => Number(run.peakRssKiB))),
      gates: {
        allRunsPassed: scenarioRuns.every((run) => run.status === 'passed'),
        pendingDrained,
        noStaleApplied: !staleApplied,
        noBlankRender: !blankRendered,
        noSustainedRssGrowth,
      },
    }
  })

  const failures = runs
    .filter((run) => run.status !== 'passed')
    .map((run) => ({ runId: run.runId, scenario: run.scenario, error: run.error || 'unknown error' }))
  const weibo = scenarios.find((scenario) => scenario.name === requiredScenarios[0])
  const gates = {
    completeTenRuns: true,
    allRunsPassed: failures.length === 0,
    rtreeBackendP95: weibo.warm.backendQueryMs.p95 != null && weibo.warm.backendQueryMs.p95 <= 100,
    moveendToRenderP95: scenarios.every((scenario) =>
      scenario.warm.moveendToRenderMs.p95 != null && scenario.warm.moveendToRenderMs.p95 <= 300,
    ),
    pendingDrained: scenarios.every((scenario) => scenario.gates.pendingDrained),
    noStaleApplied: scenarios.every((scenario) => scenario.gates.noStaleApplied),
    noBlankRender: scenarios.every((scenario) => scenario.gates.noBlankRender),
    noSustainedRssGrowth: scenarios.every((scenario) => scenario.gates.noSustainedRssGrowth),
  }
  const maxConcurrentQueries = Math.max(...runs.map((run) => Number(run.maxConcurrentQueries ?? 0)))
  return {
    status: Object.values(gates).every(Boolean) ? 'passed' : 'failed',
    generatedAt: new Date().toISOString(),
    completeTenRunGate: true,
    maxConcurrentQueries,
    appPath: runs[0].appPath,
    environment: runs[0].environment ?? {},
    samples: [...new Map(runs.map((run) => [run.environment?.samplePath, {
      path: run.environment?.samplePath,
      sha256: run.environment?.sampleSha256,
      sizeBytes: run.environment?.sampleSizeBytes,
    }])).values()],
    scenarios,
    failures,
    gates,
  }
}

function concurrencyComparisonValue(summary, selector) {
  return Math.max(...summary.scenarios.map(selector))
}

function sampleFingerprint(samples) {
  return JSON.stringify(
    [...(samples ?? [])]
      .map((sample) => ({ path: sample.path, sha256: sample.sha256, sizeBytes: Number(sample.sizeBytes) }))
      .sort((left, right) => left.path.localeCompare(right.path)),
  )
}

function assertSameCandidateInputs(baseline, candidate) {
  const sameApp = candidate.appPath === baseline.appPath
    && candidate.environment?.gitCommit === baseline.environment?.gitCommit
    && candidate.environment?.appSha256
    && candidate.environment.appSha256 === baseline.environment?.appSha256
  if (!sameApp) {
    throw new Error('concurrency candidates must use the same packaged app')
  }
  if (sampleFingerprint(candidate.samples) !== sampleFingerprint(baseline.samples)) {
    throw new Error('concurrency candidates must use the same samples')
  }
  const hostFields = ['macOSVersion', 'cpu', 'memoryBytes']
  if (hostFields.some((field) => candidate.environment?.[field] !== baseline.environment?.[field])) {
    throw new Error('concurrency candidates must use the same host')
  }
}

export function selectConcurrency(baseline, candidates) {
  if (baseline.status !== 'passed' || baseline.maxConcurrentQueries !== 1) {
    throw new Error('concurrency baseline must be a passed concurrency-1 summary')
  }
  const candidateConcurrencies = candidates.map((candidate) => candidate.maxConcurrentQueries).sort((a, b) => a - b)
  if (candidateConcurrencies.length !== 2 || candidateConcurrencies[0] !== 2 || candidateConcurrencies[1] !== 3) {
    throw new Error('concurrency candidates must contain concurrency 2 and 3 exactly once')
  }
  candidates.forEach((candidate) => assertSameCandidateInputs(baseline, candidate))
  const baselineP95 = concurrencyComparisonValue(baseline, (scenario) => scenario.warm.moveendToRenderMs.p95)
  const baselineRss = concurrencyComparisonValue(baseline, (scenario) => scenario.peakRssKiB)
  const comparisons = candidates
    .slice()
    .sort((left, right) => left.maxConcurrentQueries - right.maxConcurrentQueries)
    .map((candidate) => {
      const moveendP95 = concurrencyComparisonValue(candidate, (scenario) => scenario.warm.moveendToRenderMs.p95)
      const peakRssKiB = concurrencyComparisonValue(candidate, (scenario) => scenario.peakRssKiB)
      const latencyRatio = moveendP95 / baselineP95
      const rssRatio = peakRssKiB / baselineRss
      return {
        concurrency: candidate.maxConcurrentQueries,
        latencyRatio,
        rssRatio,
        qualified: candidate.status === 'passed' && latencyRatio <= 0.95 && rssRatio <= 1.10,
      }
    })
  return {
    selected: comparisons.find((candidate) => candidate.qualified)?.concurrency ?? 1,
    baselineP95,
    baselineRssKiB: baselineRss,
    comparisons,
  }
}

function formatNumber(value) {
  return value == null ? 'N/A' : Number(value).toFixed(2)
}

function formatMiB(kib) {
  return kib == null ? 'N/A' : (Number(kib) / 1024).toFixed(2)
}

function pass(value) {
  return value ? 'PASS' : 'FAIL'
}

function renderSelectionMarkdown(selection) {
  const lines = [
    '# udbx-viewer 并发候选选择', '',
    `- 自动选择：${selection.selected}`,
    `- 并发 1 热端到端 P95：${formatNumber(selection.baselineP95)} ms`,
    `- 并发 1 峰值 RSS：${formatMiB(selection.baselineRssKiB)} MiB`, '',
    '| 并发 | 相对延迟 | 相对 RSS | 合格 |',
    '| ---: | ---: | ---: | --- |',
    ...selection.comparisons.map((item) => `| ${item.concurrency} | ${item.latencyRatio.toFixed(3)} | ${item.rssRatio.toFixed(3)} | ${pass(item.qualified)} |`),
    '',
  ]
  return `${lines.join('\n')}\n`
}

export function renderMarkdown(summary, workflow = null) {
  const lines = [
    '# udbx-viewer 视口空间查询验收报告', '',
    `- 自动验收：${summary.status.toUpperCase()}`,
    `- 生成时间：${summary.generatedAt}`,
    `- Git 提交：${summary.environment.gitCommit ?? 'N/A'}`,
    `- 应用 SHA256：${summary.environment.appSha256 ?? 'N/A'}`,
    `- macOS：${summary.environment.macOSVersion ?? 'N/A'}`,
    `- CPU：${summary.environment.cpu ?? 'N/A'}`,
    `- 物理内存：${summary.environment.memoryBytes ? `${(summary.environment.memoryBytes / 1024 ** 3).toFixed(2)} GiB` : 'N/A'}`,
    `- 应用路径：${summary.appPath ?? 'N/A'}`,
    `- 最终并发：${summary.maxConcurrentQueries}`, '',
  ]

  if (workflow) {
    const candidateByConcurrency = new Map(workflow.candidates.map((candidate) => [candidate.maxConcurrentQueries, candidate]))
    const comparisonByConcurrency = new Map(workflow.selection.comparisons.map((item) => [item.concurrency, item]))
    lines.push(
      '## 并发候选比较', '',
      '| 并发 | 热 moveend -> render P95 ms | 峰值 RSS MiB | 相对延迟 | 相对 RSS | 结论 |',
      '| ---: | ---: | ---: | ---: | ---: | --- |',
    )
    for (const concurrency of [1, 2, 3]) {
      const candidate = candidateByConcurrency.get(concurrency)
      if (!candidate) throw new Error(`acceptance report is missing concurrency-${concurrency} candidate`)
      const p95 = concurrencyComparisonValue(candidate, (scenario) => scenario.warm.moveendToRenderMs.p95)
      const rss = concurrencyComparisonValue(candidate, (scenario) => scenario.peakRssKiB)
      const comparison = comparisonByConcurrency.get(concurrency)
      lines.push(`| ${concurrency} | ${formatNumber(p95)} | ${formatMiB(rss)} | ${comparison ? comparison.latencyRatio.toFixed(3) : '1.000'} | ${comparison ? comparison.rssRatio.toFixed(3) : '1.000'} | ${comparison ? pass(comparison.qualified) : 'BASELINE'} |`)
    }
    const finalRerunPassed = workflow.finalRerun === true
      && summary.status === 'passed'
      && summary.maxConcurrentQueries === workflow.selection.selected
    lines.push(
      '',
      `- 自动选择：${workflow.selection.selected}`,
      `- 最终重建后独立重跑：${pass(finalRerunPassed)}`,
      '- 候选结果仅用于选择；本报告的原始轮次来自重建后的最终运行。', '',
    )
  }

  lines.push(
    '## 样本', '',
    '| 路径 | 样本 SHA256 | 字节数 |', '| --- | --- | ---: |',
    ...summary.samples.map((sample) => `| ${sample.path} | ${sample.sha256} | ${sample.sizeBytes} |`), '',
    '## 自动门禁', '',
    '| 门禁 | 结果 |', '| --- | --- |',
    `| 三场景各冷 5 + 热 5 完整十轮 | ${pass(summary.gates.completeTenRuns)} |`,
    `| 全部轮次无错误 | ${pass(summary.gates.allRunsPassed)} |`,
    `| weibo RTree 热后端 P95 <= 100 ms | ${pass(summary.gates.rtreeBackendP95)} |`,
    `| 全场景热 moveend -> render P95 <= 300 ms | ${pass(summary.gates.moveendToRenderP95)} |`,
    `| pending 最终清空 | ${pass(summary.gates.pendingDrained)} |`,
    `| 无旧结果应用 | ${pass(summary.gates.noStaleApplied)} |`,
    `| 无白屏 | ${pass(summary.gates.noBlankRender)} |`,
    `| RSS 结束增长 <= 64 MiB | ${pass(summary.gates.noSustainedRssGrowth)} |`, '',
  )

  for (const scenario of summary.scenarios) {
    lines.push(`## ${scenario.name}`, '')
    lines.push('| 温度 | 后端查询 P50/P95 ms | moveend -> render P50/P95 ms | RSS 峰值 P50/P95 MiB | RSS 增长 P50/P95 MiB |')
    lines.push('| --- | ---: | ---: | ---: | ---: |')
    for (const temperature of ['cold', 'warm']) {
      const item = scenario[temperature]
      lines.push(`| ${temperature} | ${formatNumber(item.backendQueryMs.p50)} / ${formatNumber(item.backendQueryMs.p95)} | ${formatNumber(item.moveendToRenderMs.p50)} / ${formatNumber(item.moveendToRenderMs.p95)} | ${formatMiB(item.peakRssKiB.p50)} / ${formatMiB(item.peakRssKiB.p95)} | ${formatMiB(item.rssGrowthKiB.p50)} / ${formatMiB(item.rssGrowthKiB.p95)} |`)
    }
    lines.push('', '### 原始十轮', '')
    lines.push('| 温度 | 轮次 | backendQueryMs | moveendToRenderMs | maxConcurrent | pendingPeak/final | stale discard/applied | final count | 白屏 | RSS peak/start/end MiB | 状态 |')
    lines.push('| --- | ---: | --- | --- | ---: | --- | --- | ---: | ---: | --- | --- |')
    for (const run of scenario.runs) {
      lines.push(`| ${run.temperature} | ${run.iteration} | ${(run.metrics.backendQueryMs ?? []).map(formatNumber).join(', ')} | ${(run.metrics.moveendToRenderMs ?? []).map(formatNumber).join(', ')} | ${run.metrics.maxConcurrentQueries} | ${run.metrics.pendingPeak}/${run.metrics.pendingFinal} | ${run.metrics.staleResultsDiscarded}/${run.metrics.staleResultApplied} | ${run.metrics.finalFeatureCount} | ${run.metrics.blankRenderCount} | ${formatMiB(run.peakRssKiB)}/${formatMiB(run.rssStartKiB)}/${formatMiB(run.rssEndKiB)} | ${run.status} |`)
    }
    lines.push('')
  }

  lines.push('## 失败详情', '')
  if (summary.failures.length === 0) lines.push('无。', '')
  else lines.push(...summary.failures.map((failure) => `- ${failure.runId}（${failure.scenario}）：${failure.error}`), '')
  lines.push(
    '## 人工验收', '',
    '- PENDING：henan.udbx/weibo 平移缩放只加载当前范围，放大后截断提示消失。',
    '- PENDING：县级行政区划 164 条按不同视口浏览；第二页表格选择可定位并高亮。',
    '- PENDING：SampleData.udbx 点、线、面、CAD 多图层显隐和移除；CAD 保持有界预览。',
    '- PENDING：快速连续缩放无白屏、无旧范围回跳。',
    '- PENDING：单图层查询失败保留旧图形，其他图层可继续操作。',
    '- PENDING：关闭或切换文件后旧请求不写入新地图。', '',
  )
  return `${lines.join('\n')}\n`
}

function parseArgs(args) {
  const options = {}
  const allowed = new Set([
    '--input-dir', '--json-out', '--markdown-out', '--acceptance-report',
    '--selection-json', '--candidate-summary-1', '--candidate-summary-2', '--candidate-summary-3',
    '--select-baseline', '--select-candidate-2', '--select-candidate-3',
  ])
  for (let index = 0; index < args.length; index += 2) {
    const flag = args[index]
    const value = args[index + 1]
    if (!value || !allowed.has(flag)) {
      throw new Error(`invalid argument: ${flag ?? ''}`)
    }
    options[flag.slice(2)] = value
  }
  if (!options['json-out'] || !options['markdown-out']) throw new Error('--json-out and --markdown-out are required')
  if (options['acceptance-report'] && !path.isAbsolute(options['acceptance-report'])) {
    throw new Error('--acceptance-report must be absolute')
  }
  return options
}

async function main() {
  const options = parseArgs(process.argv.slice(2))
  if (options['select-baseline']) {
    for (const name of ['select-baseline', 'select-candidate-2', 'select-candidate-3']) {
      if (!options[name]) throw new Error(`--${name} is required in selection mode`)
    }
    const baseline = JSON.parse(await fs.readFile(options['select-baseline'], 'utf8'))
    const candidates = await Promise.all([2, 3].map(async (concurrency) =>
      JSON.parse(await fs.readFile(options[`select-candidate-${concurrency}`], 'utf8')),
    ))
    const selection = selectConcurrency(baseline, candidates)
    await fs.mkdir(path.dirname(options['json-out']), { recursive: true })
    await fs.writeFile(options['json-out'], `${JSON.stringify(selection, null, 2)}\n`, 'utf8')
    await fs.writeFile(options['markdown-out'], renderSelectionMarkdown(selection), 'utf8')
    return
  }
  if (!options['input-dir']) throw new Error('--input-dir is required in summary mode')
  const names = (await fs.readdir(options['input-dir'])).filter((name) => name.endsWith('.json')).sort()
  const runs = await Promise.all(names.map(async (name) => JSON.parse(await fs.readFile(path.join(options['input-dir'], name), 'utf8'))))
  const summary = summarizeRuns(runs)
  let workflow = null
  if (options['selection-json']) {
    for (const concurrency of [1, 2, 3]) {
      if (!options[`candidate-summary-${concurrency}`]) {
        throw new Error(`--candidate-summary-${concurrency} is required with --selection-json`)
      }
    }
    workflow = {
      selection: JSON.parse(await fs.readFile(options['selection-json'], 'utf8')),
      candidates: await Promise.all([1, 2, 3].map(async (concurrency) =>
        JSON.parse(await fs.readFile(options[`candidate-summary-${concurrency}`], 'utf8')),
      )),
      finalRerun: true,
    }
  }
  const markdown = renderMarkdown(summary, workflow)
  await fs.mkdir(path.dirname(options['json-out']), { recursive: true })
  await fs.writeFile(options['json-out'], `${JSON.stringify(summary, null, 2)}\n`, 'utf8')
  await fs.writeFile(options['markdown-out'], markdown, 'utf8')
  if (options['acceptance-report']) {
    await fs.mkdir(path.dirname(options['acceptance-report']), { recursive: true })
    await fs.writeFile(options['acceptance-report'], markdown, 'utf8')
  }
  if (summary.status !== 'passed') process.exitCode = 1
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : String(error))
    process.exitCode = 1
  })
}
