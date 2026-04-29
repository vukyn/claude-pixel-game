import { Handle, Position, type NodeProps } from 'reactflow'

export function WaitNode({ selected, data }: NodeProps) {
  return (
    <div
      className={`px-3 py-2 rounded border-2 bg-[#232831] min-w-[160px] ${selected ? 'ring-2 ring-blue-400' : ''}`}
      style={{ borderColor: '#79c7e0' }}
    >
      <Handle type="target" position={Position.Left} style={{ background: '#79c7e0' }} />
      <div className="text-[10px] uppercase tracking-wide text-[#79c7e0]">wait</div>
      <div className="text-xs text-[#b8c0cc]">{(data.seconds as number)?.toString() ?? '?'}s</div>
    </div>
  )
}
