import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { BoundingBox, SpatialPreview } from '../types'
import {
  ViewportQueryCoordinator,
  type ViewportQueryJob,
  type ViewportQueryLayer,
} from './ViewportQueryCoordinator'

const viewportA: BoundingBox = { minX: 0, minY: 10, maxX: 100, maxY: 50 }
const viewportB: BoundingBox = { minX: 100, minY: 50, maxX: 200, maxY: 150 }
const viewportC: BoundingBox = { minX: 200, minY: 150, maxX: 400, maxY: 350 }
const bufferedA: BoundingBox = { minX: -15, minY: 4, maxX: 115, maxY: 56 }

describe('ViewportQueryCoordinator', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('等待 250ms 后使用四周 15% buffer 查询', async () => {
    const harness = createHarness()

    harness.coordinator.scheduleViewport(viewportA, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(249)
    expect(harness.loadPreview).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(1)
    expect(harness.loadPreview).toHaveBeenCalledOnce()
    expect(harness.loadPreview.mock.calls[0][0]).toMatchObject({
      datasetName: 'points',
      bounds: bufferedA,
      fileGeneration: 1,
      requiredIds: [],
    })
  })

  it('requiredIds 去重后进入请求', async () => {
    const harness = createHarness()

    harness.coordinator.scheduleViewport(
      viewportA,
      [{ datasetName: 'points', visible: true, requiredIds: [7, 7, 7] }],
      1,
    )
    await vi.advanceTimersByTimeAsync(250)

    expect(harness.loadPreview.mock.calls[0][0].requiredIds).toEqual([7])
  })

  it('同图层只保留一个执行中请求和一个最新 pending，并覆盖中间视口', async () => {
    const harness = createHarness()

    harness.coordinator.scheduleViewport(viewportA, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)
    const first = harness.requests[0]

    harness.coordinator.scheduleViewport(viewportB, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)
    harness.coordinator.scheduleViewport(viewportC, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)

    expect(harness.loadPreview).toHaveBeenCalledOnce()
    first.resolve(previewFor(first.job))
    await flushPromises()

    expect(harness.loadPreview).toHaveBeenCalledTimes(2)
    expect(harness.loadPreview.mock.calls[1][0].bounds).toEqual({
      minX: 170,
      minY: 120,
      maxX: 430,
      maxY: 380,
    })
    expect(harness.applyPreview).not.toHaveBeenCalledWith('points', previewFor(first.job))
  })

  it('debounce 窗口内同层旧 version 不启动，只执行最新视口', async () => {
    const harness = createHarness()

    harness.coordinator.scheduleViewport(viewportA, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(200)
    harness.coordinator.scheduleViewport(viewportB, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)

    expect(harness.loadPreview).toHaveBeenCalledOnce()
    expect(harness.requests[0].job.bounds).toEqual({ minX: 85, minY: 35, maxX: 215, maxY: 165 })
  })

  it('按配置限制并发并暴露 pending、并发和迟到丢弃观测', async () => {
    const harness = createHarness(2)
    harness.layers.set('counties', { visible: true })

    harness.coordinator.scheduleViewport(
      viewportA,
      [layer('points'), layer('roads'), layer('counties')],
      1,
    )
    await vi.advanceTimersByTimeAsync(250)

    expect(harness.requests).toHaveLength(2)
    expect(harness.coordinator.getMetrics()).toMatchObject({
      maxConcurrentQueries: 2,
      pendingPeak: 3,
      activeQueries: 2,
      pendingQueries: 1,
      staleResultsDiscarded: 0,
      staleResultApplied: false,
    })

    harness.coordinator.scheduleViewport(
      viewportB,
      [layer('points'), layer('roads'), layer('counties')],
      1,
    )
    await vi.advanceTimersByTimeAsync(250)
    harness.requests[0].resolve(previewFor(harness.requests[0].job))
    await flushPromises()

    expect(harness.coordinator.getMetrics().staleResultsDiscarded).toBe(1)
    expect(harness.applyPreview).not.toHaveBeenCalled()
  })

  it('全局并发为 1，前一层完成后立即执行其他层最新 pending', async () => {
    const harness = createHarness()

    harness.coordinator.scheduleViewport(
      viewportA,
      [layer('points'), layer('roads')],
      1,
    )
    await vi.advanceTimersByTimeAsync(250)

    expect(harness.loadPreview).toHaveBeenCalledOnce()
    expect(harness.activeRequests()).toBe(1)

    const first = harness.requests[0]
    first.resolve(previewFor(first.job))
    await flushPromises()

    expect(harness.loadPreview).toHaveBeenCalledTimes(2)
    expect(harness.loadPreview.mock.calls[1][0].datasetName).toBe('roads')
    expect(harness.activeRequests()).toBe(1)
  })

  it.each([
    ['generation 改变', (harness: Harness) => {
      harness.fileGeneration = 2
    }],
    ['图层隐藏', (harness: Harness) => {
      harness.layers.set('points', { visible: false })
    }],
    ['图层移除', (harness: Harness) => {
      harness.layers.delete('points')
    }],
  ])('debounce 窗口内%s时不启动旧 pending', async (_name, invalidate) => {
    const harness = createHarness()

    harness.coordinator.scheduleViewport(viewportA, [layer('points')], 1)
    invalidate(harness)
    await vi.advanceTimersByTimeAsync(250)

    expect(harness.loadPreview).not.toHaveBeenCalled()
  })

  it.each([
    ['version 变旧', (harness: Harness) => {
      harness.coordinator.scheduleViewport(viewportC, [layer('roads')], 1)
    }],
    ['generation 改变', (harness: Harness) => {
      harness.fileGeneration = 2
    }],
    ['图层隐藏', (harness: Harness) => {
      harness.layers.set('roads', { visible: false })
    }],
    ['图层移除', (harness: Harness) => {
      harness.layers.delete('roads')
    }],
  ])('等待执行槽时%s会丢弃旧 pending', async (_name, invalidate) => {
    const harness = createHarness()
    harness.coordinator.scheduleViewport(viewportA, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)
    const blocker = harness.requests[0]
    harness.coordinator.scheduleViewport(viewportB, [layer('roads')], 1)
    await vi.advanceTimersByTimeAsync(250)

    invalidate(harness)
    blocker.resolve(previewFor(blocker.job))
    await flushPromises()

    expect(harness.loadPreview).toHaveBeenCalledOnce()
  })

  it('两图层持续更新时轮转执行且每层只运行最新 pending', async () => {
    const harness = createHarness()
    harness.coordinator.scheduleViewport(viewportA, [layer('points'), layer('roads')], 1)
    await vi.advanceTimersByTimeAsync(250)
    expect(harness.requests[0].job.datasetName).toBe('points')

    harness.coordinator.scheduleViewport(viewportB, [layer('points'), layer('roads')], 1)
    await vi.advanceTimersByTimeAsync(250)
    harness.coordinator.scheduleViewport(viewportC, [layer('points'), layer('roads')], 1)
    await vi.advanceTimersByTimeAsync(250)
    harness.requests[0].resolve(previewFor(harness.requests[0].job))
    await flushPromises()

    expect(harness.requests[1].job.datasetName).toBe('roads')
    expect(harness.requests[1].job.bounds).toEqual({ minX: 170, minY: 120, maxX: 430, maxY: 380 })
    harness.coordinator.scheduleViewport(viewportB, [layer('points'), layer('roads')], 1)
    await vi.advanceTimersByTimeAsync(250)
    harness.coordinator.scheduleViewport(viewportC, [layer('points'), layer('roads')], 1)
    await vi.advanceTimersByTimeAsync(250)
    harness.requests[1].resolve(previewFor(harness.requests[1].job))
    await flushPromises()

    expect(harness.requests[2].job.datasetName).toBe('points')
    expect(harness.requests[2].job.bounds).toEqual({ minX: 170, minY: 120, maxX: 430, maxY: 380 })
    harness.requests[2].resolve(previewFor(harness.requests[2].job))
    await flushPromises()

    expect(harness.requests[3].job.datasetName).toBe('roads')
    expect(harness.requests[3].job.bounds).toEqual({ minX: 170, minY: 120, maxX: 430, maxY: 380 })
    expect(harness.requests.filter(({ job }) => job.datasetName === 'points')).toHaveLength(2)
    expect(harness.requests.filter(({ job }) => job.datasetName === 'roads')).toHaveLength(2)
  })

  it.each([
    ['旧 version', (harness: Harness, request: ControlledRequest) => {
      harness.coordinator.scheduleViewport(viewportB, [layer('points')], 1)
    }],
    ['旧 generation', (harness: Harness) => {
      harness.fileGeneration = 2
    }],
    ['响应 generation 不一致', (_harness: Harness, request: ControlledRequest) => {
      request.previewOverrides.fileGeneration = 2
    }],
    ['图层隐藏', (harness: Harness) => {
      harness.layers.set('points', { visible: false })
    }],
    ['图层移除', (harness: Harness) => {
      harness.layers.delete('points')
    }],
    ['queriedBounds 不一致', (_harness: Harness, request: ControlledRequest) => {
      request.previewOverrides.queriedBounds = viewportB
    }],
  ])('%s 的结果不会应用', async (_name, makeStale) => {
    const harness = createHarness()
    harness.coordinator.scheduleViewport(viewportA, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)
    const request = harness.requests[0]

    makeStale(harness, request)
    request.resolve(previewFor(request.job, request.previewOverrides))
    await flushPromises()

    expect(harness.applyPreview).not.toHaveBeenCalled()
  })

  it('invalidateLayer 清除 pending 并使执行中结果失效', async () => {
    const harness = createHarness()
    harness.coordinator.scheduleViewport(viewportA, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)
    const request = harness.requests[0]
    harness.coordinator.scheduleViewport(viewportB, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)

    harness.coordinator.invalidateLayer('points')
    request.resolve(previewFor(request.job))
    await flushPromises()

    expect(harness.applyPreview).not.toHaveBeenCalled()
    expect(harness.loadPreview).toHaveBeenCalledOnce()
  })

  it('invalidateAll 清除防抖和所有 pending，并使执行中结果失效', async () => {
    const harness = createHarness()
    harness.coordinator.scheduleViewport(viewportA, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)
    const request = harness.requests[0]
    harness.coordinator.scheduleViewport(viewportB, [layer('roads')], 1)

    harness.coordinator.invalidateAll()
    await vi.advanceTimersByTimeAsync(250)
    request.resolve(previewFor(request.job))
    await flushPromises()

    expect(harness.loadPreview).toHaveBeenCalledOnce()
    expect(harness.applyPreview).not.toHaveBeenCalled()
  })

  it('失败不应用 preview，并在最新有效请求上报告错误', async () => {
    const harness = createHarness()
    harness.coordinator.scheduleViewport(viewportA, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)

    harness.requests[0].reject(new Error('query failed'))
    await flushPromises()

    expect(harness.applyPreview).not.toHaveBeenCalled()
    expect(harness.applyError).toHaveBeenCalledWith('points', 'query failed')
  })

  it('保留 Wails 字符串拒绝原因用于诊断', async () => {
    const harness = createHarness()
    harness.coordinator.scheduleViewport(viewportA, [layer('points')], 1)
    await vi.advanceTimersByTimeAsync(250)

    harness.requests[0].reject('backend query failed')
    await flushPromises()

    expect(harness.applyError).toHaveBeenCalledWith('points', 'backend query failed')
  })
})

interface ControlledRequest {
  job: ViewportQueryJob
  previewOverrides: Partial<SpatialPreview>
  settled: boolean
  resolve: (preview: SpatialPreview) => void
  reject: (error: unknown) => void
}

interface Harness {
  coordinator: ViewportQueryCoordinator
  fileGeneration: number
  layers: Map<string, { visible: boolean }>
  requests: ControlledRequest[]
  loadPreview: ReturnType<typeof vi.fn>
  applyPreview: ReturnType<typeof vi.fn>
  applyError: ReturnType<typeof vi.fn>
  activeRequests: () => number
}

function createHarness(maxConcurrentQueries = 1): Harness {
  const harness = {
    fileGeneration: 1,
    layers: new Map([['points', { visible: true }], ['roads', { visible: true }]]),
    requests: [] as ControlledRequest[],
    loadPreview: vi.fn(),
    applyPreview: vi.fn(),
    applyError: vi.fn(),
    activeRequests: () => harness.requests.filter((request) => !request.settled).length,
  }

  harness.loadPreview.mockImplementation((job: ViewportQueryJob) => {
    let resolvePromise!: (preview: SpatialPreview) => void
    let rejectPromise!: (error: unknown) => void
    const promise = new Promise<SpatialPreview>((resolve, reject) => {
      resolvePromise = resolve
      rejectPromise = reject
    })
    const request: ControlledRequest = {
      job,
      previewOverrides: {},
      settled: false,
      resolve: (preview) => {
        request.settled = true
        resolvePromise(preview)
      },
      reject: (error) => {
        request.settled = true
        rejectPromise(error)
      },
    }
    harness.requests.push(request)
    return promise
  })

  const result = { ...harness } as Harness
  const coordinator = new ViewportQueryCoordinator({
    loadPreview: harness.loadPreview,
    applyPreview: harness.applyPreview,
    applyLoading: vi.fn(),
    applyError: harness.applyError,
    getFileGeneration: () => result.fileGeneration,
    getLayer: (datasetName) => harness.layers.get(datasetName),
  }, 250, 0.15, maxConcurrentQueries)

  result.coordinator = coordinator
  return result
}

function layer(datasetName: string): ViewportQueryLayer {
  return { datasetName, visible: true, requiredIds: [] }
}

function previewFor(job: ViewportQueryJob, overrides: Partial<SpatialPreview> = {}): SpatialPreview {
  return {
    datasetName: job.datasetName,
    kind: 'point',
    features: [],
    estimatedVertexCount: 0,
    sampled: false,
    queriedBounds: job.bounds,
    strategy: 'rtree',
    hasMore: false,
    queryDurationMs: 1,
    fileGeneration: job.fileGeneration,
    ...overrides,
  }
}

async function flushPromises(): Promise<void> {
  await Promise.resolve()
  await Promise.resolve()
}
