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

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1">
      <div className="text-[10px] uppercase tracking-wider opacity-60">{title}</div>
      <ul className="flex flex-col gap-0.5 text-[11px] leading-snug">{children}</ul>
    </div>
  )
}

function Row({ keys, desc }: { keys: React.ReactNode; desc: string }) {
  return (
    <li className="flex items-start gap-2">
      <span className="flex shrink-0 items-center gap-1">{keys}</span>
      <span className="opacity-90">{desc}</span>
    </li>
  )
}

export function CanvasHelp() {
  return (
    <div className="flex max-w-md flex-col gap-3 p-1 text-xs">
      <div className="text-sm font-semibold">Canvas controls</div>

      <Section title="Pan / move canvas">
        <Row
          keys={<><Kbd>Two-finger</Kbd> <span className="opacity-50">scroll</span></>}
          desc="Pan canvas in any direction (touchpad)."
        />
        <Row
          keys={<><Kbd>Space</Kbd> + <Kbd>drag</Kbd></>}
          desc="Hold Space and drag with left mouse to pan (Figma-style; works in any mode)."
        />
        <Row
          keys={<><Kbd>Middle</Kbd> / <Kbd>Right</Kbd> drag</>}
          desc="In Hand mode, drag with middle or right mouse button to pan."
        />
      </Section>

      <Section title="Zoom">
        <Row
          keys={<><Kbd>Pinch</Kbd></>}
          desc="Two-finger pinch in/out on touchpad to zoom."
        />
        <Row
          keys={<><Kbd>⌘</Kbd>/<Kbd>Ctrl</Kbd> + <span className="opacity-50">scroll</span></>}
          desc="Hold Command (Mac) or Control (Win/Linux) and scroll to zoom in/out."
        />
        <Row
          keys={<><Kbd>+</Kbd> / <Kbd>-</Kbd> in Controls</>}
          desc="Use the + / − / fit-view buttons in the bottom-left Controls panel."
        />
      </Section>

      <Section title="Select &amp; move nodes">
        <Row
          keys={<><Kbd>Click</Kbd></>}
          desc="Click any node to select it; the Inspector on the right shows its details."
        />
        <Row
          keys={<><Kbd>Left</Kbd> drag a node</>}
          desc="Drag a node body with the left mouse button to reposition it on the canvas."
        />
        <Row
          keys={<>In Pointer mode: <Kbd>Left</Kbd> drag pane</>}
          desc="Switch to Pointer mode (top-right toggle) and drag the empty canvas to box-select multiple nodes."
        />
      </Section>

      <Section title="Right-click menu (edit BT)">
        <Row
          keys={<><Kbd>Right-click</Kbd> a node</>}
          desc="Open the context menu. Composite nodes (selector / sequence / chance) show 'Add child', most nodes show 'Convert to', and non-root nodes show 'Delete'."
        />
        <Row
          keys={<>Submenus</>}
          desc="'Add child' cascades into Selector / Sequence / Chance / Wait / Action ► / Condition ►. Pick an action or condition from the registry sublist to insert it with default arguments."
        />
        <Row
          keys={<>'Convert to'</>}
          desc="Only valid targets are listed. Composite ↔ composite preserves children; leaf ↔ leaf resets payload; composite → leaf is blocked when the composite still has children — delete them first."
        />
        <Row
          keys={<>'Delete'</>}
          desc="Hidden on the root node. On chance branches it removes the {weight, node} pair together. Deleting a composite cascades over all descendants."
        />
      </Section>

      <Section title="Add root (when state has no BT)">
        <Row
          keys={<><Kbd>Add root</Kbd> button</>}
          desc="If a decision state has no BT yet, the canvas shows an 'Add root' button. Click it and pick a type from the same cascade to seed a root node."
        />
      </Section>

      <Section title="Modes (top-right toggle)">
        <Row
          keys={<>Hand</>}
          desc="Default mode. Left-click drags nodes, middle/right drag pans the canvas."
        />
        <Row
          keys={<>Pointer</>}
          desc="Left-click drags nodes; left-drag on empty pane creates a selection box."
        />
      </Section>

      <Section title="Undo / Redo">
        <Row
          keys={<><Kbd>⌘</Kbd>/<Kbd>Ctrl</Kbd> + <Kbd>Z</Kbd></>}
          desc="Undo the last edit. Skipped while focus is in a text input (native field undo runs instead)."
        />
        <Row
          keys={<><Kbd>⌘</Kbd> + <Kbd>Shift</Kbd> + <Kbd>Z</Kbd></>}
          desc="Redo (Mac)."
        />
        <Row
          keys={<><Kbd>Ctrl</Kbd> + <Kbd>Y</Kbd></>}
          desc="Redo (Win/Linux)."
        />
        <Row
          keys={<>TopBar buttons</>}
          desc="↶ Undo and ↷ Redo buttons in the TopBar mirror the keyboard. They disable when their stack is empty. Fast typing inside an input is coalesced into a single undo step (400ms window)."
        />
      </Section>

      <Section title="Save &amp; reload">
        <Row
          keys={<>Save</>}
          desc="Click Save in the TopBar to persist changes. The save is blocked when validation fails (red badge); the unsaved-edit badge clears on successful save."
        />
        <Row
          keys={<><Kbd>F5</Kbd> in-game</>}
          desc="Press F5 inside the game window to reparse all behavior JSON files; a toast confirms success or shows the first error."
        />
      </Section>
    </div>
  )
}
