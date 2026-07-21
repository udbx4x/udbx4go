#!/usr/bin/env node

import fs from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

export async function setViewerConcurrency(filePath, value) {
  const concurrency = Number(value)
  if (![1, 2, 3].includes(concurrency)) throw new Error('concurrency must be 1, 2 or 3')
  if (!path.isAbsolute(filePath)) throw new Error('policy file path must be absolute')
  const source = await fs.readFile(filePath, 'utf8')
  const pattern = /^export const VIEWPORT_QUERY_MAX_CONCURRENCY = [123]\n$/
  if (!pattern.test(source)) throw new Error('policy file does not contain the expected concurrency constant')
  const next = `export const VIEWPORT_QUERY_MAX_CONCURRENCY = ${concurrency}\n`
  const temporaryPath = `${filePath}.tmp-${process.pid}`
  await fs.writeFile(temporaryPath, next, 'utf8')
  await fs.rename(temporaryPath, filePath)
}

function parseArgs(args) {
  if (args.length !== 4 || args[0] !== '--file' || args[2] !== '--value') {
    throw new Error('usage: set-viewer-concurrency.mjs --file ABSOLUTE_PATH --value 1|2|3')
  }
  return { filePath: args[1], value: args[3] }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const { filePath, value } = parseArgs(process.argv.slice(2))
  setViewerConcurrency(filePath, value).catch((error) => {
    console.error(error instanceof Error ? error.message : String(error))
    process.exitCode = 1
  })
}
