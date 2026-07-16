import React, { useEffect, useRef } from 'react'
import {
  GetDatasetSpatialSummary,
  GetFeatureAttributes,
  ListDatasets,
  LoadDatasetPage,
  LoadSpatialPreview,
  OpenUDBXFile,
  QuitBenchmark,
  SaveBenchmarkResult,
} from '../../wailsjs/go/main/App'
import { main } from '../../wailsjs/go/models'
import { OpenLayersSpatialRendererAdapter } from '../spatial/OpenLayersSpatialRendererAdapter'
import { runBenchmarkScenario as defaultRunBenchmarkScenario } from './runBenchmarkScenario'
import type {
  BenchmarkConfig,
  BenchmarkDependencies,
  BenchmarkResult,
  SpatialPreviewRequest,
} from './types'
import type { MapLayerState, SelectedMapFeature } from '../types'

interface BenchmarkMapAdapter {
  mount(container: HTMLElement): void
  destroy(): void
  setLayer(layer: MapLayerState): void
  fitAllVisibleLayers(): void
  setSelection(selection: SelectedMapFeature): void
  fitFeature(datasetName: string, featureID: number): void
}

interface BenchmarkRunnerProps {
  config: BenchmarkConfig
  adapterFactory?: () => BenchmarkMapAdapter
  runScenario?: typeof defaultRunBenchmarkScenario
  saveResult?: (result: BenchmarkResult) => Promise<void>
  quitBenchmark?: () => Promise<void>
}

const zeroMetrics = {
  openFileMs: 0,
  loadLayersMs: 0,
  fitVisibleLayersMs: 0,
  selectAndFitMs: 0,
}

function waitForExitFrame(): Promise<void> {
  return new Promise((resolve) => {
    let settled = false
    let timeoutID: number | undefined
    const finish = () => {
      if (settled) {
        return
      }
      settled = true
      if (timeoutID !== undefined) {
        window.clearTimeout(timeoutID)
      }
      resolve()
    }

    timeoutID = window.setTimeout(finish, 250)
    window.requestAnimationFrame(finish)
  })
}

export const BenchmarkRunner: React.FC<BenchmarkRunnerProps> = ({
  config,
  adapterFactory = () => new OpenLayersSpatialRendererAdapter(),
  runScenario = defaultRunBenchmarkScenario,
  saveResult = async (result) => SaveBenchmarkResult(new main.BenchmarkResultDTO(result)),
  quitBenchmark = QuitBenchmark,
}) => {
  const containerRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!containerRef.current) {
      return
    }

    const adapter = adapterFactory()
    adapter.mount(containerRef.current)
    let cancelled = false

    const dependencies: BenchmarkDependencies = {
      now: () => performance.now(),
      openFile: OpenUDBXFile,
      listDatasets: ListDatasets,
      getSpatialSummary: GetDatasetSpatialSummary,
      loadSpatialPreview: (datasetName: string, request: SpatialPreviewRequest) =>
        LoadSpatialPreview(datasetName, new main.SpatialPreviewRequestDTO({
          viewport: undefined,
          ...request,
        })),
      loadDatasetPage: LoadDatasetPage,
      getFeatureAttributes: GetFeatureAttributes,
      setLayer: (layer) => adapter.setLayer(layer),
      fitAllVisibleLayers: () => adapter.fitAllVisibleLayers(),
      setSelection: (selection) => adapter.setSelection(selection),
      fitFeature: (datasetName, featureID) => adapter.fitFeature(datasetName, featureID),
    }

    const execute = async () => {
      let result: BenchmarkResult
      try {
        result = await runScenario(config, dependencies)
      } catch (error) {
        result = {
          runId: config.runId,
          status: 'failed',
          startedAt: new Date().toISOString(),
          scenario: config.scenario.name,
          metrics: zeroMetrics,
          error: error instanceof Error ? error.message : String(error),
        }
      }

      if (cancelled) {
        return
      }
      try {
        await saveResult(result)
      } finally {
        await waitForExitFrame()
        await quitBenchmark()
      }
    }

    void execute()

    return () => {
      cancelled = true
      adapter.destroy()
    }
  }, [adapterFactory, config, quitBenchmark, runScenario, saveResult])

  return (
    <main style={{ position: 'fixed', inset: 0, background: '#f8f9fa' }}>
      <div ref={containerRef} style={{ position: 'absolute', inset: 0 }} />
      <div
        style={{
          position: 'absolute',
          top: 16,
          left: 16,
          padding: '10px 12px',
          background: '#ffffff',
          border: '1px solid #dee2e6',
          borderRadius: 4,
          fontFamily: 'system-ui, sans-serif',
        }}
      >
        <strong>本机基准运行中</strong>
        <div>{config.scenario.name}</div>
      </div>
    </main>
  )
}
