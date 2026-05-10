import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import * as App from '../../wailsjs/go/main/App'
import { scheduler as schedulerModels } from '../../wailsjs/go/models'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Clock, Plus, Trash2, Power, PowerOff, Loader2 } from 'lucide-react'
import { useTimeAgo } from '@/lib/time'

type ScheduleConfig = schedulerModels.ScheduleConfig

const CRON_PRESETS = [
  { labelKey: 'cron.daily3am', expr: '0 3 * * *' },
  { labelKey: 'cron.weeklySun2am', expr: '0 2 * * 0' },
  { labelKey: 'cron.every6h', expr: '0 */6 * * *' },
  { labelKey: 'cron.hourly', expr: '0 * * * *' },
  { labelKey: 'cron.custom', expr: '' },
]

function formatNextRun(dateStr: string): string {
  if (!dateStr || dateStr.startsWith('0001')) return '--'
  return new Date(dateStr).toLocaleString()
}

export function Scheduler() {
  const { t } = useTranslation()
  const timeAgo = useTimeAgo()
  const qc = useQueryClient()
  const [showForm, setShowForm] = useState(false)

  const { data: schedules = [] } = useQuery({
    queryKey: ['schedules'],
    queryFn: () => App.GetSchedules() as Promise<ScheduleConfig[]>,
  })

  const deleteMut = useMutation({
    mutationFn: (id: string) => App.DeleteSchedule(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['schedules'] }),
  })

  const toggleMut = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      App.ToggleSchedule(id, enabled),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['schedules'] }),
  })

  return (
    <div className="p-6 max-w-3xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t('scheduler.title')}</h1>
        <Button size="sm" onClick={() => setShowForm(!showForm)}>
          <Plus className="w-4 h-4 mr-1.5" />
          {t('scheduler.add')}
        </Button>
      </div>

      <p className="text-sm text-muted-foreground">{t('scheduler.description')}</p>

      {showForm && (
        <ScheduleForm
          onSave={() => {
            setShowForm(false)
            qc.invalidateQueries({ queryKey: ['schedules'] })
          }}
          onCancel={() => setShowForm(false)}
        />
      )}

      <Separator />

      {schedules.length === 0 ? (
        <div className="text-center py-12 text-muted-foreground">
          <Clock className="w-10 h-10 mx-auto mb-3" />
          <p>{t('scheduler.empty')}</p>
        </div>
      ) : (
        <ScrollArea className="max-h-[60vh]">
          <div className="space-y-3 pr-3">
            {schedules.map((s) => (
              <div key={s.id} className="p-4 rounded-lg border border-border space-y-2">
                <div className="flex items-center gap-3">
                  <Badge variant={s.enabled ? 'default' : 'secondary'}>
                    <Clock className="w-3 h-3 mr-1" />
                    {s.cronExpr}
                  </Badge>
                  <span className="text-sm font-medium truncate flex-1">{s.scanPath}</span>
                  <Badge variant="outline" className="text-xs">
                    {s.maxAutoRisk === 'safe' ? t('scanner.risk.safe') : t('scanner.risk.caution')}
                  </Badge>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    onClick={() => toggleMut.mutate({ id: s.id, enabled: !s.enabled })}
                  >
                    {s.enabled ? <Power className="w-3.5 h-3.5 text-safe" /> : <PowerOff className="w-3.5 h-3.5" />}
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7 text-destructive"
                    onClick={() => deleteMut.mutate(s.id)}
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </Button>
                </div>
                <div className="flex items-center gap-4 text-xs text-muted-foreground">
                  <span>{t('scheduler.lastRun')}: {timeAgo(s.lastRun)}</span>
                  <span>{t('scheduler.nextRun')}: {formatNextRun(s.nextRun)}</span>
                </div>
              </div>
            ))}
          </div>
        </ScrollArea>
      )}
    </div>
  )
}

function ScheduleForm({ onSave, onCancel }: { onSave: () => void; onCancel: () => void }) {
  const { t } = useTranslation()
  const cronLabels = useMemo(() => CRON_PRESETS.map(p => t(p.labelKey)), [t])
  const [cronExpr, setCronExpr] = useState('0 3 * * *')
  const [scanPath, setScanPath] = useState('')
  const [maxRisk, setMaxRisk] = useState('safe')
  const [presetIdx, setPresetIdx] = useState(0)
  const [saving, setSaving] = useState(false)

  const save = useMutation({
    mutationFn: (cfg: schedulerModels.ScheduleConfig) => App.SaveSchedule(cfg) as Promise<void>,
    onSuccess: onSave,
    onSettled: () => setSaving(false),
  })

  function handleSave() {
    if (!scanPath.trim()) return
    setSaving(true)
    save.mutate(schedulerModels.ScheduleConfig.createFrom({
      id: '',
      cronExpr,
      scanPath: scanPath.trim(),
      maxAutoRisk: maxRisk,
      enabled: true,
      lastRun: '',
      nextRun: '',
    }))
  }

  return (
    <div className="p-4 rounded-lg border border-border bg-card space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>{t('scheduler.frequency')}</Label>
          <Select
            value={String(presetIdx)}
            onValueChange={(v) => {
              const idx = Number(v)
              setPresetIdx(idx)
              if (CRON_PRESETS[idx].expr) setCronExpr(CRON_PRESETS[idx].expr)
            }}
          >
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              {CRON_PRESETS.map((p, i) => (
                <SelectItem key={i} value={String(i)}>{cronLabels[i]}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>{t('scheduler.cronExpr')}</Label>
          <Input
            value={cronExpr}
            onChange={(e) => { setCronExpr(e.target.value); setPresetIdx(CRON_PRESETS.length - 1) }}
            placeholder="0 3 * * *"
          />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>{t('scheduler.scanPath')}</Label>
          <Input value={scanPath} onChange={(e) => setScanPath(e.target.value)} placeholder="/home/user" />
        </div>
        <div className="space-y-2">
          <Label>{t('scheduler.maxRisk')}</Label>
          <Select value={maxRisk} onValueChange={setMaxRisk}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="safe">{t('scanner.risk.safe')}</SelectItem>
              <SelectItem value="caution">{t('scanner.risk.caution')}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="flex gap-2">
        <Button onClick={handleSave} disabled={saving || !scanPath.trim()}>
          {saving && <Loader2 className="w-4 h-4 mr-1 animate-spin" />}
          {t('common.save')}
        </Button>
        <Button variant="secondary" onClick={onCancel}>{t('common.cancel')}</Button>
      </div>
    </div>
  )
}
