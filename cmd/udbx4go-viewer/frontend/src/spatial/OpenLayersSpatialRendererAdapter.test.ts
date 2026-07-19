import { beforeEach, describe, expect, it, vi } from 'vitest'
import type VectorLayer from 'ol/layer/Vector'
import type VectorSource from 'ol/source/Vector'
import type CircleStyle from 'ol/style/Circle'
import type { BoundingBox, MapLayerState, PreviewFeature } from '../types'
import type { SpatialRendererAdapter } from './SpatialRendererAdapter'

type IsExact<Actual, Expected> =
  (<Value>() => Value extends Actual ? 1 : 2) extends
  (<Value>() => Value extends Expected ? 1 : 2)
    ? (<Value>() => Value extends Expected ? 1 : 2) extends
      (<Value>() => Value extends Actual ? 1 : 2)
      ? true
      : false
    : false
type Assert<Condition extends true> = Condition
type ExpectedFitBounds = (
  bounds: BoundingBox,
  geometryKind: 'point' | 'line' | 'polygon',
) => void
type ExpectedViewportSubscription = (
  handler: (viewport: BoundingBox) => void,
) => () => void
type ExpectedGetViewport = () => BoundingBox | null

type _FitBoundsContract = Assert<IsExact<SpatialRendererAdapter['fitBounds'], ExpectedFitBounds>>
type _ViewportSubscriptionContract = Assert<
  IsExact<SpatialRendererAdapter['onViewportChange'], ExpectedViewportSubscription>
>
type _GetViewportContract = Assert<IsExact<SpatialRendererAdapter['getViewport'], ExpectedGetViewport>>

const openLayersMocks = vi.hoisted(() => {
  class MockView {
    static instances: MockView[] = []

    readonly on = vi.fn()
    readonly fit = vi.fn((extent: number[]) => {
      this.extent = extent
      this.onFit?.()
    })
    extent = [0, 0, 100, 100]
    onFit: (() => void) | null = null

    constructor() {
      MockView.instances.push(this)
    }

    calculateExtent(): number[] {
      return this.extent
    }
  }

  class MockMap {
    static instances: MockMap[] = []

    readonly on = vi.fn((type: string, listener: () => void) => {
      this.listeners.set(type, listener)
    })
    readonly once = vi.fn((type: string, listener: () => void) => {
      this.listeners.set(type, listener)
    })
    readonly un = vi.fn((type: string, listener: () => void) => {
      if (this.listeners.get(type) === listener) {
        this.listeners.delete(type)
      }
    })
    readonly addLayer = vi.fn()
    readonly removeLayer = vi.fn()
    readonly updateSize = vi.fn()
    readonly setTarget = vi.fn((target?: HTMLElement) => {
      this.targetActive = Boolean(target)
    })
    readonly render = vi.fn()
    readonly renderSync = vi.fn(() => {
      this.hasRenderBaseline = true
    })
    readonly view: MockView
    private readonly listeners = new Map<string, () => void>()
    private hasRenderBaseline = false
    private targetActive: boolean

    constructor(options: { target: HTMLElement, view: MockView }) {
      this.view = options.view
      this.targetActive = Boolean(options.target)
      this.view.onFit = () => {
        if (this.hasRenderBaseline && this.targetActive) {
          this.emit('moveend')
        }
      }
      MockMap.instances.push(this)
    }

    getView(): MockView {
      return this.view
    }

    getSize(): [number, number] {
      return [800, 600]
    }

    forEachFeatureAtPixel(): undefined {
      return undefined
    }

    emit(type: string): void {
      this.listeners.get(type)?.()
    }
  }

  return { MockMap, MockView }
})

vi.mock('ol/Map', () => ({ default: openLayersMocks.MockMap }))
vi.mock('ol/View', () => ({ default: openLayersMocks.MockView }))

import {
  OpenLayersSpatialRendererAdapter,
  calculateSelectedFeatureFitExtent,
} from './OpenLayersSpatialRendererAdapter'

