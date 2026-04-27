import { Handle, Position, type NodeProps } from 'reactflow'

export function SelectorNode({ selected }: NodeProps) {
  return (
    <div
      className={`px-3 py-2 rounded border-2 bg-[#232831] min-w-[160px] ${selected ? 'ring-2 ring-blue-400' : ''}`}
      style={{ borderColor: '#5aa3f0' }}
    >
      <Handle type="target" position={Position.Left} style={{ background: '#5aa3f0' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#5aa3f0]">selector</div>
      <div className="text-xs text-[#b8c0cc]">first non-Failure wins</div>
      <Handle type="source" position={Position.Right} style={{ background: '#5aa3f0' }} />
    </div>
  )
}
