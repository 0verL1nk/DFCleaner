import { useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { ResponsiveTreeMap } from '@nivo/treemap'
import type { CleanableItem } from '@/stores/scanner'
import { cleanableItemsToTreemapData, getCategoryColor, formatBytes, type TreemapNode } from './treemap-utils'
import { TreemapTooltip } from './TreemapTooltip'
import { BreadcrumbNav } from './BreadcrumbNav'

interface TreemapChartProps {
  items: CleanableItem[]
  onItemClick?: (item: CleanableItem) => void
}

interface DrillState {
  path: string
  label: string
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type NodeData = any

export function TreemapChart({ items, onItemClick }: TreemapChartProps) {
  const { t } = useTranslation()
  const [drillStack, setDrillStack] = useState<DrillState[]>([])
  const data = cleanableItemsToTreemapData(items)

  const handleNavigate = useCallback((path: string) => {
    setDrillStack((prev) => {
      const idx = prev.findIndex((d) => d.path === path)
      if (idx >= 0) return prev.slice(0, idx + 1)
      return prev
    })
  }, [])

  const handleDrillDown = useCallback((node: TreemapNode) => {
    if (node.isDir && node.path) {
      setDrillStack((prev) => [...prev, { path: node.path!, label: node.id }])
    }
  }, [])

  if (!data.children || data.children.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        <p>{t('treemap.noData')}</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      {drillStack.length > 0 && (
        <BreadcrumbNav stack={drillStack} onNavigate={handleNavigate} />
      )}
      <div className="flex-1 min-h-0">
        <ResponsiveTreeMap
          data={data}
          identity="id"
          value="value"
          valueFormat={(v: number) => formatBytes(v)}
          colors={(node: NodeData) => {
            const n = (node.data ?? node) as TreemapNode
            return getCategoryColor(n.category)
          }}
          colorBy="id"
          nodeOpacity={0.85}
          borderColor={{ theme: 'background' }}
          borderWidth={2}
          label={(node: NodeData) => {
            const n = (node.data ?? node) as TreemapNode
            const name = n.id === '__other__' ? t('treemap.other') : String(n.id).split('/').pop() || n.id
            return `${name}\n${formatBytes(node.value)}`
          }}
          labelTextColor={{ theme: 'background' }}
          tooltip={({ node }: { node: NodeData }) => {
            const n = (node.data ?? node) as TreemapNode
            return (
              <TreemapTooltip
                id={n.id}
                value={node.value}
                category={n.category}
                riskLevel={n.riskLevel}
                isDir={n.isDir}
              />
            )
          }}
          onClick={(node: NodeData) => {
            const n = (node.data ?? node) as TreemapNode
            if (n.isDir) {
              handleDrillDown(n)
            } else if (onItemClick) {
              const item = items.find((i) => i.path === n.path)
              if (item) onItemClick(item)
            }
          }}
          animate
          motionConfig="gentle"
        />
      </div>
    </div>
  )
}
