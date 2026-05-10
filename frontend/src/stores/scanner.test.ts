import { describe, it, expect, beforeEach } from 'vitest'
import { useScannerStore, type CleanableItem } from './scanner'

describe('useScannerStore', () => {
  beforeEach(() => {
    useScannerStore.getState().reset()
  })

  describe('cleanable items', () => {
    it('adds cleanable items', () => {
      const items: CleanableItem[] = [
        { path: '/tmp/a', name: 'a', size: 100, isDir: false, riskLevel: 'safe', reason: 'test', category: 'cache' },
        { path: '/tmp/b', name: 'b', size: 200, isDir: true, riskLevel: 'caution', reason: 'old', category: 'temp' },
      ]
      useScannerStore.getState().addCleanableItems(items)
      expect(useScannerStore.getState().cleanableItems).toEqual(items)
    })

    it('deduplicates items by path', () => {
      const items: CleanableItem[] = [
        { path: '/tmp/a', name: 'a', size: 100, isDir: false, riskLevel: 'safe', reason: 'r', category: 'c' },
      ]
      useScannerStore.getState().addCleanableItems(items)
      useScannerStore.getState().addCleanableItems(items)
      expect(useScannerStore.getState().cleanableItems.length).toBe(1)
    })

    it('removes a cleanable item', () => {
      const items: CleanableItem[] = [
        { path: '/tmp/a', name: 'a', size: 100, isDir: false, riskLevel: 'safe', reason: 'r', category: 'c' },
        { path: '/tmp/b', name: 'b', size: 200, isDir: false, riskLevel: 'safe', reason: 'r', category: 'c' },
      ]
      useScannerStore.getState().addCleanableItems(items)
      useScannerStore.getState().removeCleanableItem('/tmp/a')
      expect(useScannerStore.getState().cleanableItems.length).toBe(1)
      expect(useScannerStore.getState().cleanableItems[0].path).toBe('/tmp/b')
    })

    it('removes item from selectedPaths when removing cleanable item', () => {
      const items: CleanableItem[] = [
        { path: '/tmp/a', name: 'a', size: 100, isDir: false, riskLevel: 'safe', reason: 'r', category: 'c' },
      ]
      useScannerStore.getState().addCleanableItems(items)
      useScannerStore.getState().toggleSelect('/tmp/a')
      expect(useScannerStore.getState().selectedPaths.has('/tmp/a')).toBe(true)
      useScannerStore.getState().removeCleanableItem('/tmp/a')
      expect(useScannerStore.getState().selectedPaths.has('/tmp/a')).toBe(false)
    })
  })

  describe('selection', () => {
    const items: CleanableItem[] = [
      { path: '/tmp/a', name: 'a', size: 100, isDir: false, riskLevel: 'safe', reason: 'r', category: 'c' },
      { path: '/tmp/b', name: 'b', size: 200, isDir: false, riskLevel: 'caution', reason: 'r', category: 'c' },
      { path: '/tmp/c', name: 'c', size: 300, isDir: false, riskLevel: 'dangerous', reason: 'r', category: 'c' },
    ]

    beforeEach(() => {
      useScannerStore.getState().addCleanableItems(items)
    })

    it('toggles selection on', () => {
      useScannerStore.getState().toggleSelect('/tmp/a')
      expect(useScannerStore.getState().selectedPaths.has('/tmp/a')).toBe(true)
    })

    it('toggles selection off', () => {
      useScannerStore.getState().toggleSelect('/tmp/a')
      useScannerStore.getState().toggleSelect('/tmp/a')
      expect(useScannerStore.getState().selectedPaths.has('/tmp/a')).toBe(false)
    })

    it('selects all cleanable items', () => {
      useScannerStore.getState().selectAllCleanable()
      expect(useScannerStore.getState().selectedPaths.size).toBe(3)
    })

    it('clears selection', () => {
      useScannerStore.getState().selectAllCleanable()
      useScannerStore.getState().clearSelection()
      expect(useScannerStore.getState().selectedPaths.size).toBe(0)
    })
  })

  describe('smart scan state', () => {
    it('sets smart scan phase', () => {
      useScannerStore.getState().setSmartScanPhase('scanning')
      expect(useScannerStore.getState().smartScanPhase).toBe('scanning')
    })

    it('updates smart scan progress partially', () => {
      useScannerStore.getState().setSmartScanProgress({ dirsExplored: 42 })
      expect(useScannerStore.getState().smartScanProgress.dirsExplored).toBe(42)
      expect(useScannerStore.getState().smartScanProgress.itemsFound).toBe(0)
    })

    it('resets smart scan state', () => {
      useScannerStore.getState().setSmartScanPhase('analyzing')
      useScannerStore.getState().addCleanableItems([
        { path: '/x', name: 'x', size: 1, isDir: false, riskLevel: 'safe', reason: 'r', category: 'c' },
      ])
      useScannerStore.getState().resetSmartScan()
      expect(useScannerStore.getState().smartScanPhase).toBe('idle')
      expect(useScannerStore.getState().cleanableItems.length).toBe(0)
    })
  })

  describe('breadcrumb navigation', () => {
    it('drills down into a path', () => {
      useScannerStore.getState().setScanPath('/home')
      useScannerStore.getState().setScanResult({
        entries: [], totalFiles: 0, totalDirs: 0, totalSize: 0, errors: [],
      })
      useScannerStore.getState().drillDown('/home/user')
      expect(useScannerStore.getState().currentPath).toBe('/home/user')
      expect(useScannerStore.getState().breadcrumb).toEqual(['/home', '/home/user'])
    })

    it('navigates to breadcrumb index', () => {
      useScannerStore.getState().setScanPath('/home')
      useScannerStore.getState().setScanResult({
        entries: [], totalFiles: 0, totalDirs: 0, totalSize: 0, errors: [],
      })
      useScannerStore.getState().drillDown('/home/user')
      useScannerStore.getState().drillDown('/home/user/docs')
      useScannerStore.getState().navigateTo(1)
      expect(useScannerStore.getState().currentPath).toBe('/home/user')
      expect(useScannerStore.getState().breadcrumb).toEqual(['/home', '/home/user'])
    })
  })

  describe('reset', () => {
    it('resets all state to initial values', () => {
      const store = useScannerStore.getState()
      store.setSmartScanPhase('analyzing')
      store.addCleanableItems([
        { path: '/x', name: 'x', size: 1, isDir: false, riskLevel: 'safe', reason: 'r', category: 'c' },
      ])
      store.toggleSelect('/x')
      store.reset()
      const after = useScannerStore.getState()
      expect(after.scanning).toBe(false)
      expect(after.cleanableItems).toEqual([])
      expect(after.selectedPaths.size).toBe(0)
      expect(after.smartScanPhase).toBe('idle')
      expect(after.entries).toEqual([])
    })
  })
})
