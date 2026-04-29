import { Handle, Position, type NodeProps } from 'reactflow'
import { useMemo } from 'react'
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from '@/components/ui/context-menu'
import { NodeContextMenu } from '@/components/NodeContextMenu'
import { useNodeMenuApi } from './menuContext'
import { useEditorStore } from '@/state/editorStore'
import { readAtPath } from '@/state/btMutations'
import type { BTNode } from '../types'

export function ChanceNode({ id, selected }: NodeProps) {
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
      style={{ borderColor: '#f0a35a' }}
    >
      <Handle type="target" position={Position.Left} style={{ background: '#f0a35a' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#f0a35a]">chance</div>
      <div className="text-xs text-[#b8c0cc]">weighted branches</div>
      <Handle type="source" position={Position.Right} style={{ background: '#f0a35a' }} />
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
