import { useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'

export function useKeyboardShortcuts() {
  const navigate = useNavigate()

  useEffect(() => {
    function handler(e: KeyboardEvent) {
      const mod = e.ctrlKey || e.metaKey
      if (!mod) return

      switch (e.key) {
        case ',':
          e.preventDefault()
          navigate({ to: '/settings' })
          break
        case 's':
          e.preventDefault()
          navigate({ to: '/scanner' })
          break
        case 'q':
          e.preventDefault()
          window.close()
          break
      }
    }

    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [navigate])
}
