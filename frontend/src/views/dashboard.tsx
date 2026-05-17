import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useRecentCleanups, useActiveLLMConfig, useSystemDrives, useCleanableSize, useCleanableItems } from '@/hooks/wails'
import { HardDrive, Zap, Trash2, Wifi, WifiOff, ChevronDown, ChevronRight, Database, FileText, FolderOpen, Archive, Image, File, Download } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Link } from '@tanstack/react-router'
import { useScannerStore, type CleanableItem, type RiskLevel } from '@/stores/scanner'
import { useTimeAgo } from '@/lib/time'

const CATEGORY_ICONS: Record<string, React.ReactNode> = {
  cache: <Database className="w-4 h-4" />,
  temp: <FileText className="w-4 h-4" />,
  log: <FileText className="w-4 h-4" />,
  build: <Archive className="w-4 h-4" />,
  download: <Download className="w-4 h-4" />,
  media: <Image className="w-4 h-4" />,
  source: <File className="w-4 h-4" />,
  other: <File className="w-4 h-4" />,
}

const CATEGORY_COLORS: Record<string, string> = {
  cache: 'bg-cyan-500/15 text-cyan-600 dark:text-cyan-400',
  temp: 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
  log: 'bg-violet-500/15 text-violet-600 dark:text-violet-400',
  build: 'bg-pink-500/15 text-pink-600 dark:text-pink-400',
  download: 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
  media: 'bg-rose-500/15 text-rose-600 dark:text-rose-400',
  source: 'bg-blue-500/15 text-blue-600 dark:text-blue-400',
  other: 'bg-gray-500/15 text-gray-600 dark:text-gray-400',
}

