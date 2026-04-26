import { TopBar } from './components/TopBar'
import { StatesPanel } from './components/StatesPanel'
import { BTCanvas } from './components/BTCanvas'
import { Inspector } from './components/Inspector'
import { TuningDrawer } from './components/TuningDrawer'

export default function App() {
  return (
    <div className="h-screen flex flex-col bg-[#1a1d23]">
      <TopBar />
      <TuningDrawer />
      <main className="flex-1 flex min-h-0">
        <StatesPanel />
        <div className="flex-1 min-w-0"><BTCanvas /></div>
        <Inspector />
      </main>
    </div>
  )
}
