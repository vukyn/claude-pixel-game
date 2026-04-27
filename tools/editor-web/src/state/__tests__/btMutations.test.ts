import { describe, it, expect } from 'vitest'
import { addChild, deleteAt, convertAt, setRoot } from '../btMutations'
import type { BTNode } from '../../bt/types'

describe('addChild', () => {
  it('appends to selector children', () => {
    const root: BTNode = { type: 'selector', children: [{ type: 'action', name: 'a' }] }
    const out = addChild(root, 'root', { type: 'wait', seconds: 1 })
    expect(out).toEqual({
      type: 'selector',
      children: [{ type: 'action', name: 'a' }, { type: 'wait', seconds: 1 }],
    })
  })

  it('appends to chance branches with weight=1', () => {
    const root: BTNode = { type: 'chance', branches: [{ weight: 5, node: { type: 'action', name: 'a' } }] }
    const out = addChild(root, 'root', { type: 'action', name: 'b' })
    expect(out).toEqual({
      type: 'chance',
      branches: [
        { weight: 5, node: { type: 'action', name: 'a' } },
        { weight: 1, node: { type: 'action', name: 'b' } },
      ],
    })
  })

  it('appends to nested composite by path', () => {
    const root: BTNode = {
      type: 'selector',
      children: [{ type: 'sequence', children: [{ type: 'action', name: 'a' }] }],
    }
    const out = addChild(root, 'root.children.0', { type: 'wait', seconds: 2 })
    expect(out).toEqual({
      type: 'selector',
      children: [{ type: 'sequence', children: [{ type: 'action', name: 'a' }, { type: 'wait', seconds: 2 }] }],
    })
  })

  it('throws when parent is leaf', () => {
    const root: BTNode = { type: 'action', name: 'x' }
    expect(() => addChild(root, 'root', { type: 'wait', seconds: 1 })).toThrow()
  })
})

describe('deleteAt', () => {
  it('removes child from selector', () => {
    const root: BTNode = {
      type: 'selector',
      children: [{ type: 'action', name: 'a' }, { type: 'action', name: 'b' }],
    }
    const out = deleteAt(root, 'root.children.0')
    expect(out).toEqual({ type: 'selector', children: [{ type: 'action', name: 'b' }] })
  })

  it('removes branch from chance (weight + node together)', () => {
    const root: BTNode = {
      type: 'chance',
      branches: [
        { weight: 3, node: { type: 'action', name: 'a' } },
        { weight: 7, node: { type: 'action', name: 'b' } },
      ],
    }
    const out = deleteAt(root, 'root.branches.0.node')
    expect(out).toEqual({
      type: 'chance',
      branches: [{ weight: 7, node: { type: 'action', name: 'b' } }],
    })
  })

  it('cascade removes whole subtree silently', () => {
    const root: BTNode = {
      type: 'selector',
      children: [
        { type: 'sequence', children: [{ type: 'action', name: 'a' }, { type: 'wait', seconds: 1 }] },
        { type: 'action', name: 'b' },
      ],
    }
    const out = deleteAt(root, 'root.children.0')
    expect(out).toEqual({ type: 'selector', children: [{ type: 'action', name: 'b' }] })
  })

  it('throws when called on root path', () => {
    const root: BTNode = { type: 'action', name: 'a' }
    expect(() => deleteAt(root, 'root')).toThrow()
  })
})

describe('convertAt', () => {
  it('converts node at path', () => {
    const root: BTNode = {
      type: 'selector',
      children: [{ type: 'action', name: 'x' }],
    }
    const out = convertAt(root, 'root.children.0', 'condition')
    expect(out).toEqual({
      type: 'selector',
      children: [{ type: 'condition', name: '', args: {} }],
    })
  })

  it('converts root', () => {
    const root: BTNode = { type: 'selector', children: [] }
    const out = convertAt(root, 'root', 'sequence')
    expect(out).toEqual({ type: 'sequence', children: [] })
  })

  it('throws on invalid conversion', () => {
    const root: BTNode = {
      type: 'selector',
      children: [{ type: 'action', name: 'x' }],
    }
    expect(() => convertAt(root, 'root', 'action')).toThrow()
  })
})

describe('setRoot', () => {
  it('makes selector default', () => {
    expect(setRoot('selector')).toEqual({ type: 'selector', children: [] })
  })
  it('makes chance default', () => {
    expect(setRoot('chance')).toEqual({ type: 'chance', branches: [] })
  })
  it('makes wait default seconds=1', () => {
    expect(setRoot('wait')).toEqual({ type: 'wait', seconds: 1 })
  })
  it('makes action with provided name', () => {
    expect(setRoot('action', { name: 'goto' })).toEqual({ type: 'action', name: 'goto', args: {} })
  })
})
