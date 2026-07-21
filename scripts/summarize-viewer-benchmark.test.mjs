import test from 'node:test'
import assert from 'node:assert/strict'
import { renderMarkdown, selectConcurrency, summarizeRuns } from './summarize-viewer-benchmark.mjs'

const scenarioNames = [
  'henan-weibo-rtree-pan-zoom',
  'henan-county-envelope-selection',
  'sampledata-multilayer-viewport',
]
const stepCounts = new Map([
  [scenarioNames[0], 9],
  [scenarioNames[1], 4],
  [scenarioNames[2], 5],
])

function run(scenario, iteration, temperature, overrides = {}) {
  const base = temperature === 'cold' ? 30 : 10
  const stepCount = stepCounts.get(scenario)
  return {
    runId: `${scenario}-${temperature}-${iteration}`,
    status: 'passed',
    startedAt: `2026-07-17T00:00:${String(iteration).padStart(2, '0')}Z`,
    scenario,
    metrics: {
      openFileMs: base + 1,
      loadLayersMs: base + 2,
      fitVisibleLayersMs: 2,
      selectAndFitMs: 5,
      backendQueryMs: Array.from({ length: stepCount }, (_, index) => base + index + iteration),
      moveendToRenderMs: Array.from({ length: stepCount }, (_, index) => base + 40 + index + iteration),
      maxConcurrentQueries: 1,
      pendingPeak: 1,
      pendingFinal: 0,
      staleResultsDiscarded: 1,
      staleResultApplied: false,
      finalFeatureCount: scenario.includes('county') ? 164 : 1000,
      blankRenderCount: 0,
    },
    error: '',
    iteration,
    temperature,
    peakRssKiB: 200000 + iteration,
    rssStartKiB: 180000,
    rssEndKiB: 190000,
    memoryCaptureError: '',
    appPath: '/tmp/udbx4go-viewer-wails.app',
    maxConcurrentQueries: 1,
    environment: {
      gitCommit: 'abc123',
      appSha256: 'app-sha',
      macOSVersion: '26.5.2',
      cpu: 'Apple M3 Pro',
      memoryBytes: 19327352832,
      samplePath: scenario.includes('sampledata') ? '/data/SampleData.udbx' : '/data/henan.udbx',
      sampleSha256: scenario.includes('sampledata') ? 'sample-sha' : 'henan-sha',
      sampleSizeBytes: 123,
    },
    ...overrides,
  }
}

function completeRuns() {
  return scenarioNames.flatMap((scenario) => [
    ...Array.from({ length: 5 }, (_, index) => run(scenario, index + 1, 'cold')),
    ...Array.from({ length: 5 }, (_, index) => run(scenario, index + 1, 'warm')),
  ])
}

test('summarizeRuns reports complete cold/warm P50/P95, RSS and gates', () => {
  const summary = summarizeRuns(completeRuns())

  assert.equal(summary.status, 'passed')
  assert.equal(summary.completeTenRunGate, true)
  assert.equal(summary.maxConcurrentQueries, 1)
  assert.equal(summary.scenarios.length, 3)
  assert.equal(summary.scenarios[0].runs.length, 10)
  assert.equal(summary.scenarios[0].warm.backendQueryMs.p50, 17)
  assert.equal(summary.scenarios[0].warm.backendQueryMs.p95, 22)
  assert.equal(summary.scenarios[0].peakRssKiB, 200005)
  assert.equal(summary.scenarios[0].gates.pendingDrained, true)
  assert.equal(summary.scenarios[0].gates.observedStaleDiscard, true)
  assert.equal(summary.scenarios[0].gates.noStaleApplied, true)
  assert.equal(summary.gates.rtreeBackendP95, true)
  assert.equal(summary.gates.moveendToRenderP95, true)
  assert.equal(summary.gates.observedStaleDiscard, true)
  assert.equal(summary.gates.noSustainedRssGrowth, true)

  const markdown = renderMarkdown(summary)
  assert.match(markdown, /后端查询 P50\/P95/)
  assert.match(markdown, /moveend -> render P50\/P95/)
  assert.match(markdown, /原始十轮/)
  assert.match(markdown, /样本 SHA256/)
  assert.match(markdown, /实际观察到旧结果丢弃.*PASS/)
  assert.match(markdown, /人工验收/)
})

