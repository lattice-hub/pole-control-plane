import React from 'react';
import {
  Accordion,
  AccordionHeader,
  AccordionItem,
  AccordionPanel,
  Button,
  Checkbox,
  Dropdown as FluentSelect,
  Input,
  List as FluentList,
  ListItem as FluentListItem,
  Menu as FluentMenu,
  MenuItem as FluentMenuItem,
  MenuList,
  MenuPopover,
  MenuTrigger,
  Radio,
  RadioGroup,
  Slider,
  Spinner,
  Option,
  Tooltip,
} from '@fluentui/react-components';
import { ChevronDown16Regular, ChevronRight16Regular } from '@fluentui/react-icons';
import type { DateRangePickerProps, DateRangeValue } from './DateRangePicker';

type AnyRecord = Record<string, any>;
type LegacyValue = string | number;

const palette = {
  border: 'var(--app-border, #d1d1d1)',
  subtle: 'var(--app-surface-subtle, #f5f5f5)',
  text: 'var(--app-text, #242424)',
  muted: 'var(--app-text-secondary, #616161)',
  brand: 'var(--app-brand, #0f6cbd)',
};

export interface BreadcrumbProps extends React.HTMLAttributes<HTMLElement> {
  maxItemWidth?: string | number;
  separator?: React.ReactNode;
}

const BreadcrumbItem: React.FC<React.HTMLAttributes<HTMLElement>> = ({ children, className, style, onClick, ...props }) => {
  const itemClassName = `fluent-breadcrumb__item ${className || ''}`.trim();
  const itemStyle = { display: 'inline-flex', alignItems: 'center', minWidth: 0, ...style };
  if (onClick) {
    return (
      <Button
        {...props as React.ComponentProps<typeof Button>}
        appearance="transparent"
        className={itemClassName}
        style={{
          ...itemStyle,
          padding: 0,
          color: 'inherit',
          font: 'inherit',
          cursor: 'pointer',
        }}
        onClick={onClick as React.MouseEventHandler<HTMLButtonElement>}
      >
        {children}
      </Button>
    );
  }
  return (
    <span {...props} className={itemClassName} style={itemStyle}>
      {children}
    </span>
  );
};

