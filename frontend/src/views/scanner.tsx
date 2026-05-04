import React, { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useScannerStore, type FileEntry, type AnalysisMark, type RiskLevel } from '@/stores/scanner'
import { useScan, useCleanup, useAnalyzeFiles, useSystemDrives, useQuickTargets } from '@/hooks/wails'
import { FolderOpen, ChevronRight, Trash2, ShieldAlert, ShieldCheck, AlertTriangle, CheckSquare, Square, Loader2, Sparkles, HardDrive, Download, Archive, File, Trash, Image } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

type SortKey = 'name' | 'size' | 'modTime' | 'riskLevel'
type SortDir = 'asc' | 'desc'

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
  const { entries, breadcrumb, totalSize, scanning, selectedPaths, analysisMap, currentPath } = store
  const scanMutation = useScan()
  const analyzeMutation = useAnalyzeFiles()
  const { data: drives } = useSystemDrives()
  const { data: quickTargets } = useQuickTargets()
  const [sortKey, setSortKey] = useState<SortKey>('size')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [scanPath, setScanPath] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [analyzing, setAnalyzing] = useState(false)
  const cleanupMutation = useCleanup()

  // Listen for AI analysis results
  useEffect(() => {
    let unsub: (() => void) | undefined
    ;(async () => {
      const { EventsOn } = await import('../../wailsjs/runtime/runtime')
      unsub = EventsOn('analysis:complete', (results: any[]) => {
        for (const r of results) {
          if (!r.Error) {
            useScannerStore.getState().setAnalysis(r.Path, {
              riskLevel: r.RiskLevel as RiskLevel,
              reason: r.Reason,
              category: r.Category,
              confidence: r.Confidence,
            })
          }
        }
        setAnalyzing(false)
      })
      EventsOn('analysis:error', () => {
        setAnalyzing(false)
      })
    })()
    return () => { unsub?.() }
  }, [])

  const currentEntries = entries.filter((e) => {
    const parent = e.path.substring(0, e.path.lastIndexOf('/'))
    return parent === currentPath || (currentPath === '' && !e.path.includes('/'))
  })

  const sortedEntries = [...currentEntries].sort((a, b) => {
    let cmp = 0
    switch (sortKey) {
      case 'name': cmp = a.name.localeCompare(b.name); break
      case 'size': cmp = a.size - b.size; break
      case 'modTime': cmp = a.modTime.localeCompare(b.modTime); break
      case 'riskLevel':
        const riskOrder: Record<string, number> = { dangerous: 3, caution: 2, safe: 1, '': 0 }
        cmp = (riskOrder[analysisMap[a.path]?.riskLevel || ''] || 0) - (riskOrder[analysisMap[b.path]?.riskLevel || ''] || 0)
        break
    }
    return sortDir === 'asc' ? cmp : -cmp
  })

  function handleSort(key: SortKey) {
    if (sortKey === key) {
      setSortDir((d) => d === 'asc' ? 'desc' : 'asc')
    } else {
      setSortKey(key)
      setSortDir('desc')
    }
  }

  function startScan(path: string) {
    if (!path.trim()) return
    setScanPath(path)
    store.reset()
    store.setScanning(true)
    store.setScanPath(path)
    scanMutation.mutate(
      { path, opts: { maxDepth: 0, excludeDirs: [] } },
      {
        onSuccess: (result: any) => {
          store.setScanResult({
            entries: result.entries || [],
            totalFiles: result.totalFiles || 0,
            totalDirs: result.totalDirs || 0,
            totalSize: result.totalSize || 0,
            errors: result.errors || [],
          })
          if (result.entries?.length > 0) {
            setAnalyzing(true)
            analyzeMutation.mutate(result.entries, {
              onError: () => setAnalyzing(false),
            })
          }
        },
        onError: () => store.setScanning(false),
      },
    )
  }

  function handleDrillDown(entry: FileEntry) {
    if (entry.isDir) {
      store.drillDown(entry.path)
    }
  }

  function handleCleanup(operation: 'trash' | 'delete') {
    const items = Array.from(selectedPaths).map((path) => ({ path, operation }))
    cleanupMutation.mutate(items, {
      onSuccess: () => {
        store.clearSelection()
        setConfirmOpen(false)
      },
    })
  }

  const analyzedCount = Object.keys(analysisMap).length
  const safeCount = Object.values(analysisMap).filter(a => a.riskLevel === 'safe').length

  return (
    <div className="flex flex-col h-full">
      {/* Scan bar */}
      <div className="p-4 border-b border-border">
        <div className="flex items-center gap-3 mb-3">
          <Input
            placeholder="/path/to/scan"
            value={scanPath}
            onChange={(e) => setScanPath(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && startScan(scanPath)}
            className="flex-1 max-w-md"
          />
          <Button onClick={() => startScan(scanPath)} disabled={scanning || !scanPath.trim()}>
            {scanning ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
            {t('scanner.scan')}
          </Button>

          {entries.length > 0 && (
            <div className="ml-auto flex items-center gap-3 text-sm text-muted-foreground">
              <span>{entries.length} items</span>
              <span>{formatBytes(totalSize)}</span>
              {analyzing && (
                <Badge variant="secondary" className="animate-pulse">
                  <Sparkles className="w-3 h-3 mr-1" /> {t('scanner.analyzing')}
                </Badge>
              )}
              {!analyzing && analyzedCount > 0 && (
                <Badge variant="secondary" className="text-safe">
                  <ShieldCheck className="w-3 h-3 mr-1" /> {analyzedCount} analyzed, {safeCount} safe
                </Badge>
              )}
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
                onClick={() => startScan(d.path)}
                className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-md border border-border hover:bg-accent transition-colors"
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
            {quickTargets.map((t: any) => (
              <button
                key={t.path}
                onClick={() => startScan(t.path)}
                className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-md border border-border hover:bg-accent transition-colors"
                title={t.description}
              >
                {targetIcons[t.icon] || <FolderOpen className="w-3 h-3" />}
                <span>{t.label}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Breadcrumb */}
      {breadcrumb.length > 0 && (
        <nav className="px-4 py-2 flex items-center gap-1 text-sm text-muted-foreground border-b border-border">
          {breadcrumb.map((p, i) => (
            <React.Fragment key={i}>
              {i > 0 && <ChevronRight className="w-3 h-3" />}
              <button
                onClick={() => useScannerStore.getState().navigateTo(i)}
                className="hover:text-foreground transition-colors"
              >
                {i === 0 ? t('scanner.root') : p.split('/').pop()}
              </button>
            </React.Fragment>
          ))}
        </nav>
      )}

      {/* Action bar */}
      {selectedPaths.size > 0 && (
        <div className="px-4 py-2 flex items-center gap-3 bg-muted/50 border-b border-border">
          <span className="text-sm">{selectedPaths.size} selected</span>
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
        {entries.length === 0 ? (
          <EmptyState />
        ) : (
          <FileTable
            entries={sortedEntries}
            analysisMap={analysisMap}
            selectedPaths={selectedPaths}
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={handleSort}
            onToggleSelect={(path) => store.toggleSelect(path)}
            onSelectAll={() => {
              if (selectedPaths.size === sortedEntries.length) store.clearSelection()
              else sortedEntries.forEach((e) => { if (!selectedPaths.has(e.path)) store.toggleSelect(e.path) })
            }}
            onDrillDown={handleDrillDown}
          />
        )}
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

function FileTable({
  entries, analysisMap, selectedPaths, sortKey, sortDir, onSort, onToggleSelect, onSelectAll, onDrillDown,
}: {
  entries: FileEntry[]
  analysisMap: Record<string, AnalysisMark>
  selectedPaths: Set<string>
  sortKey: SortKey
  sortDir: SortDir
  onSort: (key: SortKey) => void
  onToggleSelect: (path: string) => void
  onSelectAll: () => void
  onDrillDown: (entry: FileEntry) => void
}) {
  const { t } = useTranslation()
  return (
    <table className="w-full text-sm">
      <thead className="sticky top-0 bg-background border-b border-border">
        <tr>
          <th className="w-10 px-3 py-2">
            <button onClick={onSelectAll}>
              {selectedPaths.size === entries.length && entries.length > 0
                ? <CheckSquare className="w-4 h-4 text-primary" />
                : <Square className="w-4 h-4 text-muted-foreground" />}
            </button>
          </th>
          <th className="text-left px-3 py-2 cursor-pointer" onClick={() => onSort('name')}>
            {t('scanner.name')} {sortKey === 'name' && (sortDir === 'asc' ? '↑' : '↓')}
          </th>
          <th className="text-right px-3 py-2 cursor-pointer w-24" onClick={() => onSort('size')}>
            {t('scanner.size')} {sortKey === 'size' && (sortDir === 'asc' ? '↑' : '↓')}
          </th>
          <th className="text-left px-3 py-2 cursor-pointer w-28" onClick={() => onSort('modTime')}>
            {t('scanner.modified')} {sortKey === 'modTime' && (sortDir === 'asc' ? '↑' : '↓')}
          </th>
          <th className="text-left px-3 py-2 cursor-pointer w-32" onClick={() => onSort('riskLevel')}>
            {t('scanner.risk')} {sortKey === 'riskLevel' && (sortDir === 'asc' ? '↑' : '↓')}
          </th>
        </tr>
      </thead>
      <tbody>
        {entries.map((entry) => (
          <FileRow
            key={entry.path}
            entry={entry}
            analysis={analysisMap[entry.path]}
            selected={selectedPaths.has(entry.path)}
            onToggleSelect={onToggleSelect}
            onDrillDown={onDrillDown}
          />
        ))}
      </tbody>
    </table>
  )
}

function FileRow({ entry, analysis, selected, onToggleSelect, onDrillDown }: {
  entry: FileEntry
  analysis?: AnalysisMark
  selected: boolean
  onToggleSelect: (path: string) => void
  onDrillDown: (entry: FileEntry) => void
}) {
  return (
    <tr className={`border-b border-border hover:bg-muted/50 ${selected ? 'bg-primary/5' : ''}`}>
      <td className="px-3 py-2">
        <button onClick={() => onToggleSelect(entry.path)}>
          {selected
            ? <CheckSquare className="w-4 h-4 text-primary" />
            : <Square className="w-4 h-4 text-muted-foreground" />}
        </button>
      </td>
      <td className="px-3 py-2">
        <button
          onClick={() => onDrillDown(entry)}
          className={`text-left ${entry.isDir ? 'text-primary font-medium' : ''}`}
          disabled={!entry.isDir}
        >
          {entry.isDir ? '📁 ' : '📄 '}
          {entry.name}
        </button>
      </td>
      <td className="text-right px-3 py-2 text-muted-foreground">
        {formatBytes(entry.size)}
      </td>
      <td className="px-3 py-2 text-muted-foreground">
        {entry.modTime ? new Date(entry.modTime).toLocaleDateString() : '--'}
      </td>
      <td className="px-3 py-2">
        {analysis ? <RiskBadge risk={analysis.riskLevel} reason={analysis.reason} /> : <span className="text-muted-foreground">--</span>}
      </td>
    </tr>
  )
}

function RiskBadge({ risk, reason }: { risk: RiskLevel; reason: string }) {
  const config: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
    safe: { icon: <ShieldCheck className="w-3 h-3" />, color: 'text-safe border-safe/30', label: 'Safe' },
    caution: { icon: <AlertTriangle className="w-3 h-3" />, color: 'text-caution border-caution/30', label: 'Caution' },
    dangerous: { icon: <ShieldAlert className="w-3 h-3" />, color: 'text-destructive border-destructive/30', label: 'Dangerous' },
  }
  const c = config[risk] || config['']
  if (!c) return null

  return (
    <Tooltip>
      <TooltipTrigger>
        <Badge variant="outline" className={`${c.color} text-xs`}>
          {c.icon} <span className="ml-1">{c.label}</span>
        </Badge>
      </TooltipTrigger>
      {reason && <TooltipContent><p className="max-w-xs">{reason}</p></TooltipContent>}
    </Tooltip>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}
