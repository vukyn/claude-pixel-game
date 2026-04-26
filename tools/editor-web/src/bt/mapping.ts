import type { BTNode, FlowEdge, FlowNode } from './types'

export function toGraph(root: BTNode): { nodes: FlowNode[]; edges: FlowEdge[] } {
  const nodes: FlowNode[] = []
  const edges: FlowEdge[] = []
  walk(root, 'root', null, undefined, undefined, nodes, edges)
  return { nodes, edges }
}

function walk(node: BTNode, id: string, parentId: string | null, weight: number | undefined, order: number | undefined,
              nodes: FlowNode[], edges: FlowEdge[]): void {
  const data: Record<string, unknown> = {}
  if (node.type === 'wait') data.seconds = node.seconds
  if (node.type === 'action' || node.type === 'condition') {
    data.name = node.name
    data.args = node.args ?? {}
  }
  nodes.push({ id, type: node.type, data, position: { x: 0, y: 0 } })

  if (parentId) {
    edges.push({
      id: `${parentId}->${id}`,
      source: parentId,
      target: id,
      label: weight !== undefined ? `w${weight}` : undefined,
      data: { weight, order: order ?? 0 },
    })
  }

  if (node.type === 'sequence' || node.type === 'selector') {
    node.children.forEach((child, i) => walk(child, `${id}.children.${i}`, id, undefined, i, nodes, edges))
  } else if (node.type === 'chance') {
    node.branches.forEach((b, i) => walk(b.node, `${id}.branches.${i}.node`, id, b.weight, i, nodes, edges))
  }
}

export function fromGraph(nodes: FlowNode[], edges: FlowEdge[]): BTNode {
  const byId = new Map(nodes.map(n => [n.id, n]))
  const childrenOf = new Map<string, FlowEdge[]>()
  for (const e of edges) {
    const list = childrenOf.get(e.source) ?? []
    list.push(e)
    childrenOf.set(e.source, list)
  }
  for (const list of childrenOf.values()) {
    list.sort((a, b) => (a.data?.order ?? 0) - (b.data?.order ?? 0))
  }
  return rebuild('root', byId, childrenOf)
}

function rebuild(id: string, byId: Map<string, FlowNode>, childrenOf: Map<string, FlowEdge[]>): BTNode {
  const n = byId.get(id)
  if (!n) throw new Error(`fromGraph: missing node ${id}`)
  const childEdges = childrenOf.get(id) ?? []
  switch (n.type) {
    case 'selector':
    case 'sequence':
      return { type: n.type, children: childEdges.map(e => rebuild(e.target, byId, childrenOf)) } as BTNode
    case 'chance':
      return {
        type: 'chance',
        branches: childEdges.map(e => ({
          weight: (e.data?.weight as number) ?? 0,
          node: rebuild(e.target, byId, childrenOf),
        })),
      }
    case 'wait':
      return { type: 'wait', seconds: n.data.seconds as number }
    case 'action':
      return omitEmptyArgs({ type: 'action', name: n.data.name as string, args: n.data.args as Record<string, unknown> })
    case 'condition':
      return omitEmptyArgs({ type: 'condition', name: n.data.name as string, args: n.data.args as Record<string, unknown> })
  }
}

function omitEmptyArgs(node: BTNode): BTNode {
  if ((node.type === 'action' || node.type === 'condition') && node.args && Object.keys(node.args).length === 0) {
    const { args, ...rest } = node
    void args
    return rest as BTNode
  }
  return node
}
