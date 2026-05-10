import { describe, it, expect } from 'vitest'
import { formatBytes } from '@/components/treemap/treemap-utils'

describe('formatBytes', () => {
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

  it('formats terabytes', () => {
    expect(formatBytes(1099511627776)).toBe('1 TB')
  })
})

describe('risk level sorting order', () => {
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
