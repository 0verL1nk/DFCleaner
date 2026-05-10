import { Suspense, lazy } from 'react'
import {
  createRouter,
  createRootRoute,
  createRoute,
  Outlet,
} from '@tanstack/react-router'
import { AppLayout } from '@/components/layout/app-layout'
import { Dashboard } from '@/views/dashboard'
import { Scanner } from '@/views/scanner'
import { Settings } from '@/views/settings'

const Scheduler = lazy(() => import('@/views/scheduler').then(m => ({ default: m.Scheduler })))

function ViewSuspense({ children }: { children: React.ReactNode }) {
  return (
    <Suspense fallback={
      <div className="flex items-center justify-center h-full">
        <div className="animate-pulse text-muted-foreground">Loading...</div>
      </div>
    }>
      {children}
    </Suspense>
  )
}

const rootRoute = createRootRoute({
  component: () => <AppLayout><Outlet /></AppLayout>,
})

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: Dashboard,
})

const scannerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/scanner',
  component: Scanner,
})

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/settings',
  component: Settings,
})

const schedulerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/scheduler',
  component: () => <ViewSuspense><Scheduler /></ViewSuspense>,
})

const routeTree = rootRoute.addChildren([
  dashboardRoute,
  scannerRoute,
  schedulerRoute,
  settingsRoute,
])

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
