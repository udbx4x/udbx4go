import type { BoundingBox, SpatialPreview } from '../types'

export interface ViewportQueryLayer {
  datasetName: string
  visible: boolean
  requiredIds: number[]
}

export interface ViewportQueryJob {
  datasetName: string
  bounds: BoundingBox
  requiredIds: number[]
  fileGeneration: number
  version: number
}

interface LayerRequestState {
  appliedVersion: number
  nextVersion: number
  inFlight: ViewportQueryJob | null
  pending: ViewportQueryJob | null
}

interface CoordinatorDependencies {
  loadPreview: (job: ViewportQueryJob) => Promise<SpatialPreview>
  applyPreview: (datasetName: string, preview: SpatialPreview) => void
  applyLoading: (datasetName: string) => void
  applyError: (datasetName: string, error: string) => void
  getFileGeneration: () => number
  getLayer: (datasetName: string) => { visible: boolean } | undefined
}

interface DebouncedViewport {
  bounds: BoundingBox
  layers: Array<ViewportQueryLayer & { version: number }>
  fileGeneration: number
}

const DEFAULT_DEBOUNCE_MS = 250
const DEFAULT_BUFFER_RATIO = 0.15

export class ViewportQueryCoordinator {
  private readonly layerStates = new Map<string, LayerRequestState>()
  private debounceTimer: ReturnType<typeof setTimeout> | null = null
  private debouncedViewport: DebouncedViewport | null = null
  private activeJobs = 0

  constructor(
    private readonly dependencies: CoordinatorDependencies,
    private readonly debounceMs = DEFAULT_DEBOUNCE_MS,
    private readonly bufferRatio = DEFAULT_BUFFER_RATIO,
  ) {}

  scheduleViewport(
    viewport: BoundingBox,
    layers: ViewportQueryLayer[],
    fileGeneration: number,
  ): void {
    const bounds = bufferBounds(viewport, this.bufferRatio)
    const scheduledLayers = layers
      .filter((layer) => layer.visible)
      .map((layer) => {
        const state = this.getOrCreateLayerState(layer.datasetName)
        state.nextVersion += 1
        return {
          ...layer,
          requiredIds: uniqueIDs(layer.requiredIds),
          version: state.nextVersion,
        }
      })

    this.debouncedViewport = { bounds, layers: scheduledLayers, fileGeneration }
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer)
    }
    this.debounceTimer = setTimeout(() => {
      this.debounceTimer = null
      this.publishDebouncedViewport()
    }, this.debounceMs)
  }

  invalidateLayer(datasetName: string): void {
    const state = this.layerStates.get(datasetName)
    if (!state) {
      return
    }
    state.nextVersion += 1
    state.pending = null
  }

  invalidateAll(): void {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer)
      this.debounceTimer = null
    }
    this.debouncedViewport = null
    this.layerStates.forEach((state) => {
      state.nextVersion += 1
      state.pending = null
    })
    this.layerStates.clear()
  }

  private publishDebouncedViewport(): void {
    const scheduled = this.debouncedViewport
    this.debouncedViewport = null
    if (!scheduled) {
      return
    }

    scheduled.layers.forEach((layer) => {
      const state = this.layerStates.get(layer.datasetName)
      if (!state || state.nextVersion !== layer.version) {
        return
      }
      state.pending = {
        datasetName: layer.datasetName,
        bounds: scheduled.bounds,
        requiredIds: layer.requiredIds,
        fileGeneration: scheduled.fileGeneration,
        version: layer.version,
      }
    })
    this.pump()
  }

  private pump(): void {
    if (this.activeJobs >= 1) {
      return
    }

    for (const [datasetName, state] of this.layerStates) {
      if (state.inFlight || !state.pending) {
        continue
      }

      const job = state.pending
      state.pending = null
      state.inFlight = job
      this.activeJobs += 1
      this.dependencies.applyLoading(datasetName)
      void this.runJob(state, job)
      return
    }
  }

  private async runJob(state: LayerRequestState, job: ViewportQueryJob): Promise<void> {
    try {
      const preview = await this.dependencies.loadPreview(job)
      if (this.canApply(state, job, preview)) {
        state.appliedVersion = job.version
        this.dependencies.applyPreview(job.datasetName, preview)
      }
    } catch (error) {
      if (this.canApply(state, job)) {
        this.dependencies.applyError(job.datasetName, errorMessage(error))
      }
    } finally {
      if (state.inFlight === job) {
        state.inFlight = null
      }
      this.activeJobs -= 1
      this.pump()
    }
  }

  private canApply(
    state: LayerRequestState,
    job: ViewportQueryJob,
    preview?: SpatialPreview,
  ): boolean {
    const currentState = this.layerStates.get(job.datasetName)
    const layer = this.dependencies.getLayer(job.datasetName)
    if (
      currentState !== state ||
      state.nextVersion !== job.version ||
      this.dependencies.getFileGeneration() !== job.fileGeneration ||
      !layer?.visible
    ) {
      return false
    }

    if (!preview) {
      return true
    }

    return preview.fileGeneration === job.fileGeneration && boundsEqual(preview.queriedBounds, job.bounds)
  }

  private getOrCreateLayerState(datasetName: string): LayerRequestState {
    const existing = this.layerStates.get(datasetName)
    if (existing) {
      return existing
    }

    const state: LayerRequestState = {
      appliedVersion: 0,
      nextVersion: 0,
      inFlight: null,
      pending: null,
    }
    this.layerStates.set(datasetName, state)
    return state
  }
}

function bufferBounds(bounds: BoundingBox, ratio: number): BoundingBox {
  const horizontalBuffer = (bounds.maxX - bounds.minX) * ratio
  const verticalBuffer = (bounds.maxY - bounds.minY) * ratio
  return {
    minX: bounds.minX - horizontalBuffer,
    minY: bounds.minY - verticalBuffer,
    maxX: bounds.maxX + horizontalBuffer,
    maxY: bounds.maxY + verticalBuffer,
  }
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

function uniqueIDs(ids: number[]): number[] {
  return [...new Set(ids)]
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : '当前范围加载失败'
}
