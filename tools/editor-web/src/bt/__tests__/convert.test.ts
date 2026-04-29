import { describe, it, expect } from 'vitest'
import { canConvert, convertNode } from '../convert'
import type { BTNode } from '../types'

describe('canConvert', () => {
  it('allows selector ↔ sequence', () => {
    const sel: BTNode = { type: 'selector', children: [] }
    const seq: BTNode = { type: 'sequence', children: [] }
    expect(canConvert(sel, 'sequence').ok).toBe(true)
    expect(canConvert(seq, 'selector').ok).toBe(true)
  })

  it('allows composite ↔ chance', () => {
    const sel: BTNode = { type: 'selector', children: [] }
    const ch: BTNode = { type: 'chance', branches: [] }
    expect(canConvert(sel, 'chance').ok).toBe(true)
    expect(canConvert(ch, 'selector').ok).toBe(true)
  })

  it('allows action ↔ condition ↔ wait', () => {
    const a: BTNode = { type: 'action', name: 'x', args: {} }
    const c: BTNode = { type: 'condition', name: 'y', args: {} }
    const w: BTNode = { type: 'wait', seconds: 1 }
    expect(canConvert(a, 'condition').ok).toBe(true)
    expect(canConvert(c, 'wait').ok).toBe(true)
    expect(canConvert(w, 'action').ok).toBe(true)
  })

  it('allows leaf → composite', () => {
    const a: BTNode = { type: 'action', name: 'x' }
    expect(canConvert(a, 'selector').ok).toBe(true)
    expect(canConvert(a, 'chance').ok).toBe(true)
  })

  it('blocks composite → leaf when non-empty', () => {
    const sel: BTNode = { type: 'selector', children: [{ type: 'action', name: 'x' }] }
    const r = canConvert(sel, 'action')
    expect(r.ok).toBe(false)
    expect(r.reason).toMatch(/delete children first/i)
  })

  it('allows composite → leaf when empty', () => {
    const sel: BTNode = { type: 'selector', children: [] }
    expect(canConvert(sel, 'action').ok).toBe(true)
  })

  it('rejects same-type convert', () => {
    const sel: BTNode = { type: 'selector', children: [] }
    expect(canConvert(sel, 'selector').ok).toBe(false)
  })
})

describe('convertNode', () => {
  it('selector → sequence preserves children', () => {
    const sel: BTNode = { type: 'selector', children: [{ type: 'action', name: 'x' }] }
    const out = convertNode(sel, 'sequence')
    expect(out).toEqual({ type: 'sequence', children: [{ type: 'action', name: 'x' }] })
  })

  it('sequence → chance wraps children with weight=1', () => {
    const seq: BTNode = { type: 'sequence', children: [{ type: 'action', name: 'x' }, { type: 'wait', seconds: 1 }] }
    const out = convertNode(seq, 'chance')
    expect(out).toEqual({
      type: 'chance',
      branches: [
        { weight: 1, node: { type: 'action', name: 'x' } },
        { weight: 1, node: { type: 'wait', seconds: 1 } },
      ],
    })
  })

  it('chance → selector unwraps branches', () => {
    const ch: BTNode = {
      type: 'chance',
      branches: [
        { weight: 5, node: { type: 'action', name: 'x' } },
        { weight: 7, node: { type: 'action', name: 'y' } },
      ],
    }
    const out = convertNode(ch, 'selector')
    expect(out).toEqual({
      type: 'selector',
      children: [{ type: 'action', name: 'x' }, { type: 'action', name: 'y' }],
    })
  })

  it('action → condition resets name and args', () => {
    const a: BTNode = { type: 'action', name: 'set_vx', args: { speed: 80 } }
    const out = convertNode(a, 'condition')
    expect(out).toEqual({ type: 'condition', name: '', args: {} })
  })

  it('action → wait sets seconds=1', () => {
    const a: BTNode = { type: 'action', name: 'x' }
    expect(convertNode(a, 'wait')).toEqual({ type: 'wait', seconds: 1 })
  })

  it('leaf → composite produces empty children', () => {
    const a: BTNode = { type: 'action', name: 'x' }
    expect(convertNode(a, 'selector')).toEqual({ type: 'selector', children: [] })
    expect(convertNode(a, 'chance')).toEqual({ type: 'chance', branches: [] })
  })

  it('throws when canConvert is false', () => {
    const sel: BTNode = { type: 'selector', children: [{ type: 'action', name: 'x' }] }
    expect(() => convertNode(sel, 'action')).toThrow()
  })
})
