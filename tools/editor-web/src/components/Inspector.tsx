import { useState } from 'react'
import { useEditorStore } from '../state/editorStore'
import type { BTNode } from '../bt/types'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import JsonView from '@uiw/react-json-view'
import { darkTheme } from '@uiw/react-json-view/dark'
import { Copy, Download, Maximize2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

type Tab = 'node' | 'state' | 'json'

export function Inspector() {
  const [tab, setTab] = useState<Tab>('node')
  const selectedNodePath = useEditorStore(s => s.selectedNodePath)

  return (
    <aside className="w-72 border-l border-border bg-card flex flex-col">
      <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)} className="flex flex-col flex-1 min-h-0">
        <TabsList className="rounded-none border-b border-border bg-transparent p-0 h-auto">
          <TabsTrigger value="node" className="rounded-none">Node</TabsTrigger>
          <TabsTrigger value="state" className="rounded-none">State</TabsTrigger>
          <TabsTrigger value="json" className="rounded-none">JSON</TabsTrigger>
        </TabsList>
        <TabsContent value="node" className="flex-1 overflow-y-auto p-3">
          <NodeInspector path={selectedNodePath} />
        </TabsContent>
        <TabsContent value="state" className="flex-1 overflow-y-auto p-3">
          <StateInspector />
        </TabsContent>
        <TabsContent value="json" className="flex-1 overflow-y-auto p-3">
          <JsonInspector />
        </TabsContent>
      </Tabs>
    </aside>
  )
}

function NodeInspector({ path }: { path: string | null }) {
  const behavior = useEditorStore(s => s.behavior)
  const selectedStateId = useEditorStore(s => s.selectedStateId)
  const registry = useEditorStore(s => s.registry)
  const setBehavior = useEditorStore(s => s.setBehavior)
  if (!path || !behavior || !selectedStateId) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>Click a node to inspect</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )
  }
  const state = behavior.states.find(s => s.id === selectedStateId)
  if (!state?.bt) return null
  const node = getAtPath(state.bt, path) as BTNode | null
  if (!node) return <p className="text-muted-foreground text-xs">Node not found.</p>

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
      <FieldGroup className="gap-3">
        <Field>
          <FieldLabel htmlFor="node-type">type</FieldLabel>
          <Input id="node-type" disabled value={node.type} />
        </Field>
        <Field>
          <FieldLabel htmlFor="node-name">name</FieldLabel>
          <Select value={node.name} onValueChange={v => updateNode({ name: v, args: {} })}>
            <SelectTrigger id="node-name"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {metas.map(m => <SelectItem key={m.name} value={m.name}>{m.name}</SelectItem>)}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        {meta?.args.map(arg => {
          const argId = `node-arg-${arg.name}`
          const value = String((node.args as Record<string, unknown>)?.[arg.name] ?? '')
          const setArg = (v: unknown) =>
            updateNode({ args: { ...(node.args ?? {}), [arg.name]: v } })

          if (arg.type === 'state_id') {
            return (
              <Field key={arg.name}>
                <FieldLabel htmlFor={argId}>{arg.name} (state_id){arg.required ? ' *' : ''}</FieldLabel>
                <Select value={value} onValueChange={setArg}>
                  <SelectTrigger id={argId}><SelectValue placeholder="—" /></SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="__dead">__dead</SelectItem>
                      {behavior.states.map(s => (
                        <SelectItem key={s.id} value={s.id}>{s.id}</SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <p className="text-muted-foreground text-[11px] mt-1">Pick from existing state ids or __dead.</p>
              </Field>
            )
          }

          return (
            <Field key={arg.name}>
              <FieldLabel htmlFor={argId}>{arg.name} ({arg.type}){arg.required ? ' *' : ''}</FieldLabel>
              <Input
                id={argId}
                type={arg.type === 'int' || arg.type === 'float' ? 'number' : 'text'}
                value={value}
                onChange={e => setArg(coerceArg(arg.type, e.target.value))}
              />
            </Field>
          )
        })}
      </FieldGroup>
    )
  }

  if (node.type === 'wait') {
    return (
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor="node-wait-seconds">seconds</FieldLabel>
          <Input id="node-wait-seconds" type="number" step="0.1" value={node.seconds}
            onChange={e => updateNode({ seconds: Number(e.target.value) })} />
        </Field>
      </FieldGroup>
    )
  }

  if (node.type === 'chance') {
    return (
      <FieldGroup className="gap-2">
        {node.branches.map((b, i) => (
          <Field key={i}>
            <FieldLabel htmlFor={`node-branch-${i}`}>branch {i} weight</FieldLabel>
            <Input id={`node-branch-${i}`} type="number" value={b.weight}
              onChange={e => {
                const next = [...node.branches]
                next[i] = { ...b, weight: Number(e.target.value) }
                updateNode({ branches: next })
              }} />
          </Field>
        ))}
      </FieldGroup>
    )
  }

  return <p className="text-muted-foreground text-xs">{node.type} has no editable args; use canvas to add/remove children.</p>
}

