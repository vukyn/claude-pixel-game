import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useEditorStore, __resetForTest } from '../editorStore'

vi.mock('../../api/client', () => ({
  listBehaviors: vi.fn(async () => [{ kind: 'orc', path: '/x/orc.json', state_count: 1 }]),
  getBehavior:   vi.fn(async () => ({ kind: 'orc', states: [{ id: 'a', anim: 'idle', decision: false, exit_on: 'anim_done', next: 'a' }] })),
  putBehavior:   vi.fn(async () => ({ ok: true })),
  validateBehavior: vi.fn(async () => ({ valid: true, errors: [] })),
  listActions:   vi.fn(async () => [{ name: 'goto', args: [{ name: 'state', type: 'state_id', required: true }] }]),
  listConditions: vi.fn(async () => [{ name: 'grounded', args: [] }]),
}))

beforeEach(() => __resetForTest())

describe('editorStore', () => {
  it('load fetches behavior + registry', async () => {
    await useEditorStore.getState().load('orc')
    const s = useEditorStore.getState()
    expect(s.behavior?.kind).toBe('orc')
    expect(s.registry.actions.length).toBe(1)
  })
  it('selectState marks selected', async () => {
    await useEditorStore.getState().load('orc')
    useEditorStore.getState().selectState('a')
    expect(useEditorStore.getState().selectedStateId).toBe('a')
  })
  it('save resets dirty', async () => {
    await useEditorStore.getState().load('orc')
    useEditorStore.setState({ dirty: true })
    await useEditorStore.getState().save()
    expect(useEditorStore.getState().dirty).toBe(false)
  })
})
