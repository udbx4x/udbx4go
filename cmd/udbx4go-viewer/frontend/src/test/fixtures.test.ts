import { datasetFixtures, featureAttributesFixture, mapLayerFixtures, pageDataFixture } from './fixtures'

describe('test fixtures', () => {
  it('provide representative viewer data without runtime dependencies', () => {
    expect(datasetFixtures.map((dataset) => dataset.name)).toContain('BaseMap_P')
    expect(pageDataFixture.columns).toEqual(['SmID', 'Name', 'Category'])
    expect(featureAttributesFixture.datasetName).toBe('BaseMap_P')
    expect(mapLayerFixtures[0].style.selected.color).toBe('#E85D04')
  })
})
