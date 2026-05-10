import '@testing-library/jest-dom/vitest'
import { vi } from 'vitest'

// Mock Wails runtime
vi.mock('../../wailsjs/go/main/App', () => ({
  GetSettings: vi.fn(() => Promise.resolve({ theme: 'system', language: 'en', safe_mode: 'false' })),
  GetLLMConfigs: vi.fn(() => Promise.resolve([])),
  GetActiveLLMConfig: vi.fn(() => Promise.resolve(null)),
  GetRecentCleanups: vi.fn(() => Promise.resolve([])),
  GetSystemDrives: vi.fn(() => Promise.resolve([])),
  GetQuickTargets: vi.fn(() => Promise.resolve([])),
  GetCleanableItems: vi.fn(() => Promise.resolve([])),
  GetCleanableSize: vi.fn(() => Promise.resolve(0)),
  SaveLLMConfig: vi.fn(() => Promise.resolve()),
  TestLLMConnection: vi.fn(() => Promise.resolve({ success: true, noFunctionCalling: false })),
  SetSetting: vi.fn(() => Promise.resolve()),
  Scan: vi.fn(() => Promise.resolve({ root: '', totalFiles: 0, totalDirs: 0, totalSize: 0, entries: [], errors: [], durationMs: 0 })),
  SmartScan: vi.fn(() => Promise.resolve()),
  CancelSmartScan: vi.fn(),
  Cleanup: vi.fn(() => Promise.resolve([])),
  AnalyzeFiles: vi.fn(() => Promise.resolve()),
  CancelAnalysis: vi.fn(),
  CheckForUpdate: vi.fn(() => Promise.resolve({ hasUpdate: false })),
  PerformUpdate: vi.fn(() => Promise.resolve()),
  GetSchedules: vi.fn(() => Promise.resolve([])),
  SaveSchedule: vi.fn(() => Promise.resolve()),
  DeleteSchedule: vi.fn(() => Promise.resolve()),
  ToggleSchedule: vi.fn(() => Promise.resolve()),
  GetVersion: vi.fn(() => 'dev'),
  ClearCleanableItems: vi.fn(() => Promise.resolve()),
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(() => vi.fn()),
  EventsEmit: vi.fn(),
  WindowShow: vi.fn(),
  WindowClose: vi.fn(),
}))
