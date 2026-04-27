import {
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
} from '@/components/ui/context-menu'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from '@/components/ui/dropdown-menu'
import type { ActionMeta } from '../api/schemas'
import type { BTNode, BTNodeType } from '../bt/types'
import { canConvert } from '../bt/convert'

export type MenuKind = 'context' | 'dropdown'

export interface NodeMenuActions {
  onAddChild?(child: BTNode): void
  onConvert?(toType: BTNodeType): void
  onDelete?(): void
  onSetRoot?(rootType: BTNodeType, opts?: { name?: string }): void
}

interface Props {
  kind: MenuKind
  node?: BTNode
  isRoot?: boolean
  registry: { actions: ActionMeta[]; conditions: ActionMeta[] }
  actions: NodeMenuActions
}

const ALL_TYPES: BTNodeType[] = ['selector', 'sequence', 'chance', 'action', 'condition', 'wait']

export function NodeContextMenu({ kind, node, isRoot, registry, actions }: Props) {
  // Pane menu (no node) — only "Add root" cascade.
  if (!node) {
    return <AddRootCascade kind={kind} registry={registry} onSetRoot={actions.onSetRoot!} />
  }

  const isComposite = node.type === 'selector' || node.type === 'sequence' || node.type === 'chance'
  const convertOptions = ALL_TYPES.filter(t => canConvert(node, t).ok)

  return (
    <>
      {isComposite && (
        <AddChildCascade kind={kind} registry={registry} onAddChild={actions.onAddChild!} />
      )}
      {convertOptions.length > 0 && (
        <ConvertCascade kind={kind} options={convertOptions} onConvert={actions.onConvert!} />
      )}
      <Sep kind={kind} />
      <Item kind={kind} disabled={isRoot} onSelect={() => actions.onDelete?.()} className="text-destructive">
        Delete
      </Item>
    </>
  )
}

function AddChildCascade({
  kind, registry, onAddChild,
}: { kind: MenuKind; registry: Props['registry']; onAddChild: (c: BTNode) => void }) {
  return (
    <Sub kind={kind} label="Add child">
      <SimpleTypeItems kind={kind} onPick={(t) => onAddChild(makeDefault(t))} />
      <ActionRegistrySub kind={kind} kindLabel="Action" actions={registry.actions} onPick={(name) => onAddChild({ type: 'action', name, args: {} })} />
      <ActionRegistrySub kind={kind} kindLabel="Condition" actions={registry.conditions} onPick={(name) => onAddChild({ type: 'condition', name, args: {} })} />
    </Sub>
  )
}

function AddRootCascade({
  kind, registry, onSetRoot,
}: { kind: MenuKind; registry: Props['registry']; onSetRoot: NonNullable<NodeMenuActions['onSetRoot']> }) {
  return (
    <Sub kind={kind} label="Add root">
      <SimpleTypeItems kind={kind} onPick={(t) => onSetRoot(t)} />
      <ActionRegistrySub kind={kind} kindLabel="Action" actions={registry.actions} onPick={(name) => onSetRoot('action', { name })} />
      <ActionRegistrySub kind={kind} kindLabel="Condition" actions={registry.conditions} onPick={(name) => onSetRoot('condition', { name })} />
    </Sub>
  )
}

function ConvertCascade({
  kind, options, onConvert,
}: { kind: MenuKind; options: BTNodeType[]; onConvert: (t: BTNodeType) => void }) {
  return (
    <Sub kind={kind} label="Convert to">
      {options.map(t => (
        <Item kind={kind} key={t} onSelect={() => onConvert(t)}>{t}</Item>
      ))}
    </Sub>
  )
}

function SimpleTypeItems({ kind, onPick }: { kind: MenuKind; onPick: (t: BTNodeType) => void }) {
  return (
    <>
      <Item kind={kind} onSelect={() => onPick('selector')}>Selector</Item>
      <Item kind={kind} onSelect={() => onPick('sequence')}>Sequence</Item>
      <Item kind={kind} onSelect={() => onPick('chance')}>Chance</Item>
      <Item kind={kind} onSelect={() => onPick('wait')}>Wait</Item>
    </>
  )
}

function ActionRegistrySub({
  kind, kindLabel, actions, onPick,
}: { kind: MenuKind; kindLabel: string; actions: ActionMeta[]; onPick: (name: string) => void }) {
  return (
    <Sub kind={kind} label={kindLabel}>
      {actions.length === 0 && <Item kind={kind} disabled>(none registered)</Item>}
      {actions.map(a => (
        <Item kind={kind} key={a.name} onSelect={() => onPick(a.name)}>{a.name}</Item>
      ))}
    </Sub>
  )
}

function makeDefault(t: BTNodeType): BTNode {
  if (t === 'selector' || t === 'sequence') return { type: t, children: [] }
  if (t === 'chance') return { type: 'chance', branches: [] }
  if (t === 'wait') return { type: 'wait', seconds: 1 }
  return { type: t as 'action' | 'condition', name: '', args: {} }
}

function Sub({ kind, label, children }: { kind: MenuKind; label: string; children: React.ReactNode }) {
  if (kind === 'context') {
    return (
      <ContextMenuSub>
        <ContextMenuSubTrigger>{label}</ContextMenuSubTrigger>
        <ContextMenuSubContent>{children}</ContextMenuSubContent>
      </ContextMenuSub>
    )
  }
  return (
    <DropdownMenuSub>
      <DropdownMenuSubTrigger>{label}</DropdownMenuSubTrigger>
      <DropdownMenuSubContent>{children}</DropdownMenuSubContent>
    </DropdownMenuSub>
  )
}

function Item({
  kind, onSelect, disabled, className, children,
}: {
  kind: MenuKind; onSelect?: () => void; disabled?: boolean; className?: string; children: React.ReactNode
}) {
  if (kind === 'context') {
    return <ContextMenuItem disabled={disabled} className={className} onSelect={onSelect}>{children}</ContextMenuItem>
  }
  return <DropdownMenuItem disabled={disabled} className={className} onSelect={onSelect}>{children}</DropdownMenuItem>
}

function Sep({ kind }: { kind: MenuKind }) {
  return kind === 'context' ? <ContextMenuSeparator /> : <DropdownMenuSeparator />
}
