import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { formatTimeAgo } from './time'

const mockT = vi.fn((key: string, opts?: Record<string, unknown>) => {
  if (opts) return `${key} ${JSON.stringify(opts)}`
  return key
})

describe('formatTimeAgo', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-10T12:00:00Z'))
    mockT.mockClear()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('returns -- for empty string', () => {
    expect(formatTimeAgo('', mockT)).toBe('--')
    expect(mockT).not.toHaveBeenCalled()
  })

  it('returns -- for zero-value date (0001-01-01)', () => {
    expect(formatTimeAgo('0001-01-01T00:00:00Z', mockT)).toBe('--')
    expect(mockT).not.toHaveBeenCalled()
  })

  it('returns "just now" for < 1 minute ago', () => {
    const date = new Date('2026-05-10T11:59:30Z').toISOString()
    expect(formatTimeAgo(date, mockT)).toBe('time.justNow')
    expect(mockT).toHaveBeenCalledWith('time.justNow')
  })

  it('returns minutes ago for < 1 hour', () => {
    const date = new Date('2026-05-10T11:30:00Z').toISOString()
    formatTimeAgo(date, mockT)
    expect(mockT).toHaveBeenCalledWith('time.minutesAgo', { count: 30 })
  })

  it('returns hours ago for < 1 day', () => {
    const date = new Date('2026-05-10T09:00:00Z').toISOString()
    formatTimeAgo(date, mockT)
    expect(mockT).toHaveBeenCalledWith('time.hoursAgo', { count: 3 })
  })

  it('returns days ago for >= 1 day', () => {
    const date = new Date('2026-05-08T12:00:00Z').toISOString()
    formatTimeAgo(date, mockT)
    expect(mockT).toHaveBeenCalledWith('time.daysAgo', { count: 2 })
  })

  it('handles exactly 1 minute boundary', () => {
    const date = new Date('2026-05-10T11:59:00Z').toISOString()
    formatTimeAgo(date, mockT)
    expect(mockT).toHaveBeenCalledWith('time.minutesAgo', { count: 1 })
  })
})