export const Breadcrumb = Object.assign(
  React.forwardRef<HTMLElement, BreadcrumbProps>(({ children, maxItemWidth, separator = '/', className, style, ...props }, ref) => {
    const items = React.Children.toArray(children);
    return (
      <nav ref={ref} aria-label="breadcrumb" {...props} className={`fluent-breadcrumb ${className || ''}`.trim()} style={{ display: 'flex', alignItems: 'center', gap: 8, ...style }}>
        {items.map((item, index) => (
          <React.Fragment key={index}>
            <span style={{ maxWidth: maxItemWidth, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{item}</span>
            {index < items.length - 1 && <span aria-hidden style={{ color: palette.muted }}>{separator}</span>}
          </React.Fragment>
        ))}
      </nav>
    );
  }),
  { BreadcrumbItem },
);

const CollapsePanel: React.FC<any> = ({ value, header, children, disabled, className }) => (
  <AccordionItem className={`fluent-collapse__panel ${className || ''}`.trim()} value={value} disabled={disabled}>
    <AccordionHeader>{header}</AccordionHeader>
    <AccordionPanel>{children}</AccordionPanel>
  </AccordionItem>
);

export const Collapse = Object.assign(
  ({ children, defaultValue, value, onChange, expandIconPlacement, className, ...props }: any) => (
    <Accordion
      {...props}
      className={`fluent-collapse ${className || ''}`.trim()}
      collapsible
      multiple={Array.isArray(value || defaultValue)}
      defaultOpenItems={defaultValue}
      openItems={value}
      onToggle={(_event, data) => onChange?.(data.openItems)}
    >
      {children}
    </Accordion>
  ),
  { Panel: CollapsePanel, CollapsePanel },
);

export interface ColorPickerPanelProps extends Omit<React.HTMLAttributes<HTMLDivElement>, 'onChange' | 'defaultValue'> {
  value?: string;
  defaultValue?: string;
  onChange?: (value: string) => void;
  swatchColors?: string[];
  colorModes?: string[];
  format?: string;
}

export const ColorPickerPanel: React.FC<ColorPickerPanelProps> = ({ value, defaultValue = '#005fb8', onChange, swatchColors = [], colorModes: _colorModes, format: _format, className, style, ...props }) => {
  const [current, setCurrent] = React.useState(defaultValue);
  const selected = value ?? current;
  const update = (next: string) => {
    if (value === undefined) setCurrent(next);
    onChange?.(next.toUpperCase());
  };
  const channels = /^#[0-9a-f]{6}$/i.test(selected)
    ? [Number.parseInt(selected.slice(1, 3), 16), Number.parseInt(selected.slice(3, 5), 16), Number.parseInt(selected.slice(5, 7), 16)]
    : [0, 95, 184];
  const updateChannel = (index: number, channel: number) => {
    const next = [...channels];
    next[index] = channel;
    update(`#${next.map((item) => Math.round(item).toString(16).padStart(2, '0')).join('')}`);
  };
  return (
    <div {...props} className={`fluent-color-picker ${className || ''}`.trim()} style={{ width: 260, padding: 16, background: 'var(--app-surface, #fff)', color: palette.text, ...style }}>
      <div className="fluent-color-picker__preview" aria-label={`当前主题色 ${selected}`} style={{ width: '100%', height: 96, marginBottom: 12, borderRadius: 4, border: `1px solid ${palette.border}`, background: selected }} />
      {(['红', '绿', '蓝'] as const).map((label, index) => (
        <label className="fluent-color-picker__channel" key={label} style={{ display: 'grid', gridTemplateColumns: '28px 1fr 36px', alignItems: 'center', gap: 8, marginBottom: 6 }}>
          <span>{label}</span>
          <Slider aria-label={`${label}色通道`} min={0} max={255} value={channels[index]} onChange={(_event, data) => updateChannel(index, data.value)} />
          <span>{channels[index]}</span>
        </label>
      ))}
      <Input value={selected.toUpperCase()} onChange={(_event, data) => /^#[0-9a-f]{6}$/i.test(data.value) && update(data.value)} />
      {swatchColors.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 12 }}>
          {swatchColors.map((color) => (
            <Button key={color} className="fluent-color-picker__swatch" aria-label={color} appearance="subtle" shape="circular" size="small" onClick={() => update(color)} style={{ minWidth: 24, width: 24, height: 24, border: `1px solid ${palette.border}`, background: color }} />
          ))}
        </div>
      )}
    </div>
  );
};

export const TimeRangePicker: React.FC<DateRangePickerProps> = ({ value, defaultValue, onChange, placeholder = ['开始时间', '结束时间'], disabled, className, style, ...props }) => {
  const [inner, setInner] = React.useState<DateRangeValue>(defaultValue || []);
  const selected = value ?? inner;
  const update = (index: number, next: string) => {
    const result: DateRangeValue = [selected[0] || '', selected[1] || ''];
    result[index] = next;
    if (value === undefined) setInner(result);
    onChange?.(result);
  };
  return <div {...props} className={`fluent-time-range ${className || ''}`.trim()} style={{ display: 'inline-flex', alignItems: 'center', gap: 6, ...style }}>{[0, 1].map((index) => <React.Fragment key={index}><Input className="fluent-time-range__input" disabled={disabled} aria-label={placeholder[index]} type="time" value={String(selected[index] || '')} onChange={(_event, data) => update(index, data.value)} />{index === 0 && <span style={{ color: palette.muted }}>—</span>}</React.Fragment>)}</div>;
};

