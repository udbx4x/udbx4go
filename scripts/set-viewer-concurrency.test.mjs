import test from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

const script = new URL('./set-viewer-concurrency.mjs', import.meta.url).pathname

test('set-viewer-concurrency updates only the policy constant', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'udbx-viewer-policy-'))
  const policyPath = path.join(dir, 'viewportQueryPolicy.ts')
  fs.writeFileSync(policyPath, 'export const VIEWPORT_QUERY_MAX_CONCURRENCY = 1\n', 'utf8')

  execFileSync(process.execPath, [script, '--file', policyPath, '--value', '2'])

  assert.equal(fs.readFileSync(policyPath, 'utf8'), 'export const VIEWPORT_QUERY_MAX_CONCURRENCY = 2\n')
  assert.throws(
    () => execFileSync(process.execPath, [script, '--file', policyPath, '--value', '4'], { stdio: 'pipe' }),
    /Command failed/,
  )
})
