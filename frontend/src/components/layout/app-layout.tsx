import { ReactNode } from 'react'
import { Link, useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { LayoutDashboard, Search, Settings, HardDrive } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger, TooltipProvider } from '@/components/ui/tooltip'
import { Titlebar } from './titlebar'

export function AppLayout({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const location = useLocation()

  const navItems = [
    { path: '/', icon: LayoutDashboard, label: t('nav.dashboard') },
    { path: '/scanner', icon: Search, label: t('nav.scanner') },
    { path: '/settings', icon: Settings, label: t('nav.settings') },
  ]

  return (
    <TooltipProvider>
      <div className="flex flex-col h-screen bg-background text-foreground">
        <Titlebar />
        <div className="flex flex-1 overflow-hidden">
          <aside className="w-16 flex flex-col items-center py-4 gap-2 border-r border-border bg-card">
            <HardDrive className="w-6 h-6 text-primary mb-4" />
            {navItems.map((item) => {
              const active = location.pathname === item.path
              return (
                <Tooltip key={item.path}>
                  <TooltipTrigger asChild>
                    <Link
                      to={item.path}
                      className={`w-10 h-10 flex items-center justify-center rounded-lg transition-colors ${
                        active
                          ? 'bg-primary/10 text-primary'
                          : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                      }`}
                    >
                      <item.icon className="w-5 h-5" />
                    </Link>
                  </TooltipTrigger>
                  <TooltipContent side="right">{item.label}</TooltipContent>
                </Tooltip>
              )
            })}
          </aside>

          <main className="flex-1 overflow-auto">
            {children}
          </main>
        </div>
      </div>
    </TooltipProvider>
  )
}
