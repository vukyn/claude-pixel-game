import { useMemo } from 'react'
import ReactFlow, { Background, Controls, MiniMap, type Edge, type Node, ReactFlowProvider } from 'reactflow'
import 'reactflow/dist/style.css'
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
  selector: SelectorNode, sequence: SequenceNode, chance: ChanceNode,
  action: ActionNode, condition: ConditionNode, wait: WaitNode,
}

export function BTCanvas() {
  const behavior = useEditorStore(s => s.behavior)
  const selectedStateId = useEditorStore(s => s.selectedStateId)
  const selectNode = useEditorStore(s => s.selectNode)
  const state = behavior?.states.find(s => s.id === selectedStateId)

  const { nodes, edges } = useMemo(() => {
    if (!state?.bt) return { nodes: [] as Node[], edges: [] as Edge[] }
    const { nodes, edges } = toGraph(state.bt as BTNode)
    const laid = layout(nodes, edges)
    return {
      nodes: laid.map(n => ({ id: n.id, type: n.type, data: n.data, position: n.position })),
      edges: edges.map(e => ({ id: e.id, source: e.source, target: e.target, label: e.label, data: e.data })),
    }
  }, [state])

  if (!state) return <div className="flex items-center justify-center h-full text-[#8a93a3]">Select a state with a BT.</div>
  if (!state.decision) return <div className="flex items-center justify-center h-full text-[#8a93a3]">Non-decision state — no BT.</div>

  return (
    <ReactFlowProvider>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodeClick={(_, n) => selectNode(n.id)}
        fitView
      >
        <Background gap={24} color="#2c3340" />
        <MiniMap />
        <Controls />
      </ReactFlow>
    </ReactFlowProvider>
  )
}