test('summarizeRuns 要求每个场景的每一真实轮次都观察到旧结果丢弃', () => {
  const runs = completeRuns()
  runs[0].metrics.staleResultsDiscarded = 0

  const summary = summarizeRuns(runs)

  assert.equal(summary.status, 'failed')
  assert.equal(summary.scenarios[0].gates.observedStaleDiscard, false)
  assert.equal(summary.gates.observedStaleDiscard, false)
  assert.equal(summary.gates.noStaleApplied, true)
})

test('summarizeRuns rejects a missing run, scenario or RSS sample', () => {
  const runs = completeRuns()
  assert.throws(() => summarizeRuns(runs.slice(1)), /exactly 30 runs|exactly 10 runs|cold runs/)
  assert.throws(
    () => summarizeRuns(runs.filter((item) => item.scenario !== scenarioNames[2])),
    /exactly 30 runs|required scenario/,
  )
  const missingRss = completeRuns()
  missingRss[0].peakRssKiB = 0
  missingRss[0].memoryCaptureError = 'no RSS sample captured'
  assert.throws(() => summarizeRuns(missingRss), /RSS/)
})

test('summarizeRuns rejects duplicate scenario temperature iterations', () => {
  const runs = completeRuns()
  runs[1].iteration = runs[0].iteration

  assert.throws(() => summarizeRuns(runs), /unique|iterations 1 through 5/)
})

test('summarizeRuns rejects mixed build, host, sample and concurrency identities', () => {
  const mutations = [
    ['Git commit', (run) => { run.environment.gitCommit = 'other-commit' }],
    ['app path', (run) => { run.appPath = '/tmp/other.app' }],
    ['app SHA256', (run) => { run.environment.appSha256 = 'other-app' }],
    ['macOS', (run) => { run.environment.macOSVersion = 'other-macos' }],
    ['CPU', (run) => { run.environment.cpu = 'other-cpu' }],
    ['memory', (run) => { run.environment.memoryBytes += 1 }],
    ['sample SHA256', (run) => { run.environment.sampleSha256 = 'other-sample' }],
    ['maxConcurrentQueries', (run) => { run.maxConcurrentQueries = 2 }],
  ]

  for (const [name, mutate] of mutations) {
    const runs = completeRuns()
    mutate(runs[1])
    assert.throws(() => summarizeRuns(runs), new RegExp(name, 'i'), name)
  }
})

test('summarizeRuns rejects missing or invalid per-run metrics and errors', () => {
  const mutations = [
    ['rssStartKiB', (run) => { delete run.rssStartKiB }, /RSS/],
    ['rssEndKiB', (run) => { run.rssEndKiB = Number.NaN }, /RSS/],
    ['backendQueryMs', (run) => { delete run.metrics.backendQueryMs }, /backendQueryMs/],
    ['backendQueryMs finite', (run) => { run.metrics.backendQueryMs = [Number.NaN] }, /backendQueryMs/],
    ['moveendToRenderMs', (run) => { run.metrics.moveendToRenderMs = [] }, /moveendToRenderMs/],
    ['maxConcurrentQueries', (run) => { delete run.metrics.maxConcurrentQueries }, /maxConcurrentQueries/],
    ['pendingPeak', (run) => { delete run.metrics.pendingPeak }, /pendingPeak/],
    ['pendingFinal', (run) => { delete run.metrics.pendingFinal }, /pendingFinal/],
    ['staleResultApplied', (run) => { delete run.metrics.staleResultApplied }, /staleResultApplied/],
    ['finalFeatureCount', (run) => { delete run.metrics.finalFeatureCount }, /finalFeatureCount/],
    ['error', (run) => { run.error = 'query failed' }, /error/],
  ]

  for (const [name, mutate, expected] of mutations) {
    const runs = completeRuns()
    mutate(runs[0])
    assert.throws(() => summarizeRuns(runs), expected, name)
  }
})

