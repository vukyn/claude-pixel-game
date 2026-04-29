import dagre from 'dagre'
import type { FlowEdge } from './types'

const NODE_W = 180
const NODE_H = 64

export interface LayoutOpts {
  direction?: 'TB' | 'LR'
}

interface Positionable {
  id: string
  position: { x: number; y: number }
}

export function layout<N extends Positionable>(nodes: N[], edges: FlowEdge[], opts: LayoutOpts = {}): N[] {
  const g = new dagre.graphlib.Graph()
  g.setGraph({ rankdir: opts.direction ?? 'LR', nodesep: 40, ranksep: 80 })
  g.setDefaultEdgeLabel(() => ({}))
  for (const n of nodes) g.setNode(n.id, { width: NODE_W, height: NODE_H })
  for (const e of edges) g.setEdge(e.source, e.target)
  dagre.layout(g)
  return nodes.map(n => {
    const pos = g.node(n.id)
    return { ...n, position: { x: pos.x - NODE_W / 2, y: pos.y - NODE_H / 2 } }
  })
}
