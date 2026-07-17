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
import {
  VIEWPORT_QUERY_DEBOUNCE_MS,
  ViewportQueryCoordinator,
} from '../spatial/ViewportQueryCoordinator'
import { createDefaultLayerStyle } from '../spatial/layerStyle'
import { runBenchmarkScenario as defaultRunBenchmarkScenario } from './runBenchmarkScenario'
import type {
  BenchmarkConfig,
  BenchmarkDependencies,
  BenchmarkResult,
  SpatialPreviewRequest,
  BenchmarkViewportResult,
} from './types'
import type { BoundingBox, MapLayerState, SelectedMapFeature, SpatialPreview } from '../types'

interface BenchmarkMapAdapter {
  mount(container: HTMLElement): void
  destroy(): void
  setLayer(layer: MapLayerState): void
  fitAllVisibleLayers(): void
  setSelection(selection: SelectedMapFeature): void
  fitFeature(datasetName: string, featureID: number): void
  fitBounds(bounds: BoundingBox, geometryKind: 'point' | 'line' | 'polygon'): void
  hasFeature(datasetName: string, featureID: number): boolean
  isSelectionHighlighted(selection: SelectedMapFeature): boolean
  setLayerVisible(datasetName: string, visible: boolean): void
  removeLayer(datasetName: string): void
  onViewportChange(handler: (viewport: BoundingBox) => void): () => void
  getViewport(): BoundingBox | null
  waitForRenderComplete(): Promise<void>
  getVisibleFeatureCount(): number
}

interface BenchmarkRunnerProps {
  config: BenchmarkConfig
  adapterFactory?: () => BenchmarkMapAdapter
  runScenario?: typeof defaultRunBenchmarkScenario
  saveResult?: (result: BenchmarkResult) => Promise<void>
  quitBenchmark?: () => Promise<void>
  viewportStepTimeoutMs?: number
}

const zeroMetrics = {
  openFileMs: 0,
  loadLayersMs: 0,
  fitVisibleLayersMs: 0,
  selectAndFitMs: 0,
  backendQueryMs: [],
  moveendToRenderMs: [],
  maxConcurrentQueries: 0,
  pendingPeak: 0,
  pendingFinal: 0,
  staleResultsDiscarded: 0,
  staleResultApplied: false,
  finalFeatureCount: 0,
  blankRenderCount: 0,
}