describe('OpenLayersSpatialRendererAdapter viewport', () => {
  beforeEach(() => {
    openLayersMocks.MockMap.instances.length = 0
    openLayersMocks.MockView.instances.length = 0
  })

  it('mount 只监听一次 moveend，并只发送有限有序视口', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    const handler = vi.fn()

    adapter.onViewportChange(handler)
    adapter.mount(document.createElement('div'))

    const map = openLayersMocks.MockMap.instances[0]
    const view = openLayersMocks.MockView.instances[0]
    expect(map.on).toHaveBeenCalledWith('moveend', expect.any(Function))
    expect(map.on.mock.calls.filter(([type]) => type === 'moveend')).toHaveLength(1)
    expect(view.on).not.toHaveBeenCalledWith('change:center', expect.any(Function))
    expect(view.on).not.toHaveBeenCalledWith('change:resolution', expect.any(Function))

    view.extent = [Number.NaN, 0, 10, 10]
    map.emit('moveend')
    view.extent = [10, 0, 0, 10]
    map.emit('moveend')
    view.extent = [-10, -5, 20, 30]
    map.emit('moveend')

    expect(handler).toHaveBeenCalledOnce()
    expect(handler).toHaveBeenCalledWith({ minX: -10, minY: -5, maxX: 20, maxY: 30 })
  })

  it('同步返回当前有限有序视口，未挂载或非法范围返回 null', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    expect(adapter.getViewport()).toBeNull()
    adapter.mount(document.createElement('div'))
    const view = openLayersMocks.MockView.instances[0]

    view.extent = [-10, -5, 20, 30]
    expect(adapter.getViewport()).toEqual({ minX: -10, minY: -5, maxX: 20, maxY: 30 })

    view.extent = [Number.NaN, 0, 10, 10]
    expect(adapter.getViewport()).toBeNull()
    view.extent = [10, 0, 0, 10]
    expect(adapter.getViewport()).toBeNull()
  })

  it('mount 建立渲染基线后，首次 fitBounds 由 moveend 发送视口', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    const handler = vi.fn()
    adapter.onViewportChange(handler)

    adapter.mount(document.createElement('div'))
    adapter.fitBounds({ minX: 0, minY: 0, maxX: 10, maxY: 5 }, 'polygon')

    const map = openLayersMocks.MockMap.instances[0]
    expect(map.renderSync).toHaveBeenCalledOnce()
    expect(map.renderSync.mock.invocationCallOrder[0]).toBeLessThan(
      openLayersMocks.MockView.instances[0].fit.mock.invocationCallOrder[0],
    )
    expect(handler).toHaveBeenCalledOnce()
    expect(handler).toHaveBeenCalledWith({ minX: -2, minY: -1, maxX: 12, maxY: 6 })
  })

  it('重复 mount 先停用旧 map，旧 map 不再发送视口', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    const handler = vi.fn()
    adapter.onViewportChange(handler)
    adapter.mount(document.createElement('div'))
    const oldMap = openLayersMocks.MockMap.instances[0]
    const oldMoveendListener = oldMap.on.mock.calls.find(([type]) => type === 'moveend')?.[1]
    const oldSingleClickListener = oldMap.on.mock.calls.find(([type]) => type === 'singleclick')?.[1]

    adapter.mount(document.createElement('div'))

    expect(oldMap.un).toHaveBeenCalledWith('moveend', oldMoveendListener)
    expect(oldMap.un).toHaveBeenCalledWith('singleclick', oldSingleClickListener)
    expect(oldMap.setTarget).toHaveBeenCalledWith(undefined)
    oldMap.emit('moveend')
    expect(handler).not.toHaveBeenCalled()

    adapter.fitBounds({ minX: 0, minY: 0, maxX: 10, maxY: 5 }, 'polygon')
    expect(handler).toHaveBeenCalledOnce()
  })

  it('destroy 注销 moveend 且不再发送视口', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    const handler = vi.fn()
    adapter.onViewportChange(handler)
    adapter.mount(document.createElement('div'))
    const map = openLayersMocks.MockMap.instances[0]
    const moveendListener = map.on.mock.calls.find(([type]) => type === 'moveend')?.[1]

    adapter.destroy()

    expect(map.un).toHaveBeenCalledWith('moveend', moveendListener)
    map.emit('moveend')
    expect(handler).not.toHaveBeenCalled()
  })

  it('取消 viewport 订阅后不再调用 handler', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    const handler = vi.fn()
    const unsubscribe = adapter.onViewportChange(handler)
    adapter.mount(document.createElement('div'))

    unsubscribe()
    openLayersMocks.MockMap.instances[0].emit('moveend')

    expect(handler).not.toHaveBeenCalled()
  })

  it('postrender 不结束等待，只有 rendercomplete 表示绘制完成', async () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    adapter.mount(document.createElement('div'))
    const map = openLayersMocks.MockMap.instances[0]

    let resolved = false
    const rendered = adapter.waitForRenderComplete().then(() => {
      resolved = true
    })
    map.emit('postrender')
    await Promise.resolve()
    const resolvedAfterPostrender = resolved
    map.emit('rendercomplete')
    await rendered

    expect(map.once).toHaveBeenCalledWith('rendercomplete', expect.any(Function))
    expect(map.renderSync).toHaveBeenCalledTimes(2)
    expect(resolvedAfterPostrender).toBe(false)
    expect(resolved).toBe(true)
  })

  it('rendercomplete 超时时解绑同一事件并报告对应错误', async () => {
    vi.useFakeTimers()
    try {
      const adapter = new OpenLayersSpatialRendererAdapter()
      adapter.mount(document.createElement('div'))
      const map = openLayersMocks.MockMap.instances[0]

      const rendered = adapter.waitForRenderComplete(25)
      const capturedError = rendered.then(
        () => null,
        (error: unknown) => error,
      )
      const listener = map.once.mock.calls.find(([type]) => type === 'rendercomplete')?.[1]

      await vi.advanceTimersByTimeAsync(25)
      expect(await capturedError).toEqual(new Error('rendercomplete timed out after 25ms'))
      expect(map.un).toHaveBeenCalledWith('rendercomplete', listener)
    } finally {
      vi.useRealTimers()
    }
  })
})

