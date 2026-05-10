import { describe, it, expect } from 'vitest'
import { useSettingsStore } from './settings'

describe('useSettingsStore', () => {
  it('has default values', () => {
    const state = useSettingsStore.getState()
    expect(state.theme).toBe('system')
    expect(state.language).toBe('en')
  })

  it('sets theme', () => {
    useSettingsStore.getState().setTheme('dark')
    expect(useSettingsStore.getState().theme).toBe('dark')
    useSettingsStore.getState().setTheme('system')
  })

  it('sets language', () => {
    useSettingsStore.getState().setLanguage('zh')
    expect(useSettingsStore.getState().language).toBe('zh')
    useSettingsStore.getState().setLanguage('en')
  })

  it('loads from map', () => {
    useSettingsStore.getState().loadFromMap({ theme: 'light', language: 'zh', safe_mode: 'true' })
    expect(useSettingsStore.getState().theme).toBe('light')
    expect(useSettingsStore.getState().language).toBe('zh')
    useSettingsStore.getState().loadFromMap({ theme: 'system', language: 'en' })
  })

  it('handles missing map keys with defaults', () => {
    useSettingsStore.getState().loadFromMap({})
    expect(useSettingsStore.getState().theme).toBe('system')
    expect(useSettingsStore.getState().language).toBe('en')
  })
})
