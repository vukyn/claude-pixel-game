import type { ActionMeta, BehaviorJSON } from '../api/schemas'
import type { BTNode } from './types'

export interface ValidationError { message: string; node_path?: string }
export interface ValidationResult { valid: boolean; errors: ValidationError[] }

export interface Registry { actions: ActionMeta[]; conditions: ActionMeta[] }

export function validateBehavior(b: BehaviorJSON, expectedKind: string, reg: Registry): ValidationResult {
  const errors: ValidationError[] = []
  if (b.kind !== expectedKind) errors.push({ message: `kind mismatch: file ${expectedKind}, body ${b.kind}` })

  const ids = new Set<string>()
  for (const s of b.states) {
    if (ids.has(s.id)) errors.push({ message: `duplicate state id: ${s.id}` })
    ids.add(s.id)
    if (s.decision && !s.bt) errors.push({ message: `decision state ${s.id} missing bt` })
    if (!s.decision && !s.exit_on) errors.push({ message: `non-decision state ${s.id} missing exit_on` })
  }
  for (const s of b.states) {
    if (!s.decision && s.next && s.next !== '__dead' && !ids.has(s.next)) {
      errors.push({ message: `state ${s.id} next "${s.next}" undeclared` })
    }
    if (s.decision && s.bt) walkNode(s.bt as BTNode, `states.${s.id}.bt`, ids, reg, errors)
  }
  return { valid: errors.length === 0, errors }
}

const actionByName = (reg: Registry, n: string) => reg.actions.find(a => a.name === n)
const condByName = (reg: Registry, n: string) => reg.conditions.find(c => c.name === n)

function walkNode(n: BTNode, path: string, ids: Set<string>, reg: Registry, errors: ValidationError[]): void {
  switch (n.type) {
    case 'selector':
    case 'sequence':
      n.children.forEach((c, i) => walkNode(c, `${path}.children.${i}`, ids, reg, errors))
      break
    case 'chance':
      n.branches.forEach((b, i) => {
        if (!b.weight || b.weight <= 0) errors.push({ message: `chance branch weight must be > 0`, node_path: `${path}.branches.${i}` })
        walkNode(b.node, `${path}.branches.${i}.node`, ids, reg, errors)
      })
      break
    case 'action': {
      const meta = actionByName(reg, n.name)
      if (!meta) { errors.push({ message: `unknown action "${n.name}"`, node_path: path }); break }
      for (const arg of meta.args) {
        if (arg.required && (n.args === undefined || n.args[arg.name] === undefined)) {
          errors.push({ message: `action ${n.name} missing required arg "${arg.name}"`, node_path: path })
        }
      }
      if (n.name === 'goto' && n.args?.state && n.args.state !== '__dead' && !ids.has(n.args.state as string)) {
        errors.push({ message: `goto state "${n.args.state}" undeclared`, node_path: path })
      }
      break
    }
    case 'condition': {
      const meta = condByName(reg, n.name)
      if (!meta) errors.push({ message: `unknown condition "${n.name}"`, node_path: path })
      break
    }
    case 'wait':
      if (typeof n.seconds !== 'number' || n.seconds < 0) errors.push({ message: `wait.seconds must be >= 0`, node_path: path })
      break
  }
}
