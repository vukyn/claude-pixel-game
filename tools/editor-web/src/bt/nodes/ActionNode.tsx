import { Handle, Position, type NodeProps } from 'reactflow'

function summarizeArgs(name: string, args: Record<string, unknown> | undefined): string {
  if (!args || Object.keys(args).length === 0) return name
  if (name === 'goto') return `goto → ${args.state ?? '?'}`
  const pairs = Object.entries(args).map(([k, v]) => `${k}=${v}`).join(', ')
  return `${name}(${pairs})`
}

export function ActionNode({ selected, data }: NodeProps) {
  const name = data.name as string
  const args = data.args as Record<string, unknown> | undefined
  return (
    <div
      className={`px-3 py-2 rounded border-2 bg-[#232831] min-w-[160px] ${selected ? 'ring-2 ring-blue-400' : ''}`}
      style={{ borderColor: '#c779e0' }}
    >
      <Handle type="target" position={Position.Left} style={{ background: '#c779e0' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#c779e0]">action</div>
      <div className="text-xs text-[#b8c0cc]">{summarizeArgs(name, args)}</div>
    </div>
  )
}
