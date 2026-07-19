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
  hasRenderedFeaturePixels(): boolean
}

interface BenchmarkRunnerProps {
  config: BenchmarkConfig
  adapterFactory?: () => BenchmarkMapAdapter
  runScenario?: typeof defaultRunBenchmarkScenario
  saveResult?: (result: BenchmarkResult) => Promise<void>
  quitBenchmark?: () => Promise<void>
  viewportStepTimeoutMs?: number
}

interface StepMeasurement {
  startedAt: number
  backendQueryMS: number[]
  strategies: string[]
  resolve: (result: BenchmarkViewportResult) => void
  reject: (error: Error) => void
  timeoutID: number
  viewportObserved: boolean
  requiredLayerNames: string[]
  requestedBounds: BoundingBox | null
}

interface StaleProbeGate {
  captured: boolean
  firstHeld: ReturnType<typeof createSignal>
  releaseFirst: ReturnType<typeof createSignal>
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
    let stepMeasurement: StepMeasurement | null = null
    let staleProbeGate: StaleProbeGate | null = null

    const abortStaleProbe = (error: Error) => {
      const gate = staleProbeGate
      if (!gate) {
        return
      }
      staleProbeGate = null
      gate.firstHeld.reject(error)
      gate.releaseFirst.resolve()
    }

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
        const error = new Error(queriedLayers.find((layer) => layer.queryStatus === 'error')?.queryError || 'viewport query failed')
        abortStaleProbe(error)
        measurement.reject(error)
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
      try {
        await adapter.waitForRenderComplete()
      } catch (error) {
        if (stepMeasurement !== measurement) {
          return
        }
        window.clearTimeout(measurement.timeoutID)
        stepMeasurement = null
        const renderError = error instanceof Error ? error : new Error(String(error))
        abortStaleProbe(renderError)
        measurement.reject(renderError)
        return
      }
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
        blankRender: finalFeatureCount === 0 || !adapter.hasRenderedFeaturePixels(),
        strategies: measurement.strategies,
        featureIDs,
      })
    }

    const coordinator = new ViewportQueryCoordinator({
      loadPreview: async (job) => {
        try {
          const preview = await LoadSpatialPreview(job.datasetName, new main.SpatialPreviewRequestDTO({
            viewport: job.bounds,
            requiredIds: job.requiredIds,
            limit: 1000,
            maxVertices: 1000000,
            simplify: false,
          }))
          const gate = staleProbeGate
          if (gate && !gate.captured) {
            gate.captured = true
            stepMeasurement?.backendQueryMS.push(preview.queryDurationMs)
            gate.firstHeld.resolve()
            await gate.releaseFirst.promise
          }
          return preview
        } catch (error) {
          const gate = staleProbeGate
          if (gate && !gate.captured) {
            gate.firstHeld.reject(asError(error))
          }
          throw error
        }
      },
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
        fileGeneration = file.fileGeneration
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
              .map((layer) => `${layer.datasetName}=${layer.queryStatus}/${formatBounds(layer.lastQueriedBounds)}`)
              .join(',')
            const viewportObserved = stepMeasurement?.viewportObserved ?? false
            const requestedBounds = formatBounds(stepMeasurement?.requestedBounds)
            stepMeasurement = null
            reject(new Error(`viewport step timed out after ${viewportStepTimeoutMs}ms; viewportObserved=${viewportObserved}; requested=${requestedBounds}; layers=${statuses}`))
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
      runStaleViewportProbe: (first, latest, requiredIDs) => {
        if (stepMeasurement || staleProbeGate) {
          return Promise.reject(new Error('stale viewport probe requires an idle benchmark runner'))
        }
        const fitLayer = [...layerStates.values()].find((layer) => layer.visible && layer.summary?.viewportQuerySupported)
        if (!fitLayer) {
          return Promise.reject(new Error('stale viewport probe has no queryable visible layer'))
        }
        currentRequiredIDs = requiredIDs
        const requiredLayerNames = [...layerStates.values()]
          .filter((layer) => layer.visible && layer.summary?.viewportQuerySupported)
          .map((layer) => layer.datasetName)
        const gate: StaleProbeGate = {
          captured: false,
          firstHeld: createSignal(),
          releaseFirst: createSignal(),
        }
        staleProbeGate = gate

        const result = createDeferred<BenchmarkViewportResult>()
        const timeoutID = window.setTimeout(() => {
          const error = new Error(`stale viewport probe timed out after ${viewportStepTimeoutMs}ms`)
          if (stepMeasurement === measurement) {
            stepMeasurement = null
          }
          abortStaleProbe(error)
          result.reject(error)
        }, viewportStepTimeoutMs)
        const measurement: StepMeasurement = {
          startedAt: performance.now(),
          backendQueryMS: [],
          strategies: [],
          resolve: result.resolve,
          reject: result.reject,
          timeoutID,
          viewportObserved: false,
          requiredLayerNames,
          requestedBounds: null,
        }
        stepMeasurement = measurement
        const resultPromise = result.promise

        adapter.fitBounds(first.bounds, first.geometryKind ?? geometryKind(fitLayer.kind))
        handleViewport(adapter.getViewport() ?? first.bounds)

        return (async () => {
          try {
            await Promise.race([
              gate.firstHeld.promise,
              resultPromise.then(() => {
                throw new Error('stale viewport probe completed before first response was held')
              }),
            ])
            if (cancelled) {
              throw new Error('benchmark runner unmounted')
            }
            adapter.fitBounds(latest.bounds, latest.geometryKind ?? geometryKind(fitLayer.kind))
            handleViewport(adapter.getViewport() ?? latest.bounds)
            gate.releaseFirst.resolve()
            return await resultPromise
          } catch (error) {
            const probeError = asError(error)
            if (stepMeasurement === measurement) {
              window.clearTimeout(measurement.timeoutID)
              stepMeasurement = null
              measurement.reject(probeError)
            }
            abortStaleProbe(probeError)
            throw probeError
          } finally {
            if (staleProbeGate === gate) {
              staleProbeGate = null
            }
            gate.releaseFirst.resolve()
          }
        })()
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
      const error = new Error('benchmark runner unmounted')
      abortStaleProbe(error)
      if (stepMeasurement) {
        window.clearTimeout(stepMeasurement.timeoutID)
        stepMeasurement.reject(error)
        stepMeasurement = null
      }
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

function formatBounds(bounds: BoundingBox | null | undefined): string {
  if (!bounds) {
    return 'none'
  }
  return `${bounds.minX},${bounds.minY},${bounds.maxX},${bounds.maxY}`
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

function createSignal() {
  let resolve!: () => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<void>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

function asError(error: unknown): Error {
  return error instanceof Error ? error : new Error(String(error))
}