function StateInspector() {
  const behavior = useEditorStore(s => s.behavior)
  const selectedStateId = useEditorStore(s => s.selectedStateId)
  const setBehavior = useEditorStore(s => s.setBehavior)
  const state = behavior?.states.find(s => s.id === selectedStateId)
  if (!state || !behavior) return <p className="text-muted-foreground text-xs">No state selected.</p>
  const update = (patch: Partial<typeof state>) => {
    setBehavior({
      ...behavior,
      states: behavior.states.map(s => s.id === state.id ? { ...s, ...patch } : s),
    })
  }
  return (
    <FieldGroup className="gap-3">
      <Field>
        <FieldLabel htmlFor="state-anim">anim</FieldLabel>
        <Input id="state-anim" value={state.anim} onChange={e => update({ anim: e.target.value })} />
      </Field>
      <Field>
        <FieldLabel htmlFor="state-decision">decision</FieldLabel>
        <Checkbox id="state-decision" checked={state.decision} onCheckedChange={(v) => update({ decision: !!v })} />
      </Field>
      {!state.decision && (
        <>
          <Field>
            <FieldLabel htmlFor="state-exit-on">exit_on</FieldLabel>
            <Select value={state.exit_on ?? ''} onValueChange={(v) => update({ exit_on: v })}>
              <SelectTrigger id="state-exit-on"><SelectValue placeholder="—" /></SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value="anim_done">anim_done</SelectItem>
                  <SelectItem value="anim_done_and_grounded">anim_done_and_grounded</SelectItem>
                  <SelectItem value="grounded">grounded</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel htmlFor="state-next">next</FieldLabel>
            <Select value={state.next ?? ''} onValueChange={(v) => update({ next: v })}>
              <SelectTrigger id="state-next"><SelectValue placeholder="—" /></SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value="__dead">__dead</SelectItem>
                  {behavior.states.filter(s => s.id !== state.id).map(s => (
                    <SelectItem key={s.id} value={s.id}>{s.id}</SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
        </>
      )}
    </FieldGroup>
  )
}

function JsonInspector() {
  const behavior = useEditorStore(s => s.behavior)
  const selectedStateId = useEditorStore(s => s.selectedStateId)
  const state = behavior?.states.find(s => s.id === selectedStateId)
  const [expanded, setExpanded] = useState(false)
  if (!state) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>Select a state to view its JSON</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )
  }
  const data = (state.bt ?? state) as object
  const text = JSON.stringify(data)

  const copy = () => navigator.clipboard.writeText(text)
  const download = () => {
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${behavior?.kind ?? 'behavior'}-${state.id}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="flex flex-col gap-2 h-full min-h-0">
      <TooltipProvider>
        <div className="flex items-center justify-end gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" onClick={copy} aria-label="Copy JSON"><Copy /></Button>
            </TooltipTrigger>
            <TooltipContent>Copy</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" onClick={download} aria-label="Download JSON"><Download /></Button>
            </TooltipTrigger>
            <TooltipContent>Download</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" onClick={() => setExpanded(true)} aria-label="Expand JSON"><Maximize2 /></Button>
            </TooltipTrigger>
            <TooltipContent>Expand</TooltipContent>
          </Tooltip>
        </div>
      </TooltipProvider>
      <div className="flex-1 overflow-auto rounded border border-border bg-muted p-2 text-xs">
        <JsonView value={data} style={darkTheme} collapsed={2} displayDataTypes={false} />
      </div>
      <Dialog open={expanded} onOpenChange={setExpanded}>
        <DialogContent className="max-w-3xl max-h-[80vh] overflow-auto">
          <DialogTitle>JSON: {state.id}</DialogTitle>
          <div className="text-xs">
            <JsonView value={data} style={darkTheme} collapsed={false} displayDataTypes={false} />
          </div>
        </DialogContent>
      </Dialog>
    </div>
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
