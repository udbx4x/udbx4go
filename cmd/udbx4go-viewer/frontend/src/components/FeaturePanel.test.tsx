import { ThemeProvider } from '@mui/material/styles'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { FeaturePanel } from './FeaturePanel'
import { featureAttributesFixture } from '../test/fixtures'
import { viewerTheme } from '../theme/viewerTheme'
import type { FeatureAttributes } from '../types'

const renderFeaturePanel = (attributes: FeatureAttributes | null) => {
  render(
    <ThemeProvider theme={viewerTheme}>
      <FeaturePanel attributes={attributes} />
    </ThemeProvider>,
  )
}

describe('FeaturePanel', () => {
  it('无选中属性时显示空状态', () => {
    renderFeaturePanel(null)

    expect(screen.getByText('要素属性')).toBeInTheDocument()
    expect(screen.getByText('点击地图要素或属性表行查看属性')).toBeInTheDocument()
  })

  it('有选中属性时显示要素摘要和属性值', () => {
    renderFeaturePanel(featureAttributesFixture)

    expect(screen.getByText('BaseMap_P')).toBeInTheDocument()
    expect(screen.getByText('SmID 1')).toBeInTheDocument()
    expect(screen.getByText('Name')).toBeInTheDocument()
    expect(screen.getByText('示例点')).toBeInTheDocument()
    expect(screen.getByText('Name')).toHaveAttribute('title', 'Name')
    expect(screen.getByText('示例点')).toHaveAttribute('title', '示例点')
  })
})
