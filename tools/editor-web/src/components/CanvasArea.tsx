import { useState } from 'react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { BTCanvas } from './BTCanvas'
import { TransitionsCanvas } from './TransitionsCanvas'

type CanvasTab = 'bt' | 'transitions'

export function CanvasArea() {
  const [tab, setTab] = useState<CanvasTab>('bt')
  return (
    <Tabs value={tab} onValueChange={(v) => setTab(v as CanvasTab)} className="flex flex-col flex-1 min-h-0">
      <TabsList className="rounded-none border-b border-border bg-transparent p-0 h-auto">
        <TabsTrigger value="bt" className="rounded-none">BT</TabsTrigger>
        <TabsTrigger value="transitions" className="rounded-none">Transitions</TabsTrigger>
      </TabsList>
      <TabsContent value="bt" className="flex-1 min-h-0 mt-0"><BTCanvas /></TabsContent>
      <TabsContent value="transitions" className="flex-1 min-h-0 mt-0">
        <TransitionsCanvas onJumpToState={() => setTab('bt')} />
      </TabsContent>
    </Tabs>
  )
}
