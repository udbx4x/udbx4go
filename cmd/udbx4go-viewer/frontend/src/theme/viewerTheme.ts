import { createTheme } from '@mui/material/styles'

export const viewerColors = {
  bg: '#F6F8FA',
  surface: '#FFFFFF',
  surfaceMuted: '#F9FAFB',
  mapBg: '#EEF2F5',
  textPrimary: '#17202A',
  textSecondary: '#52616F',
  textTertiary: '#8492A0',
  border: '#DDE3EA',
  borderSubtle: '#E8EDF2',
  accent: '#0B7FAB',
  accentHover: '#096C91',
  accentSoft: '#E5F4F9',
  success: '#2E7D32',
  warning: '#B26A00',
  danger: '#C62828',
  selection: '#E85D04',
}

export const viewerLayerColors = {
  point: '#1971C2',
  line: '#2B8A3E',
  polygon: '#F08C00',
  selection: '#E85D04',
}

export const viewerLayout = {
  toolbarHeight: 44,
  datasetPanelWidth: 300,
  inspectorWidth: 340,
  tableCollapsedHeight: 40,
  tableHalfHeight: 260,
  tableExpandedHeight: 260,
  tableFullMaxHeightRatio: 0.5,
}

export const viewerTheme = createTheme({
  palette: {
    mode: 'light',
    background: {
      default: viewerColors.bg,
      paper: viewerColors.surface,
    },
    primary: {
      main: viewerColors.accent,
      dark: viewerColors.accentHover,
      light: viewerColors.accentSoft,
      contrastText: '#FFFFFF',
    },
    text: {
      primary: viewerColors.textPrimary,
      secondary: viewerColors.textSecondary,
    },
    divider: viewerColors.border,
    error: {
      main: viewerColors.danger,
    },
    warning: {
      main: viewerColors.warning,
    },
    success: {
      main: viewerColors.success,
    },
  },
  typography: {
    fontFamily:
      '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif',
    h6: {
      fontSize: 18,
      lineHeight: '26px',
      fontWeight: 600,
    },
    subtitle1: {
      fontSize: 16,
      lineHeight: '24px',
      fontWeight: 600,
    },
    subtitle2: {
      fontSize: 14,
      lineHeight: '20px',
      fontWeight: 600,
    },
    body1: {
      fontSize: 14,
      lineHeight: '22px',
    },
    body2: {
      fontSize: 13,
      lineHeight: '20px',
    },
    caption: {
      fontSize: 12,
      lineHeight: '18px',
    },
    button: {
      fontSize: 13,
      fontWeight: 600,
      textTransform: 'none',
    },
  },
  shape: {
    borderRadius: 6,
  },
  components: {
    MuiButton: {
      defaultProps: {
        disableElevation: true,
      },
    },
    MuiIconButton: {
      styleOverrides: {
        root: {
          borderRadius: 6,
        },
      },
    },
    MuiPaper: {
      defaultProps: {
        elevation: 0,
      },
    },
  },
})
