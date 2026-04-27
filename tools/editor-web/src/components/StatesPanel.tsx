import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useEditorStore } from '../state/editorStore'

export function StatesPanel() {
  const behavior = useEditorStore(s => s.behavior)
  const selectedStateId = useEditorStore(s => s.selectedStateId)
  const selectState = useEditorStore(s => s.selectState)
  if (!behavior) return null
  return (
    <aside className="w-60 border-r border-border bg-card flex flex-col">
      <div className="px-3 py-2 text-[11px] uppercase tracking-wide text-muted-foreground border-b border-border font-semibold">
        States
      </div>
      <ScrollArea className="flex-1">
        <div className="p-1 flex flex-col gap-0.5">
          {behavior.states.map(s => (
            <Button
              key={s.id}
              variant={selectedStateId === s.id ? 'default' : 'ghost'}
              size="sm"
              onClick={() => selectState(s.id)}
              className="w-full justify-between"
            >
              {s.id}
              {s.decision && <Badge variant="outline" className="text-emerald-500 border-emerald-500/40">BT</Badge>}
            </Button>
          ))}
        </div>
      </ScrollArea>
    </aside>
  )
}
