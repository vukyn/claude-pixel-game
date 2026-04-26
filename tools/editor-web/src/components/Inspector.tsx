import { useState } from 'react'
import { useEditorStore } from '../state/editorStore'
import type { BTNode } from '../bt/types'

type Tab = 'node' | 'state' | 'json'

export function Inspector() {
  const [tab, setTab] = useState<Tab>('node')
  const behavior = useEditorStore(s => s.behavior)
  const selectedStateId = useEditorStore(s => s.selectedStateId)
  const selectedNodePath = useEditorStore(s => s.selectedNodePath)
  const state = behavior?.states.find(s => s.id === selectedStateId)

  return (
    <aside className="w-72 border-l border-[#3a4150] bg-[#232831] flex flex-col">
      <div className="flex border-b border-[#3a4150]">
        {(['node', 'state', 'json'] as Tab[]).map(t => (
          <button key={t} onClick={() => setTab(t)}
            className={`flex-1 px-3 py-2.5 text-xs uppercase tracking-wide border-b-2 ${
              tab === t ? 'border-[#5aa3f0] text-white' : 'border-transparent text-[#8a93a3]'
            }`}>{t}</button>
        ))}
      </div>
      <div className="flex-1 overflow-y-auto p-3 text-sm">
        {tab === 'node' && <NodeInspector path={selectedNodePath} />}
        {tab === 'state' && <StateInspector />}
        {tab === 'json' && state && (
          <pre className="text-xs whitespace-pre-wrap bg-[#1a1d23] p-3 rounded border border-[#3a4150]">
            {JSON.stringify(state.bt ?? state, null, 2)}
          </pre>
        )}
      </div>
    </aside>
  )
}

function NodeInspector({ path }: { path: string | null }) {
  const behavior = useEditorStore(s => s.behavior)
  const selectedStateId = useEditorStore(s => s.selectedStateId)
  const registry = useEditorStore(s => s.registry)
  const setBehavior = useEditorStore(s => s.setBehavior)
  if (!path || !behavior || !selectedStateId) return <p className="text-[#8a93a3] text-xs">Click a node to inspect.</p>
  const state = behavior.states.find(s => s.id === selectedStateId)
  if (!state?.bt) return null
  const node = getAtPath(state.bt, path) as BTNode | null
  if (!node) return <p className="text-[#8a93a3] text-xs">Node not found.</p>

  const updateNode = (patch: Record<string, unknown>) => {
    const newBT = setAtPath(state.bt as Record<string, unknown>, path, { ...node, ...patch })
    setBehavior({
      ...behavior,
      states: behavior.states.map(s => s.id === selectedStateId ? { ...s, bt: newBT } : s),
    })
  }

  if (node.type === 'action' || node.type === 'condition') {
    const metas = node.type === 'action' ? registry.actions : registry.conditions
    const meta = metas.find(m => m.name === node.name)
    return (
      <div className="space-y-3">
        <Field label="type"><input className="input" disabled value={node.type} /></Field>
        <Field label="name">
          <select className="input" value={node.name}
            onChange={e => updateNode({ name: e.target.value, args: {} })}>
            {metas.map(m => <option key={m.name}>{m.name}</option>)}
          </select>
        </Field>
        {meta?.args.map(arg => (
          <Field key={arg.name} label={`${arg.name} (${arg.type})${arg.required ? ' *' : ''}`}>
            <input className="input"
              type={arg.type === 'int' || arg.type === 'float' ? 'number' : 'text'}
              value={String((node.args as Record<string, unknown>)?.[arg.name] ?? '')}
              onChange={e => updateNode({ args: { ...(node.args ?? {}), [arg.name]: coerceArg(arg.type, e.target.value) } })}
            />
          </Field>
        ))}
      </div>
    )
  }

  if (node.type === 'wait') {
    return (
      <Field label="seconds">
        <input className="input" type="number" step="0.1" value={node.seconds}
               onChange={e => updateNode({ seconds: Number(e.target.value) })} />
      </Field>
    )
  }

  if (node.type === 'chance') {
    return (
      <div className="space-y-2">
        {node.branches.map((b, i) => (
          <Field key={i} label={`branch ${i} weight`}>
            <input className="input" type="number" value={b.weight}
              onChange={e => {
                const next = [...node.branches]
                next[i] = { ...b, weight: Number(e.target.value) }
                updateNode({ branches: next })
              }} />
          </Field>
        ))}
      </div>
    )
  }

  return <p className="text-[#8a93a3] text-xs">{node.type} has no editable args; use canvas to add/remove children.</p>
}

function StateInspector() {
  const behavior = useEditorStore(s => s.behavior)
  const selectedStateId = useEditorStore(s => s.selectedStateId)
  const setBehavior = useEditorStore(s => s.setBehavior)
  const state = behavior?.states.find(s => s.id === selectedStateId)
  if (!state || !behavior) return <p className="text-[#8a93a3] text-xs">No state selected.</p>
  const update = (patch: Partial<typeof state>) => {
    setBehavior({
      ...behavior,
      states: behavior.states.map(s => s.id === state.id ? { ...s, ...patch } : s),
    })
  }
  return (
    <div className="space-y-3">
      <Field label="anim">
        <input className="input" value={state.anim} onChange={e => update({ anim: e.target.value })} />
      </Field>
      <Field label="decision">
        <input type="checkbox" checked={state.decision} onChange={e => update({ decision: e.target.checked })} />
      </Field>
      {!state.decision && (
        <>
          <Field label="exit_on">
            <select className="input" value={state.exit_on ?? ''} onChange={e => update({ exit_on: e.target.value })}>
              <option value="">—</option>
              <option value="anim_done">anim_done</option>
              <option value="anim_done_and_grounded">anim_done_and_grounded</option>
              <option value="grounded">grounded</option>
            </select>
          </Field>
          <Field label="next">
            <select className="input" value={state.next ?? ''} onChange={e => update({ next: e.target.value })}>
              <option value="">—</option>
              <option value="__dead">__dead</option>
              {behavior.states.filter(s => s.id !== state.id).map(s => <option key={s.id} value={s.id}>{s.id}</option>)}
            </select>
          </Field>
        </>
      )}
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="block text-[11px] uppercase tracking-wide text-[#8a93a3] mb-1">{label}</span>
      {children}
      <style>{`.input { width: 100%; background: #2c3340; color: #e6e9ef; border: 1px solid #3a4150; border-radius: 4px; padding: 4px 6px; font-size: 13px; font-family: ui-monospace, monospace; }`}</style>
    </label>
  )
}

function coerceArg(type: string, raw: string): unknown {
  if (type === 'int') return parseInt(raw, 10)
  if (type === 'float') return parseFloat(raw)
  return raw
}

function getAtPath(root: unknown, path: string): unknown {
  if (path === 'root') return root
  const parts = path.split('.').slice(1)
  let cur: any = root
  for (let i = 0; i < parts.length; i++) cur = cur?.[parts[i]]
  return cur
}

function setAtPath(root: Record<string, unknown>, path: string, value: unknown): Record<string, unknown> {
  if (path === 'root') return value as Record<string, unknown>
  const parts = path.split('.').slice(1)
  const clone = JSON.parse(JSON.stringify(root))
  let cur: any = clone
  for (let i = 0; i < parts.length - 1; i++) cur = cur[parts[i]]
  cur[parts[parts.length - 1]] = value
  return clone
}
