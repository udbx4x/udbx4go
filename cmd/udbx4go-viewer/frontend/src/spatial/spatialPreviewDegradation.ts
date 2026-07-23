import type { SpatialPreview } from '../types'

export const spatialPreviewDegradedReasons = [
  'envelope_cache_budget_exceeded',
  'spatial_index_unavailable',
] as const

export type SpatialPreviewDegradedReason = typeof spatialPreviewDegradedReasons[number]

export const spatialPreviewDegradedStatusMessages: Readonly<
  Record<SpatialPreviewDegradedReason, string>
> = {
  envelope_cache_budget_exceeded: '缓存预算不足，显示有界预览',
  spatial_index_unavailable: '范围索引不可用，显示有界预览',
}

const spatialPreviewDegradedReasonSet: ReadonlySet<string> = new Set(
  spatialPreviewDegradedReasons,
)

export type DegradedSpatialPreview = Pick<SpatialPreview, 'strategy' | 'degradedReason'> & {
  strategy: 'bounded_sample'
  degradedReason: SpatialPreviewDegradedReason
}

export function isSpatialPreviewDegradedReason(
  value: unknown,
): value is SpatialPreviewDegradedReason {
  return typeof value === 'string' && spatialPreviewDegradedReasonSet.has(value)
}

export function isDegradedSpatialPreview(
  preview: Pick<SpatialPreview, 'strategy' | 'degradedReason'>,
): preview is DegradedSpatialPreview {
  return preview.strategy === 'bounded_sample' &&
    isSpatialPreviewDegradedReason(preview.degradedReason)
}
