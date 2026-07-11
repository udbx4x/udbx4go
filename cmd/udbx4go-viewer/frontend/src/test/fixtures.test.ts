import {
  datasetFixtures,
  featureAttributesFixture,
  mapLayerFixtures,
  pageDataFixture,
  sampledMapLayerFixture,
} from './fixtures'

describe('test fixtures', () => {
  it('provide representative viewer data without runtime dependencies', () => {
    expect(datasetFixtures.map((dataset) => dataset.name)).toContain('BaseMap_P')
    expect(pageDataFixture.columns).toEqual(['SmID', 'Name', 'Category'])
    expect(featureAttributesFixture.datasetName).toBe('BaseMap_P')
    expect(mapLayerFixtures[0].style.selected.color).toBe('#E85D04')
  })

  it('keeps sampled layer style independent from base layer fixtures', () => {
    expect(sampledMapLayerFixture.style).not.toBe(mapLayerFixtures[0].style)
    expect(sampledMapLayerFixture.style.point).not.toBe(mapLayerFixtures[0].style.point)
    expect(sampledMapLayerFixture.style.line).not.toBe(mapLayerFixtures[0].style.line)
    expect(sampledMapLayerFixture.style.polygon).not.toBe(mapLayerFixtures[0].style.polygon)
    expect(sampledMapLayerFixture.style.selected).not.toBe(mapLayerFixtures[0].style.selected)
  })
})
