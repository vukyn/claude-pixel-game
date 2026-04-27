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
import { Hand, MousePointer2, Plus } from 'lucide-react'
import {
  Empty, EmptyHeader, EmptyTitle,
} from '@/components/ui/empty'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useEditorStore } from '../state/editorStore'
import { toGraph } from '../bt/mapping'
import { layout } from '../bt/layout'
import type { BTNode, BTNodeType } from '../bt/types'
import { addChild, convertAt, deleteAt, setRoot } from '../state/btMutations'
import { NodeContextMenu } from './NodeContextMenu'
import { NodeMenuContext, type NodeMenuApi } from '../bt/nodes/menuContext'
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
  const registry = useEditorStore((s) => s.registry)
  const setBehavior = useEditorStore((s) => s.setBehavior)
  const selectedStateId = useEditorStore((s) => s.selectedStateId)
  const selectNode = useEditorStore((s) => s.selectNode)
  const state = behavior?.states.find((s) => s.id === selectedStateId)
  const [mode, setMode] = useState<CanvasMode>('hand')

  const updateBT = (nextBT: BTNode | null) => {
    if (!behavior || !selectedStateId) return
    setBehavior({
      ...behavior,
      states: behavior.states.map(s =>
        s.id === selectedStateId ? { ...s, bt: nextBT ?? undefined } : s
      ),
    })
  }

  const api: NodeMenuApi = useMemo(() => ({
    registry,
    onAddChild: (path, child) => {
      if (!state?.bt) return
      updateBT(addChild(state.bt as BTNode, path, child))
    },
    onConvert: (path, toType) => {
      if (!state?.bt) return
      updateBT(convertAt(state.bt as BTNode, path, toType))
    },
    onDelete: (path) => {
      if (!state?.bt) return
      if (path === 'root') return
      updateBT(deleteAt(state.bt as BTNode, path))
    },
  }), [state, behavior, registry])

  const onSetRoot = (rootType: BTNodeType, opts?: { name?: string }) => {
    updateBT(setRoot(rootType, opts))
  }

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

  if (!state.bt) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>No BT for this state</EmptyTitle>
        </EmptyHeader>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button size="sm" variant="default" className="mt-3">
              <Plus className="size-4" /> Add root
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <NodeContextMenu
              kind="dropdown"
              registry={registry}
              actions={{ onSetRoot }}
            />
          </DropdownMenuContent>
        </DropdownMenu>
      </Empty>
    )
  }

  return (
    <NodeMenuContext.Provider value={api}>
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
          className={mode === 'select' ? '[&_.react-flow__pane]:!cursor-default' : ''}
        >
          <Background gap={24} />
          <MiniMap pannable zoomable style={{ width: 140, height: 90 }} />
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
    </NodeMenuContext.Provider>
  )
}
