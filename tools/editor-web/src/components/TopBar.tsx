import { useEffect, useState } from 'react'
import { Loader2, Save, Undo2, Redo2 } from 'lucide-react'
import { useStore } from 'zustand'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useEditorStore } from '../state/editorStore'
import { listBehaviors } from '../api/client'

export function TopBar() {
  const currentKind = useEditorStore(s => s.currentKind)
  const dirty = useEditorStore(s => s.dirty)
  const validation = useEditorStore(s => s.validation)
  const load = useEditorStore(s => s.load)
  const save = useEditorStore(s => s.save)
  const past = useStore(useEditorStore.temporal, t => t.pastStates.length)
  const future = useStore(useEditorStore.temporal, t => t.futureStates.length)
  const [kinds, setKinds] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    listBehaviors()
      .then(refs => setKinds(refs.map(r => r.kind)))
      .catch(e => setError(String(e)))
  }, [])

  const handleSave = async () => {
    setSaving(true); setError(null)
    try { await save() } catch (e) { setError(String(e)) }
    finally { setSaving(false) }
  }

  return (
    <header className="h-11 px-4 border-b border-border bg-card flex items-center gap-3">
      <span className="font-semibold text-primary">⚙ Behavior Editor</span>
      <Select value={currentKind ?? ''} onValueChange={v => load(v)}>
        <SelectTrigger className="w-44 h-8 text-sm">
          <SelectValue placeholder="— pick file —" />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {kinds.map(k => <SelectItem key={k} value={k}>{k}</SelectItem>)}
          </SelectGroup>
        </SelectContent>
      </Select>

      <Button
        size="sm" variant="ghost"
        disabled={past === 0}
        aria-label="Undo (Cmd+Z)"
        onClick={() => useEditorStore.temporal.getState().undo()}
      >
        <Undo2 />
      </Button>
      <Button
        size="sm" variant="ghost"
        disabled={future === 0}
        aria-label="Redo (Cmd+Shift+Z)"
        onClick={() => useEditorStore.temporal.getState().redo()}
      >
        <Redo2 />
      </Button>

      {dirty && <Badge variant="outline" className="text-amber-500 border-amber-500/40">● unsaved</Badge>}
      <span className="flex-1" />
      {error && <span className="text-destructive text-xs">{error}</span>}
      {!validation.valid && <Badge variant="destructive">✗ {validation.errors.length} validation errors</Badge>}
      <Button onClick={handleSave} disabled={!dirty || saving || !validation.valid} size="sm">
        {saving ? <><Loader2 data-icon="inline-start" className="animate-spin" />Saving</> : <><Save data-icon="inline-start" />Save</>}
      </Button>
    </header>
  )
}
