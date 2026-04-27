import { describe, it, expect } from 'vitest'
import { layout } from '../layout'
import { toGraph } from '../mapping'

const seq = {
  type: 'sequence' as const,
  children: [
    { type: 'action' as const, name: 'stop' },
    { type: 'action' as const, name: 'flip_facing' },
  ],
}

describe('layout', () => {
  it('assigns x/y to every node', () => {
    const { nodes, edges } = toGraph(seq)
    const laid = layout(nodes, edges)
    for (const n of laid) {
      expect(typeof n.position.x).toBe('number')
      expect(typeof n.position.y).toBe('number')
    }
  })
  it('root x is leftmost (LR layout)', () => {
    const { nodes, edges } = toGraph(seq)
    const laid = layout(nodes, edges)
    const root = laid.find(n => n.id === 'root')!
    const others = laid.filter(n => n.id !== 'root')
    for (const o of others) expect(o.position.x).toBeGreaterThanOrEqual(root.position.x)
  })
  it('supports rankdir override', () => {
    const nodes = [
      { id: 'root', type: 'selector' as const, data: {}, position: { x: 0, y: 0 } },
      { id: 'a', type: 'action' as const, data: {}, position: { x: 0, y: 0 } },
    ]
    const edges = [{ id: 'e', source: 'root', target: 'a', data: { order: 0 } }]
    const horizontal = layout(nodes, edges, { direction: 'LR' })
    const vertical   = layout(nodes, edges, { direction: 'TB' })
    expect(horizontal[1].position.x).toBeGreaterThan(horizontal[0].position.x)
    expect(vertical[1].position.y).toBeGreaterThan(vertical[0].position.y)
  })
})
