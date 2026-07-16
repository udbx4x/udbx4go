import type { DatasetInfo } from '../types'

export type DatasetCategory = 'spatial' | 'tabular' | 'unknown'

const spatialDatasetKinds = new Set([
  'point',
  'pointZ',
  'line',
  'lineZ',
  'region',
  'regionZ',
  'text',
  'cad',
])

export const isUnknownDataset = (dataset: DatasetInfo) => dataset.kind === 'unknown'

export const isSpatialDataset = (dataset: DatasetInfo) =>
  spatialDatasetKinds.has(dataset.kind)

export const getDatasetCategory = (dataset: DatasetInfo): DatasetCategory => {
  if (isUnknownDataset(dataset)) {
    return 'unknown'
  }
  if (dataset.kind === 'tabular') {
    return 'tabular'
  }
  return isSpatialDataset(dataset) ? 'spatial' : 'unknown'
}
