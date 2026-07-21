import type { BoundingBox, FeatureAttributes } from '../types'

export type GeometryKind = 'point' | 'line' | 'polygon'

export function featureGeometryKind(geometryType: string): GeometryKind | null {
  const normalized = geometryType.toLowerCase()
  if (normalized === 'point' || normalized === 'text' || normalized === 'cadpoint') {
    return 'point'
  }
  if (normalized === 'linestring' || normalized === 'multilinestring' || normalized === 'cadline') {
    return 'line'
  }
  if (normalized === 'polygon' || normalized === 'multipolygon' || normalized === 'cadregion') {
    return 'polygon'
  }
  return null
}

export function isValidBounds(bounds: BoundingBox | undefined): bounds is BoundingBox {
  return Boolean(
    bounds &&
      [bounds.minX, bounds.minY, bounds.maxX, bounds.maxY].every(Number.isFinite) &&
      bounds.minX <= bounds.maxX &&
      bounds.minY <= bounds.maxY,
  )
}

export function isLocatableFeature(attributes: FeatureAttributes): boolean {
  return isValidBounds(attributes.bbox) && featureGeometryKind(attributes.geometryType) !== null
}
