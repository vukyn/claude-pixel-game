import { useEffect, useState } from 'react'
import { ChevronDown } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Slider } from '@/components/ui/slider'
import { cn } from '@/lib/utils'
import { listTuning, putTuning } from '../api/client'
import type { TuningRow } from '../api/schemas'

const PREFIXES = ['physics', 'stamina', 'soldier', 'orc', 'slime', 'enemy_spawn']

export function TuningDrawer() {
  const [open, setOpen] = useState(false)
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

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="border-b border-border bg-card">
      <div className="px-4 py-2 flex items-center gap-2 text-sm">
        <CollapsibleTrigger asChild>
          <Button variant="ghost" size="icon" aria-label="Toggle tuning">
            <ChevronDown className={cn('transition-transform', !open && '-rotate-90')} />
          </Button>
        </CollapsibleTrigger>
        <span className="font-semibold text-primary">Tuning</span>
        <Badge variant="secondary">{rows.length} keys</Badge>
        <span className="flex-1" />
        <div className="flex gap-1">
          {PREFIXES.map(p => (
            <Button key={p} variant={prefix === p ? 'secondary' : 'ghost'} size="sm" onClick={() => setPrefix(p)}>
              {p}
            </Button>
          ))}
        </div>
      </div>
      <CollapsibleContent>
        <table className="w-full text-xs">
          <tbody>
            {rows.map(r => (
              <tr key={r.key} className="border-t border-border">
                <td className="px-3 py-2 w-1/3">
                  <div className="font-mono text-emerald-500">{r.key}</div>
                  <div className="text-muted-foreground">{r.description}</div>
                </td>
                <td className="px-3 py-2 w-1/3">
                  <div className="flex flex-col gap-1">
                    <Slider
                      min={r.min}
                      max={r.max}
                      value={[r.value]}
                      onValueChange={([v]) => handleChange(r.key, v)}
                      className="[&_[data-slot=slider-track]]:bg-foreground/15 [&_[data-slot=slider-range]]:bg-primary"
                    />
                    <span className="text-muted-foreground text-[10px] leading-none">[{r.min} .. {r.max}]</span>
                  </div>
                </td>
                <td className="px-3 py-2 w-1/6">
                  <div className="flex items-center gap-1">
                    <Input type="number" value={r.value} onChange={e => handleChange(r.key, Number(e.target.value))} className="w-20 text-right h-7" />
                    <span className="text-muted-foreground text-xs">{r.unit}</span>
                  </div>
                </td>
                <td className="px-3 py-2 text-right text-[11px]">
                  {pending[r.key] === 'saving' && <Badge variant="outline" className="text-amber-500 border-amber-500/40">⟳ saving</Badge>}
                  {pending[r.key] === 'saved'  && <Badge variant="outline" className="text-emerald-500 border-emerald-500/40">✓ saved</Badge>}
                  {pending[r.key] === 'error'  && <Badge variant="destructive">✗ error</Badge>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </CollapsibleContent>
    </Collapsible>
  )
}
