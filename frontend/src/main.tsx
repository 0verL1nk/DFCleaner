import React, { useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import './i18n'
import './style.css'
import { router } from './routes/router'
import { useSettingsStore } from './stores/settings'
import { useSettings } from './hooks/wails'
import { Toaster, toast } from './components/ui/sonner'
import * as AppBackend from '../wailsjs/go/main/App'

const queryClient = new QueryClient()

function ThemeSync() {
  const { data } = useSettings()
  const { loadFromMap, theme } = useSettingsStore()

  useEffect(() => {
    if (data) loadFromMap(data)
  }, [data, loadFromMap])

  useEffect(() => {
    const root = document.documentElement
    root.classList.remove('light', 'dark')
    if (theme === 'system') {
      const mq = window.matchMedia('(prefers-color-scheme: dark)')
      root.classList.toggle('dark', mq.matches)
      const handler = (e: MediaQueryListEvent) => root.classList.toggle('dark', e.matches)
      mq.addEventListener('change', handler)
      return () => mq.removeEventListener('change', handler)
    } else {
      root.classList.add(theme)
    }
  }, [theme])

  return null
}

function UpdateChecker() {
  useEffect(() => {
    const timer = setTimeout(async () => {
      try {
        const result = await AppBackend.CheckForUpdate() as any
        if (result?.hasUpdate) {
          toast.info(`New version ${result.latestVer} available!`, {
            description: 'Visit Settings to download.',
            duration: 8000,
          })
        }
      } catch {
        // silently ignore
      }
    }, 3000)
    return () => clearTimeout(timer)
  }, [])

  return null
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeSync />
      <UpdateChecker />
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>
  )
}

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