const DescriptionsItem: React.FC<any> = ({ label, children, span = 1, className, style }) => (
  <div className={`fluent-descriptions__item ${className || ''}`.trim()} style={{ display: 'grid', gridTemplateColumns: 'minmax(88px, auto) minmax(0, 1fr)', gridColumn: `span ${span}`, minHeight: 40, borderBottom: `1px solid ${palette.border}`, ...style }}>
    <div className="fluent-descriptions__label" style={{ padding: '10px 12px', color: palette.muted, background: palette.subtle }}>{label}</div>
    <div className="fluent-descriptions__content" style={{ padding: '10px 12px', minWidth: 0 }}>{children}</div>
  </div>
);

export const Descriptions = Object.assign(
  ({ children, column = 2, className, style, ...props }: any) => <div {...props} className={`fluent-descriptions ${className || ''}`.trim()} style={{ display: 'grid', gridTemplateColumns: `repeat(${typeof column === 'number' ? column : 2}, minmax(0, 1fr))`, border: `1px solid ${palette.border}`, borderBottom: 0, borderRadius: 4, overflow: 'hidden', ...style }}>{children}</div>,
  { DescriptionsItem },
);

interface DropdownContextValue { close: () => void; select: (data: AnyRecord, event: React.MouseEvent) => void }
const DropdownContext = React.createContext<DropdownContextValue | null>(null);
const DropdownMenu: React.FC<React.HTMLAttributes<HTMLDivElement>> = ({ children, className, ...props }) => <div className={`fluent-dropdown__menu ${className || ''}`.trim()} role="menu" {...props}>{children}</div>;
const DropdownItem: React.FC<any> = ({ value, children, onClick, disabled, className }) => {
  const context = React.useContext(DropdownContext);
  return <FluentMenuItem className={`fluent-dropdown__item ${className || ''}`.trim()} disabled={disabled} onClick={(event) => { onClick?.(event); context?.select({ value, content: children }, event); }}>{children}</FluentMenuItem>;
};
export const Dropdown = Object.assign(
  ({ children, options, onClick, trigger = 'hover', placement, minColumnWidth, maxHeight }: any) => {
    const nodes = React.Children.toArray(children);
    const triggerNode = nodes[0] as React.ReactElement;
    const menuNode = nodes.length > 1 ? nodes.slice(1) : options?.map((item: any) => <DropdownItem key={item.value} {...item}>{item.content}</DropdownItem>);
    return <FluentMenu positioning={{ position: placement?.includes('top') ? 'above' : 'below' }} openOnHover={trigger === 'hover'}><MenuTrigger disableButtonEnhancement>{triggerNode}</MenuTrigger><MenuPopover className="fluent-dropdown"><DropdownContext.Provider value={{ close: () => undefined, select: (data, event) => onClick?.(data, { e: event }) }}><MenuList className="fluent-dropdown__list" style={{ minWidth: minColumnWidth, maxHeight, overflowY: 'auto' }}>{menuNode}</MenuList></DropdownContext.Provider></MenuPopover></FluentMenu>;
  },
  { DropdownMenu, DropdownItem },
);

export const List = Object.assign(FluentList, { ListItem: FluentListItem });
export type MenuValue = LegacyValue;
export const Menu = Object.assign(FluentMenu, { MenuItem: FluentMenuItem, MenuList, MenuPopover, MenuTrigger });

export interface PopupProps extends Omit<React.HTMLAttributes<HTMLSpanElement>, 'content'> {
  content?: React.ReactNode;
  trigger?: 'hover' | 'click' | 'focus';
  placement?: string;
  visible?: boolean;
  onVisibleChange?: (visible: boolean, context?: AnyRecord) => void;
  showArrow?: boolean;
  destroyOnClose?: boolean;
  overlayInnerStyle?: React.CSSProperties;
  expandAnimation?: boolean;
}

