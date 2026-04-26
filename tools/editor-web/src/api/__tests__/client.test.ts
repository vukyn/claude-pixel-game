import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { listBehaviors, getBehavior, putBehavior, validateBehavior, listTuning, putTuning, listActions, listConditions } from '../client'

const fetchMock = vi.fn()

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
  fetchMock.mockReset()
})
afterEach(() => vi.unstubAllGlobals())

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

describe('api client', () => {
  it('listBehaviors returns refs', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([{ kind: 'orc', path: '/x/orc.json', state_count: 6 }]))
    const refs = await listBehaviors()
    expect(refs[0].kind).toBe('orc')
    expect(refs[0].state_count).toBe(6)
  })

  it('getBehavior returns parsed json', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ kind: 'orc', states: [] }))
    const b = await getBehavior('orc')
    expect(b.kind).toBe('orc')
  })

  it('putBehavior throws on 400 with errors', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ valid: false, errors: [{ message: 'bad' }] }, 400))
    await expect(putBehavior('orc', { kind: 'orc', states: [] })).rejects.toThrow(/bad/)
  })

  it('validateBehavior returns ValidationResult', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ valid: true }))
    const r = await validateBehavior('orc', { kind: 'orc', states: [] })
    expect(r.valid).toBe(true)
  })

  it('listTuning passes prefix', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([{ key: 'orc_max_lives', value: 2, min: 1, max: 10, unit: '', description: 'x' }]))
    await listTuning('orc')
    expect(fetchMock).toHaveBeenCalledWith('/api/tuning?prefix=orc', expect.anything())
  })

  it('putTuning sends value', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ ok: true, old: 2, new: 5 }))
    const r = await putTuning('orc_max_lives', 5)
    expect(r.new).toBe(5)
  })

  it('listActions returns metas', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([{ name: 'goto', args: [{ name: 'state', type: 'state_id', required: true }] }]))
    const a = await listActions()
    expect(a[0].name).toBe('goto')
  })

  it('listConditions returns metas', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([{ name: 'grounded', args: [] }]))
    const c = await listConditions()
    expect(c[0].name).toBe('grounded')
  })
})
