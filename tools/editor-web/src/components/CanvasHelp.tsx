function Kbd({ children }: { children: React.ReactNode }) {
  return (
    <kbd
      data-slot="kbd"
      className="inline-flex items-center justify-center rounded-sm border border-background/30 bg-background/15 px-1 py-0.5 font-mono text-[10px] leading-none"
    >
      {children}
    </kbd>
  )
}

interface Row {
  keys: React.ReactNode
  desc: string
}

function Section({ title, rows }: { title: string; rows: Row[] }) {
  return (
    <div className="flex flex-col gap-1">
      <div className="text-[10px] uppercase tracking-wider opacity-60">{title}</div>
      <table className="w-full border-collapse text-[11px] leading-snug">
        <tbody>
          {rows.map((r, i) => (
            <tr key={i} className="align-top">
              <td className="whitespace-nowrap py-0.5 pr-3 align-top">
                <span className="flex flex-wrap items-center gap-1">{r.keys}</span>
              </td>
              <td className="py-0.5 align-top opacity-90">{r.desc}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function CanvasHelp() {
  return (
    <div className="flex max-w-md flex-col gap-3 p-1 text-xs">
      <div className="text-sm font-semibold">Canvas controls</div>

      <Section
        title="Pan / move canvas"
        rows={[
          {
            keys: <><Kbd>Two-finger</Kbd><span className="opacity-50">scroll</span></>,
            desc: 'Pan canvas in any direction (touchpad).',
          },
          {
            keys: <><Kbd>Space</Kbd>+<Kbd>drag</Kbd></>,
            desc: 'Hold Space and left-drag to pan (Figma-style; works in any mode).',
          },
          {
            keys: <><Kbd>Middle</Kbd>/<Kbd>Right</Kbd> drag</>,
            desc: 'In Hand mode, drag with middle or right mouse to pan.',
          },
        ]}
      />

      <Section
        title="Zoom"
        rows={[
          {
            keys: <Kbd>Pinch</Kbd>,
            desc: 'Two-finger pinch in/out on touchpad to zoom.',
          },
          {
            keys: <><Kbd>⌘</Kbd>/<Kbd>Ctrl</Kbd>+<span className="opacity-50">scroll</span></>,
            desc: 'Hold Command (Mac) or Control (Win/Linux) and scroll to zoom in/out.',
          },
          {
            keys: <><Kbd>+</Kbd>/<Kbd>−</Kbd> Controls</>,
            desc: 'Use the +/−/fit-view buttons in the bottom-left Controls panel.',
          },
        ]}
      />

      <Section
        title="Select & move nodes"
        rows={[
          {
            keys: <Kbd>Click</Kbd>,
            desc: 'Click a node to select; the right Inspector shows its details.',
          },
          {
            keys: <><Kbd>Left</Kbd> drag node</>,
            desc: 'Drag a node body with the left mouse to reposition it.',
          },
          {
            keys: <>Pointer mode: <Kbd>Left</Kbd> drag pane</>,
            desc: 'Toggle Pointer mode (top-right) and drag empty canvas to box-select multiple nodes.',
          },
        ]}
      />

      <Section
        title="Right-click menu (edit BT)"
        rows={[
          {
            keys: <><Kbd>Right-click</Kbd> node</>,
            desc: 'Open the context menu. Composites show "Add child"; most nodes show "Convert to"; non-root nodes show "Delete".',
          },
          {
            keys: <>Add child ►</>,
            desc: 'Cascades into Selector / Sequence / Chance / Wait / Action ► / Condition ►. Pick a registry name to insert with default args.',
          },
          {
            keys: <>Convert to ►</>,
            desc: 'Lists only valid targets. Composite ↔ composite preserves children; leaf ↔ leaf resets payload; composite → leaf is blocked when children exist — delete them first.',
          },
          {
            keys: <>Delete</>,
            desc: 'Hidden on root. On a chance branch, removes the {weight, node} pair together. Composite deletes cascade across descendants.',
          },
        ]}
      />

      <Section
        title="Add root (state with no BT)"
        rows={[
          {
            keys: <><Kbd>Add root</Kbd> button</>,
            desc: 'If a decision state has no BT, click "Add root" in the empty canvas and pick a type to seed the tree.',
          },
        ]}
      />

      <Section
        title="Modes (top-right toggle)"
        rows={[
          {
            keys: <>Hand</>,
            desc: 'Default. Left-click drags nodes; middle/right drag pans the canvas.',
          },
          {
            keys: <>Pointer</>,
            desc: 'Left-click drags nodes; left-drag on empty pane creates a selection box.',
          },
        ]}
      />

      <Section
        title="Undo / Redo"
        rows={[
          {
            keys: <><Kbd>⌘</Kbd>/<Kbd>Ctrl</Kbd>+<Kbd>Z</Kbd></>,
            desc: 'Undo last edit. Skipped while focus is in a text input (native field undo runs instead).',
          },
          {
            keys: <><Kbd>⌘</Kbd>+<Kbd>Shift</Kbd>+<Kbd>Z</Kbd></>,
            desc: 'Redo (Mac).',
          },
          {
            keys: <><Kbd>Ctrl</Kbd>+<Kbd>Y</Kbd></>,
            desc: 'Redo (Win/Linux).',
          },
          {
            keys: <>TopBar ↶ ↷</>,
            desc: 'Undo/Redo buttons in TopBar mirror the keyboard; disabled when their stack is empty. Fast typing inside an input is coalesced into one undo step (400ms window).',
          },
        ]}
      />

      <Section
        title="Save & reload"
        rows={[
          {
            keys: <>Save</>,
            desc: 'Click Save in TopBar to persist changes. Blocked while validation fails (red badge); the unsaved badge clears on success.',
          },
          {
            keys: <><Kbd>F5</Kbd> in-game</>,
            desc: 'Press F5 in the game window to reparse all behavior JSON; a toast confirms success or shows the first error.',
          },
        ]}
      />
    </div>
  )
}