test('summarizeRuns rejects null runs and numeric strings', () => {
  const nullRun = completeRuns()
  nullRun[0] = null
  assert.throws(() => summarizeRuns(nullRun), /object|environment/)

  const stringRSS = completeRuns()
  stringRSS[0].rssStartKiB = '180000'
  assert.throws(() => summarizeRuns(stringRSS), /RSS/)

  const stringMetric = completeRuns()
  stringMetric[0].metrics.pendingFinal = '0'
  assert.throws(() => summarizeRuns(stringMetric), /pendingFinal/)
})

test('summarizeRuns rejects failed or errored runs', () => {
  const runs = completeRuns()
  runs[0].status = 'failed'
  runs[0].error = 'query failed'

  assert.throws(() => summarizeRuns(runs), /status must be passed|error/)
})

test('summarizeRuns marks stale application, blank render and pending growth failed', () => {
  const runs = completeRuns()
  runs[1].metrics.staleResultApplied = true
  runs[2].metrics.blankRenderCount = 1
  runs[3].metrics.pendingFinal = 2

  const summary = summarizeRuns(runs)
  assert.equal(summary.status, 'failed')
  assert.equal(summary.failures.length, 0)
  assert.equal(summary.gates.noStaleApplied, false)
  assert.equal(summary.gates.noBlankRender, false)
  assert.equal(summary.gates.pendingDrained, false)
})

test('summarizeRuns enforces RTree and end-to-end P95 gates', () => {
  const runs = completeRuns()
  const weiboWarm = runs.find((item) => item.scenario === scenarioNames[0] && item.temperature === 'warm')
  weiboWarm.metrics.backendQueryMs = Array(9).fill(101)
  const countyWarm = runs.find((item) => item.scenario === scenarioNames[1] && item.temperature === 'warm')
  countyWarm.metrics.moveendToRenderMs = Array(4).fill(301)

  const summary = summarizeRuns(runs)
  assert.equal(summary.status, 'failed')
  assert.equal(summary.gates.rtreeBackendP95, false)
  assert.equal(summary.gates.moveendToRenderP95, false)
})

test('selectConcurrency chooses the smaller qualified candidate or keeps one', () => {
  const identity = {
    appPath: '/tmp/udbx4go-viewer-wails.app',
    environment: { gitCommit: 'abc123', appSha256: 'app-sha' },
    samples: [
      { path: '/data/henan.udbx', sha256: 'henan-sha', sizeBytes: 123 },
      { path: '/data/SampleData.udbx', sha256: 'sample-sha', sizeBytes: 456 },
    ],
  }
  const scenario = (observed, p95, peakRssKiB) => ({
    name: 'sampledata-multilayer-viewport',
    observedMaxConcurrentQueries: observed,
    warm: { moveendToRenderMs: { p95 } },
    peakRssKiB,
  })
  const baseline = { ...identity, status: 'passed', maxConcurrentQueries: 1, scenarios: [scenario(1, 200, 200000)] }
  const qualifiedTwo = { ...identity, status: 'passed', maxConcurrentQueries: 2, scenarios: [scenario(2, 180, 210000)] }
  const qualifiedThree = { ...identity, status: 'passed', maxConcurrentQueries: 3, scenarios: [scenario(3, 170, 215000)] }
  const slowTwo = { ...qualifiedTwo, scenarios: [{ warm: { moveendToRenderMs: { p95: 195 } }, peakRssKiB: 210000 }] }
  const hungryThree = { ...qualifiedThree, scenarios: [{ warm: { moveendToRenderMs: { p95: 170 } }, peakRssKiB: 221000 }] }

  assert.equal(selectConcurrency(baseline, [qualifiedTwo, qualifiedThree]).selected, 2)
  assert.equal(selectConcurrency(baseline, [slowTwo, hungryThree]).selected, 1)
})

