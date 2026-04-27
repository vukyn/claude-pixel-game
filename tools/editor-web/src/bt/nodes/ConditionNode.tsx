import { Handle, Position, type NodeProps } from 'reactflow'

function summarizeArgs(name: string, args: Record<string, unknown> | undefined): string {
  if (!args || Object.keys(args).length === 0) return name
  const pairs = Object.entries(args).map(([k, v]) => `${k}=${v}`).join(', ')
  return `${name}(${pairs})`
}

export function ConditionNode({ selected, data }: NodeProps) {
  const name = data.name as string
  const args = data.args as Record<string, unknown> | undefined
  return (
    <div
      className={`px-3 py-2 rounded border-2 bg-[#232831] min-w-[160px] ${selected ? 'ring-2 ring-blue-400' : ''}`}
      style={{ borderColor: '#e0c779' }}
    >
      <Handle type="target" position={Position.Left} style={{ background: '#e0c779' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#e0c779]">condition</div>
      <div className="text-xs text-[#b8c0cc]">{summarizeArgs(name, args)}</div>
    </div>
  )
}
