// Dataset information from backend
export interface DatasetInfo {
  name: string
  kind: string
  objectCount: number
  iconType: string
}

// Page of data from backend
export interface PageData {
  rows: string[][]
  columns: string[]
  currentPage: number
  totalPages: number
}

// File information
export interface FileInfo {
  path: string
  datasetCount: number
}

// Application state
export interface AppState {
  currentFile: string | null
  datasets: DatasetInfo[]
  selectedDataset: string | null
  pageData: PageData | null
  loading: boolean
  error: string | null
}
