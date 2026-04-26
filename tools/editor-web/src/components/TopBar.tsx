import { useEffect, useState } from 'react'
import { useEditorStore } from '../state/editorStore'
import { listBehaviors } from '../api/client'

export function TopBar() {
  const currentKind = useEditorStore(s => s.currentKind)
  const dirty = useEditorStore(s => s.dirty)
  const validation = useEditorStore(s => s.validation)
  const load = useEditorStore(s => s.load)
  const save = useEditorStore(s => s.save)
  const [kinds, setKinds] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => { listBehaviors().then(refs => setKinds(refs.map(r => r.kind))).catch(e => setError(String(e))) }, [])

  const handleSave = async () => {
    setSaving(true); setError(null)
    try { await save() } catch (e) { setError(String(e)) }
    finally { setSaving(false) }
  }

  return (
    <header className="h-11 px-4 bg-[#232831] border-b border-[#3a4150] flex items-center gap-3">
      <span className="font-semibold text-[#5aa3f0]">⚙ Behavior Editor</span>
      <select className="bg-[#2c3340] text-[#e6e9ef] border border-[#3a4150] rounded px-2 py-1 text-sm"
              value={currentKind ?? ''} onChange={e => load(e.target.value)}>
        <option value="">— pick file —</option>
        {kinds.map(k => <option key={k}>{k}</option>)}
      </select>
      {dirty && <span className="text-[#f0a35a] text-xs">● unsaved changes</span>}
      <span className="flex-1" />
      {error && <span className="text-red-400 text-xs">{error}</span>}
      {!validation.valid && <span className="text-red-400 text-xs">✗ {validation.errors.length} validation errors</span>}
      <button onClick={handleSave} disabled={!dirty || saving || !validation.valid}
        className="bg-[#5aa3f0] disabled:bg-[#3a4150] text-white px-3 py-1 rounded text-sm">
        {saving ? 'Saving…' : 'Save'}
      </button>
    </header>
  )
}
