import { useEditorStore } from '../state/editorStore'

export function StatesPanel() {
  const behavior = useEditorStore(s => s.behavior)
  const selectedStateId = useEditorStore(s => s.selectedStateId)
  const selectState = useEditorStore(s => s.selectState)
  if (!behavior) return null
  return (
    <aside className="w-60 border-r border-[#3a4150] bg-[#232831] flex flex-col">
      <div className="px-3 py-2 text-[11px] uppercase tracking-wide text-[#8a93a3] border-b border-[#3a4150] font-semibold">
        States
      </div>
      <div className="flex-1 overflow-y-auto p-1">
        {behavior.states.map(s => (
          <button
            key={s.id}
            onClick={() => selectState(s.id)}
            className={`w-full text-left px-2 py-2 rounded text-sm flex items-center justify-between ${
              selectedStateId === s.id ? 'bg-[#5aa3f0] text-white' : 'hover:bg-[#2c3340]'
            }`}
          >
            <span>{s.id}</span>
            <span className="flex gap-1">
              {s.decision && <span className="text-[9px] px-1.5 py-0.5 rounded-full border border-[#7ed957] text-[#7ed957]">BT</span>}
            </span>
          </button>
        ))}
      </div>
    </aside>
  )
}
