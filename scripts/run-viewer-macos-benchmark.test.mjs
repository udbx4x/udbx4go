import test from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
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