test('selectConcurrency rejects candidates that never observe their configured concurrency', () => {
  const identity = {
    appPath: '/tmp/udbx4go-viewer-wails.app',
    environment: { gitCommit: 'abc123', appSha256: 'app-sha' },
    samples: [{ path: '/data/SampleData.udbx', sha256: 'sample-sha', sizeBytes: 456 }],
  }
  const summary = (configured, observed, p95) => ({
    ...identity,
    status: 'passed',
    maxConcurrentQueries: configured,
    scenarios: [{
      name: 'sampledata-multilayer-viewport',
      observedMaxConcurrentQueries: observed,
      warm: { moveendToRenderMs: { p95 } },
      peakRssKiB: 200000,
    }],
  })

  const selection = selectConcurrency(summary(1, 1, 200), [summary(2, 1, 170), summary(3, 1, 160)])

  assert.equal(selection.selected, 1)
  assert.deepEqual(selection.comparisons.map((item) => item.observedConcurrencyQualified), [false, false])
  assert.deepEqual(selection.comparisons.map((item) => item.qualified), [false, false])
  assert.throws(
    () => selectConcurrency(summary(1, 2, 200), [summary(2, 2, 170), summary(3, 3, 160)]),
    /concurrency-1.*observed/i,
  )
})

test('selectConcurrency rejects candidates from a different build or sample set', () => {
  const identity = {
    appPath: '/tmp/udbx4go-viewer-wails.app',
    environment: { gitCommit: 'abc123', appSha256: 'app-sha' },
    samples: [{ path: '/data/henan.udbx', sha256: 'henan-sha', sizeBytes: 123 }],
  }
  const summary = (concurrency, overrides = {}) => ({
    ...identity,
    status: 'passed',
    maxConcurrentQueries: concurrency,
    scenarios: [{ name: 'sampledata-multilayer-viewport', observedMaxConcurrentQueries: concurrency, warm: { moveendToRenderMs: { p95: 100 } }, peakRssKiB: 200000 }],
    ...overrides,
  })

  assert.throws(
    () => selectConcurrency(summary(1), [summary(2, { environment: { ...identity.environment, appSha256: 'other-app' } }), summary(3)]),
    /same packaged app/,
  )
  assert.throws(
    () => selectConcurrency(summary(1), [summary(2, { samples: [{ path: '/data/henan.udbx', sha256: 'other-sample', sizeBytes: 123 }] }), summary(3)]),
    /same samples/,
  )
  assert.throws(
    () => selectConcurrency(summary(1), [summary(2, { environment: { ...identity.environment, cpu: 'other-cpu' } }), summary(3)]),
    /same host/,
  )
})

test('renderMarkdown records candidate comparison and a separate final rerun', () => {
  const candidateRuns = (concurrency, latencyOffset, appSha256 = 'candidate-app') => completeRuns().map((item) => ({
    ...item,
    maxConcurrentQueries: concurrency,
    metrics: {
      ...item.metrics,
      maxConcurrentQueries: Math.min(concurrency, item.scenario.includes('sampledata') ? 3 : 1),
      moveendToRenderMs: item.metrics.moveendToRenderMs.map((value) => value + latencyOffset),
    },
    environment: { ...item.environment, appSha256 },
  }))
  const candidates = [
    summarizeRuns(candidateRuns(1, 80)),
    summarizeRuns(candidateRuns(2, 60)),
    summarizeRuns(candidateRuns(3, 50)),
  ]
  const selection = selectConcurrency(candidates[0], candidates.slice(1))
  const finalSummary = summarizeRuns(candidateRuns(selection.selected, 60, 'final-app'))

  const markdown = renderMarkdown(finalSummary, { candidates, selection, finalRerun: true })

  assert.match(markdown, /并发候选比较/)
  assert.match(markdown, /\| 1 \|/)
  assert.match(markdown, /\| 2 \|/)
  assert.match(markdown, /\| 3 \|/)
  assert.match(markdown, new RegExp(`自动选择.*${selection.selected}`))
  assert.match(markdown, /最终重建后独立重跑.*PASS/)
})
