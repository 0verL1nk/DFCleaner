import type { CleanableItem } from '@/stores/scanner'

export interface TreemapNode {
  id: string
  value: number
  category?: string
  riskLevel?: string
  path?: string
  isDir?: boolean
  children?: TreemapNode[]
}

export interface AggregationResult {
  nodes: TreemapNode[]
  otherNode: TreemapNode | null
  aggregatedCount: number
}

const SIZE_THRESHOLD = 1024 * 1024 // 1MB
const MAX_ITEMS = 200

const CATEGORY_COLORS: Record<string, string> = {
  cache: '#06b6d4',
  temp: '#f59e0b',
  log: '#8b5cf6',
  build: '#ec4899',
  download: '#10b981',
  media: '#f43f5e',
  source: '#3b82f6',
  other: '#6b7280',
}

export function getCategoryColor(category?: string): string {
  return CATEGORY_COLORS[category ?? 'other'] ?? CATEGORY_COLORS.other
}

export function aggregateItems(items: CleanableItem[]): AggregationResult {
  const sorted = [...items].sort((a, b) => b.size - a.size)

  const main: CleanableItem[] = []
  const small: CleanableItem[] = []

  for (const item of sorted) {
    if (main.length < MAX_ITEMS && item.size >= SIZE_THRESHOLD) {
      main.push(item)
    } else {
      small.push(item)
    }
  }

  const nodes: TreemapNode[] = main.map((item) => ({
    id: item.path,
    value: item.size,
    category: item.category,
    riskLevel: item.riskLevel,
    path: item.path,
    isDir: item.isDir,
  }))

  let otherNode: TreemapNode | null = null
  if (small.length > 0) {
    const totalSize = small.reduce((sum, i) => sum + i.size, 0)
    otherNode = {
      id: '__other__',
      value: totalSize,
      category: 'other',
      path: `__other__ (${small.length} items)`,
    }
    nodes.push(otherNode)
  }

  return { nodes, otherNode, aggregatedCount: small.length }
}

export function cleanableItemsToTreemapData(items: CleanableItem[]): TreemapNode {
  const { nodes } = aggregateItems(items)
  return {
    id: 'root',
    value: nodes.reduce((sum, n) => sum + n.value, 0),
    children: nodes,
  }
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}