const defaultViewportStepTimeoutMS = 30000

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
  viewportStepTimeoutMs = defaultViewportStepTimeoutMS,
}) => {
  const containerRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!containerRef.current) {
      return
    }

    const adapter = adapterFactory()
    adapter.mount(containerRef.current)
    let cancelled = false
    let fileGeneration = 0
    const layerStates = new Map<string, MapLayerState>()
    let stepMeasurement: {
      startedAt: number
      backendQueryMS: number[]
      strategies: string[]
      resolve: (result: BenchmarkViewportResult) => void
      reject: (error: Error) => void
      timeoutID: number
      viewportObserved: boolean
      requiredLayerNames: string[]
      requestedBounds: BoundingBox | null
    } | null = null

    const publishLayer = (layer: MapLayerState) => {
      layerStates.set(layer.datasetName, layer)
      if (layer.preview) {
        adapter.setLayer(layer)
      }
    }

    const maybeFinishViewportStep = async () => {
      const measurement = stepMeasurement
      if (!measurement) {
        return
      }
      const queriedLayers = measurement.requiredLayerNames
        .map((datasetName) => layerStates.get(datasetName))
        .filter((layer): layer is MapLayerState => Boolean(layer))
      if (!measurement.requestedBounds || queriedLayers.length !== measurement.requiredLayerNames.length) {
        return
      }
      if (queriedLayers.some((layer) => layer.queryStatus === 'error')) {
        measurement.reject(new Error(queriedLayers.find((layer) => layer.queryStatus === 'error')?.queryError || 'viewport query failed'))
        window.clearTimeout(measurement.timeoutID)
        stepMeasurement = null
        return
      }
      if (queriedLayers.some((layer) =>
        (layer.queryStatus !== 'ready' && layer.queryStatus !== 'degraded') ||
        !boundsEqual(layer.lastQueriedBounds, measurement.requestedBounds!),
      )) {
        return
      }
      await adapter.waitForRenderComplete()
      if (stepMeasurement !== measurement) {
        return
      }
      window.clearTimeout(measurement.timeoutID)
      const finalFeatureCount = adapter.getVisibleFeatureCount()
      const selectionLayer = layerStates.get(config.scenario.selection.datasetName)
      const featureIDs = (selectionLayer?.preview?.features ?? [])
        .map((feature) => feature.id)
        .filter((featureID) => adapter.hasFeature(config.scenario.selection.datasetName, featureID))
      stepMeasurement = null
      measurement.resolve({
        backendQueryMs: measurement.backendQueryMS,
        moveendToRenderMs: performance.now() - measurement.startedAt,
        finalFeatureCount,
        blankRender: finalFeatureCount === 0,
        strategies: measurement.strategies,
        featureIDs,
      })
    }

    const coordinator = new ViewportQueryCoordinator({
      loadPreview: async (job) => LoadSpatialPreview(job.datasetName, new main.SpatialPreviewRequestDTO({
        viewport: job.bounds,
        requiredIds: job.requiredIds,
        limit: 1000,
        maxVertices: 1000000,
        simplify: false,
      })),
      applyLoading: (datasetName) => {
        const layer = layerStates.get(datasetName)
        if (layer) {
          publishLayer({ ...layer, queryStatus: 'loading', queryError: null })
        }
      },
      applyPreview: (datasetName, preview: SpatialPreview) => {
        const layer = layerStates.get(datasetName)
        if (layer) {
          publishLayer({
            ...layer,
            preview,
            queryStatus: preview.degradedReason ? 'degraded' : 'ready',
            queryError: null,
            lastQueriedBounds: preview.queriedBounds,
          })
          stepMeasurement?.backendQueryMS.push(preview.queryDurationMs)
          stepMeasurement?.strategies.push(preview.strategy)
        }
        void maybeFinishViewportStep()
      },
      applyError: (datasetName, error) => {
        const layer = layerStates.get(datasetName)
        if (layer) {
          publishLayer({ ...layer, queryStatus: 'error', queryError: error })
        }
        void maybeFinishViewportStep()
      },
      getFileGeneration: () => fileGeneration,
      getLayer: (datasetName) => layerStates.get(datasetName),
    }, VIEWPORT_QUERY_DEBOUNCE_MS, 0.15, config.maxConcurrentQueries)

    const handleViewport = (viewport: BoundingBox) => {
      if (stepMeasurement) {
        stepMeasurement.startedAt = performance.now()
        stepMeasurement.viewportObserved = true
      }
      const requestedBounds = coordinator.scheduleViewport(
        viewport,
        [...layerStates.values()].map((layer) => ({
          datasetName: layer.datasetName,
          visible: layer.visible && Boolean(layer.summary?.viewportQuerySupported),
          requiredIds: layer.datasetName === config.scenario.selection.datasetName
            ? currentRequiredIDs
            : [],
        })),
        fileGeneration,
      )
      if (stepMeasurement) {
        stepMeasurement.requestedBounds = requestedBounds
      }
    }
    let currentRequiredIDs: number[] = []

    const dependencies: BenchmarkDependencies = {
      now: () => performance.now(),
      openFile: async (path) => {
        const file = await OpenUDBXFile(path)
        fileGeneration += 1
        return file
      },
      listDatasets: ListDatasets,
      getSpatialSummary: GetDatasetSpatialSummary,
      loadSpatialPreview: (datasetName: string, request: SpatialPreviewRequest) =>
        LoadSpatialPreview(datasetName, new main.SpatialPreviewRequestDTO({
          viewport: undefined,
          ...request,
        })),
      loadDatasetPage: LoadDatasetPage,
      getFeatureAttributes: GetFeatureAttributes,
      setLayer: (layer) => publishLayer({
        ...layer,
        style: layer.style ?? createDefaultLayerStyle(layer.kind),
      }),
      fitAllVisibleLayers: () => adapter.fitAllVisibleLayers(),
      setSelection: async (selection) => {
        adapter.setSelection(selection)
        await adapter.waitForRenderComplete()
        return adapter.isSelectionHighlighted(selection)
      },
      runViewportStep: (step, requiredIDs) => {
        currentRequiredIDs = requiredIDs
        for (const datasetName of step.hideLayers ?? []) {
          const layer = layerStates.get(datasetName)
          if (layer) {
            coordinator.invalidateLayer(datasetName)
            publishLayer({ ...layer, visible: false })
            adapter.setLayerVisible(datasetName, false)
          }
        }
        for (const datasetName of step.showLayers ?? []) {
          const layer = layerStates.get(datasetName)
          if (layer) {
            publishLayer({ ...layer, visible: true, queryStatus: layer.preview ? 'ready' : 'idle' })
            adapter.setLayerVisible(datasetName, true)
          }
        }
        for (const datasetName of step.removeLayers ?? []) {
          coordinator.invalidateLayer(datasetName)
          layerStates.delete(datasetName)
          adapter.removeLayer(datasetName)
        }
        const fitLayer = [...layerStates.values()].find((layer) => layer.visible && layer.summary?.viewportQuerySupported)
        if (!fitLayer) {
          return Promise.reject(new Error('viewport step has no queryable visible layer'))
        }
        const requiredLayerNames = [...layerStates.values()]
          .filter((layer) => layer.visible && layer.summary?.viewportQuerySupported)
          .map((layer) => layer.datasetName)
        return new Promise<BenchmarkViewportResult>((resolve, reject) => {
          const timeoutID = window.setTimeout(() => {
            const statuses = [...layerStates.values()]
              .map((layer) => `${layer.datasetName}=${layer.queryStatus}`)
              .join(',')
            const viewportObserved = stepMeasurement?.viewportObserved ?? false
            stepMeasurement = null
            reject(new Error(`viewport step timed out after ${viewportStepTimeoutMs}ms; viewportObserved=${viewportObserved}; layers=${statuses}`))
          }, viewportStepTimeoutMs)
          stepMeasurement = {
            startedAt: performance.now(),
            backendQueryMS: [],
            strategies: [],
            resolve,
            reject,
            timeoutID,
            viewportObserved: false,
            requiredLayerNames,
            requestedBounds: null,
          }
          adapter.fitBounds(step.bounds, step.geometryKind ?? geometryKind(fitLayer.kind))
          handleViewport(adapter.getViewport() ?? step.bounds)
        })
      },
      getCoordinatorMetrics: () => coordinator.getMetrics(),
      resetCoordinatorMetrics: () => coordinator.resetMetrics(),
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
      coordinator.invalidateAll()
      adapter.destroy()
    }
  }, [adapterFactory, config, quitBenchmark, runScenario, saveResult, viewportStepTimeoutMs])

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

function geometryKind(kind: string): 'point' | 'line' | 'polygon' {
  if (kind === 'line' || kind === 'line_z') {
    return 'line'
  }
  if (kind === 'region' || kind === 'region_z') {
    return 'polygon'
  }
  return 'point'
}

function boundsEqual(actual: BoundingBox | undefined, expected: BoundingBox): boolean {
  return Boolean(
    actual &&
      actual.minX === expected.minX &&
      actual.minY === expected.minY &&
      actual.maxX === expected.maxX &&
      actual.maxY === expected.maxY,
  )
}
