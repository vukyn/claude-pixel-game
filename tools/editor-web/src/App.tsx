import { useEffect } from 'react'
import { TopBar } from './components/TopBar'
import { StatesPanel } from './components/StatesPanel'
import { CanvasArea } from './components/CanvasArea'
import { Inspector } from './components/Inspector'
import { TuningDrawer } from './components/TuningDrawer'
import { useEditorStore } from './state/editorStore'

export default function App() {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const target = e.target as HTMLElement | null
      if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) return
      const meta = e.metaKey || e.ctrlKey
      if (!meta) return
      const k = e.key.toLowerCase()
      if (k === 'z' && !e.shiftKey) {
        if (useEditorStore.temporal.getState().pastStates.length > 0) {
          e.preventDefault()
          useEditorStore.temporal.getState().undo()
        }
      } else if ((k === 'z' && e.shiftKey) || k === 'y') {
        if (useEditorStore.temporal.getState().futureStates.length > 0) {
          e.preventDefault()
          useEditorStore.temporal.getState().redo()
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div className="h-screen flex flex-col bg-[#1a1d23]">
      <TopBar />
      <TuningDrawer />
      <main className="flex-1 flex min-h-0">
        <StatesPanel />
        <div className="flex-1 min-w-0 flex flex-col"><CanvasArea /></div>
        <Inspector />
      </main>
    </div>
  )
}
