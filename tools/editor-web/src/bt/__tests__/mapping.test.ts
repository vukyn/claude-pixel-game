import { describe, it, expect } from 'vitest'
import { toGraph, fromGraph } from '../mapping'

const orcRunBT = {
  type: 'sequence' as const,
  children: [
    { type: 'action' as const, name: 'set_vx_forward', args: { speed: 80 } },
    { type: 'wait' as const, seconds: 2 },
    {
      type: 'chance' as const,
      branches: [
        { weight: 50, node: { type: 'action' as const, name: 'flip_facing' } },
        { weight: 50, node: { type: 'action' as const, name: 'stop' } },
      ],
    },
  ],
}

describe('mapping', () => {
  it('toGraph produces nodes with stable ids and edges', () => {
    const { nodes, edges } = toGraph(orcRunBT)
    expect(nodes.length).toBe(6)
    expect(nodes[0].id).toBe('root')
    expect(nodes[0].type).toBe('sequence')
    expect(edges.length).toBeGreaterThan(0)
    const chanceEdges = edges.filter(e => e.label && String(e.label).startsWith('w'))
    expect(chanceEdges.length).toBe(2)
  })

  it('round-trip preserves structure and order', () => {
    const { nodes, edges } = toGraph(orcRunBT)
    const back = fromGraph(nodes, edges)
    expect(back).toEqual(orcRunBT)
  })

  it('handles single action root', () => {
    const single = { type: 'action' as const, name: 'stop' }
    const { nodes, edges } = toGraph(single)
    expect(nodes.length).toBe(1)
    expect(edges.length).toBe(0)
    expect(fromGraph(nodes, edges)).toEqual(single)
  })

  it('handles condition node', () => {
    const cond = {
      type: 'sequence' as const,
      children: [
        { type: 'condition' as const, name: 'grounded' },
        { type: 'action' as const, name: 'stop' },
      ],
    }
    const { nodes, edges } = toGraph(cond)
    expect(fromGraph(nodes, edges)).toEqual(cond)
  })
})