describe('OpenLayersSpatialRendererAdapter rendered pixels', () => {
  beforeEach(() => {
    openLayersMocks.MockMap.instances.length = 0
    openLayersMocks.MockView.instances.length = 0
  })

  it('未挂载或所有 canvas 透明时返回 false，任一 alpha 非零时返回 true', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    expect(adapter.hasRenderedFeaturePixels()).toBe(false)

    const container = document.createElement('div')
    const pixels = new Uint8ClampedArray(8)
    appendLayerCanvas(container, 2, 1, () => pixels)
    adapter.mount(container)

    expect(adapter.hasRenderedFeaturePixels()).toBe(false)
    pixels[7] = 255
    expect(adapter.hasRenderedFeaturePixels()).toBe(true)
  })

  it('零尺寸、无 2D context 或像素读取异常按无可验证像素处理', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    const container = document.createElement('div')
    const zeroSizeContext = { getImageData: vi.fn() }
    const zeroSizeCanvas = appendLayerCanvas(container, 0, 0, () => new Uint8ClampedArray())
    Object.defineProperty(zeroSizeCanvas, 'getContext', {
      value: vi.fn(() => zeroSizeContext),
      configurable: true,
    })
    appendLayerCanvas(container, 1, 1, null)
    appendLayerCanvas(container, 1, 1, () => {
      throw new DOMException('tainted canvas', 'SecurityError')
    })
    adapter.mount(container)

    expect(() => adapter.hasRenderedFeaturePixels()).not.toThrow()
    expect(adapter.hasRenderedFeaturePixels()).toBe(false)
    expect(zeroSizeCanvas.getContext).not.toHaveBeenCalled()
  })

  it('destroy 后不读取旧 container 的 canvas', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    const container = document.createElement('div')
    const readPixels = vi.fn(() => new Uint8ClampedArray([0, 0, 0, 255]))
    appendLayerCanvas(container, 1, 1, readPixels)
    adapter.mount(container)

    adapter.destroy()

    expect(adapter.hasRenderedFeaturePixels()).toBe(false)
    expect(readPixels).not.toHaveBeenCalled()
  })
})