export function Dashboard() {
  const { t } = useTranslation()
  const timeAgo = useTimeAgo()
  const { data: cleanups } = useRecentCleanups(5)
  const { data: activeConfig, isError } = useActiveLLMConfig()
  const { data: drives } = useSystemDrives()
  const { data: cleanableSize } = useCleanableSize()
  const store = useScannerStore()

  const { data: cachedItems } = useCleanableItems()
  const cleanableItems = store.cleanableItems.length > 0
    ? store.cleanableItems
    : (cachedItems?.map((item: any): CleanableItem => ({
        path: item.path, name: item.name, size: item.size,
        isDir: item.isDir, riskLevel: item.riskLevel,
        reason: item.reason, category: item.category,
      })) ?? [])

  useEffect(() => {
    if (cachedItems && cachedItems.length > 0 && store.cleanableItems.length === 0) {
      const items: CleanableItem[] = cachedItems.map((item: any) => ({
        path: item.path, name: item.name, size: item.size,
        isDir: item.isDir, riskLevel: item.riskLevel,
        reason: item.reason, category: item.category,
      }))
      store.addCleanableItems(items)
    }
  }, [cachedItems])

  const allDrives = drives ?? []
  const totalCleanable = cleanableItems.reduce((sum, i) => sum + i.size, 0)
  const grouped = groupByCategory(cleanableItems)

  return (
    <div className="flex flex-col h-full overflow-auto">
      {/* Summary bar */}
      <div className="shrink-0 px-5 py-3 border-b border-border bg-card flex items-center gap-6">
        <h1 className="text-lg font-bold">{t('nav.dashboard')}</h1>
        <div className="flex items-center gap-4 text-sm">
          <SummaryItem label={t('dashboard.suggestedFree')} value={cleanableSize ? formatBytes(cleanableSize) : '--'} highlight />
          <Separator orientation="vertical" className="h-4" />
          <SummaryItem label={t('dashboard.lastCleanup')} value={cleanups && cleanups.length > 0 ? timeAgo(cleanups[0].createdAt) : '--'} />
        </div>
        <div className="ml-auto flex items-center gap-3">
          <LLMStatus connected={!!activeConfig && !isError} model={activeConfig?.modelName} />
          <Link to="/scanner">
            <Button size="sm">
              <Zap className="w-4 h-4 mr-1.5" />
              {t('dashboard.quickScan')}
            </Button>
          </Link>
        </div>
      </div>

      <div className="flex-1 p-5 space-y-5 overflow-auto">
        {/* Disk cards */}
        {allDrives.length > 0 && (
          <div className="grid gap-4" style={{ gridTemplateColumns: `repeat(${Math.min(allDrives.length, 3)}, 1fr)` }}>
            {allDrives.map((d: any) => (
              <DiskCard key={d.path} drive={d} cleanableSize={cleanableSize ?? 0} />
            ))}
          </div>
        )}

        {/* Cleanable items grouped by category */}
        {cleanableItems.length > 0 ? (
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">
                {t('dashboard.reclaimable')} · {formatBytes(totalCleanable)}
              </h2>
              <span className="text-xs text-muted-foreground">{cleanableItems.length} {t('scanner.itemsToClean')}</span>
            </div>
            {grouped.map(([cat, items]) => (
              <CategoryGroup key={cat} category={cat} items={items} />
            ))}
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center py-20 text-muted-foreground">
            <HardDrive className="w-12 h-12 mb-3" />
            <p>{t('treemap.noData')}</p>
            <p className="text-sm mt-1">{t('dashboard.selectDirectory')}</p>
            <Link to="/scanner" className="mt-4">
              <Button>
                <Zap className="w-4 h-4 mr-2" />
                {t('dashboard.quickScan')}
              </Button>
            </Link>
          </div>
        )}

        {/* Recent cleanups */}
        {cleanups && cleanups.length > 0 && (
          <div className="space-y-2">
            <h2 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">{t('dashboard.recentCleanups')}</h2>
            {cleanups.map((c: any) => (
              <div key={c.id} className="flex items-center gap-3 p-2.5 rounded-lg border border-border text-sm">
                <Trash2 className="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                <span className="flex-1 truncate">{c.filePath}</span>
                <Badge variant="secondary" className="text-xs">{c.operation}</Badge>
                <span className="font-medium text-safe shrink-0">{formatBytes(c.freedBytes)}</span>
                <span className="text-xs text-muted-foreground shrink-0">{timeAgo(c.createdAt)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function DiskCard({ drive, cleanableSize }: { drive: any; cleanableSize: number }) {
  const { t } = useTranslation()
  const total = drive.total || 1
  const used = drive.used || 0
  const usedPct = Math.round((used / total) * 100)
  const cleanablePct = Math.min(Math.round((cleanableSize / total) * 100), 100 - usedPct)

  return (
    <div className="rounded-xl border border-border p-4 space-y-3">
      <div className="flex items-center gap-2.5">
        <HardDrive className="w-5 h-5 text-primary" />
        <span className="font-medium text-sm">{drive.label || drive.path}</span>
        <span className="ml-auto text-xs text-muted-foreground">{formatBytes(total)}</span>
      </div>
      <div className="space-y-1.5">
        <div className="flex items-center justify-between text-xs">
          <span className="text-muted-foreground">{t('dashboard.usedSpace')}</span>
          <span className="font-medium">{formatBytes(used)} ({usedPct}%)</span>
        </div>
        <div className="h-2 bg-muted rounded-full overflow-hidden flex">
          <div className="bg-primary rounded-full" style={{ width: `${usedPct}%` }} />
          {cleanablePct > 0 && (
            <div className="bg-safe rounded-r-full" style={{ width: `${cleanablePct}%` }} />
          )}
        </div>
        <div className="flex items-center justify-between text-xs">
          <span className="text-muted-foreground">{t('dashboard.freeSpace')}: {formatBytes(drive.free || 0)}</span>
          {cleanableSize > 0 && (
            <span className="text-safe font-medium">+{formatBytes(cleanableSize)} {t('dashboard.reclaimable')}</span>
          )}
        </div>
      </div>
    </div>
  )
}

function CategoryGroup({ category, items }: { category: string; items: CleanableItem[] }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const totalSize = items.reduce((sum, i) => sum + i.size, 0)
  const icon = CATEGORY_ICONS[category] || CATEGORY_ICONS.other
  const colorClass = CATEGORY_COLORS[category] || CATEGORY_COLORS.other

  return (
    <div className="rounded-xl border border-border overflow-hidden">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-3 px-4 py-3 hover:bg-muted/30 transition-colors"
      >
        <div className={`p-1.5 rounded-lg ${colorClass}`}>{icon}</div>
        <span className="font-medium text-sm">{t(`category.${category}`)}</span>
        <span className="text-xs text-muted-foreground">{items.length} {t('scanner.itemsToClean')}</span>
        <span className="ml-auto font-medium text-sm">{formatBytes(totalSize)}</span>
        {open ? <ChevronDown className="w-4 h-4 text-muted-foreground" /> : <ChevronRight className="w-4 h-4 text-muted-foreground" />}
      </button>

      {open && (
        <div className="border-t border-border divide-y divide-border">
          {items.map((item) => (
            <div key={item.path} className="flex items-center gap-3 px-4 py-2.5 text-sm hover:bg-muted/20">
              {item.isDir ? (
                <FolderOpen className="w-4 h-4 text-primary shrink-0" />
              ) : (
                <File className="w-4 h-4 text-muted-foreground shrink-0" />
              )}
              <div className="flex-1 min-w-0">
                <span className={item.isDir ? 'text-primary font-medium' : ''}>{item.name}</span>
                <span className="text-xs text-muted-foreground ml-2">{item.path}</span>
              </div>
              <MiniRiskBadge risk={item.riskLevel as RiskLevel} />
              <span className="text-muted-foreground tabular-nums shrink-0 w-16 text-right">{formatBytes(item.size)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function MiniRiskBadge({ risk }: { risk: RiskLevel }) {
  const config: Record<string, string> = {
    safe: 'bg-safe/15 text-safe',
    caution: 'bg-caution/15 text-caution',
    dangerous: 'bg-destructive/15 text-destructive',
  }
  const { t } = useTranslation()
  const cls = config[risk]
  if (!cls) return null
  return (
    <span className={`text-[10px] px-1.5 py-0.5 rounded-md font-medium shrink-0 ${cls}`}>
      {t(`scanner.risk.${risk}`)}
    </span>
  )
}

function SummaryItem({ label, value, highlight }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-muted-foreground">{label}</span>
      <span className={`font-medium ${highlight ? 'text-safe' : ''}`}>{value}</span>
    </div>
  )
}

function LLMStatus({ connected, model }: { connected: boolean; model?: string }) {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-2">
      {connected ? (
        <>
          <Wifi className="w-4 h-4 text-safe" />
          <span className="text-xs text-muted-foreground">{model}</span>
        </>
      ) : (
        <>
          <WifiOff className="w-4 h-4 text-destructive" />
          <span className="text-xs text-destructive">{t('dashboard.noLLM')}</span>
        </>
      )}
    </div>
  )
}

function groupByCategory(items: CleanableItem[]): [string, CleanableItem[]][] {
  const map = new Map<string, CleanableItem[]>()
  for (const item of items) {
    const cat = item.category || 'other'
    if (!map.has(cat)) map.set(cat, [])
    map.get(cat)!.push(item)
  }
  return [...map.entries()].sort((a, b) => b[1].reduce((s, i) => s + i.size, 0) - a[1].reduce((s, i) => s + i.size, 0))
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}
