import type {
  DatasetInfo,
  FeatureAttributes,
  MapLayerState,
  PageData,
  SelectedMapFeature,
} from '../types'

export const datasetFixtures: DatasetInfo[] = [
  { name: 'BaseMap_P', kind: 'point', objectCount: 10, iconType: 'point' },
  { name: 'BaseMap_L', kind: 'line', objectCount: 8, iconType: 'line' },
  { name: 'BaseMap_R', kind: 'region', objectCount: 6, iconType: 'region' },
  { name: 'TabularDT', kind: 'tabular', objectCount: 3, iconType: 'tabular' },
]

export const selectedFeatureFixture: SelectedMapFeature = {
  datasetName: 'BaseMap_P',
  featureID: 1,
}

export const featureAttributesFixture: FeatureAttributes = {
  datasetName: 'BaseMap_P',
  id: 1,
  geometryType: 'Point',
  properties: {
    Name: '示例点',
    Category: 'POI',
    Code: 'P001',
  },
}

export const pageDataFixture: PageData = {
  columns: ['SmID', 'Name', 'Category'],
  rows: [
    ['1', '示例点', 'POI'],
    ['2', '第二个点', 'POI'],
  ],
  currentPage: 1,
  totalPages: 2,
}

export const mapLayerFixtures: MapLayerState[] = [
  {
    datasetName: 'BaseMap_P',
    kind: 'point',
    visible: true,
    loading: false,
    error: null,
    summary: null,
    preview: {
      datasetName: 'BaseMap_P',
      kind: 'point',
      features: [],
      estimatedVertexCount: 0,
      sampled: false,
    },
    style: {
      point: { radius: 4, fillColor: '#1971C2', strokeColor: '#FFFFFF', strokeWidth: 1 },
      line: { strokeColor: '#1971C2', strokeWidth: 1.5 },
      polygon: { fillColor: 'rgba(25, 113, 194, 0.16)', strokeColor: '#1971C2', strokeWidth: 1.5 },
      selected: {
        color: '#E85D04',
        pointRadius: 6,
        strokeWidth: 3,
        fillColor: 'rgba(232, 93, 4, 0.24)',
      },
    },
  },
]
