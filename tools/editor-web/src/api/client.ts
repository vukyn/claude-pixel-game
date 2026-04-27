import {
  ActionMetaSchema, BehaviorRefSchema, BehaviorJSONSchema, TuningRowSchema, ValidationResultSchema,
  type ActionMeta, type BehaviorJSON, type BehaviorRef, type TuningRow, type ValidationResult,
} from './schemas'
import { z } from 'zod'

class ApiError extends Error {
  constructor(public status: number, message: string, public body?: unknown) { super(message) }
}

async function getJson<T>(url: string, schema: z.ZodType<T>): Promise<T> {
  const res = await fetch(url, { headers: { Accept: 'application/json' } })
  if (!res.ok) throw new ApiError(res.status, await res.text())
  return schema.parse(await res.json())
}

async function sendJson<T>(method: 'POST' | 'PUT', url: string, body: unknown, schema: z.ZodType<T>): Promise<T> {
  const res = await fetch(url, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  const text = await res.text()
  if (!res.ok) {
    let errMsg = text
    try {
      const parsed = JSON.parse(text) as ValidationResult
      if (parsed.errors?.length) errMsg = parsed.errors.map(e => e.message).join('; ')
    } catch { /* keep text */ }
    throw new ApiError(res.status, errMsg)
  }
  return schema.parse(JSON.parse(text))
}

export async function listBehaviors(): Promise<BehaviorRef[]> {
  return getJson('/api/behaviors', z.array(BehaviorRefSchema))
}
export async function getBehavior(kind: string): Promise<BehaviorJSON> {
  return getJson(`/api/behaviors/${kind}`, BehaviorJSONSchema)
}
export async function putBehavior(kind: string, body: BehaviorJSON): Promise<{ ok: boolean }> {
  return sendJson('PUT', `/api/behaviors/${kind}`, body, z.object({ ok: z.boolean() }))
}
export async function validateBehavior(kind: string, body: BehaviorJSON): Promise<ValidationResult> {
  return sendJson('POST', `/api/behaviors/${kind}/validate`, body, ValidationResultSchema)
}
export async function listTuning(prefix: string): Promise<TuningRow[]> {
  return getJson(`/api/tuning?prefix=${encodeURIComponent(prefix)}`, z.array(TuningRowSchema))
}
export async function putTuning(key: string, value: number): Promise<{ ok: boolean; old: number; new: number }> {
  return sendJson('PUT', `/api/tuning/${encodeURIComponent(key)}`, { value }, z.object({ ok: z.boolean(), old: z.number(), new: z.number() }))
}
export async function listActions(): Promise<ActionMeta[]> {
  return getJson('/api/registry/actions', z.array(ActionMetaSchema))
}
export async function listConditions(): Promise<ActionMeta[]> {
  return getJson('/api/registry/conditions', z.array(ActionMetaSchema))
}

export { ApiError }
