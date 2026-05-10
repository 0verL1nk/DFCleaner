import { ChevronRight, Home } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ScrollArea, ScrollBar } from '@/components/ui/scroll-area'

interface BreadcrumbNavProps {
  stack: { path: string; label: string }[]
  onNavigate: (path: string) => void
}

export function BreadcrumbNav({ stack, onNavigate }: BreadcrumbNavProps) {
  return (
    <div className="border-b border-border bg-muted/30 shrink-0">
      <ScrollArea className="w-full">
        <nav className="flex items-center gap-1 px-4 py-2 text-sm">
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6"
            onClick={() => onNavigate(stack[0].path)}
          >
            <Home className="w-3.5 h-3.5" />
          </Button>
          {stack.map((item, idx) => (
            <span key={item.path} className="flex items-center gap-1">
              <ChevronRight className="w-3 h-3 text-muted-foreground" />
              <Button
                variant="ghost"
                size="sm"
                className={`h-6 px-2 text-xs truncate max-w-[120px] ${
                  idx === stack.length - 1 ? 'font-medium' : 'text-muted-foreground'
                }`}
                onClick={() => onNavigate(item.path)}
              >
                {item.label}
              </Button>
            </span>
          ))}
        </nav>
        <ScrollBar orientation="horizontal" />
      </ScrollArea>
    </div>
  )
}
