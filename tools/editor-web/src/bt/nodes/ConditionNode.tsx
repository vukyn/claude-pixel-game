import { Handle, Position, type NodeProps } from 'reactflow'

export function ConditionNode({ selected, data }: NodeProps) {
  return (
    <div
      className={`px-3 py-2 rounded border-2 bg-[#232831] min-w-[160px] ${selected ? 'ring-2 ring-blue-400' : ''}`}
      style={{ borderColor: '#e0c779' }}
    >
      <Handle type="target" position={Position.Left} style={{ background: '#e0c779' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#e0c779]">condition</div>
      <div className="text-xs text-[#b8c0cc]">{data.name as string}</div>
    </div>
  )
}
