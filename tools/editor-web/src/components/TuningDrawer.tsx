import { useEffect, useState } from 'react'
import { listTuning, putTuning } from '../api/client'
import type { TuningRow } from '../api/schemas'

const PREFIXES = ['physics', 'stamina', 'soldier', 'orc', 'slime', 'enemy_spawn']

export function TuningDrawer() {
  const [open, setOpen] = useState(true)
  const [prefix, setPrefix] = useState('orc')
  const [rows, setRows] = useState<TuningRow[]>([])
  const [pending, setPending] = useState<Record<string, 'saving' | 'saved' | 'error'>>({})

  useEffect(() => {
    const eff = prefix === 'physics' ? '' : prefix
    listTuning(eff).then(setRows).catch(() => setRows([]))
  }, [prefix])

  const handleChange = (key: string, value: number) => {
    setRows(rs => rs.map(r => r.key === key ? { ...r, value } : r))
    setPending(p => ({ ...p, [key]: 'saving' }))
    putTuning(key, value)
      .then(() => setPending(p => ({ ...p, [key]: 'saved' })))
      .catch(() => setPending(p => ({ ...p, [key]: 'error' })))
  }

  if (!open) return (
    <button onClick={() => setOpen(true)} className="px-3 py-1 bg-[#232831] border-b border-[#3a4150] text-xs">▸ Tuning</button>
  )

  return (
    <section className="bg-[#232831] border-b border-[#3a4150]">
      <div className="px-4 py-2 flex items-center gap-2 text-sm">
        <button onClick={() => setOpen(false)} className="text-[#8a93a3]">▾</button>
        <span className="font-semibold text-[#5aa3f0]">Tuning</span>
        <span className="text-[#8a93a3] text-xs">{rows.length} keys</span>
        <span className="flex-1" />
        {PREFIXES.map(p => (
          <button key={p} onClick={() => setPrefix(p)}
            className={`px-2 py-1 text-xs rounded ${prefix === p ? 'bg-[#2c3340] text-white' : 'text-[#8a93a3]'}`}>
            {p}
          </button>
        ))}
      </div>
      <table className="w-full text-xs">
        <tbody>
        {rows.map(r => (
          <tr key={r.key} className="border-t border-[#2c3340]">
            <td className="px-3 py-2 w-1/3"><div className="text-[#7ed957] font-mono">{r.key}</div><div className="text-[#8a93a3]">{r.description}</div></td>
            <td className="px-3 py-2 w-1/3">
              <input type="range" min={r.min} max={r.max} value={r.value}
                     onChange={e => handleChange(r.key, Number(e.target.value))} className="w-full" />
              <span className="text-[#8a93a3]">[{r.min} .. {r.max}]</span>
            </td>
            <td className="px-3 py-2 w-1/6">
              <input className="w-20 bg-[#2c3340] border border-[#3a4150] rounded px-2 py-1 text-right" type="number"
                     value={r.value} onChange={e => handleChange(r.key, Number(e.target.value))} />
              <span className="text-[#8a93a3] ml-1">{r.unit}</span>
            </td>
            <td className="px-3 py-2 text-right text-[11px]">
              {pending[r.key] === 'saving' && <span className="text-[#f0a35a]">⟳ saving</span>}
              {pending[r.key] === 'saved'  && <span className="text-[#7ed957]">✓ saved</span>}
              {pending[r.key] === 'error'  && <span className="text-red-400">✗ error</span>}
            </td>
          </tr>
        ))}
        </tbody>
      </table>
    </section>
  )
}
