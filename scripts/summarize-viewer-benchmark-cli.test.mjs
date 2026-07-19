import test from 'node:test'
import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { selectConcurrency, summarizeRuns } from './summarize-viewer-benchmark.mjs'

const script = new URL('./summarize-viewer-benchmark.mjs', import.meta.url).pathname
const scenarios = [
  ['henan-weibo-rtree-pan-zoom', 9],
  ['henan-county-envelope-selection', 4],
  ['sampledata-multilayer-viewport', 5],
]

function runsFor(concurrency, renderBase, appSha256, stale = false) {
  return scenarios.flatMap(([scenario, stepCount]) => ['cold', 'warm'].flatMap((temperature) =>
    Array.from({ length: 5 }, (_, index) => ({
      runId: `${scenario}-${temperature}-${index + 1}`,
      status: 'passed',
      startedAt: '2026-07-17T00:00:00Z',
      scenario,
      metrics: {
        openFileMs: 1,
        loadLayersMs: 1,
        fitVisibleLayersMs: 1,
        selectAndFitMs: 1,
        backendQueryMs: Array(stepCount).fill(20),
        moveendToRenderMs: Array(stepCount).fill(renderBase),
        maxConcurrentQueries: scenario === 'sampledata-multilayer-viewport' ? concurrency : 1,
        pendingPeak: 1,
        pendingFinal: 0,
        staleResultsDiscarded: 1,
        staleResultApplied: stale && scenario === scenarios[0][0] && index === 0,
        finalFeatureCount: 1,
        blankRenderCount: 0,
      },
      error: '',
      iteration: index + 1,
      temperature,
      peakRssKiB: 200000,
      rssStartKiB: 180000,
      rssEndKiB: 185000,
      memoryCaptureError: '',
      appPath: '/tmp/viewer.app',
      maxConcurrentQueries: concurrency,
      environment: {
        gitCommit: 'abc123',
        appSha256,
        macOSVersion: '26.5',
        cpu: 'Apple M3',
        memoryBytes: 16000000000,
        samplePath: scenario === 'sampledata-multilayer-viewport' ? '/data/SampleData.udbx' : '/data/henan.udbx',
        sampleSha256: scenario === 'sampledata-multilayer-viewport' ? 'sample-sha' : 'henan-sha',
        sampleSizeBytes: 123,
      },
    })),
  ))
}

function prepareWorkflow(t, finalRuns) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'udbx-summary-cli-'))
  const inputDir = path.join(dir, 'raw')
  fs.mkdirSync(inputDir)
  finalRuns.forEach((run) => fs.writeFileSync(path.join(inputDir, `${run.runId}.json`), JSON.stringify(run)))
  const candidates = [
    summarizeRuns(runsFor(1, 200, 'candidate-app')),
    summarizeRuns(runsFor(2, 180, 'candidate-app')),
    summarizeRuns(runsFor(3, 170, 'candidate-app')),
  ]
  const selection = selectConcurrency(candidates[0], candidates.slice(1))
  fs.writeFileSync(path.join(dir, 'selection.json'), JSON.stringify(selection))
  candidates.forEach((summary, index) => fs.writeFileSync(path.join(dir, `candidate-${index + 1}.json`), JSON.stringify(summary)))
  const acceptance = path.join(dir, 'acceptance.md')
  fs.writeFileSync(acceptance, 'existing successful acceptance\n')
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }))
  return { dir, inputDir, acceptance }
}

function runCLI(workflow) {
  return spawnSync(process.execPath, [
    script,
    '--input-dir', workflow.inputDir,
    '--json-out', path.join(workflow.dir, 'summary.json'),
    '--markdown-out', path.join(workflow.dir, 'summary.md'),
    '--acceptance-report', workflow.acceptance,
    '--selection-json', path.join(workflow.dir, 'selection.json'),
    '--candidate-summary-1', path.join(workflow.dir, 'candidate-1.json'),
    '--candidate-summary-2', path.join(workflow.dir, 'candidate-2.json'),
    '--candidate-summary-3', path.join(workflow.dir, 'candidate-3.json'),
  ], { encoding: 'utf8' })
}

test('final concurrency mismatch exits nonzero without replacing successful acceptance', (t) => {
  const workflow = prepareWorkflow(t, runsFor(1, 200, 'final-app'))

  const result = runCLI(workflow)

  assert.notEqual(result.status, 0)
  assert.equal(fs.readFileSync(workflow.acceptance, 'utf8'), 'existing successful acceptance\n')
})

test('failed final gates exit nonzero without replacing successful acceptance', (t) => {
  const workflow = prepareWorkflow(t, runsFor(2, 180, 'final-app', true))

  const result = runCLI(workflow)

  assert.notEqual(result.status, 0)
  assert.equal(fs.readFileSync(workflow.acceptance, 'utf8'), 'existing successful acceptance\n')
})

test('failed single summary also preserves an existing acceptance report', (t) => {
  const workflow = prepareWorkflow(t, runsFor(1, 200, 'single-app', true))
  const result = spawnSync(process.execPath, [
    script,
    '--input-dir', workflow.inputDir,
    '--json-out', path.join(workflow.dir, 'single-summary.json'),
    '--markdown-out', path.join(workflow.dir, 'single-summary.md'),
    '--acceptance-report', workflow.acceptance,
  ], { encoding: 'utf8' })

  assert.notEqual(result.status, 0)
  assert.equal(fs.readFileSync(workflow.acceptance, 'utf8'), 'existing successful acceptance\n')
})
