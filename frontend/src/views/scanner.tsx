import React, { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { useScannerStore, type CleanableItem, type RiskLevel } from '@/stores/scanner'
import { useSmartScan, useCancelSmartScan, useCleanup, useSystemDrives, useQuickTargets, useCleanableItems, useClearCleanableItems } from '@/hooks/wails'
import { FolderOpen, ChevronRight, Trash2, ShieldAlert, ShieldCheck, AlertTriangle, CheckSquare, Square, Loader2, HardDrive, Download, Archive, File, Trash, Image, X, FolderSearch, Sparkles, BarChart3 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

const targetIcons: Record<string, React.ReactNode> = {
  download: <Download className="w-4 h-4" />,
  archive: <Archive className="w-4 h-4" />,
  trash: <Trash className="w-4 h-4" />,
  image: <Image className="w-4 h-4" />,
  file: <File className="w-4 h-4" />,
}

export function Scanner() {
  const { t } = useTranslation()
  const store = useScannerStore()
  const { cleanableItems, smartScanPhase, smartScanProgress, selectedPaths } = store
  const smartScanMutation = useSmartScan()
  const cancelSmartScan = useCancelSmartScan()
  const { data: drives } = useSystemDrives()
  const { data: quickTargets } = useQuickTargets()
  const [scanPath, setScanPath] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const cleanupMutation = useCleanup()

  // Load cached cleanable items from DB on mount
  const { data: cachedItems } = useCleanableItems()
  useEffect(() => {
    if (cachedItems && cachedItems.length > 0 && cleanableItems.length === 0) {
      const items: CleanableItem[] = cachedItems.map((item: any) => ({
        path: item.path,
        name: item.name,
        size: item.size,
        isDir: item.isDir,
        riskLevel: item.riskLevel,
        reason: item.reason,
        category: item.category,
      }))
      store.addCleanableItems(items)
    }
  }, [cachedItems])

  // Listen for smart scan events
  useEffect(() => {
    let unsubs: (() => void)[] = []
    ;(async () => {
      const { EventsOn } = await import('../../wailsjs/runtime/runtime')

      const u1 = EventsOn('smartscan:progress', (p: any) => {
        useScannerStore.getState().setSmartScanProgress(p)
        useScannerStore.getState().setSmartScanPhase(p.phase || 'analyzing')
      })
      unsubs.push(u1)

      const u2 = EventsOn('smartscan:items', (items: any[]) => {
        useScannerStore.getState().addCleanableItems(items)
      })
      unsubs.push(u2)

      const u3 = EventsOn('smartscan:complete', () => {
        useScannerStore.getState().setSmartScanPhase('done')
      })
      unsubs.push(u3)

      const u4 = EventsOn('smartscan:error', (err: string) => {
        useScannerStore.getState().setSmartScanPhase('done')
        console.error('SmartScan error:', err)
      })
      unsubs.push(u4)
    })()
    return () => { unsubs.forEach((u) => u()) }
  }, [])

  const startSmartScan = useCallback((path: string) => {
    if (!path.trim()) return
    setScanPath(path)
    // Only reset progress, keep existing cleanable items (accumulate across scans)
    store.setSmartScanPhase('scanning')
    store.setSmartScanProgress({ dirsExplored: 0, itemsFound: 0, currentAction: '' })
    store.clearSelection()
    smartScanMutation.mutate(
      { path, opts: { maxDepth: 0, excludeDirs: [] } },
      { onError: () => store.setSmartScanPhase('done') },
    )
  }, [smartScanMutation, store])

  function handleCleanup(operation: 'trash' | 'delete') {
    const items = Array.from(selectedPaths).map((path) => ({ path, operation }))
    cleanupMutation.mutate(items, {
      onSuccess: () => {
        // Remove cleaned items from the list
        selectedPaths.forEach((path) => {
          useScannerStore.getState().removeCleanableItem(path)
        })
        store.clearSelection()
        setConfirmOpen(false)
      },
    })
  }

  const isRunning = smartScanPhase === 'scanning' || smartScanPhase === 'analyzing'
  const totalCleanableSize = cleanableItems.reduce((sum, i) => sum + i.size, 0)

  return (
    <div className="flex flex-col h-full">
      {/* Scan bar */}
      <div className="p-4 border-b border-border">
        <div className="flex items-center gap-3 mb-3">
          <Input
            placeholder="/path/to/scan"
            value={scanPath}
            onChange={(e) => setScanPath(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && startSmartScan(scanPath)}
            className="flex-1 max-w-md"
          />
          {isRunning ? (
            <Button variant="destructive" onClick={cancelSmartScan}>
              <X className="w-4 h-4 mr-2" />
              {t('common.cancel')}
            </Button>
          ) : (
            <Button onClick={() => startSmartScan(scanPath)} disabled={!scanPath.trim()}>
              {t('scanner.smartScan')}
            </Button>
          )}

          {cleanableItems.length > 0 && (
            <div className="ml-auto flex items-center gap-3 text-sm text-muted-foreground">
              <span>{cleanableItems.length} {t('scanner.itemsToClean')}</span>
              <span>{formatBytes(totalCleanableSize)}</span>
            </div>
          )}
        </div>

        {/* Drives */}
        {drives && drives.length > 0 && (
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-xs text-muted-foreground mr-1">{t('scanner.drives')}:</span>
            {drives.map((d: any) => (
              <button
                key={d.path}
                onClick={() => startSmartScan(d.path)}
                disabled={isRunning}
                className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-md border border-border hover:bg-accent transition-colors disabled:opacity-50"
              >
                <HardDrive className="w-3 h-3" />
                <span className="font-medium">{d.label || d.path}</span>
                <span className="text-muted-foreground">{formatBytes(d.free)} free</span>
              </button>
            ))}
          </div>
        )}

        {/* Quick targets */}
        {quickTargets && quickTargets.length > 0 && (
          <div className="flex items-center gap-2 flex-wrap mt-2">
            <span className="text-xs text-muted-foreground mr-1">{t('scanner.quickTargets')}:</span>
            {quickTargets.map((qt: any) => (
              <button
                key={qt.path}
                onClick={() => startSmartScan(qt.path)}
                disabled={isRunning}
                className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-md border border-border hover:bg-accent transition-colors disabled:opacity-50"
                title={qt.description}
              >
                {targetIcons[qt.icon] || <FolderOpen className="w-3 h-3" />}
                <span>{qt.label}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Progress */}
      {isRunning && (
        <div className="relative overflow-hidden border-b border-border bg-muted/20">
          {/* Shimmer bar */}
          <div className="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-transparent via-primary/60 to-transparent animate-shimmer" />

          <div className="px-5 py-4 space-y-3">
            {/* Header */}
            <div className="flex items-center gap-2.5">
              <div className="relative">
                <Sparkles className="w-4 h-4 text-primary animate-pulse" />
              </div>
              <span className="text-sm font-medium">
                {t('scanner.agentExploring', {
                  dirs: smartScanProgress.dirsExplored || 0,
                  found: smartScanProgress.itemsFound || 0,
                })}
              </span>
            </div>

            {/* Stats */}
            <div className="grid grid-cols-3 gap-3">
              <div className="rounded-lg bg-background/60 border border-border/50 px-3 py-2">
                <div className="flex items-center gap-1.5 text-muted-foreground mb-0.5">
                  <FolderSearch className="w-3 h-3" />
                  <span className="text-[10px] uppercase tracking-wider">{t('scanner.progress.dirs')}</span>
                </div>
                <span className="text-lg font-semibold tabular-nums">{smartScanProgress.dirsExplored || 0}</span>
              </div>
              <div className="rounded-lg bg-background/60 border border-border/50 px-3 py-2">
                <div className="flex items-center gap-1.5 text-muted-foreground mb-0.5">
                  <Trash2 className="w-3 h-3" />
                  <span className="text-[10px] uppercase tracking-wider">{t('scanner.progress.found')}</span>
                </div>
                <span className="text-lg font-semibold tabular-nums">{smartScanProgress.itemsFound || 0}</span>
              </div>
              <div className="rounded-lg bg-background/60 border border-border/50 px-3 py-2">
                <div className="flex items-center gap-1.5 text-muted-foreground mb-0.5">
                  <BarChart3 className="w-3 h-3" />
                  <span className="text-[10px] uppercase tracking-wider">{t('scanner.progress.size')}</span>
                </div>
                <span className="text-lg font-semibold">{formatBytes(totalCleanableSize)}</span>
              </div>
            </div>

            {/* Current action */}
            {smartScanProgress.currentAction && (
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <span className="inline-block w-1.5 h-1.5 rounded-full bg-primary animate-pulse shrink-0" />
                <span className="truncate">{smartScanProgress.currentAction}</span>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Action bar */}
      {selectedPaths.size > 0 && (
        <div className="px-4 py-2 flex items-center gap-3 bg-muted/50 border-b border-border">
          <span className="text-sm">{selectedPaths.size} {t('scanner.selected')}</span>
          <Button size="sm" onClick={() => setConfirmOpen(true)}>
            <Trash2 className="w-3 h-3 mr-1" />
            {t('scanner.cleanup')}
          </Button>
          <Button size="sm" variant="outline" onClick={() => store.clearSelection()}>
            {t('common.cancel')}
          </Button>
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-auto">
        {cleanableItems.length === 0 && !isRunning ? (
          <EmptyState />
        ) : cleanableItems.length > 0 ? (
          <CleanableTable
            items={cleanableItems}
            selectedPaths={selectedPaths}
            onToggleSelect={(path) => store.toggleSelect(path)}
            onSelectAll={() => {
              if (selectedPaths.size === cleanableItems.length) store.clearSelection()
              else store.selectAllCleanable()
            }}
          />
        ) : null}
      </div>

      {/* Cleanup confirmation dialog */}
      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('scanner.confirmCleanup')}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t('scanner.confirmCleanupDesc', { count: selectedPaths.size })}
          </p>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setConfirmOpen(false)}>{t('common.cancel')}</Button>
            <Button variant="secondary" onClick={() => handleCleanup('trash')}>
              {t('scanner.moveToTrash')}
            </Button>
            <Button variant="destructive" onClick={() => handleCleanup('delete')}>
              {t('scanner.permanentDelete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function EmptyState() {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
      <FolderOpen className="w-12 h-12 mb-3" />
      <p>{t('scanner.noData')}</p>
      <p className="text-sm">{t('scanner.selectDirectory')}</p>
    </div>
  )
}

function CleanableTable({ items, selectedPaths, onToggleSelect, onSelectAll }: {
  items: CleanableItem[]
  selectedPaths: Set<string>
  onToggleSelect: (path: string) => void
  onSelectAll: () => void
}) {
  const { t } = useTranslation()
  return (
    <table className="w-full text-sm">
      <thead className="sticky top-0 bg-background border-b border-border">
        <tr>
          <th className="w-10 px-3 py-2">
            <button onClick={onSelectAll}>
              {selectedPaths.size === items.length && items.length > 0
                ? <CheckSquare className="w-4 h-4 text-primary" />
                : <Square className="w-4 h-4 text-muted-foreground" />}
            </button>
          </th>
          <th className="text-left px-3 py-2">{t('scanner.name')}</th>
          <th className="text-right px-3 py-2 w-24">{t('scanner.size')}</th>
          <th className="text-left px-3 py-2 w-32">{t('scanner.riskLevel')}</th>
          <th className="text-left px-3 py-2 w-40">{t('scanner.reason')}</th>
        </tr>
      </thead>
      <tbody>
        {items.map((item) => (
          <CleanableRow
            key={item.path}
            item={item}
            selected={selectedPaths.has(item.path)}
            onToggleSelect={onToggleSelect}
          />
        ))}
      </tbody>
    </table>
  )
}

function CleanableRow({ item, selected, onToggleSelect }: {
  item: CleanableItem
  selected: boolean
  onToggleSelect: (path: string) => void
}) {
  return (
    <tr className={`border-b border-border hover:bg-muted/50 ${selected ? 'bg-primary/5' : ''}`}>
      <td className="px-3 py-2">
        <button onClick={() => onToggleSelect(item.path)}>
          {selected
            ? <CheckSquare className="w-4 h-4 text-primary" />
            : <Square className="w-4 h-4 text-muted-foreground" />}
        </button>
      </td>
      <td className="px-3 py-2">
        <div className="flex flex-col">
          <span className={item.isDir ? 'text-primary font-medium' : ''}>
            {item.isDir ? '📁 ' : '📄 '}
            {item.name}
          </span>
          <span className="text-xs text-muted-foreground truncate max-w-xs" title={item.path}>
            {item.path}
          </span>
        </div>
      </td>
      <td className="text-right px-3 py-2 text-muted-foreground">
        {formatBytes(item.size)}
      </td>
      <td className="px-3 py-2">
        <RiskBadge risk={item.riskLevel as RiskLevel} />
      </td>
      <td className="px-3 py-2 text-muted-foreground text-xs">
        <span className="line-clamp-2">{item.reason}</span>
      </td>
    </tr>
  )
}

function RiskBadge({ risk }: { risk: RiskLevel }) {
  const config: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
    safe: { icon: <ShieldCheck className="w-3 h-3" />, color: 'text-safe border-safe/30', label: 'Safe' },
    caution: { icon: <AlertTriangle className="w-3 h-3" />, color: 'text-caution border-caution/30', label: 'Caution' },
    dangerous: { icon: <ShieldAlert className="w-3 h-3" />, color: 'text-destructive border-destructive/30', label: 'Dangerous' },
  }
  const c = config[risk]
  if (!c) return null

  return (
    <Badge variant="outline" className={`${c.color} text-xs`}>
      {c.icon} <span className="ml-1">{c.label}</span>
    </Badge>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}
