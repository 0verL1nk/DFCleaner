import { useTranslation } from 'react-i18next'
import { Separator } from '@/components/ui/separator'
import { Badge } from '@/components/ui/badge'
import { getCategoryColor, formatBytes } from './treemap-utils'

interface TreemapTooltipProps {
  id: string
  value: number
  category?: string
  riskLevel?: string
  isDir?: boolean
}

export function TreemapTooltip({ id, value, category, riskLevel, isDir }: TreemapTooltipProps) {
  const { t } = useTranslation()
  const name = id === '__other__' ? t('treemap.other') : String(id).split('/').pop() || id
  const color = getCategoryColor(category)

  return (
    <div className="bg-popover border border-border rounded-lg px-3 py-2 shadow-lg text-sm min-w-[180px]">
      <div className="flex items-center gap-2 mb-1.5">
        <span
          className="w-3 h-3 rounded-sm shrink-0"
          style={{ backgroundColor: color }}
        />
        <span className="font-medium truncate">{name}</span>
        {isDir && <span className="text-xs text-muted-foreground">/</span>}
      </div>
      <Separator className="mb-1.5" />
      <div className="text-muted-foreground space-y-1 text-xs">
        <div className="flex justify-between">
          <span>{t('treemap.size')}</span>
          <span className="font-medium text-foreground">{formatBytes(value)}</span>
        </div>
        {category && (
          <div className="flex justify-between items-center">
            <span>{t('treemap.category')}</span>
            <Badge variant="secondary" className="text-[10px] px-1.5 py-0 capitalize">{category}</Badge>
          </div>
        )}
        {riskLevel && (
          <div className="flex justify-between items-center">
            <span>{t('treemap.risk')}</span>
            <Badge
              variant="outline"
              className={`text-[10px] px-1.5 py-0 ${
                riskLevel === 'safe'
                  ? 'border-safe/30 text-safe'
                  : riskLevel === 'caution'
                    ? 'border-caution/30 text-caution'
                    : 'border-destructive/30 text-destructive'
              }`}
            >
              {t(`scanner.risk.${riskLevel}`, riskLevel)}
            </Badge>
          </div>
        )}
      </div>
    </div>
  )
}
