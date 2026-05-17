import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import type { CleanableItem } from '@/stores/scanner'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Checkbox } from '@/components/ui/checkbox'
import { ChevronRight, ChevronDown, Folder, File, Trash2, ShieldCheck, AlertTriangle } from 'lucide-react'
import { formatBytes } from '@/lib/format'

interface CleanupPreviewDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  items: CleanableItem[]
  selectedPaths: Set<string>
  onConfirm: (operation: 'trash' | 'delete') => void
}

interface TreeNode {
  name: string
  path: string
  size: number
  children: Map<string, TreeNode>
  items: CleanableItem[]
}

function buildTree(items: CleanableItem[]): TreeNode {
  const root: TreeNode = { name: '/', path: '', size: 0, children: new Map(), items: [] }
  for (const item of items) {
    const parts = item.path.split('/').filter(Boolean)
    let node = root
    for (let i = 0; i < parts.length - 1; i++) {
      const seg = parts[i]
      if (!node.children.has(seg)) {
        node.children.set(seg, { name: seg, path: parts.slice(0, i + 1).join('/'), size: 0, children: new Map(), items: [] })
      }
      node = node.children.get(seg)!
    }
    const leaf = parts[parts.length - 1]
    if (item.isDir) {
      node.items.push(item)
    } else {
      if (!node.children.has(leaf)) {
        node.children.set(leaf, { name: leaf, path: item.path, size: item.size, children: new Map(), items: [item] })
      }
    }
  }
  function calcSize(n: TreeNode): number {
    let s = n.items.reduce((sum, i) => sum + i.size, 0)
    for (const child of n.children.values()) {
      s += calcSize(child)
    }
    n.size = s
    return s
  }
  calcSize(root)
  return root
}