export const Popup: React.FC<PopupProps> = ({ content, trigger = 'hover', children, visible, onVisibleChange, placement = 'top', overlayInnerStyle, showArrow: _showArrow, destroyOnClose: _destroyOnClose, expandAnimation: _expandAnimation, ...props }) => {
  const [innerOpen, setInnerOpen] = React.useState(false);
  if (trigger === 'hover') {
    return <Tooltip content={{ children: <span className="fluent-popup__content" style={overlayInnerStyle}>{content}</span> }} relationship="description" positioning={placement as any}>{React.Children.only(children) as React.ReactElement}</Tooltip>;
  }
  const node = React.Children.only(children) as React.ReactElement;
  const open = visible ?? innerOpen;
  const setOpen = (next: boolean) => { if (visible === undefined) setInnerOpen(next); onVisibleChange?.(next); };
  return <span {...props} className={`fluent-popup ${props.className || ''}`.trim()} style={{ position: 'relative', display: 'inline-flex', ...props.style }}>{React.cloneElement(node, { onClick: (event: React.MouseEvent) => { node.props.onClick?.(event); setOpen(!open); } })}{open && <span className="fluent-popup__content" role="dialog" style={{ position: 'absolute', zIndex: 10000, top: placement.includes('top') ? 'auto' : 'calc(100% + 6px)', bottom: placement.includes('top') ? 'calc(100% + 6px)' : 'auto', right: placement.includes('right') ? 0 : 'auto', left: placement.includes('right') ? 'auto' : 0, padding: 8, borderRadius: 4, boxShadow: '0 8px 24px rgba(0,0,0,.18)', background: 'var(--app-surface, #fff)', ...overlayInnerStyle }}>{content}</span>}</span>;
};

export const SelectInput: React.FC<any> = ({ value, inputValue, onInputChange, placeholder, disabled, readonly, clearable, children, className, ...props }) => <span {...props} className={`fluent-select-input ${className || ''}`.trim()} style={{ display: 'inline-flex', alignItems: 'center', ...props.style }}><Input value={inputValue ?? (Array.isArray(value) ? value.join(', ') : value ?? '')} placeholder={placeholder} disabled={disabled} readOnly={readonly} onChange={(_event, data) => onInputChange?.(data.value, { trigger: 'input' })} contentAfter={<ChevronDown16Regular />} />{children}</span>;

const StepItem: React.FC<any> = ({ title, content, children, status, index, current, layout, className }) => {
  const active = index === current;
  const complete = index < current || status === 'finish';
  return <div className={`fluent-steps__item ${active ? 'is-active' : ''} ${complete ? 'is-complete' : ''} ${className || ''}`.trim()} style={{ display: 'flex', alignItems: 'flex-start', flex: layout === 'vertical' ? undefined : 1, gap: 10, minHeight: layout === 'vertical' ? 56 : undefined }}><span className="fluent-steps__indicator" style={{ display: 'inline-grid', placeItems: 'center', flex: '0 0 28px', width: 28, height: 28, borderRadius: '50%', border: `2px solid ${active || complete ? palette.brand : palette.border}`, background: complete ? palette.brand : 'transparent', color: complete ? '#fff' : active ? palette.brand : palette.muted }}>{complete ? '✓' : index + 1}</span><span className="fluent-steps__body"><strong style={{ color: active ? palette.brand : palette.text }}>{title || children}</strong>{content && <div style={{ color: palette.muted, marginTop: 4 }}>{content}</div>}</span></div>;
};
export const Steps = Object.assign(({ children, current = 0, layout = 'horizontal', onChange, readonly, className, style, ...props }: any) => <div {...props} className={`fluent-steps fluent-steps--${layout} ${className || ''}`.trim()} style={{ display: 'flex', flexDirection: layout === 'vertical' ? 'column' : 'row', gap: layout === 'vertical' ? 4 : 20, ...style }}>{React.Children.map(children, (child, index) => React.isValidElement(child) ? <span role={readonly ? undefined : 'button'} tabIndex={readonly ? undefined : 0} onClick={() => !readonly && onChange?.(index)} style={{ display: 'contents' }}>{React.cloneElement(child as React.ReactElement<any>, { index, current, layout })}</span> : child)}</div>, { StepItem });

