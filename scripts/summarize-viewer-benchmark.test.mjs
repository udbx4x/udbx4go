import test from 'node:test'
import assert from 'node:assert/strict'
import { renderMarkdown, selectConcurrency, summarizeRuns } from './summarize-viewer-benchmark.mjs'

const scenarioNames = [
  'henan-weibo-rtree-pan-zoom',
  'henan-county-envelope-selection',
  'sampledata-multilayer-viewport',
]

function run(scenario, iteration, temperature, overrides = {}) {
  const base = temperature === 'cold' ? 30 : 10
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
      backendQueryMs: [base, base + iteration],
      moveendToRenderMs: [base + 40, base + 50 + iteration],
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
  assert.equal(summary.scenarios[0].warm.backendQueryMs.p50, 10)
  assert.equal(summary.scenarios[0].warm.backendQueryMs.p95, 15)
  assert.equal(summary.scenarios[0].peakRssKiB, 200005)
  assert.equal(summary.scenarios[0].gates.pendingDrained, true)
  assert.equal(summary.scenarios[0].gates.noStaleApplied, true)
  assert.equal(summary.gates.rtreeBackendP95, true)
  assert.equal(summary.gates.moveendToRenderP95, true)
  assert.equal(summary.gates.noSustainedRssGrowth, true)

  const markdown = renderMarkdown(summary)
  assert.match(markdown, /后端查询 P50\/P95/)
  assert.match(markdown, /moveend -> render P50\/P95/)
  assert.match(markdown, /原始十轮/)
  assert.match(markdown, /样本 SHA256/)
  assert.match(markdown, /人工验收/)
})

test('summarizeRuns rejects a missing run, scenario or RSS sample', () => {
  const runs = completeRuns()
  assert.throws(() => summarizeRuns(runs.slice(1)), /exactly 10 runs|cold runs/)
  assert.throws(
    () => summarizeRuns(runs.filter((item) => item.scenario !== scenarioNames[2])),
    /required scenario/,
  )
  const missingRss = completeRuns()
  missingRss[0].peakRssKiB = 0
  missingRss[0].memoryCaptureError = 'no RSS sample captured'
  assert.throws(() => summarizeRuns(missingRss), /RSS/)
})

test('summarizeRuns marks errors, stale application, blank render and pending growth failed', () => {
  const runs = completeRuns()
  runs[0].status = 'failed'
  runs[0].error = 'query failed'
  runs[1].metrics.staleResultApplied = true
  runs[2].metrics.blankRenderCount = 1
  runs[3].metrics.pendingFinal = 2

  const summary = summarizeRuns(runs)
  assert.equal(summary.status, 'failed')
  assert.equal(summary.failures.length, 1)
  assert.equal(summary.gates.noStaleApplied, false)
  assert.equal(summary.gates.noBlankRender, false)
  assert.equal(summary.gates.pendingDrained, false)
  assert.match(renderMarkdown(summary), /query failed/)
})

test('summarizeRuns enforces RTree and end-to-end P95 gates', () => {
  const runs = completeRuns()
  const weiboWarm = runs.find((item) => item.scenario === scenarioNames[0] && item.temperature === 'warm')
  weiboWarm.metrics.backendQueryMs = [101]
  const countyWarm = runs.find((item) => item.scenario === scenarioNames[1] && item.temperature === 'warm')
  countyWarm.metrics.moveendToRenderMs = [301]

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
  const baseline = { ...identity, status: 'passed', maxConcurrentQueries: 1, scenarios: [{ warm: { moveendToRenderMs: { p95: 200 } }, peakRssKiB: 200000 }] }
  const qualifiedTwo = { ...identity, status: 'passed', maxConcurrentQueries: 2, scenarios: [{ warm: { moveendToRenderMs: { p95: 180 } }, peakRssKiB: 210000 }] }
  const qualifiedThree = { ...identity, status: 'passed', maxConcurrentQueries: 3, scenarios: [{ warm: { moveendToRenderMs: { p95: 170 } }, peakRssKiB: 215000 }] }
  const slowTwo = { ...qualifiedTwo, scenarios: [{ warm: { moveendToRenderMs: { p95: 195 } }, peakRssKiB: 210000 }] }
  const hungryThree = { ...qualifiedThree, scenarios: [{ warm: { moveendToRenderMs: { p95: 170 } }, peakRssKiB: 221000 }] }

  assert.equal(selectConcurrency(baseline, [qualifiedTwo, qualifiedThree]).selected, 2)
  assert.equal(selectConcurrency(baseline, [slowTwo, hungryThree]).selected, 1)
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
    scenarios: [{ warm: { moveendToRenderMs: { p95: 100 } }, peakRssKiB: 200000 }],
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
})
