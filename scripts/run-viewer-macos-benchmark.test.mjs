import test from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

const scriptDir = path.dirname(new URL(import.meta.url).pathname)
const script = path.join(scriptDir, 'run-viewer-macos-benchmark.sh')
const fixtures = path.join(scriptDir, 'fixtures', 'viewer-benchmark')

test('mock workflow measures 1/2/3, selects, rebuilds and runs a separate final suite', () => {
  const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'udbx-viewer-benchmark-'))
  const reportPath = path.join(outputDir, 'acceptance.md')

  execFileSync('bash', [
    script,
    '--mock-fixtures', fixtures,
    '--output-dir', outputDir,
    '--acceptance-report', reportPath,
  ], { stdio: 'pipe', timeout: 30000 })

  const selection = JSON.parse(fs.readFileSync(path.join(outputDir, 'selection.json'), 'utf8'))
  assert.equal(selection.selected, 2)

  const candidates = [1, 2, 3].map((concurrency) => {
    const dir = path.join(outputDir, 'candidates', `concurrency-${concurrency}`)
    assert.equal(fs.readdirSync(path.join(dir, 'raw')).filter((name) => name.endsWith('.json')).length, 30)
    return JSON.parse(fs.readFileSync(path.join(dir, 'summary.json'), 'utf8'))
  })
  assert.deepEqual(candidates.map((summary) => summary.maxConcurrentQueries), [1, 2, 3])
  assert.equal(new Set(candidates.map((summary) => summary.environment.appSha256)).size, 1)

  const finalDir = path.join(outputDir, 'final')
  assert.equal(fs.readdirSync(path.join(finalDir, 'raw')).filter((name) => name.endsWith('.json')).length, 30)
  const finalSummary = JSON.parse(fs.readFileSync(path.join(finalDir, 'summary.json'), 'utf8'))
  assert.equal(finalSummary.maxConcurrentQueries, 2)
  assert.notEqual(finalSummary.environment.appSha256, candidates[0].environment.appSha256)

  const policy = fs.readFileSync(
    path.join(outputDir, 'mock-workspace', 'frontend', 'src', 'spatial', 'viewportQueryPolicy.ts'),
    'utf8',
  )
  assert.equal(policy, 'export const VIEWPORT_QUERY_MAX_CONCURRENCY = 2\n')

  const report = fs.readFileSync(reportPath, 'utf8')
  assert.match(report, /并发候选比较/)
  assert.match(report, /自动选择：2/)
  assert.match(report, /最终重建后独立重跑：PASS/)
})

test('nonzero app exit fails the run even after a passed result was written', () => {
  const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'udbx-viewer-exit-'))
  const result = spawnSync('bash', [
    script,
    '--mock-fixtures', fixtures,
    '--max-concurrent', '1',
    '--output-dir', outputDir,
  ], {
    encoding: 'utf8',
    env: { ...process.env, UDBX_BENCHMARK_MOCK_EXIT_CODE: '7' },
    timeout: 30000,
  })

  assert.notEqual(result.status, 0)
  const rawDir = path.join(outputDir, 'raw')
  const rawFiles = fs.readdirSync(rawDir).filter((name) => name.endsWith('.json'))
  assert.ok(rawFiles.length >= 1)
  const run = JSON.parse(fs.readFileSync(path.join(rawDir, rawFiles[0]), 'utf8'))
  assert.equal(run.status, 'failed')
  assert.equal(run.processExitCode, 7)
  assert.match(run.error, /exit code 7/)
})

for (const failure of ['build', 'final']) {
  test(`${failure} failure restores the original concurrency policy`, () => {
    const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), `udbx-viewer-${failure}-`))
    const result = spawnSync('bash', [
      script,
      '--mock-fixtures', fixtures,
      '--output-dir', outputDir,
    ], {
      encoding: 'utf8',
      env: { ...process.env, UDBX_BENCHMARK_MOCK_FAIL_STAGE: failure },
      timeout: 30000,
    })

    assert.notEqual(result.status, 0)
    const policy = fs.readFileSync(
      path.join(outputDir, 'mock-workspace', 'frontend', 'src', 'spatial', 'viewportQueryPolicy.ts'),
      'utf8',
    )
    assert.equal(policy, 'export const VIEWPORT_QUERY_MAX_CONCURRENCY = 1\n')
  })
}

test('acceptance report publication failure restores policy and preserves the existing target', () => {
  const outputDir = fs.mkdtempSync(path.join(os.tmpdir(), 'udbx-viewer-report-'))
  const reportPath = path.join(outputDir, 'acceptance-target')
  fs.mkdirSync(reportPath)

  const result = spawnSync('bash', [
    script,
    '--mock-fixtures', fixtures,
    '--output-dir', outputDir,
    '--acceptance-report', reportPath,
  ], { encoding: 'utf8', timeout: 30000 })

  assert.notEqual(result.status, 0)
  const policy = fs.readFileSync(
    path.join(outputDir, 'mock-workspace', 'frontend', 'src', 'spatial', 'viewportQueryPolicy.ts'),
    'utf8',
  )
  assert.equal(policy, 'export const VIEWPORT_QUERY_MAX_CONCURRENCY = 1\n')
  assert.ok(fs.statSync(reportPath).isDirectory())
})