const StickyItem: React.FC<any> = ({ label, icon, children, onClick, className, style }) => <Button className={`fluent-sticky-tool__item ${className || ''}`.trim()} appearance="secondary" icon={icon} onClick={onClick} style={{ minWidth: 40, ...style }}>{label || children}</Button>;
export const StickyTool = Object.assign(({ children, placement = 'right-bottom', offset = [16, 16], className, style, ...props }: any) => <div {...props} className={`fluent-sticky-tool fluent-sticky-tool--${placement} ${className || ''}`.trim()} style={{ position: 'fixed', display: 'flex', flexDirection: 'column', gap: 8, right: Math.abs(offset[0] ?? 16), bottom: Math.abs(offset[1] ?? 16), ...style }}>{children}</div>, { StickyItem });

export type TransferValue = LegacyValue[];
export const Transfer: React.FC<any> = ({ data = [], value, defaultValue = [], onChange, disabled, loading = false, search, title = ['源列表', '目标列表'], className, style, ...props }) => {
  const [inner, setInner] = React.useState<TransferValue>(defaultValue);
  const [query, setQuery] = React.useState('');
  const selected: TransferValue = value ?? inner;
  const choose = (item: AnyRecord, checked: boolean) => {
    const next = checked ? [...selected, item.value] : selected.filter((entry) => entry !== item.value);
    if (value === undefined) setInner(next);
    if (!loading) onChange?.(next, { type: checked ? 'source' : 'target', movedValue: [item.value] });
  };
  const visible = data.filter((item: AnyRecord) => String(item.label ?? item.value).toLowerCase().includes(query.toLowerCase()));
  const pane = (target: boolean) => {
    const items = visible.filter((item: AnyRecord) => selected.includes(item.value) === target);
    return <div className={`fluent-transfer__pane fluent-transfer__pane--${target ? 'target' : 'source'}`} style={{ flex: 1, minWidth: 180, height: 260, overflow: 'auto', border: `1px solid ${palette.border}`, borderRadius: 4 }}><div className="fluent-transfer__title" style={{ padding: '9px 12px', borderBottom: `1px solid ${palette.border}`, fontWeight: 600 }}>{title[target ? 1 : 0]}</div>{items.map((item: AnyRecord) => <Checkbox className="fluent-transfer__item" key={item.value} checked={target} disabled={disabled || loading || item.disabled} label={item.label ?? item.value} onChange={(_event, data) => choose(item, Boolean(data.checked))} style={{ display: 'flex', padding: '7px 12px' }} />)}{!loading && items.length === 0 && <div className="fluent-transfer__empty" style={{ display: 'grid', minHeight: 180, placeItems: 'center', color: palette.muted }}>暂无数据</div>}</div>;
  };
  return <div {...props} aria-busy={loading || undefined} className={`fluent-transfer ${className || ''}`.trim()} style={{ position: 'relative', width: '100%', ...style }}>{search && <Input aria-label="搜索列表项" className="fluent-transfer__search" value={query} placeholder="搜索" disabled={disabled || loading} onChange={(_event, data) => setQuery(data.value)} style={{ marginBottom: 8 }} />}<div className="fluent-transfer__panes" style={{ display: 'flex', gap: 12 }}>{pane(false)}{pane(true)}</div>{loading && <div className="fluent-loading-overlay" role="status"><Spinner label="正在加载成员列表" /></div>}</div>;
};

export interface TreeNodeData extends AnyRecord { value: LegacyValue; label?: React.ReactNode; children?: TreeNodeData[] | boolean }
export interface TreeNodeModel {
  value: LegacyValue;
  label?: React.ReactNode;
  data: TreeNodeData;
  expanded: boolean;
  loading: boolean;
  getChildren: (deep?: boolean) => TreeNodeModel[] | boolean;
}
export interface TreeInstanceFunctions { expandAll?: () => void; collapseAll?: () => void }
export interface TreeProps extends Omit<React.HTMLAttributes<HTMLDivElement>, 'onClick'> {
  data?: TreeNodeData[];
  value?: LegacyValue[];
  actived?: LegacyValue[];
  defaultActived?: LegacyValue[];
  expandAll?: boolean;
  icon?: React.ReactNode | ((node: TreeNodeModel) => React.ReactNode);
  operations?: React.ReactNode | ((node: TreeNodeModel) => React.ReactNode);
  filter?: (node: TreeNodeModel) => boolean;
  onClick?: (context: { node: TreeNodeModel; e: React.MouseEvent }) => void;
  onActive?: (value: LegacyValue[], context: { node: TreeNodeModel; e: React.MouseEvent }) => void;
  activable?: boolean;
  disabled?: boolean;
  [key: string]: any;
}

