import { describe, it, expect } from 'vitest'
import { validateBehavior } from '../validation'
import type { BehaviorJSON } from '../../api/schemas'
import type { ActionMeta } from '../../api/schemas'

const registry: { actions: ActionMeta[]; conditions: ActionMeta[] } = {
  actions: [
    { name: 'goto', args: [{ name: 'state', type: 'state_id', required: true }] },
    { name: 'flip_facing', args: [] },
    { name: 'set_vx_forward', args: [{ name: 'speed', type: 'float', required: true }] },
  ],
  conditions: [{ name: 'grounded', args: [] }],
}

const ok: BehaviorJSON = {
  kind: 'orc',
  states: [
    { id: 'a', anim: 'idle', decision: false, exit_on: 'anim_done', next: 'a' },
  ],
}

describe('validateBehavior', () => {
  it('passes valid file', () => {
    const r = validateBehavior(ok, 'orc', registry)
    expect(r.valid).toBe(true)
  })
  it('catches kind mismatch', () => {
    const r = validateBehavior({ ...ok, kind: 'slime' }, 'orc', registry)
    expect(r.valid).toBe(false)
    expect(r.errors[0].message).toMatch(/kind/)
  })
  it('catches duplicate state id', () => {
    const dup: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'idle', decision: false, exit_on: 'anim_done', next: 'a' },
      { id: 'a', anim: 'run', decision: false, exit_on: 'anim_done', next: 'a' },
    ]}
    expect(validateBehavior(dup, 'orc', registry).valid).toBe(false)
  })
  it('catches goto to undeclared state', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: true, bt: { type: 'action', name: 'goto', args: { state: 'ghost' } } },
    ]}
    expect(validateBehavior(bad, 'orc', registry).valid).toBe(false)
  })
  it('accepts goto __dead', () => {
    const dead: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'die', decision: true, bt: { type: 'action', name: 'goto', args: { state: '__dead' } } },
    ]}
    expect(validateBehavior(dead, 'orc', registry).valid).toBe(true)
  })
  it('catches unknown action', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: true, bt: { type: 'action', name: 'do_evil' } },
    ]}
    expect(validateBehavior(bad, 'orc', registry).valid).toBe(false)
  })
  it('catches missing required action arg', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: true, bt: { type: 'action', name: 'set_vx_forward', args: {} } },
    ]}
    const r = validateBehavior(bad, 'orc', registry)
    expect(r.valid).toBe(false)
    expect(r.errors[0].message).toMatch(/speed/)
  })
  it('catches chance branch weight <= 0', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: true, bt: { type: 'chance', branches: [
        { weight: 0, node: { type: 'action', name: 'flip_facing' } },
      ]}},
    ]}
    expect(validateBehavior(bad, 'orc', registry).valid).toBe(false)
  })
  it('catches decision state without bt', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: true },
    ]}
    expect(validateBehavior(bad, 'orc', registry).valid).toBe(false)
  })
  it('catches non-decision state without exit_on', () => {
    const bad: BehaviorJSON = { kind: 'orc', states: [
      { id: 'a', anim: 'run', decision: false },
    ]}
    expect(validateBehavior(bad, 'orc', registry).valid).toBe(false)
  })
})
