import { z } from 'zod'

export const ArgMetaSchema = z.object({
  name: z.string(),
  type: z.enum(['int', 'float', 'string', 'state_id', 'anim_key']),
  required: z.boolean(),
})
export type ArgMeta = z.infer<typeof ArgMetaSchema>

export const ActionMetaSchema = z.object({
  name: z.string(),
  args: z.array(ArgMetaSchema).nullable().default([]).transform(a => a ?? []),
})
export type ActionMeta = z.infer<typeof ActionMetaSchema>

export const BehaviorRefSchema = z.object({
  kind: z.string(),
  path: z.string(),
  state_count: z.number(),
})
export type BehaviorRef = z.infer<typeof BehaviorRefSchema>

export const FrameVXSchema = z.object({
  frame_start: z.number(),
  frame_end: z.number(),
  vx: z.number(),
})

export const StateDeclSchema = z.object({
  id: z.string(),
  anim: z.string(),
  decision: z.boolean(),
  bt: z.any().optional(),
  exit_on: z.string().optional(),
  next: z.string().optional(),
  on_exit_actions: z.array(z.string()).optional(),
  on_frame_vx: z.array(FrameVXSchema).optional(),
})
export type StateDecl = z.infer<typeof StateDeclSchema>

export const BehaviorJSONSchema = z.object({
  kind: z.string(),
  states: z.array(StateDeclSchema),
})
export type BehaviorJSON = z.infer<typeof BehaviorJSONSchema>

export const TuningRowSchema = z.object({
  key: z.string(),
  value: z.number(),
  min: z.number(),
  max: z.number(),
  unit: z.string(),
  description: z.string(),
})
export type TuningRow = z.infer<typeof TuningRowSchema>

export const ValidationErrorSchema = z.object({
  message: z.string(),
  node_path: z.string().optional(),
})
export type ValidationError = z.infer<typeof ValidationErrorSchema>

export const ValidationResultSchema = z.object({
  valid: z.boolean(),
  errors: z.array(ValidationErrorSchema).optional().default([]),
})
export type ValidationResult = z.infer<typeof ValidationResultSchema>
