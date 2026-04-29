import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useEditorStore, __resetForTest } from '../editorStore'

vi.mock('../../api/client', () => ({
  listBehaviors: vi.fn(async () => [{ kind: 'orc', path: '/x/orc.json', state_count: 1 }]),
  getBehavior: vi.fn(async () => ({
    kind: 'orc',
    states: [{ id: 'a', anim: 'idle', decision: false, exit_on: 'anim_done', next: 'a' }],
  })),
  putBehavior: vi.fn(async () => ({ ok: true })),
  validateBehavior: vi.fn(async () => ({ valid: true, errors: [] })),
  listActions: vi.fn(async () => [{ name: 'goto', args: [{ name: 'state', type: 'state_id', required: true }] }]),
  listConditions: vi.fn(async () => []),
}))

beforeEach(() => __resetForTest())

const mutate = (anim: string) => {
  const s = useEditorStore.getState()
  if (!s.behavior) throw new Error('no behavior')
  s.setBehavior({
    ...s.behavior,
    states: s.behavior.states.map(st => ({ ...st, anim })),
  })
}

describe('editorStore undo/redo', () => {
  it('undo restores previous behavior', async () => {
    await useEditorStore.getState().load('orc')
    expect(useEditorStore.getState().behavior?.states[0].anim).toBe('idle')
    mutate('walk')
    expect(useEditorStore.getState().behavior?.states[0].anim).toBe('walk')
    useEditorStore.temporal.getState().undo()
    expect(useEditorStore.getState().behavior?.states[0].anim).toBe('idle')
  })

  it('redo replays', async () => {
    await useEditorStore.getState().load('orc')
    mutate('walk')
    useEditorStore.temporal.getState().undo()
    useEditorStore.temporal.getState().redo()
    expect(useEditorStore.getState().behavior?.states[0].anim).toBe('walk')
  })

  it('load(kind) clears the past stack', async () => {
    await useEditorStore.getState().load('orc')
    mutate('walk')
    expect(useEditorStore.temporal.getState().pastStates.length).toBeGreaterThan(0)
    await useEditorStore.getState().load('orc')
    expect(useEditorStore.temporal.getState().pastStates.length).toBe(0)
  })

  it('save does NOT clear the past stack', async () => {
    await useEditorStore.getState().load('orc')
    mutate('walk')
    const beforeLen = useEditorStore.temporal.getState().pastStates.length
    await useEditorStore.getState().save()
    expect(useEditorStore.temporal.getState().pastStates.length).toBe(beforeLen)
  })

  it('cap at 50 — drops oldest', async () => {
    await useEditorStore.getState().load('orc')
    for (let i = 0; i < 60; i++) mutate(`a${i}`)
    expect(useEditorStore.temporal.getState().pastStates.length).toBeLessThanOrEqual(50)
  })

  it('dirty tracked across undo/redo', async () => {
    await useEditorStore.getState().load('orc')
    expect(useEditorStore.getState().dirty).toBe(false)
    mutate('walk')
    expect(useEditorStore.getState().dirty).toBe(true)
    useEditorStore.temporal.getState().undo()
    expect(useEditorStore.getState().dirty).toBe(false)
    useEditorStore.temporal.getState().redo()
    expect(useEditorStore.getState().dirty).toBe(true)
  })
})
