import type {
  DatasetInfo,
  FeatureAttributes,
  LayerStyle,
  MapLayerState,
  PageData,
  SelectedMapFeature,
  SpatialPreview,
} from '../types'

export const spatialPreviewDegradedReasons = [
  'envelope_cache_budget_exceeded',
  'spatial_index_unavailable',
] as const

export type SpatialPreviewDegradedReason = typeof spatialPreviewDegradedReasons[number]

export function createSpatialPreviewFixture(
  overrides: Partial<SpatialPreview> = {},
): SpatialPreview {
  return {
    datasetName: 'BaseMap_P',
    kind: 'point',
    features: [],
    estimatedVertexCount: 0,
    sampled: false,
    strategy: 'rtree',
    hasMore: false,
    queryDurationMs: 0,
    fileGeneration: 0,
    ...overrides,
  }
}

export function createDegradedSpatialPreviewFixture(
  degradedReason: SpatialPreviewDegradedReason,
  overrides: Partial<Omit<SpatialPreview, 'strategy' | 'degradedReason'>> = {},
): SpatialPreview {
  return createSpatialPreviewFixture({
    ...overrides,
    strategy: 'bounded_sample',
    degradedReason,
  })
}

const cloneLayerStyle = (style: LayerStyle): LayerStyle => ({
  point: { ...style.point },
  line: { ...style.line },
  polygon: { ...style.polygon },
  selected: { ...style.selected },
})

export const datasetFixtures: DatasetInfo[] = [
  { name: 'BaseMap_P', kind: 'point', objectCount: 10, iconType: 'point' },
  { name: 'BaseMap_L', kind: 'line', objectCount: 8, iconType: 'line' },
  { name: 'BaseMap_R', kind: 'region', objectCount: 6, iconType: 'region' },
  { name: 'TabularDT', kind: 'tabular', objectCount: 3, iconType: 'tabular' },
  {
    name: 'Jingjin_NetworkZ_Node',
    kind: 'pointZ',
    objectCount: 698,
    iconType: 'point',
  },
  {
    name: 'modeldt_Texture',
    kind: 'unknown',
    objectCount: 47,
    iconType: 'unknown',
  },
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
    preview: createSpatialPreviewFixture({ strategy: 'bounded_sample' }),
    queryStatus: 'ready',
    queryError: null,
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

export const sampledMapLayerFixture: MapLayerState = {
  ...mapLayerFixtures[0],
  datasetName: 'Jingjin_NetworkZ_Node',
  kind: 'pointZ',
  style: cloneLayerStyle(mapLayerFixtures[0].style),
  preview: createSpatialPreviewFixture({
    datasetName: 'Jingjin_NetworkZ_Node',
    kind: 'pointZ',
    estimatedVertexCount: 50000,
    sampled: true,
    sampleReason: '预览达到要素上限',
    strategy: 'bounded_sample',
  }),
}