export function CleanupPreviewDialog({ open, onOpenChange, items, selectedPaths, onConfirm }: CleanupPreviewDialogProps) {
  const { t } = useTranslation()
  const [deselected, setDeselected] = useState<Set<string>>(new Set())
  const [expandedDirs, setExpandedDirs] = useState<Set<string>>(new Set(['']))

  const tree = useMemo(() => buildTree(items), [items])

  const activeItems = items.filter((i) => !deselected.has(i.path))
  const totalSize = activeItems.reduce((s, i) => s + i.size, 0)
  const safeCount = activeItems.filter((i) => i.riskLevel === 'safe').length
  const cautionCount = activeItems.filter((i) => i.riskLevel === 'caution').length
  const safeSize = activeItems.filter((i) => i.riskLevel === 'safe').reduce((s, i) => s + i.size, 0)
  const cautionSize = activeItems.filter((i) => i.riskLevel === 'caution').reduce((s, i) => s + i.size, 0)

  const toggleDeselect = (path: string) => {
    setDeselected((prev) => {
      const next = new Set(prev)
      next.has(path) ? next.delete(path) : next.add(path)
      return next
    })
  }

  const toggleExpand = (path: string) => {
    setExpandedDirs((prev) => {
      const next = new Set(prev)
      next.has(path) ? next.delete(path) : next.add(path)
      return next
    })
  }

  const handleConfirm = (op: 'trash' | 'delete') => {
    onConfirm(op)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>{t('cleanup.title')}</DialogTitle>
        </DialogHeader>

        <div className="flex items-center gap-4 text-sm">
          <span className="font-medium">{formatBytes(totalSize)}</span>
          <Separator orientation="vertical" className="h-4" />
          <span className="text-muted-foreground">{activeItems.length} {t('scanner.itemsToClean')}</span>
          <Separator orientation="vertical" className="h-4" />
          <Badge variant="outline" className="border-safe/30 text-safe">
            <ShieldCheck className="w-3 h-3 mr-1" />{safeCount} ({formatBytes(safeSize)})
          </Badge>
          {cautionCount > 0 && (
            <Badge variant="outline" className="border-caution/30 text-caution">
              <AlertTriangle className="w-3 h-3 mr-1" />{cautionCount} ({formatBytes(cautionSize)})
            </Badge>
          )}
        </div>

        <Separator />

        <ScrollArea className="flex-1 min-h-0 max-h-[40vh]">
          <div className="pr-3">
            <TreeLevel
              node={tree}
              deselected={deselected}
              expandedDirs={expandedDirs}
              onToggleDeselect={toggleDeselect}
              onToggleExpand={toggleExpand}
              depth={0}
            />
          </div>
        </ScrollArea>

        <Separator />

        <DialogFooter className="gap-3 sm:gap-3">
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button variant="secondary" onClick={() => handleConfirm('trash')}>
            <Trash2 className="w-3.5 h-3.5 mr-1.5" />
            {t('scanner.moveToTrash')}
          </Button>
          <Button variant="destructive" onClick={() => handleConfirm('delete')}>
            {t('scanner.permanentDelete')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function TreeLevel({ node, deselected, expandedDirs, onToggleDeselect, onToggleExpand, depth }: {
  node: TreeNode
  deselected: Set<string>
  expandedDirs: Set<string>
  onToggleDeselect: (path: string) => void
  onToggleExpand: (path: string) => void
  depth: number
}) {
  const entries = [...node.children.values()].sort((a, b) => b.size - a.size)
  const isExpanded = expandedDirs.has(node.path)

  return (
    <div style={{ paddingLeft: depth > 0 ? 16 : 0 }}>
      {entries.map((child) => {
        const hasChildren = child.children.size > 0
        const isDeselected = deselected.has(child.path)
        const childItems = child.items.length > 0 ? child.items : []

        return (
          <div key={child.path}>
            <div
              className={`flex items-center gap-2 py-1.5 px-2 rounded-md hover:bg-muted/50 cursor-pointer ${
                isDeselected ? 'opacity-50' : ''
              }`}
            >
              {hasChildren ? (
                <button onClick={() => onToggleExpand(child.path)} className="shrink-0">
                  {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
                </button>
              ) : (
                <span className="w-3.5" />
              )}

              <Checkbox
                checked={!isDeselected}
                onCheckedChange={() => onToggleDeselect(child.path)}
                className="shrink-0"
              />

              {childItems[0]?.isDir ? (
                <Folder className="w-4 h-4 text-primary shrink-0" />
              ) : (
                <File className="w-4 h-4 text-muted-foreground shrink-0" />
              )}

              <span className="flex-1 truncate text-sm">{child.name}</span>
              {childItems[0]?.riskLevel && (
                <RiskBadge risk={childItems[0].riskLevel} />
              )}
              {childItems[0]?.category && (
                <Badge variant="secondary" className="text-[10px] px-1.5 py-0">{childItems[0].category}</Badge>
              )}
              <span className="text-xs text-muted-foreground tabular-nums shrink-0">{formatBytes(child.size)}</span>
            </div>

            {hasChildren && isExpanded && (
              <TreeLevel
                node={child}
                deselected={deselected}
                expandedDirs={expandedDirs}
                onToggleDeselect={onToggleDeselect}
                onToggleExpand={onToggleExpand}
                depth={depth + 1}
              />
            )}
          </div>
        )
      })}
    </div>
  )
}

function RiskBadge({ risk }: { risk: string }) {
  const config: Record<string, { icon: React.ReactNode; cls: string }> = {
    safe: { icon: <ShieldCheck className="w-3 h-3" />, cls: 'border-safe/30 text-safe' },
    caution: { icon: <AlertTriangle className="w-3 h-3" />, cls: 'border-caution/30 text-caution' },
    dangerous: { icon: <AlertTriangle className="w-3 h-3" />, cls: 'border-destructive/30 text-destructive' },
  }
  const c = config[risk]
  if (!c) return null
  return (
    <Badge variant="outline" className={`text-[10px] px-1.5 py-0 ${c.cls}`}>
      {c.icon}
    </Badge>
  )
}
