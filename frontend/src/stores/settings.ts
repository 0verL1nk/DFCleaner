import { create } from 'zustand'

interface SettingsState {
  theme: 'light' | 'dark' | 'system'
  language: 'en' | 'zh'
  contentPreview: boolean

  setTheme: (v: 'light' | 'dark' | 'system') => void
  setLanguage: (v: 'en' | 'zh') => void
  setContentPreview: (v: boolean) => void
  loadFromMap: (m: Record<string, string>) => void
}

export const useSettingsStore = create<SettingsState>((set) => ({
  theme: 'system',
  language: 'en',
  contentPreview: false,

  setTheme: (v) => set({ theme: v }),
  setLanguage: (v) => set({ language: v }),
  setContentPreview: (v) => set({ contentPreview: v }),
  loadFromMap: (m) => set({
    theme: (m.theme as 'light' | 'dark' | 'system') || 'system',
    language: (m.language as 'en' | 'zh') || 'en',
    contentPreview: m.content_preview === 'true',
  }),
}))
