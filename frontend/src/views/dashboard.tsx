import { useTranslation } from 'react-i18next'
import { useRecentCleanups, useActiveLLMConfig, useSystemDrives, useCleanableSize, useCleanableItems } from '@/hooks/wails'
import { HardDrive, Zap, Trash2, Wifi, WifiOff, ChevronDown, ChevronUp } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Link } from '@tanstack/react-router'
import { TreemapChart } from '@/components/treemap/TreemapChart'
import { useScannerStore, type CleanableItem } from '@/stores/scanner'
import { formatBytes } from '@/components/treemap/treemap-utils'
import { useTimeAgo } from '@/lib/time'
import { useState, useEffect } from 'react'

export function Dashboard() {
  const { t } = useTranslation()
  const timeAgo = useTimeAgo()
  const { data: cleanups } = useRecentCleanups(5)
  const { data: activeConfig, isError } = useActiveLLMConfig()
  const { data: drives } = useSystemDrives()
  const { data: cleanableSize } = useCleanableSize()
  const store = useScannerStore()
  const [recentOpen, setRecentOpen] = useState(false)

  // Load cached cleanable items for treemap
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
  const totalFreed = cleanups?.reduce((sum: number, c: any) => sum + (c.freedBytes || 0), 0) ?? 0
  const totalDisk = allDrives.reduce((sum: number, d: any) => sum + (d.total || 0), 0)
  const totalUsed = allDrives.reduce((sum: number, d: any) => sum + (d.used || 0), 0)

  return (
    <div className="flex flex-col h-full">
      {/* Summary bar */}
      <div className="shrink-0 px-5 py-3 border-b border-border bg-card flex items-center gap-6">
        <h1 className="text-lg font-bold">{t('nav.dashboard')}</h1>
        <div className="flex items-center gap-4 text-sm">
          <SummaryItem label={t('dashboard.diskUsage')} value={totalDisk > 0 ? `${formatBytes(totalUsed)} / ${formatBytes(totalDisk)}` : '--'} />
          <Separator orientation="vertical" className="h-4" />
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

      {/* Treemap */}
      <div className="flex-1 min-h-0">
        {cleanableItems.length > 0 ? (
          <TreemapChart items={cleanableItems} />
        ) : (
          <div className="flex flex-col items-center justify-center h-full text-muted-foreground">
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
      </div>

      {/* Recent cleanups — collapsible */}
      {cleanups && cleanups.length > 0 && (
        <div className="shrink-0 border-t border-border">
          <Button
            variant="ghost"
            className="w-full justify-start px-5 py-2 h-auto text-sm text-muted-foreground hover:text-foreground rounded-none"
            onClick={() => setRecentOpen(!recentOpen)}
          >
            <Trash2 className="w-3.5 h-3.5" />
            <span>{t('dashboard.recentCleanups')}</span>
            {recentOpen ? <ChevronUp className="w-3.5 h-3.5 ml-auto" /> : <ChevronDown className="w-3.5 h-3.5 ml-auto" />}
          </Button>
          {recentOpen && (
            <div className="px-5 pb-3 space-y-2">
              {cleanups.map((c: any) => (
                <div key={c.id} className="flex items-center gap-3 p-2 rounded-lg border border-border text-sm">
                  <Trash2 className="w-3.5 h-3.5 text-muted-foreground" />
                  <span className="flex-1 truncate">{c.filePath}</span>
                  <Badge variant="secondary" className="text-xs">{c.operation}</Badge>
                  <span className="font-medium text-safe">{formatBytes(c.freedBytes)}</span>
                  <span className="text-xs text-muted-foreground">{timeAgo(c.createdAt)}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
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
