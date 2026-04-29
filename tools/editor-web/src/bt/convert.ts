import type { BTNode, BTNodeType } from './types'

const COMPOSITES: BTNodeType[] = ['selector', 'sequence', 'chance']
const LEAVES: BTNodeType[] = ['action', 'condition', 'wait']

const isComposite = (t: BTNodeType) => COMPOSITES.includes(t)
const isLeaf = (t: BTNodeType) => LEAVES.includes(t)

function childCount(n: BTNode): number {
  if (n.type === 'selector' || n.type === 'sequence') return n.children.length
  if (n.type === 'chance') return n.branches.length
  return 0
}

export function canConvert(from: BTNode, to: BTNodeType): { ok: boolean; reason?: string } {
  if (from.type === to) return { ok: false, reason: 'already that type' }
  if (isComposite(from.type) && isLeaf(to) && childCount(from) > 0) {
    return { ok: false, reason: 'delete children first' }
  }
  return { ok: true }
}

export function convertNode(from: BTNode, to: BTNodeType): BTNode {
  const check = canConvert(from, to)
  if (!check.ok) throw new Error(`convertNode: ${check.reason}`)

  // Composite ↔ composite
  if (isComposite(from.type) && isComposite(to)) {
    const kids =
      from.type === 'chance'
        ? from.branches.map(b => b.node)
        : (from as { children: BTNode[] }).children
    if (to === 'chance') {
      return { type: 'chance', branches: kids.map(node => ({ weight: 1, node })) }
    }
    return { type: to as 'selector' | 'sequence', children: kids }
  }

  // Leaf → composite (always empty per design)
  if (isLeaf(from.type) && isComposite(to)) {
    if (to === 'chance') return { type: 'chance', branches: [] }
    return { type: to as 'selector' | 'sequence', children: [] }
  }

  // Composite → leaf (only when empty per canConvert)
  if (isComposite(from.type) && isLeaf(to)) {
    return makeLeaf(to)
  }

  // Leaf ↔ leaf
  return makeLeaf(to)
}

function makeLeaf(t: BTNodeType): BTNode {
  if (t === 'wait') return { type: 'wait', seconds: 1 }
  return { type: t as 'action' | 'condition', name: '', args: {} }
}
