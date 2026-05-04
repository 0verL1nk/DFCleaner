import { Minimize, Maximize2, X, Copy } from 'lucide-react'
import { useState } from 'react'
import {
  WindowMinimise,
  WindowToggleMaximise,
  Quit,
} from '../../../wailsjs/runtime/runtime'

export function Titlebar() {
  const [maximized, setMaximized] = useState(false)

  const handleDoubleClick = () => {
    WindowToggleMaximise()
    setMaximized((v) => !v)
  }

  return (
    <div
      className="flex h-9 select-none items-center justify-between border-b border-border bg-background px-3"
      onDoubleClick={handleDoubleClick}
    >
      <div className="flex items-center gap-2">
        <Copy className="h-4 w-4 text-primary" />
        <span className="text-sm font-medium">DFCleaner</span>
      </div>

      <div className="flex items-center gap-1">
        <button
          onClick={WindowMinimise}
          className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          <Minimize className="h-3.5 w-3.5" />
        </button>
        <button
          onClick={() => {
            WindowToggleMaximise()
            setMaximized((v) => !v)
          }}
          className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          {maximized ? (
            <Maximize2 className="h-3.5 w-3.5" />
          ) : (
            <Maximize2 className="h-3.5 w-3.5" />
          )}
        </button>
        <button
          onClick={Quit}
          className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-destructive hover:text-destructive-foreground"
        >
          <X className="h-4 w-4" />
        </button>
      </div>
    </div>
  )
}
