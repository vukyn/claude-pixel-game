import { Handle, Position, type NodeProps } from 'reactflow'
import { useMemo } from 'react'
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from '@/components/ui/context-menu'
import { NodeContextMenu } from '@/components/NodeContextMenu'
import { useNodeMenuApi } from './menuContext'
import { useEditorStore } from '@/state/editorStore'
import { readAtPath } from '@/state/btMutations'
import type { BTNode } from '../types'

function summarizeArgs(name: string, args: Record<string, unknown> | undefined): string {
  if (!args || Object.keys(args).length === 0) return name
  const pairs = Object.entries(args).map(([k, v]) => `${k}=${v}`).join(', ')
  return `${name}(${pairs})`
}

export function ConditionNode({ id, selected, data }: NodeProps) {
  const name = data.name as string
  const args = data.args as Record<string, unknown> | undefined
  const api = useNodeMenuApi()
  const behavior = useEditorStore(s => s.behavior)
  const selectedStateId = useEditorStore(s => s.selectedStateId)
  const liveNode = useMemo<BTNode | null>(() => {
    const st = behavior?.states.find(s => s.id === selectedStateId)
    if (!st?.bt) return null
    return readAtPath(st.bt as BTNode, id)
  }, [behavior, selectedStateId, id])

  const visual = (
    <div
      className={`px-3 py-2 rounded border-2 bg-[#232831] min-w-[160px] ${selected ? 'ring-2 ring-blue-400' : ''}`}
      style={{ borderColor: '#e0c779' }}
    >
      <Handle type="target" position={Position.Left} style={{ background: '#e0c779' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#e0c779]">condition</div>
      <div className="text-xs text-[#b8c0cc]">{summarizeArgs(name, args)}</div>
    </div>
  )

  if (!api || !liveNode) return visual

  return (
    <ContextMenu>
      <ContextMenuTrigger>
        {visual}
      </ContextMenuTrigger>
      <ContextMenuContent>
        <NodeContextMenu
          kind="context"
          node={liveNode}
          isRoot={id === 'root'}
          registry={api.registry}
          actions={{
            onAddChild: (c) => api.onAddChild(id, c),
            onConvert: (t) => api.onConvert(id, t),
            onDelete: () => api.onDelete(id),
          }}
        />
      </ContextMenuContent>
    </ContextMenu>
  )
}
