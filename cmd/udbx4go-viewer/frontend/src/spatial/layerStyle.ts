import type { LayerStyle } from '../types'

export function createDefaultLayerStyle(kind: string): LayerStyle {
  const selected = {
    color: '#d9480f',
    pointRadius: 6,
    strokeWidth: 3,
    fillColor: 'rgba(217,72,15,0.24)',
  }

  switch (kind) {
    case 'line':
    case 'lineZ':
      return {
        point: { radius: 4, fillColor: '#2b8a3e', strokeColor: '#ffffff', strokeWidth: 1 },
        line: { strokeColor: '#2b8a3e', strokeWidth: 1.8 },
        polygon: { fillColor: 'rgba(43,138,62,0.16)', strokeColor: '#2b8a3e', strokeWidth: 1.5 },
        selected,
      }
    case 'region':
    case 'regionZ':
      return {
        point: { radius: 4, fillColor: '#f08c00', strokeColor: '#ffffff', strokeWidth: 1 },
        line: { strokeColor: '#f08c00', strokeWidth: 1.5 },
        polygon: { fillColor: 'rgba(240,140,0,0.18)', strokeColor: '#f08c00', strokeWidth: 1.4 },
        selected,
      }
    default:
      return {
        point: { radius: 4, fillColor: '#1971c2', strokeColor: '#ffffff', strokeWidth: 1 },
        line: { strokeColor: '#1971c2', strokeWidth: 1.5 },
        polygon: { fillColor: 'rgba(25,113,194,0.16)', strokeColor: '#1971c2', strokeWidth: 1.5 },
        selected,
      }
  }
}
