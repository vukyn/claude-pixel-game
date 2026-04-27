import { useMemo, useState } from 'react'
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  Panel,
  ReactFlowProvider,
  type Edge,
  type Node,
} from 'reactflow'
import 'reactflow/dist/style.css'
import { Hand, MousePointer2 } from 'lucide-react'
import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useEditorStore } from '../state/editorStore'
import { toGraph } from '../bt/mapping'
import { layout } from '../bt/layout'
import type { BTNode } from '../bt/types'
import { SelectorNode } from '../bt/nodes/SelectorNode'
import { SequenceNode } from '../bt/nodes/SequenceNode'
import { ChanceNode } from '../bt/nodes/ChanceNode'
import { ActionNode } from '../bt/nodes/ActionNode'
import { ConditionNode } from '../bt/nodes/ConditionNode'
import { WaitNode } from '../bt/nodes/WaitNode'

const nodeTypes = {
  selector: SelectorNode,
  sequence: SequenceNode,
  chance: ChanceNode,
  action: ActionNode,
  condition: ConditionNode,
  wait: WaitNode,
}

type CanvasMode = 'hand' | 'select'

export function BTCanvas() {
  const behavior = useEditorStore((s) => s.behavior)
  const selectedStateId = useEditorStore((s) => s.selectedStateId)
  const selectNode = useEditorStore((s) => s.selectNode)
  const state = behavior?.states.find((s) => s.id === selectedStateId)
  const [mode, setMode] = useState<CanvasMode>('hand')

  const { nodes, edges } = useMemo(() => {
    if (!state?.bt) return { nodes: [] as Node[], edges: [] as Edge[] }
    const { nodes, edges } = toGraph(state.bt as BTNode)
    const laid = layout(nodes, edges)
    return {
      nodes: laid.map((n) => ({ id: n.id, type: n.type, data: n.data, position: n.position })),
      edges: edges.map((e) => ({
        id: e.id,
        source: e.source,
        target: e.target,
        label: e.label,
        data: e.data,
      })),
    }
  }, [state])

  if (!state)
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>Select a state with a BT</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )
  if (!state.decision)
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>Non-decision state — no BT</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )

  return (
    <ReactFlowProvider>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodeClick={(_, n) => selectNode(n.id)}
        panOnDrag={mode === 'hand'}
        selectionOnDrag={mode === 'select'}
        nodesDraggable
        fitView
      >
        <Background gap={24} />
        <MiniMap />
        <Controls />
        <Panel position="top-right">
          <ToggleGroup
            type="single"
            value={mode}
            onValueChange={(v) => v && setMode(v as CanvasMode)}
            variant="outline"
            size="sm"
          >
            <ToggleGroupItem value="hand" aria-label="Pan (hand)">
              <Hand />
            </ToggleGroupItem>
            <ToggleGroupItem value="select" aria-label="Select (pointer)">
              <MousePointer2 />
            </ToggleGroupItem>
          </ToggleGroup>
        </Panel>
      </ReactFlow>
    </ReactFlowProvider>
  )
}
