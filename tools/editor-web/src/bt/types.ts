export type BTNodeType = 'selector' | 'sequence' | 'chance' | 'wait' | 'action' | 'condition'

export type BTNode =
  | { type: 'selector'; children: BTNode[] }
  | { type: 'sequence'; children: BTNode[] }
  | { type: 'chance'; branches: { weight: number; node: BTNode }[] }
  | { type: 'wait'; seconds: number }
  | { type: 'action'; name: string; args?: Record<string, unknown> }
  | { type: 'condition'; name: string; args?: Record<string, unknown> }

export interface FlowNode {
  id: string
  type: BTNodeType
  data: Record<string, unknown>
  position: { x: number; y: number }
}

export interface FlowEdge {
  id: string
  source: string
  target: string
  label?: string
  data?: { weight?: number; order: number }
}
