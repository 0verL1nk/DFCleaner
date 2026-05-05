import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import * as App from '../../wailsjs/go/main/App'

export function useSettings() {
  return useQuery({
    queryKey: ['settings'],
    queryFn: () => App.GetSettings(),
  })
}

export function useLLMConfigs() {
  return useQuery({
    queryKey: ['llm-configs'],
    queryFn: () => App.GetLLMConfigs(),
  })
}

export function useActiveLLMConfig() {
  return useQuery({
    queryKey: ['llm-active'],
    queryFn: () => App.GetActiveLLMConfig(),
  })
}

export function useRecentCleanups(limit = 5) {
  return useQuery({
    queryKey: ['cleanups', limit],
    queryFn: () => App.GetRecentCleanups(limit),
  })
}

export function useSaveLLMConfig() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (config: any) => App.SaveLLMConfig(config),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['llm-configs'] }),
  })
}

export function useTestLLMConnection() {
  return useMutation({
    mutationFn: (config: any) => App.TestLLMConnection(config),
  })
}

export function useSetSetting() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) =>
      App.SetSetting(key, value),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['settings'] }),
  })
}

export function useScan() {
  return useMutation({
    mutationFn: ({ path, opts }: { path: string; opts: any }) =>
      App.Scan(path, opts),
  })
}

export function useCleanup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (items: any[]) => App.Cleanup(items),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['cleanups'] }),
  })
}

export function useAnalyzeFiles() {
  return useMutation({
    mutationFn: (entries: any[]) => App.AnalyzeFiles(entries),
  })
}

export function useSmartScan() {
  return useMutation({
    mutationFn: ({ path, opts }: { path: string; opts: any }) =>
      App.SmartScan(path, opts),
  })
}

export function useCancelSmartScan() {
  return () => App.CancelSmartScan()
}

export function useSystemDrives() {
  return useQuery({
    queryKey: ['system-drives'],
    queryFn: () => App.GetSystemDrives(),
  })
}

export function useQuickTargets() {
  return useQuery({
    queryKey: ['quick-targets'],
    queryFn: () => App.GetQuickTargets(),
  })
}

export function useCleanableItems(scanPath = '') {
  return useQuery({
    queryKey: ['cleanable-items', scanPath],
    queryFn: () => App.GetCleanableItems(scanPath),
  })
}

export function useCleanableSize(scanPath = '') {
  return useQuery({
    queryKey: ['cleanable-size', scanPath],
    queryFn: () => App.GetCleanableSize(scanPath),
  })
}

export function useClearCleanableItems() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (scanPath: string) => App.ClearCleanableItems(scanPath),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['cleanable-items'] }),
  })
}

export function useCheckForUpdate() {
  return useMutation({
    mutationFn: () => App.CheckForUpdate(),
  })
}