describe('OpenLayersSpatialRendererAdapter fitBounds', () => {
  beforeEach(() => {
    openLayersMocks.MockMap.instances.length = 0
    openLayersMocks.MockView.instances.length = 0
  })

  it.each([
    ['point', [10, 20, 10, 20], [9.995, 19.995, 10.005, 20.005]],
    ['line', [0, 0, 10, 0], [-1.5, -0.05, 11.5, 0.05]],
    ['polygon', [0, 0, 10, 5], [-2, -1, 12, 6]],
  ] as const)('%s 按类型化 BBox 定位', (geometryKind, values, expectedExtent) => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    adapter.mount(document.createElement('div'))

    adapter.fitBounds(
      { minX: values[0], minY: values[1], maxX: values[2], maxY: values[3] },
      geometryKind,
    )

    expect(openLayersMocks.MockView.instances[0].fit).toHaveBeenCalledWith(
      expectedExtent,
      { padding: [64, 64, 64, 64], duration: 0 },
    )
  })

  it.each([
    { minX: Number.NaN, minY: 0, maxX: 10, maxY: 10 },
    { minX: 10, minY: 0, maxX: 0, maxY: 10 },
    { minX: 0, minY: 10, maxX: 10, maxY: 0 },
  ])('无效 BBox 不改变 view', (bounds) => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    adapter.mount(document.createElement('div'))

    adapter.fitBounds(bounds, 'polygon')

    expect(openLayersMocks.MockView.instances[0].fit).not.toHaveBeenCalled()
  })
})

describe('OpenLayersSpatialRendererAdapter setLayer', () => {
  beforeEach(() => {
    openLayersMocks.MockMap.instances.length = 0
    openLayersMocks.MockView.instances.length = 0
  })

  it('转换失败时保留旧 Source 内容', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    adapter.mount(document.createElement('div'))
    adapter.setLayer(createLayer([pointFeature(1), pointFeature(2), pointFeature(3)]))
    const source = getAddedSource()
    const clear = vi.spyOn(source, 'clear')
    const addFeatures = vi.spyOn(source, 'addFeatures')

    expect(() => adapter.setLayer(createLayer([
      pointFeature(4),
      { id: 5, geometry: { type: 'UnsupportedGeometry', coordinates: [], hasZ: false } },
    ]))).toThrow('UnsupportedGeometry')

    expect(source.getFeatures()).toHaveLength(3)
    expect(clear).not.toHaveBeenCalled()
    expect(addFeatures).not.toHaveBeenCalled()
  })

  it('转换失败时保留旧图层状态', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    adapter.mount(document.createElement('div'))
    adapter.setLayer(createLayer([pointFeature(1)], '#1971c2'))
    const layer = getAddedLayer()
    const source = getAddedSource()

    expect(() => adapter.setLayer(createLayer([
      { id: 2, geometry: { type: 'UnsupportedGeometry', coordinates: [], hasZ: false } },
    ], '#ff0000'))).toThrow('UnsupportedGeometry')

    const style = layer.getStyleFunction()?.(source.getFeatures()[0], 1)
    const renderedStyle = Array.isArray(style) ? style[0] : style
    expect((renderedStyle?.getImage() as CircleStyle | null)?.getFill()?.getColor()).toBe('#1971c2')
  })

  it('转换成功时一次清空并批量加入完整要素', () => {
    const adapter = new OpenLayersSpatialRendererAdapter()
    adapter.mount(document.createElement('div'))
    adapter.setLayer(createLayer([pointFeature(1), pointFeature(2), pointFeature(3)]))
    const source = getAddedSource()
    const clear = vi.spyOn(source, 'clear')
    const addFeatures = vi.spyOn(source, 'addFeatures')

    adapter.setLayer(createLayer([pointFeature(4), pointFeature(5)]))

    expect(clear).toHaveBeenCalledOnce()
    expect(clear).toHaveBeenCalledWith(true)
    expect(addFeatures).toHaveBeenCalledOnce()
    expect(addFeatures.mock.calls[0][0]).toHaveLength(2)
    expect(source.getFeatures()).toHaveLength(2)
  })
})

