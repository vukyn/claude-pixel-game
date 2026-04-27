import { describe, it, expect } from 'vitest'
import { extractTransitions } from '../transitions'
import type { BehaviorJSON, ActionMeta } from '../../api/schemas'

const reg: { actions: ActionMeta[] } = {
  actions: [
    { name: 'goto', args: [{ name: 'state', type: 'state_id', required: true }] },
    { name: 'set_vx_forward', args: [{ name: 'speed', type: 'float', required: true }] },
    { name: 'flip_facing', args: [] },
  ],
}

describe('extractTransitions', () => {
  it('emits next edges for non-decision states', () => {
    const b: BehaviorJSON = {
      kind: 'k',
      states: [
        { id: 'a', anim: 'a', decision: false, exit_on: 'anim_done', next: 'b' },
        { id: 'b', anim: 'b', decision: false, exit_on: 'anim_done', next: '__dead' },
      ],
    } as BehaviorJSON
    const g = extractTransitions(b, reg)
    expect(g.nodes.map(n => n.id)).toEqual(['a', 'b', '__dead'])
    expect(g.nodes[0].isInitial).toBe(true)
    expect(g.nodes[2].isInitial).toBe(false)
    expect(g.edges.filter(e => e.kind === 'next')).toEqual([
      { id: expect.any(String), from: 'a', to: 'b', kind: 'next', label: 'anim_done' },
      { id: expect.any(String), from: 'b', to: '__dead', kind: 'next', label: 'anim_done' },
    ])
  })

  it('emits goto edges from BT goto actions', () => {
    const b: BehaviorJSON = {
      kind: 'k',
      states: [
        {
          id: 'run',
          anim: 'run',
          decision: true,
          bt: {
            type: 'sequence',
            children: [
              { type: 'action', name: 'set_vx_forward', args: { speed: 80 } },
              { type: 'action', name: 'goto', args: { state: 'attack' } },
            ],
          },
        },
        { id: 'attack', anim: 'a', decision: false, exit_on: 'anim_done', next: 'run' },
      ],
    } as BehaviorJSON
    const g = extractTransitions(b, reg)
    const gotoEdges = g.edges.filter(e => e.kind === 'goto')
    expect(gotoEdges).toHaveLength(1)
    expect(gotoEdges[0]).toMatchObject({ from: 'run', to: 'attack', kind: 'goto' })
  })

  it('walks nested chance branches', () => {
    const b: BehaviorJSON = {
      kind: 'k',
      states: [
        {
          id: 'run',
          anim: 'run',
          decision: true,
          bt: {
            type: 'chance',
            branches: [
              { weight: 1, node: { type: 'action', name: 'goto', args: { state: 'a' } } },
              { weight: 1, node: { type: 'action', name: 'goto', args: { state: 'b' } } },
            ],
          },
        },
        { id: 'a', anim: 'x', decision: false, exit_on: 'anim_done', next: 'run' },
        { id: 'b', anim: 'x', decision: false, exit_on: 'anim_done', next: 'run' },
      ],
    } as BehaviorJSON
    const g = extractTransitions(b, reg)
    expect(g.edges.filter(e => e.kind === 'goto')).toHaveLength(2)
  })

  it('skips goto without state arg', () => {
    const b: BehaviorJSON = {
      kind: 'k',
      states: [
        {
          id: 'run',
          anim: 'run',
          decision: true,
          bt: { type: 'action', name: 'goto', args: {} },
        },
      ],
    } as BehaviorJSON
    const g = extractTransitions(b, reg)
    expect(g.edges).toHaveLength(0)
  })

  it('synthesizes __dead node only when referenced', () => {
    const b: BehaviorJSON = {
      kind: 'k',
      states: [{ id: 'a', anim: 'a', decision: false, exit_on: 'anim_done', next: 'a' }],
    } as BehaviorJSON
    const g = extractTransitions(b, reg)
    expect(g.nodes.map(n => n.id)).toEqual(['a'])
  })

  it('marks first state as initial', () => {
    const b: BehaviorJSON = {
      kind: 'k',
      states: [
        { id: 'first', anim: 'a', decision: false, exit_on: 'anim_done', next: 'second' },
        { id: 'second', anim: 'a', decision: false, exit_on: 'anim_done', next: 'first' },
      ],
    } as BehaviorJSON
    const g = extractTransitions(b, reg)
    expect(g.nodes[0].isInitial).toBe(true)
    expect(g.nodes[1].isInitial).toBe(false)
  })
})
