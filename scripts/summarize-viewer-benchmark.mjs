#!/usr/bin/env node

import fs from 'node:fs/promises'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

const metricNames = [
  'openFileMs',
  'loadLayersMs',
  'fitVisibleLayersMs',
  'selectAndFitMs',
]

function median(values) {
  if (values.length === 0) {
    return null
  }
  const sorted = [...values].sort((a, b) => a - b)
  const middle = Math.floor(sorted.length / 2)
  if (sorted.length % 2 === 1) {
    return sorted[middle]
  }
  return (sorted[middle - 1] + sorted[middle]) / 2
}

export function summarizeRuns(runs) {
  if (!Array.isArray(runs) || runs.length === 0) {
    throw new Error('benchmark results must contain runs')
  }

  const groups = new Map()
  for (const run of runs) {
    const group = groups.get(run.scenario) ?? []
    group.push(run)
    groups.set(run.scenario, group)
  }

  const scenarios = []
  for (const [name, scenarioRuns] of groups) {
    if (scenarioRuns.length !== 5) {
      throw new Error(`scenario ${name} must contain exactly 5 runs`)
    }
    scenarioRuns.sort((a, b) => a.iteration - b.iteration)
    const cold = scenarioRuns.find((run) => run.iteration === 1)
    const warmRuns = scenarioRuns.filter((run) => run.iteration > 1 && run.status === 'passed')
    if (!cold) {
      throw new Error(`scenario ${name} is missing cold iteration 1`)
    }

    const warm = {}
    for (const metricName of metricNames) {
      const values = warmRuns.map((run) => Number(run.metrics[metricName]))
      warm[metricName] = {
        median: median(values),
        slowest: values.length > 0 ? Math.max(...values) : null,
      }
    }

    const rssValues = scenarioRuns
      .map((run) => Number(run.peakRssKiB))
      .filter((value) => Number.isFinite(value) && value > 0)
    scenarios.push({
      name,
      cold,
      runs: scenarioRuns,
      warm,
      peakRssKiB: rssValues.length > 0 ? Math.max(...rssValues) : null,
    })
  }

  const failures = runs
    .filter((run) => run.status !== 'passed')
    .map((run) => ({ runId: run.runId, scenario: run.scenario, error: run.error || 'unknown error' }))

  return {
    status: failures.length === 0 ? 'passed' : 'failed',
    generatedAt: new Date().toISOString(),
    environment: runs[0].environment ?? {},
    scenarios,
    failures,
  }
}

function formatNumber(value) {
  return value == null ? 'N/A' : Number(value).toFixed(2)
}

function formatMiB(kib) {
  return kib == null ? 'N/A' : (Number(kib) / 1024).toFixed(2)
}

export function renderMarkdown(summary) {
  const lines = [
    '# udbx-viewer macOS 本机性能报告',
    '',
    `- 状态：${summary.status}`,
    `- 生成时间：${summary.generatedAt}`,
    `- Git 提交：${summary.environment.gitCommit ?? 'N/A'}`,
    `- macOS：${summary.environment.macOSVersion ?? 'N/A'}`,
    `- CPU：${summary.environment.cpu ?? 'N/A'}`,
    `- 物理内存：${summary.environment.memoryBytes ? `${(summary.environment.memoryBytes / 1024 / 1024 / 1024).toFixed(2)} GiB` : 'N/A'}`,
    '',
  ]

  for (const scenario of summary.scenarios) {
    lines.push(`## ${scenario.name}`, '')
    lines.push('| 轮次 | 温度 | 打开文件 ms | 加载图层 ms | 适配范围 ms | 选择定位 ms | 峰值 RSS MiB | 状态 |')
    lines.push('| ---: | --- | ---: | ---: | ---: | ---: | ---: | --- |')
    for (const run of scenario.runs) {
      lines.push(`| ${run.iteration} | ${run.temperature} | ${formatNumber(run.metrics.openFileMs)} | ${formatNumber(run.metrics.loadLayersMs)} | ${formatNumber(run.metrics.fitVisibleLayersMs)} | ${formatNumber(run.metrics.selectAndFitMs)} | ${formatMiB(run.peakRssKiB)} | ${run.status} |`)
    }
    lines.push('', '| 指标 | 冷启动 | 热运行中位数 | 热运行最慢值 |', '| --- | ---: | ---: | ---: |')
    for (const metricName of metricNames) {
      lines.push(`| ${metricName} | ${formatNumber(scenario.cold.metrics[metricName])} | ${formatNumber(scenario.warm[metricName].median)} | ${formatNumber(scenario.warm[metricName].slowest)} |`)
    }
    lines.push(`| peakRssMiB | ${formatMiB(scenario.cold.peakRssKiB)} | N/A | ${formatMiB(scenario.peakRssKiB)} |`, '')
  }

  lines.push('## 失败详情', '')
  if (summary.failures.length === 0) {
    lines.push('无。', '')
  } else {
    for (const failure of summary.failures) {
      lines.push(`- ${failure.runId}（${failure.scenario}）：${failure.error}`)
    }
    lines.push('')
  }

  lines.push(
    '## 人工验收',
    '',
    '- [ ] SampleData 点、线、面和 CADDT 多图层加载、显隐与移除正常。',
    '- [ ] 地图与属性表双向选择，点、线、面按对应几何范围定位。',
    '- [ ] henan 县级行政区划 164 条完整可见，第 2 页记录可高亮定位。',
    '- [ ] Viewer 设置持久化、采样提示和错误提示正常。',
    '- [ ] 损坏文件或不支持数据集显示错误且不白屏、不崩溃。',
    '',
  )
  return `${lines.join('\n')}\n`
}

function parseArgs(args) {
  const options = {}
  for (let index = 0; index < args.length; index += 2) {
    const flag = args[index]
    const value = args[index + 1]
    if (!value || !['--input-dir', '--json-out', '--markdown-out'].includes(flag)) {
      throw new Error(`invalid argument: ${flag ?? ''}`)
    }
    options[flag.slice(2)] = value
  }
  if (!options['input-dir'] || !options['json-out'] || !options['markdown-out']) {
    throw new Error('--input-dir, --json-out and --markdown-out are required')
  }
  return options
}

async function main() {
  const options = parseArgs(process.argv.slice(2))
  const names = (await fs.readdir(options['input-dir']))
    .filter((name) => name.endsWith('.json') && name !== 'summary.json')
    .sort()
  const runs = await Promise.all(names.map(async (name) => {
    const data = await fs.readFile(path.join(options['input-dir'], name), 'utf8')
    return JSON.parse(data)
  }))
  const summary = summarizeRuns(runs)
  await fs.mkdir(path.dirname(options['json-out']), { recursive: true })
  await fs.writeFile(options['json-out'], `${JSON.stringify(summary, null, 2)}\n`, 'utf8')
  await fs.writeFile(options['markdown-out'], renderMarkdown(summary), 'utf8')
  if (summary.status !== 'passed') {
    process.exitCode = 1
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : String(error))
    process.exitCode = 1
  })
}
