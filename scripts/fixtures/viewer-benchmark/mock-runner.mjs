#!/usr/bin/env node

import fs from 'node:fs/promises'

const configPath = process.argv[2]
if (!configPath) throw new Error('benchmark config path is required')
const config = JSON.parse(await fs.readFile(configPath, 'utf8'))
const concurrency = Number(config.maxConcurrentQueries)
const warm = config.temperature === 'warm'
const renderBase = ({ 1: 200, 2: 180, 3: 170 })[concurrency] + (warm ? 0 : 20)
const forcedGateFailure = Number(process.env.UDBX_BENCHMARK_MOCK_GATE_FAIL_CONCURRENCY) === concurrency
const backendBase = forcedGateFailure ? 120 : (warm ? 20 : 40)
const stepCount = config.scenario.viewportSteps.length + 1
const observedConcurrency = Math.min(concurrency, config.scenario.layers.length)

const result = {
  runId: config.runId,
  status: 'passed',
  startedAt: '2026-07-17T00:00:00Z',
  scenario: config.scenario.name,
  metrics: {
    openFileMs: warm ? 10 : 20,
    loadLayersMs: warm ? 15 : 30,
    fitVisibleLayersMs: 2,
    selectAndFitMs: 5,
    backendQueryMs: Array.from({ length: stepCount }, (_, index) => backendBase + index),
    moveendToRenderMs: Array.from({ length: stepCount }, (_, index) => renderBase + index),
    maxConcurrentQueries: observedConcurrency,
    pendingPeak: config.scenario.layers.length,
    pendingFinal: 0,
    staleResultsDiscarded: 1,
    staleResultApplied: false,
    finalFeatureCount: config.scenario.name.includes('county') ? 164 : 1000,
    blankRenderCount: 0,
  },
  error: '',
}

await fs.mkdir(new URL('.', `file://${config.outputPath}`).pathname, { recursive: true }).catch(() => undefined)
await fs.writeFile(config.outputPath, `${JSON.stringify(result, null, 2)}\n`, 'utf8')
const exitCode = Number(process.env.UDBX_BENCHMARK_MOCK_EXIT_CODE ?? 0)
if (Number.isInteger(exitCode) && exitCode !== 0) process.exitCode = exitCode
