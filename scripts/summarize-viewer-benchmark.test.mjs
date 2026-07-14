import test from 'node:test'
import assert from 'node:assert/strict'
import { renderMarkdown, summarizeRuns } from './summarize-viewer-benchmark.mjs'

function run(iteration, openFileMs, status = 'passed') {
  return {
    runId: `sampledata-${iteration}`,
    status,
    startedAt: `2026-07-14T16:00:0${iteration}+08:00`,
    scenario: 'sampledata-multilayer',
    metrics: {
      openFileMs,
      loadLayersMs: openFileMs + 10,
      fitVisibleLayersMs: 2,
      selectAndFitMs: 5,
    },
    error: status === 'failed' ? 'boom' : '',
    iteration,
    temperature: iteration === 1 ? 'cold' : 'warm',
    peakRssKiB: 100000 + iteration,
    environment: {
      gitCommit: 'abc123',
      macOSVersion: '15.5',
      cpu: 'Apple M4',
      memoryBytes: 17179869184,
    },
  }
}

test('summarizeRuns reports cold value plus warm median and slowest', () => {
  const summary = summarizeRuns([
    run(1, 100),
    run(2, 10),
    run(3, 30),
    run(4, 20),
    run(5, 40),
  ])

  assert.equal(summary.status, 'passed')
  assert.equal(summary.scenarios[0].cold.metrics.openFileMs, 100)
  assert.equal(summary.scenarios[0].warm.openFileMs.median, 25)
  assert.equal(summary.scenarios[0].warm.openFileMs.slowest, 40)
  assert.equal(summary.scenarios[0].peakRssKiB, 100005)

  const markdown = renderMarkdown(summary)
  assert.match(markdown, /sampledata-multilayer/)
  assert.match(markdown, /25\.00/)
  assert.match(markdown, /人工验收/)
})

test('summarizeRuns preserves failed runs and marks the report failed', () => {
  const summary = summarizeRuns([
    run(1, 100),
    run(2, 10),
    run(3, 30, 'failed'),
    run(4, 20),
    run(5, 40),
  ])

  assert.equal(summary.status, 'failed')
  assert.equal(summary.failures.length, 1)
  assert.equal(summary.failures[0].runId, 'sampledata-3')
  assert.equal(summary.failures[0].error, 'boom')
  assert.match(renderMarkdown(summary), /boom/)
})

test('summarizeRuns rejects an incomplete five-run scenario', () => {
  assert.throws(
    () => summarizeRuns([run(1, 100), run(2, 10)]),
    /exactly 5 runs/,
  )
})
