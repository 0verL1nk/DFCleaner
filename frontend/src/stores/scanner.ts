import { create } from 'zustand'

export interface FileEntry {
  path: string
  name: string
  size: number
  modTime: string
  accessTime: string
  isDir: boolean
  extension: string
  error: string
}

export interface ScanError {
  path: string
  error: string
}

export type RiskLevel = 'safe' | 'caution' | 'dangerous' | ''

export interface AnalysisMark {
  riskLevel: RiskLevel
  reason: string
  category: string
  confidence: number
  error?: string
}

interface ScannerState {
  scanning: boolean
  scanPath: string
  entries: FileEntry[]
  totalFiles: number
  totalDirs: number
  totalSize: number
  errors: ScanError[]
  analysisMap: Record<string, AnalysisMark>
  selectedPaths: Set<string>
  currentPath: string
  breadcrumb: string[]

  setScanning: (v: boolean) => void
  setScanPath: (v: string) => void
  setScanResult: (result: { entries: FileEntry[]; totalFiles: number; totalDirs: number; totalSize: number; errors: ScanError[] }) => void
  setAnalysis: (path: string, mark: AnalysisMark) => void
  batchSetAnalysis: (map: Record<string, AnalysisMark>) => void
  toggleSelect: (path: string) => void
  selectAll: () => void
  clearSelection: () => void
  drillDown: (path: string) => void
  navigateTo: (index: number) => void
  reset: () => void
}

export const useScannerStore = create<ScannerState>((set, get) => ({
  scanning: false,
  scanPath: '',
  entries: [],
  totalFiles: 0,
  totalDirs: 0,
  totalSize: 0,
  errors: [],
  analysisMap: {},
  selectedPaths: new Set(),
  currentPath: '',
  breadcrumb: [],

  setScanning: (v) => set({ scanning: v }),
  setScanPath: (v) => set({ scanPath: v }),
  setScanResult: (result) => set({
    ...result,
    scanning: false,
    currentPath: get().scanPath,
    breadcrumb: [get().scanPath],
    selectedPaths: new Set(),
  }),
  setAnalysis: (path, mark) =>
    set((s) => ({ analysisMap: { ...s.analysisMap, [path]: mark } })),
  batchSetAnalysis: (map) =>
    set((s) => ({ analysisMap: { ...s.analysisMap, ...map } })),
  toggleSelect: (path) =>
    set((s) => {
      const next = new Set(s.selectedPaths)
      next.has(path) ? next.delete(path) : next.add(path)
      return { selectedPaths: next }
    }),
  selectAll: () =>
    set((s) => ({ selectedPaths: new Set(s.entries.map((e) => e.path)) })),
  clearSelection: () => set({ selectedPaths: new Set() }),
  drillDown: (path) =>
    set((s) => ({
      currentPath: path,
      breadcrumb: [...s.breadcrumb, path],
      selectedPaths: new Set(),
    })),
  navigateTo: (index) =>
    set((s) => ({
      currentPath: s.breadcrumb[index],
      breadcrumb: s.breadcrumb.slice(0, index + 1),
      selectedPaths: new Set(),
    })),
  reset: () => set({
    scanning: false,
    scanPath: '',
    entries: [],
    totalFiles: 0,
    totalDirs: 0,
    totalSize: 0,
    errors: [],
    analysisMap: {},
    selectedPaths: new Set(),
    currentPath: '',
    breadcrumb: [],
  }),
}))
