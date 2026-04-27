import { Handle, Position, type NodeProps } from 'reactflow'

export function SequenceNode({ selected }: NodeProps) {
  return (
    <div
      className={`px-3 py-2 rounded border-2 bg-[#232831] min-w-[160px] ${selected ? 'ring-2 ring-blue-400' : ''}`}
      style={{ borderColor: '#7ed957' }}
    >
      <Handle type="target" position={Position.Left} style={{ background: '#7ed957' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#7ed957]">sequence</div>
      <div className="text-xs text-[#b8c0cc]">in order</div>
      <Handle type="source" position={Position.Right} style={{ background: '#7ed957' }} />
    </div>
  )
}
