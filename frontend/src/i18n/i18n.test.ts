import { describe, it, expect } from 'vitest'
import en from './en.json'
import zh from './zh.json'

// Flatten nested keys: { a: { b: "x" } } → ["a.b"]
function flattenKeys(obj: Record<string, unknown>, prefix = ''): string[] {
  const keys: string[] = []
  for (const [k, v] of Object.entries(obj)) {
    const full = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'object' && v !== null) {
      keys.push(...flattenKeys(v as Record<string, unknown>, full))
    } else {
      keys.push(full)
    }
  }
  return keys
}

describe('i18n language packs', () => {
  it('EN and ZH have the same keys', () => {
    const enKeys = new Set(flattenKeys(en as Record<string, unknown>))
    const zhKeys = new Set(flattenKeys(zh as Record<string, unknown>))

    const missingInZh = [...enKeys].filter((k) => !zhKeys.has(k))
    const missingInEn = [...zhKeys].filter((k) => !enKeys.has(k))

    expect(missingInZh, 'keys missing in zh.json').toEqual([])
    expect(missingInEn, 'keys missing in en.json').toEqual([])
  })

  it('EN has no empty values', () => {
    const keys = flattenKeys(en as Record<string, unknown>)
    for (const k of keys) {
      const parts = k.split('.')
      let val: unknown = en
      for (const p of parts) {
        val = (val as Record<string, unknown>)[p]
      }
      expect(typeof val).toBe('string')
      expect((val as string).length, `empty value for ${k}`).toBeGreaterThan(0)
    }
  })
})
