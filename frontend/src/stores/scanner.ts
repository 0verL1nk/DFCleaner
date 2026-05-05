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

export interface CleanableItem {
  path: string
  name: string
  size: number
  isDir: boolean
  riskLevel: string
  reason: string
  category: string
}

export type SmartScanPhase = 'idle' | 'scanning' | 'analyzing' | 'done'

export interface SmartScanProgress {
  phase: SmartScanPhase
  dirsExplored: number
  itemsFound: number
  currentAction: string
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

  // Smart scan state
  smartScanPhase: SmartScanPhase
  smartScanProgress: SmartScanProgress
  cleanableItems: CleanableItem[]

  setScanning: (v: boolean) => void
  setScanPath: (v: string) => void
  setScanResult: (result: { entries: FileEntry[]; totalFiles: number; totalDirs: number; totalSize: number; errors: ScanError[] }) => void
  setAnalysis: (path: string, mark: AnalysisMark) => void
  batchSetAnalysis: (map: Record<string, AnalysisMark>) => void
  toggleSelect: (path: string) => void
  selectAll: () => void
  selectAllCleanable: () => void
  clearSelection: () => void
  drillDown: (path: string) => void
  navigateTo: (index: number) => void
  reset: () => void

  // Smart scan actions
  setSmartScanPhase: (phase: SmartScanPhase) => void
  setSmartScanProgress: (progress: Partial<SmartScanProgress>) => void
  addCleanableItems: (items: CleanableItem[]) => void
  removeCleanableItem: (path: string) => void
  resetSmartScan: () => void
}

const initialSmartScanProgress: SmartScanProgress = {
  phase: 'idle',
  dirsExplored: 0,
  itemsFound: 0,
  currentAction: '',
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

  smartScanPhase: 'idle',
  smartScanProgress: { ...initialSmartScanProgress },
  cleanableItems: [],

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
  selectAllCleanable: () =>
    set((s) => ({ selectedPaths: new Set(s.cleanableItems.map((i) => i.path)) })),
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
    smartScanPhase: 'idle',
    smartScanProgress: { ...initialSmartScanProgress },
    cleanableItems: [],
  }),

  // Smart scan actions
  setSmartScanPhase: (phase) => set({ smartScanPhase: phase }),
  setSmartScanProgress: (progress) =>
    set((s) => ({ smartScanProgress: { ...s.smartScanProgress, ...progress } })),
  addCleanableItems: (items) =>
    set((s) => {
      const existing = new Set(s.cleanableItems.map((i) => i.path))
      const newItems = items.filter((i) => !existing.has(i.path))
      return { cleanableItems: [...s.cleanableItems, ...newItems] }
    }),
  removeCleanableItem: (path) =>
    set((s) => {
      const next = new Set(s.selectedPaths)
      next.delete(path)
      return {
        cleanableItems: s.cleanableItems.filter((i) => i.path !== path),
        selectedPaths: next,
      }
    }),
  resetSmartScan: () => set({
    smartScanPhase: 'idle',
    smartScanProgress: { ...initialSmartScanProgress },
    cleanableItems: [],
    selectedPaths: new Set(),
  }),
}))
