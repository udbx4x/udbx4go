import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { BenchmarkRunner } from './BenchmarkRunner'
import type { BenchmarkConfig, BenchmarkResult } from './types'

const config: BenchmarkConfig = {
  runId: 'sampledata-01',
  outputPath: '/tmp/sampledata-01.json',
  scenario: {
    name: 'sampledata-multilayer',
    filePath: '/data/SampleData.udbx',
    layers: ['BaseMap_P'],
    selection: { datasetName: 'BaseMap_P', page: 1, rowIndex: 0 },
  },
}

const passedResult: BenchmarkResult = {
  runId: config.runId,
  status: 'passed',
  startedAt: '2026-07-14T16:00:00+08:00',
  scenario: config.scenario.name,
  metrics: {
    openFileMs: 10,
    loadLayersMs: 20,
    fitVisibleLayersMs: 1,
    selectAndFitMs: 5,
  },
  error: '',
}

function createAdapter() {
  return {
    mount: vi.fn(),
    destroy: vi.fn(),
    setLayer: vi.fn(),
    fitAllVisibleLayers: vi.fn(),
    setSelection: vi.fn(),
    fitFeature: vi.fn(),
  }
}

describe('BenchmarkRunner', () => {
  it('保存成功结果后退出基准应用', async () => {
    const adapter = createAdapter()
    const runScenario = vi.fn().mockResolvedValue(passedResult)
    const saveResult = vi.fn().mockResolvedValue(undefined)
    const quitBenchmark = vi.fn().mockResolvedValue(undefined)

    render(
      <BenchmarkRunner
        config={config}
        adapterFactory={() => adapter}
        runScenario={runScenario}
        saveResult={saveResult}
        quitBenchmark={quitBenchmark}
      />,
    )

    expect(screen.getByText('本机基准运行中')).toBeInTheDocument()
    expect(screen.getByText(config.scenario.name)).toBeInTheDocument()
    await waitFor(() => expect(saveResult).toHaveBeenCalledWith(passedResult))
    await waitFor(() => expect(quitBenchmark).toHaveBeenCalledTimes(1))
    expect(runScenario).toHaveBeenCalledWith(config, expect.any(Object))
    expect(adapter.mount).toHaveBeenCalledTimes(1)
  })

  it('执行失败时保存可诊断失败结果并退出', async () => {
    const runScenario = vi.fn().mockRejectedValue(new Error('boom'))
    const saveResult = vi.fn().mockResolvedValue(undefined)
    const quitBenchmark = vi.fn().mockResolvedValue(undefined)

    render(
      <BenchmarkRunner
        config={config}
        adapterFactory={createAdapter}
        runScenario={runScenario}
        saveResult={saveResult}
        quitBenchmark={quitBenchmark}
      />,
    )

    await waitFor(() => expect(saveResult).toHaveBeenCalledWith(expect.objectContaining({
      runId: config.runId,
      status: 'failed',
      scenario: config.scenario.name,
      metrics: {
        openFileMs: 0,
        loadLayersMs: 0,
        fitVisibleLayersMs: 0,
        selectAndFitMs: 0,
      },
      error: 'boom',
    })))
    await waitFor(() => expect(quitBenchmark).toHaveBeenCalledTimes(1))
  })

  it('动画帧被节流时仍在保存结果后退出', async () => {
    const requestAnimationFrameSpy = vi
      .spyOn(window, 'requestAnimationFrame')
      .mockImplementation(() => 1)
    const saveResult = vi.fn().mockResolvedValue(undefined)
    const quitBenchmark = vi.fn().mockResolvedValue(undefined)

    try {
      render(
        <BenchmarkRunner
          config={config}
          adapterFactory={createAdapter}
          runScenario={vi.fn().mockResolvedValue(passedResult)}
          saveResult={saveResult}
          quitBenchmark={quitBenchmark}
        />,
      )

      await waitFor(() => expect(saveResult).toHaveBeenCalledWith(passedResult))
      await waitFor(() => expect(quitBenchmark).toHaveBeenCalledTimes(1), { timeout: 600 })
    } finally {
      requestAnimationFrameSpy.mockRestore()
    }
  })
})
