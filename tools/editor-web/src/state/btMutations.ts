import type { BTNode, BTNodeType } from '../bt/types'
import { canConvert, convertNode } from '../bt/convert'

function clone<T>(v: T): T {
  return JSON.parse(JSON.stringify(v))
}

function getAtPath(root: unknown, path: string): unknown {
  if (path === 'root') return root
  const parts = path.split('.').slice(1)
  let cur: any = root
  for (const p of parts) cur = cur?.[p]
  return cur
}

function setAtPath<T>(root: T, path: string, value: unknown): T {
  if (path === 'root') return value as T
  const next = clone(root) as any
  const parts = path.split('.').slice(1)
  let cur = next
  for (let i = 0; i < parts.length - 1; i++) cur = cur[parts[i]]
  cur[parts[parts.length - 1]] = value
  return next
}

function parentPathOf(path: string): { parent: string; key: string } {
  if (path === 'root') throw new Error('parentPathOf: root has no parent')
  const idx = path.lastIndexOf('.')
  return { parent: path.slice(0, idx), key: path.slice(idx + 1) }
}

export function addChild(bt: BTNode, parentPath: string, child: BTNode): BTNode {
  const parent = getAtPath(bt, parentPath) as BTNode | undefined
  if (!parent) throw new Error(`addChild: missing parent at ${parentPath}`)
  if (parent.type === 'selector' || parent.type === 'sequence') {
    const updated = { ...parent, children: [...parent.children, child] }
    return setAtPath(bt, parentPath, updated)
  }
  if (parent.type === 'chance') {
    const updated = { ...parent, branches: [...parent.branches, { weight: 1, node: child }] }
    return setAtPath(bt, parentPath, updated)
  }
  throw new Error(`addChild: parent at ${parentPath} is leaf (${parent.type})`)
}

export function deleteAt(bt: BTNode, path: string): BTNode {
  if (path === 'root') throw new Error('deleteAt: cannot delete root')
  const { parent: parentPath, key } = parentPathOf(path)
  // path forms: "...children.<i>"  OR  "...branches.<i>.node"
  if (key === 'node') {
    // path = "...chance.branches.<i>.node"
    // parentPath = "...chance.branches.<i>"
    const branchPath = parentPath
    const { parent: branchesPath, key: idxStr } = parentPathOf(branchPath)
    const idx = Number(idxStr)
    // branchesPath = "...chance.branches" — go one more up to get the chance node
    const { parent: chancePath } = parentPathOf(branchesPath)
    const chance = getAtPath(bt, chancePath) as BTNode
    if (!chance || chance.type !== 'chance') throw new Error(`deleteAt: chance branch parent missing at ${chancePath}`)
    const updated = { ...chance, branches: chance.branches.filter((_, i) => i !== idx) }
    return setAtPath(bt, chancePath, updated)
  }

  // selector/sequence child
  const idx = Number(key)
  const { parent: composite } = parentPathOf(parentPath) // skip ".children"
  const compositeNode = getAtPath(bt, composite) as BTNode
  if (!compositeNode) throw new Error(`deleteAt: composite parent missing at ${composite}`)
  if (compositeNode.type !== 'selector' && compositeNode.type !== 'sequence') {
    throw new Error(`deleteAt: expected selector/sequence at ${composite}, got ${compositeNode.type}`)
  }
  const updated = { ...compositeNode, children: compositeNode.children.filter((_, i) => i !== idx) }
  return setAtPath(bt, composite, updated)
}

export function convertAt(bt: BTNode, path: string, toType: BTNodeType): BTNode {
  const node = getAtPath(bt, path) as BTNode | undefined
  if (!node) throw new Error(`convertAt: missing node at ${path}`)
  const check = canConvert(node, toType)
  if (!check.ok) throw new Error(`convertAt: ${check.reason}`)
  const converted = convertNode(node, toType)
  return setAtPath(bt, path, converted)
}

export function setRoot(rootType: BTNodeType, opts?: { name?: string }): BTNode {
  if (rootType === 'selector' || rootType === 'sequence') return { type: rootType, children: [] }
  if (rootType === 'chance') return { type: 'chance', branches: [] }
  if (rootType === 'wait') return { type: 'wait', seconds: 1 }
  return { type: rootType, name: opts?.name ?? '', args: {} }
}
