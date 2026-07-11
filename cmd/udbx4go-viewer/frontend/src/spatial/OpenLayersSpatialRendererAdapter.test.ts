import { describe, expect, it } from 'vitest'
import { calculateSelectedFeatureFitExtent } from './OpenLayersSpatialRendererAdapter'

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
