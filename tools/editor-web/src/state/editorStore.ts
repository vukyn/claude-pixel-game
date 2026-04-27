import { create } from 'zustand'
import { temporal } from 'zundo'
import type { ActionMeta, BehaviorJSON, ValidationResult } from '../api/schemas'
import { getBehavior, listActions, listConditions, putBehavior, validateBehavior } from '../api/client'
import { validateBehavior as validateLocal } from '../bt/validation'

interface EditorState {
  currentKind: string | null
  behavior: BehaviorJSON | null
  lastSaved: BehaviorJSON | null
  dirty: boolean
  selectedStateId: string | null
  selectedNodePath: string | null
  registry: { actions: ActionMeta[]; conditions: ActionMeta[] }
  validation: ValidationResult
  load(kind: string): Promise<void>
  save(): Promise<void>
  selectState(id: string | null): void
  selectNode(path: string | null): void
  setBehavior(b: BehaviorJSON): void
}

const initial = {
  currentKind: null,
  behavior: null,
  lastSaved: null,
  dirty: false,
  selectedStateId: null,
  selectedNodePath: null,
  registry: { actions: [], conditions: [] },
  validation: { valid: true, errors: [] } as ValidationResult,
}

export const useEditorStore = create<EditorState>()(
  temporal(
    (set, get) => ({
      ...initial,
      async load(kind) {
        const [behavior, actions, conditions] = await Promise.all([
          getBehavior(kind), listActions(), listConditions(),
        ])
        const registry = { actions, conditions }
        const validation = validateLocal(behavior, kind, registry)
        set({ currentKind: kind, behavior, lastSaved: behavior, dirty: false, registry, validation })
        useEditorStore.temporal.getState().clear()
      },
      async save() {
        const s = get()
        if (!s.currentKind || !s.behavior) return
        const remote = await validateBehavior(s.currentKind, s.behavior)
        if (!remote.valid) {
          set({ validation: { valid: false, errors: remote.errors } })
          throw new Error(remote.errors.map(e => e.message).join('; '))
        }
        await putBehavior(s.currentKind, s.behavior)
        set({ lastSaved: s.behavior, dirty: false })
      },
      selectState(id) { set({ selectedStateId: id, selectedNodePath: null }) },
      selectNode(path) { set({ selectedNodePath: path }) },
      setBehavior(b) {
        const s = get()
        const validation = s.currentKind ? validateLocal(b, s.currentKind, s.registry) : { valid: true, errors: [] }
        set({ behavior: b, dirty: b !== s.lastSaved, validation })
      },
    }),
    {
      partialize: (s) => ({ behavior: s.behavior }) as Partial<EditorState>,
      limit: 50,
      equality: (a, b) => (a as { behavior: unknown }).behavior === (b as { behavior: unknown }).behavior,
    }
  )
)

useEditorStore.temporal.subscribe(() => {
  const cur = useEditorStore.getState()
  const behavior = cur.behavior
  const validation = cur.currentKind && behavior
    ? validateLocal(behavior, cur.currentKind, cur.registry)
    : cur.validation
  useEditorStore.setState({ dirty: behavior !== cur.lastSaved, validation })
})

export function __resetForTest() {
  useEditorStore.setState({ ...initial })
  useEditorStore.temporal.getState().clear()
}