export const Tree = React.forwardRef<TreeInstanceFunctions, TreeProps>(({ data = [], actived, defaultActived = [], expandAll, icon, operations, filter, onClick, onActive, activable, disabled = false, className, style }, ref) => {
  const [expanded, setExpanded] = React.useState<LegacyValue[]>([]);
  const [active, setActive] = React.useState<LegacyValue[]>(defaultActived);
  const [focused, setFocused] = React.useState<LegacyValue | null>(null);
  const [focusWithin, setFocusWithin] = React.useState(false);
  const itemRefs = React.useRef(new Map<LegacyValue, HTMLDivElement>());
  const flattenValues = React.useCallback((nodes: TreeNodeData[]): LegacyValue[] => nodes.flatMap((node) => [...(node.disabled ? [] : [node.value]), ...(Array.isArray(node.children) ? flattenValues(node.children) : [])]), []);
  React.useImperativeHandle(ref, () => ({ expandAll: () => { if (!disabled) setExpanded(flattenValues(data)); }, collapseAll: () => setExpanded([]) }), [data, disabled, flattenValues]);
  React.useEffect(() => { if (expandAll && !disabled) setExpanded(flattenValues(data)); }, [data, disabled, expandAll, flattenValues]);
  const selected = actived ?? active;
  const matchesFilter = (node: TreeNodeData): boolean => {
    const model: TreeNodeModel = { value: node.value, label: node.label, data: node, expanded: expanded.includes(node.value), loading: false, getChildren: () => false };
    return !filter || filter(model) || (Array.isArray(node.children) && node.children.some(matchesFilter));
  };
  const visible: { node: TreeNodeData; parent: LegacyValue | null }[] = [];
  const collectVisible = (nodes: TreeNodeData[], parent: LegacyValue | null) => {
    nodes.forEach((node) => {
      if (!matchesFilter(node)) return;
      visible.push({ node, parent });
      if (expanded.includes(node.value) && Array.isArray(node.children)) collectVisible(node.children, node.value);
    });
  };
  collectVisible(data, null);
  const enabled = visible.filter(({ node }) => !disabled && !node.disabled);
  const tabStop = enabled.some(({ node }) => node.value === focused)
    ? focused : enabled.find(({ node }) => selected.includes(node.value))?.node.value ?? enabled[0]?.node.value;
  const focusItem = (value: LegacyValue | undefined | null) => {
    if (value == null) return;
    setFocused(value);
    itemRefs.current.get(value)?.focus();
  };
  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>, node: TreeNodeData) => {
    // Controls inside operations own their keys; only the treeitem itself navigates the tree.
    if (event.target !== event.currentTarget || disabled || node.disabled) return;
    const index = enabled.findIndex((entry) => entry.node.value === node.value);
    const children = Array.isArray(node.children) ? node.children : [];
    const open = expanded.includes(node.value);
    let target: LegacyValue | undefined | null;
    switch (event.key) {
      case 'ArrowDown': target = enabled[index + 1]?.node.value; break;
      case 'ArrowUp': target = enabled[index - 1]?.node.value; break;
      case 'Home': target = enabled[0]?.node.value; break;
      case 'End': target = enabled[enabled.length - 1]?.node.value; break;
      case 'ArrowRight':
        if (children.length && !open) setExpanded((current) => [...current, node.value]);
        else if (open) target = enabled.find((entry) => entry.parent === node.value)?.node.value;
        break;
      case 'ArrowLeft':
        if (children.length && open) setExpanded((current) => current.filter((value) => value !== node.value));
        else {
          let parent = visible.find((entry) => entry.node.value === node.value)?.parent;
          while (parent != null && !enabled.some((entry) => entry.node.value === parent)) {
            parent = visible.find((entry) => entry.node.value === parent)?.parent;
          }
          target = parent;
        }
        break;
      case 'Enter':
      case ' ':
        event.currentTarget.querySelector<HTMLDivElement>(':scope > .fluent-tree__item')?.click();
        break;
      default: return;
    }
    event.preventDefault();
    event.stopPropagation();
    focusItem(target);
  };
  const renderNodes = (nodes: TreeNodeData[], depth = 0): React.ReactNode => nodes.map((node) => {
    const children = Array.isArray(node.children) ? node.children : [];
    const open = expanded.includes(node.value);
    const nodeDisabled = disabled || Boolean(node.disabled);
    const model: TreeNodeModel = { value: node.value, label: node.label, data: node, expanded: open, loading: false, getChildren: () => children.length ? children.map((child) => ({ value: child.value, label: child.label, data: child, expanded: false, loading: false, getChildren: () => false })) : false };
    if (!matchesFilter(node)) return null;
    return <div className="fluent-tree__node" key={node.value} role="treeitem" aria-selected={selected.includes(node.value)} aria-expanded={children.length ? open : undefined} aria-level={depth + 1} aria-disabled={nodeDisabled || undefined} tabIndex={!nodeDisabled && tabStop === node.value ? 0 : -1} ref={(element) => { if (element) itemRefs.current.set(node.value, element); else itemRefs.current.delete(node.value); }} onFocus={(event) => { if (event.target === event.currentTarget) { setFocused(node.value); setFocusWithin(true); } }} onBlur={(event) => { if (event.target === event.currentTarget) setFocusWithin(false); }} onKeyDown={(event) => handleKeyDown(event, node)}>
      <div className={`fluent-tree__item ${selected.includes(node.value) ? 'is-active' : ''} ${nodeDisabled ? 'is-disabled' : ''}`.trim()} onClick={(event) => { if (nodeDisabled || (event.target instanceof Element && event.target.closest('.fluent-tree__operations'))) return; focusItem(node.value); if (activable) { const next = [node.value]; if (actived === undefined) setActive(next); onActive?.(next, { node: model, e: event }); } onClick?.({ node: model, e: event }); }} style={{ display: 'flex', alignItems: 'center', minHeight: 32, paddingLeft: depth * 20, borderRadius: 4, cursor: nodeDisabled ? 'default' : 'pointer', background: selected.includes(node.value) ? 'color-mix(in srgb, var(--app-brand, #0f6cbd) 12%, transparent)' : undefined, color: nodeDisabled ? palette.muted : palette.text, outline: focusWithin && focused === node.value ? '2px solid var(--app-brand, #0f6cbd)' : undefined, outlineOffset: -2 }}><Button className="fluent-tree__expand" appearance="subtle" size="small" aria-label={open ? '收起' : '展开'} tabIndex={-1} disabled={nodeDisabled || !children.length} icon={children.length ? open ? <ChevronDown16Regular /> : <ChevronRight16Regular /> : undefined} onClick={(event) => { event.stopPropagation(); focusItem(node.value); setExpanded((current) => open ? current.filter((item) => item !== node.value) : [...current, node.value]); }} style={{ minWidth: 24, width: 24, padding: 0 }} /><span className="fluent-tree__icon" style={{ display: 'inline-flex', width: 20 }}>{typeof icon === 'function' ? icon(model) : icon}</span><span className="fluent-tree__label" style={{ flex: 1, minWidth: 0 }}>{node.label ?? node.value}</span><span className="fluent-tree__operations">{typeof operations === 'function' ? operations(model) : operations}</span></div>
      {open && children.length > 0 && <div className="fluent-tree__children" role="group">{renderNodes(children, depth + 1)}</div>}
    </div>;
  });
  return <div role="tree" className={`fluent-tree ${className || ''}`.trim()} style={style}>{renderNodes(data)}</div>;
});
Tree.displayName = 'Tree';