describe('calculateSelectedFeatureFitExtent', () => {
  it('点要素定位时使用图层范围提供上下文', () => {
    const extent = calculateSelectedFeatureFitExtent(
      [10, 20, 10, 20],
      [0, 0, 100, 100],
      'point',
    )

    expect(extent).toEqual([6, 16, 14, 24])
  })

  it('线要素定位时基于线范围扩展，不退化为点', () => {
    const extent = calculateSelectedFeatureFitExtent(
      [10, 20, 40, 25],
      [0, 0, 100, 100],
      'line',
    )

    expect(extent).toEqual([5.5, 15, 44.5, 30])
  })

  it('小面要素定位时按面范围并补足最小上下文', () => {
    const extent = calculateSelectedFeatureFitExtent(
      [10, 20, 12, 22],
      [0, 0, 100, 100],
      'polygon',
    )

    expect(extent).toEqual([5, 15, 17, 27])
  })
})

function pointFeature(id: number): PreviewFeature {
  return {
    id,
    geometry: { type: 'Point', coordinates: [id, id], hasZ: false },
  }
}

function appendLayerCanvas(
  container: HTMLElement,
  width: number,
  height: number,
  readPixels: (() => Uint8ClampedArray) | null,
): HTMLCanvasElement {
  const layer = document.createElement('div')
  layer.className = 'ol-layer'
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const getImageData = readPixels === null
    ? null
    : vi.fn(() => ({ data: readPixels() }))
  Object.defineProperty(canvas, 'getContext', {
    value: vi.fn(() => getImageData === null ? null : { getImageData }),
    configurable: true,
  })
  layer.append(canvas)
  container.append(layer)
  return canvas
}

function createLayer(features: PreviewFeature[], fillColor = '#1971c2'): MapLayerState {
  return {
    datasetName: 'points',
    kind: 'Point',
    visible: true,
    loading: false,
    error: null,
    summary: null,
    preview: {
      datasetName: 'points',
      kind: 'Point',
      features,
      estimatedVertexCount: features.length,
      sampled: false,
      strategy: 'rtree',
      hasMore: false,
      queryDurationMs: 0,
      fileGeneration: 0,
    },
    queryStatus: 'ready',
    queryError: null,
    style: {
      point: { radius: 4, fillColor, strokeColor: '#ffffff', strokeWidth: 1 },
      line: { strokeColor: '#1971c2', strokeWidth: 1.5 },
      polygon: { fillColor: 'rgba(25,113,194,0.16)', strokeColor: '#1971c2', strokeWidth: 1.5 },
      selected: { color: '#d9480f', pointRadius: 6, strokeWidth: 3, fillColor: 'rgba(217,72,15,0.24)' },
    },
  }
}

function getAddedSource(): VectorSource {
  const layer = getAddedLayer()
  const source = layer.getSource()
  if (!source) {
    throw new Error('测试图层缺少 VectorSource')
  }
  return source
}

function getAddedLayer(): VectorLayer {
  return openLayersMocks.MockMap.instances[0].addLayer.mock.calls[0][0] as VectorLayer
}
