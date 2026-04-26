import { Handle, Position, type NodeProps } from 'reactflow'

export function ActionNode({ selected, data }: NodeProps) {
  return (
    <div
      className={`px-3 py-2 rounded border-2 bg-[#232831] min-w-[160px] ${selected ? 'ring-2 ring-blue-400' : ''}`}
      style={{ borderColor: '#c779e0' }}
    >
      <Handle type="target" position={Position.Left} style={{ background: '#c779e0' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#c779e0]">action</div>
      <div className="text-xs text-[#b8c0cc]">{data.name as string}</div>
    </div>
  )
}
