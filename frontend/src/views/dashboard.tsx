import { useTranslation } from 'react-i18next'
import { useRecentCleanups, useActiveLLMConfig, useSystemDrives, useCleanableSize } from '@/hooks/wails'
import { HardDrive, Clock, Zap, Trash2, Wifi, WifiOff } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Link } from '@tanstack/react-router'

export function Dashboard() {
  const { t } = useTranslation()
  const { data: cleanups } = useRecentCleanups(5)
  const { data: activeConfig, isError } = useActiveLLMConfig()
  const { data: drives } = useSystemDrives()
  const { data: cleanableSize } = useCleanableSize()

  const rootDrive = drives?.find((d: any) => d.isSystem) || drives?.[0]
  const totalFreed = cleanups?.reduce((sum: number, c: any) => sum + (c.freedBytes || 0), 0) ?? 0

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t('nav.dashboard')}</h1>
        <div className="flex items-center gap-3">
          <LLMStatus connected={!!activeConfig && !isError} model={activeConfig?.modelName} />
          <Link to="/scanner">
            <Button>
              <Zap className="w-4 h-4 mr-2" />
              {t('dashboard.quickScan')}
            </Button>
          </Link>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <StatCard
          icon={<HardDrive className="w-5 h-5" />}
          label={t('dashboard.diskUsage')}
          value={rootDrive ? formatBytes(rootDrive.used) : '--'}
          sub={rootDrive ? `${formatBytes(rootDrive.total)} total` : undefined}
        />
        <StatCard
          icon={<Zap className="w-5 h-5" />}
          label={t('dashboard.suggestedFree')}
          value={cleanableSize ? formatBytes(cleanableSize) : '--'}
          sub={cleanableSize ? t('dashboard.afterAnalysis') : undefined}
        />
        <StatCard
          icon={<Clock className="w-5 h-5" />}
          label={t('dashboard.lastCleanup')}
          value={cleanups && cleanups.length > 0 ? timeAgo(cleanups[0].createdAt) : '--'}
          sub={totalFreed > 0 ? `${formatBytes(totalFreed)} freed` : undefined}
        />
      </div>

      <section>
        <h2 className="text-lg font-semibold mb-3">{t('dashboard.recentCleanups')}</h2>
        {cleanups && cleanups.length > 0 ? (
          <div className="space-y-2">
            {cleanups.map((c: any) => (
              <div key={c.id} className="flex items-center gap-3 p-3 rounded-lg border border-border">
                <Trash2 className="w-4 h-4 text-muted-foreground" />
                <span className="flex-1 truncate text-sm">{c.filePath}</span>
                <Badge variant="secondary">{c.operation}</Badge>
                <span className="text-sm font-medium text-safe">{formatBytes(c.freedBytes)}</span>
                <span className="text-xs text-muted-foreground">{timeAgo(c.createdAt)}</span>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-8 text-muted-foreground">
            <Trash2 className="w-8 h-8 mx-auto mb-2" />
            <p className="text-sm">{t('dashboard.noCleanups')}</p>
          </div>
        )}
      </section>
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

function StatCard({ icon, label, value, sub }: { icon: React.ReactNode; label: string; value: string; sub?: string }) {
  return (
    <div className="p-4 rounded-xl border border-border bg-card">
      <div className="flex items-center gap-2 text-muted-foreground mb-2">
        {icon}
        <span className="text-sm">{label}</span>
      </div>
      <div className="text-xl font-semibold">{value}</div>
      {sub && <div className="text-xs text-muted-foreground mt-1">{sub}</div>}
    </div>
  )
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function timeAgo(dateStr: string): string {
  if (!dateStr) return '--'
  const diff = Date.now() - new Date(dateStr).getTime()
  if (diff < 60000) return 'just now'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
  return `${Math.floor(diff / 86400000)}d ago`
}
