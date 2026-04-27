import type { ActionMeta, BehaviorJSON, StateDecl } from '../api/schemas'
import type { BTNode } from './types'

export type TransitionEdgeKind = 'next' | 'goto'

export interface TransitionEdge {
  id: string
  from: string
  to: string
  kind: TransitionEdgeKind
  label?: string
}

export interface TransitionGraph {
  nodes: { id: string; isInitial: boolean }[]
  edges: TransitionEdge[]
}

export function extractTransitions(
  b: BehaviorJSON,
  registry: { actions: ActionMeta[] }
): TransitionGraph {
  const edges: TransitionEdge[] = []
  let dead = false

  const stateIdArgName = (actionName: string): string | undefined => {
    const meta = registry.actions.find(a => a.name === actionName)
    return meta?.args.find(a => a.type === 'state_id')?.name
  }

  const pushNext = (s: StateDecl) => {
    if (!s.next) return
    if (s.next === '__dead') dead = true
    edges.push({
      id: `${s.id}->${s.next}:next:${edges.length}`,
      from: s.id,
      to: s.next,
      kind: 'next',
      label: s.exit_on ?? '',
    })
  }

  const walkBT = (n: BTNode, fromState: string): void => {
    switch (n.type) {
      case 'selector':
      case 'sequence':
        n.children.forEach(c => walkBT(c, fromState))
        return
      case 'chance':
        n.branches.forEach(br => walkBT(br.node, fromState))
        return
      case 'action': {
        const argName = stateIdArgName(n.name)
        if (!argName) return
        const target = (n.args ?? {})[argName]
        if (typeof target !== 'string' || target === '') return
        if (target === '__dead') dead = true
        edges.push({
          id: `${fromState}->${target}:goto:${edges.length}`,
          from: fromState,
          to: target,
          kind: 'goto',
          label: '',
        })
        return
      }
      // condition, wait — no transition
    }
  }

  for (const s of b.states) {
    if (!s.decision) pushNext(s)
    if (s.decision && s.bt) walkBT(s.bt as BTNode, s.id)
  }

  const nodes = b.states.map((s, i) => ({ id: s.id, isInitial: i === 0 }))
  if (dead) nodes.push({ id: '__dead', isInitial: false })

  return { nodes, edges }
}
