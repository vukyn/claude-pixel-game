import { createContext, useContext } from 'react'
import type { ActionMeta } from '../../api/schemas'
import type { BTNode, BTNodeType } from '../types'

export interface NodeMenuApi {
  registry: { actions: ActionMeta[]; conditions: ActionMeta[] }
  onAddChild(path: string, child: BTNode): void
  onConvert(path: string, toType: BTNodeType): void
  onDelete(path: string): void
}

export const NodeMenuContext = createContext<NodeMenuApi | null>(null)

export function useNodeMenuApi(): NodeMenuApi | null {
  return useContext(NodeMenuContext)
}
