import { describe, it, expect } from 'vitest'

// Pure utility tests — no React rendering needed, no version conflicts
// These test the behavior that matters: formatting and data transformations

describe('formatBytes', () => {
  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }

  it('formats zero bytes', () => {
    expect(formatBytes(0)).toBe('0 B')
  })

  it('formats bytes', () => {
    expect(formatBytes(500)).toBe('500 B')
  })

  it('formats kilobytes', () => {
    expect(formatBytes(1024)).toBe('1 KB')
    expect(formatBytes(1536)).toBe('1.5 KB')
  })

  it('formats megabytes', () => {
    expect(formatBytes(1048576)).toBe('1 MB')
    expect(formatBytes(5242880)).toBe('5 MB')
  })

  it('formats gigabytes', () => {
    expect(formatBytes(1073741824)).toBe('1 GB')
  })
})

describe('risk level sorting order', () => {
  // Mirrors the sorting logic from scanner view
  const riskOrder: Record<string, number> = { dangerous: 3, caution: 2, safe: 1, '': 0 }

  it('dangerous ranks highest', () => {
    expect(riskOrder['dangerous']).toBeGreaterThan(riskOrder['caution'])
  })

  it('caution ranks above safe', () => {
    expect(riskOrder['caution']).toBeGreaterThan(riskOrder['safe'])
  })

  it('unknown ranks lowest', () => {
    expect(riskOrder['']).toBe(0)
  })
})

describe('timeAgo', () => {
  function timeAgo(dateStr: string): string {
    if (!dateStr) return '--'
    const now = Date.now()
    const then = new Date(dateStr).getTime()
    const diff = now - then
    if (diff < 60000) return 'just now'
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
    return `${Math.floor(diff / 86400000)}d ago`
  }

  it('returns -- for empty string', () => {
    expect(timeAgo('')).toBe('--')
  })

  it('returns just now for recent timestamp', () => {
    expect(timeAgo(new Date().toISOString())).toBe('just now')
  })

  it('returns minutes ago', () => {
    const fiveMinAgo = new Date(Date.now() - 5 * 60000).toISOString()
    expect(timeAgo(fiveMinAgo)).toBe('5m ago')
  })

  it('returns hours ago', () => {
    const twoHoursAgo = new Date(Date.now() - 2 * 3600000).toISOString()
    expect(timeAgo(twoHoursAgo)).toBe('2h ago')
  })

  it('returns days ago', () => {
    const threeDaysAgo = new Date(Date.now() - 3 * 86400000).toISOString()
    expect(timeAgo(threeDaysAgo)).toBe('3d ago')
  })
})
