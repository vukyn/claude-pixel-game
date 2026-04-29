import { Handle, Position, type NodeProps } from 'reactflow'

export function ChanceNode({ selected }: NodeProps) {
  return (
    <div
      className={`px-3 py-2 rounded border-2 bg-[#232831] min-w-[160px] ${selected ? 'ring-2 ring-blue-400' : ''}`}
      style={{ borderColor: '#f0a35a' }}
    >
      <Handle type="target" position={Position.Left} style={{ background: '#f0a35a' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#f0a35a]">chance</div>
      <div className="text-xs text-[#b8c0cc]">weighted branches</div>
      <Handle type="source" position={Position.Right} style={{ background: '#f0a35a' }} />
    </div>
  )
}
