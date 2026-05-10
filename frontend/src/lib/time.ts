import { useTranslation } from 'react-i18next'

export function formatTimeAgo(dateStr: string, t: (key: string, opts?: Record<string, unknown>) => string): string {
  if (!dateStr || dateStr.startsWith('0001')) return '--'
  const diff = Date.now() - new Date(dateStr).getTime()
  if (diff < 60000) return t('time.justNow')
  if (diff < 3600000) return t('time.minutesAgo', { count: Math.floor(diff / 60000) })
  if (diff < 86400000) return t('time.hoursAgo', { count: Math.floor(diff / 3600000) })
  return t('time.daysAgo', { count: Math.floor(diff / 86400000) })
}

export function useTimeAgo() {
  const { t } = useTranslation()
  return (dateStr: string): string => formatTimeAgo(dateStr, t)
}
