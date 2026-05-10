import { describe, it, expect } from 'vitest'
import type { CleanableItem } from '@/stores/scanner'

interface TreeNode {
  name: string
  path: string
  size: number
  children: Map<string, TreeNode>
  items: CleanableItem[]
}

// Mirrors the buildTree logic from CleanupPreviewDialog
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

function makeItem(overrides: Partial<CleanableItem> & { path: string; name: string }): CleanableItem {
  return {
    size: 1024,
    isDir: false,
    riskLevel: 'safe',
    reason: 'test',
    category: 'cache',
    ...overrides,
  }
}

describe('buildTree', () => {
  it('handles empty items', () => {
    const tree = buildTree([])
    expect(tree.size).toBe(0)
    expect(tree.children.size).toBe(0)
  })

  it('builds flat file tree', () => {
    const items = [
      makeItem({ path: '/tmp/a.log', name: 'a.log', size: 100 }),
      makeItem({ path: '/tmp/b.log', name: 'b.log', size: 200 }),
    ]
    const tree = buildTree(items)
    expect(tree.size).toBe(300)
    expect(tree.children.has('tmp')).toBe(true)
  })

  it('builds nested directory tree', () => {
    const items = [
      makeItem({ path: '/home/user/cache/a.tmp', name: 'a.tmp', size: 100 }),
      makeItem({ path: '/home/user/cache/b.tmp', name: 'b.tmp', size: 200 }),
      makeItem({ path: '/home/user/logs/c.log', name: 'c.log', size: 300 }),
    ]
    const tree = buildTree(items)
    expect(tree.size).toBe(600)
    expect(tree.children.has('home')).toBe(true)
    const home = tree.children.get('home')!
    expect(home.children.has('user')).toBe(true)
    const user = home.children.get('user')!
    expect(user.children.size).toBe(2) // cache + logs
  })

  it('aggregates sizes bottom-up', () => {
    const items = [
      makeItem({ path: '/a/x.txt', name: 'x.txt', size: 10 }),
      makeItem({ path: '/a/y.txt', name: 'y.txt', size: 20 }),
      makeItem({ path: '/b/z.txt', name: 'z.txt', size: 30 }),
    ]
    const tree = buildTree(items)
    expect(tree.size).toBe(60)
    expect(tree.children.get('a')!.size).toBe(30)
    expect(tree.children.get('b')!.size).toBe(30)
  })

  it('handles directory items', () => {
    const items = [
      makeItem({ path: '/tmp/olddir', name: 'olddir', size: 500, isDir: true }),
      makeItem({ path: '/tmp/file.txt', name: 'file.txt', size: 100, isDir: false }),
    ]
    const tree = buildTree(items)
    expect(tree.size).toBe(600)
    const tmp = tree.children.get('tmp')!
    expect(tmp.items.length).toBe(1)
    expect(tmp.children.has('file.txt')).toBe(true)
  })
})
