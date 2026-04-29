import { useMemo } from 'react'
import ReactFlow, {
  Background, Controls, MiniMap, ReactFlowProvider,
  Handle, Position,
  MarkerType,
  type Edge, type Node,
} from 'reactflow'
import 'reactflow/dist/style.css'
import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { useEditorStore } from '../state/editorStore'
import { extractTransitions } from '../bt/transitions'
import { layout } from '../bt/layout'
import type { FlowEdge } from '../bt/types'

interface Props {
  onJumpToState?(): void
}

// Handles are invisible but required for React Flow to anchor edges correctly (LR layout).
const invisibleHandle = { background: 'transparent', border: 'none', width: 0, height: 0 }

function StateNode({ data }: { data: { label: string; isInitial: boolean; isDead: boolean } }) {
  const cls = data.isDead
    ? 'rounded-full bg-muted text-muted-foreground border border-border px-3 py-1 text-xs'
    : `rounded-md bg-card border ${data.isInitial ? 'border-emerald-500' : 'border-border'} px-3 py-2 text-sm shadow`
  return (
    <div className={cls}>
      <Handle type="target" position={Position.Left} style={invisibleHandle} />
      {data.label}
      <Handle type="source" position={Position.Right} style={invisibleHandle} />
    </div>
  )
}
const nodeTypes = { state: StateNode }

export function TransitionsCanvas({ onJumpToState }: Props) {
  const behavior = useEditorStore(s => s.behavior)
  const registry = useEditorStore(s => s.registry)
  const selectState = useEditorStore(s => s.selectState)

  const { nodes, edges } = useMemo(() => {
    if (!behavior) return { nodes: [] as Node[], edges: [] as Edge[] }
    const g = extractTransitions(behavior, registry)
    const flowNodes = g.nodes.map(n => ({
      id: n.id,
      type: 'state' as const,
      data: { label: n.id, isInitial: n.isInitial, isDead: n.id === '__dead' },
      position: { x: 0, y: 0 },
    }))
    const flowEdges: FlowEdge[] = g.edges.map(e => ({
      id: e.id,
      source: e.from,
      target: e.to,
      label: e.label || undefined,
      data: { order: 0 },
    }))
    const laid = layout(flowNodes, flowEdges, { direction: 'LR' })
    const rfNodes: Node[] = laid.map(n => ({ id: n.id, type: n.type, data: n.data, position: n.position }))
    const rfEdges: Edge[] = g.edges.map(e => ({
      id: e.id,
      source: e.from,
      target: e.to,
      label: e.label || undefined,
      animated: false,
      markerEnd: { type: MarkerType.ArrowClosed },
      style:
        e.kind === 'next'
          ? { strokeDasharray: '4 2', stroke: 'var(--muted-foreground)' }
          : { stroke: '#10b981' },
    }))
    return { nodes: rfNodes, edges: rfEdges }
  }, [behavior, registry])

  if (!behavior) {
    return (
      <Empty>
        <EmptyHeader><EmptyTitle>Open a behavior to see transitions</EmptyTitle></EmptyHeader>
      </Empty>
    )
  }
  if (behavior.states.length === 0) {
    return (
      <Empty>
        <EmptyHeader><EmptyTitle>No states defined</EmptyTitle></EmptyHeader>
      </Empty>
    )
  }

  return (
    <ReactFlowProvider>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable
        fitView
        onNodeClick={(_, n) => {
          if (n.id === '__dead') return
          selectState(n.id)
          onJumpToState?.()
        }}
      >
        <Background gap={24} />
        <MiniMap pannable zoomable style={{ width: 140, height: 90 }} />
        <Controls />
      </ReactFlow>
    </ReactFlowProvider>
  )
}
