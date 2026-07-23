import { describe, expect, it } from 'vitest'
import { featureGeometryKind, isLocatableFeature } from './featureLocation'

describe('featureLocation', () => {
  it('将 CadText 识别为点定位锚点', () => {
    expect(featureGeometryKind('CadText')).toBe('point')
  })

  it('允许使用有限范围定位 CadText', () => {
    expect(isLocatableFeature({
      datasetName: 'CADDT',
      id: 83,
      geometryType: 'CadText',
      bbox: { minX: 10, minY: 20, maxX: 10, maxY: 20 },
      properties: {},
    })).toBe(true)
  })
})
