import { describe, it, expect } from 'vitest'
import { cleanableItemsToTreemapData, formatBytes, aggregateItems } from './treemap-utils'
import type { CleanableItem } from '@/stores/scanner'

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

describe('cleanableItemsToTreemapData', () => {
  it('returns root node with children for given items', () => {
    const items = [
      makeItem({ path: '/tmp/a.log', name: 'a.log', size: 2 * 1024 * 1024, category: 'cache' }),
      makeItem({ path: '/tmp/b.log', name: 'b.log', size: 4 * 1024 * 1024, category: 'log' }),
    ]
    const data = cleanableItemsToTreemapData(items)
    expect(data.id).toBe('root')
    expect(data.children!.length).toBe(2)
    expect(data.value).toBe(2 * 1024 * 1024 + 4 * 1024 * 1024)
  })

  it('aggregates items under 1MB into __other__ bucket', () => {
    // Items under 1MB go to "other", items >= 1MB stay as main nodes
    const items = [
      // 200 small items (under 1MB)
      ...Array.from({ length: 200 }, (_, i) =>
        makeItem({ path: `/tmp/small${i}.tmp`, name: `small${i}.tmp`, size: 500 * 1024, category: 'temp' }),
      ),
      // One large item
      makeItem({ path: '/tmp/large.tmp', name: 'large.tmp', size: 5 * 1024 * 1024, category: 'temp' }),
    ]
    const result = aggregateItems(items)
    expect(result.otherNode).toBeDefined()
    expect(result.otherNode!.id).toBe('__other__')
    expect(result.aggregatedCount).toBe(200)
  })

  it('handles empty items array', () => {
    const data = cleanableItemsToTreemapData([])
    expect(data.id).toBe('root')
    expect(data.value).toBe(0)
    expect(data.children).toEqual([])
  })

  it('creates one node per item (flat structure)', () => {
    const items = [
      makeItem({ path: '/a', name: 'a', size: 2 * 1024 * 1024, category: 'cache' }),
      makeItem({ path: '/b', name: 'b', size: 3 * 1024 * 1024, category: 'cache' }),
      makeItem({ path: '/c', name: 'c', size: 4 * 1024 * 1024, category: 'log' }),
    ]
    const data = cleanableItemsToTreemapData(items)
    expect(data.children!.length).toBe(3)
    // Sorted by size descending (largest first)
    expect(data.children!.map(c => c.id)).toEqual(['/c', '/b', '/a'])
  })
})

describe('formatBytes', () => {
  it('formats zero', () => expect(formatBytes(0)).toBe('0 B'))
  it('formats bytes', () => expect(formatBytes(500)).toBe('500 B'))
  it('formats KB', () => expect(formatBytes(1024)).toBe('1 KB'))
  it('formats MB', () => expect(formatBytes(1048576)).toBe('1 MB'))
  it('formats GB', () => expect(formatBytes(1073741824)).toBe('1 GB'))
  it('formats TB', () => expect(formatBytes(1099511627776)).toBe('1 TB'))
  it('formats fractional', () => expect(formatBytes(1536)).toBe('1.5 KB'))
})
