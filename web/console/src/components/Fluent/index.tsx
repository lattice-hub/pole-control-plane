import React from 'react';
import {
  Avatar as FluentAvatar,
  Badge as FluentBadge,
  Button as FluentButton,
  Checkbox as FluentCheckbox,
  Combobox,
  Divider as FluentDivider,
  Dialog as FluentDialog,
  DialogActions,
  DialogBody,
  DialogContent,
  DialogSurface,
  DialogTitle,
  DrawerBody,
  DrawerFooter,
  DrawerHeader,
  DrawerHeaderTitle,
  Dropdown as FluentDropdown,
  Input as FluentInput,
  Label,
  Link as FluentLink,
  Option,
  OverlayDrawer,
  Popover,
  PopoverSurface,
  PopoverTrigger,
  ProgressBar,
  Radio as FluentRadio,
  RadioGroup as FluentRadioGroup,
  Spinner,
  SpinButton,
  Switch as FluentSwitch,
  Tab,
  TabList,
  Table as FluentTable,
  TableBody,
  TableCell,
  TableHeader,
  TableHeaderCell,
  TableRow,
  Tag as FluentTag,
  Textarea as FluentTextarea,
  Tooltip as FluentTooltip,
} from '@fluentui/react-components';
import { Dismiss20Regular } from '@fluentui/react-icons';
import { showFluentToast } from './toast';
import { formControlKind } from './form';

export {
  Breadcrumb,
  Collapse,
  ColorPickerPanel,
  Descriptions,
  Dropdown,
  List,
  Menu,
  Popup,
  SelectInput,
  Steps,
  StickyTool,
  TimeRangePicker,
  Transfer,
  Tree,
} from './legacyWidgets';
export { DateRangePicker } from './DateRangePicker';

export type {
  CustomValidator,
  FieldData,
  FormInstanceFunctions,
  FormProps,
  InternalFormInstance,
  NamePath,
  SubmitContext,
} from './form';
export { Form, FormItem } from './form';

export type {
  MenuValue,
  TransferValue,
  TreeInstanceFunctions,
  TreeNodeModel,
  TreeProps,
} from './legacyWidgets';
export type { DateRangePickerProps, DateRangeValue } from './DateRangePicker';

export interface PageInfo {
  current?: number;
  pageSize?: number;
  total?: number;
  previous?: number;
}

export interface PaginationProps extends PageInfo {
  defaultCurrent?: number;
  defaultPageSize?: number;
  pageSizeOptions?: Array<number | { label: string; value: number }>;
  onChange?: (pageInfo: PageInfo) => void;
  [key: string]: any;
}

export interface TableRowData {
  rowKey?: string | number;
  [key: string]: any;
}

export interface TableColumnData<T extends TableRowData = TableRowData> {
  colKey?: string;
  title?: React.ReactNode;
  cell?: React.ReactNode | ((context: { row: T; rowIndex: number; col: TableColumnData<T> }) => React.ReactNode);
  width?: string | number;
  minWidth?: string | number;
  fixed?: 'left' | 'right';
  ellipsis?: boolean;
  align?: 'left' | 'center' | 'right';
  [key: string]: any;
}

export interface PrimaryTableProps<T extends TableRowData = TableRowData> extends React.HTMLAttributes<HTMLDivElement> {
  data?: T[];
  columns?: TableColumnData<T>[];
  rowKey?: string | ((row: T) => React.Key);
  loading?: boolean;
  pagination?: PaginationProps | false;
  empty?: React.ReactNode;
  onPageChange?: (pageInfo: PageInfo) => void;
  ariaLabel?: string;
  [key: string]: any;
}

export type TableProps<T extends TableRowData = TableRowData> = PrimaryTableProps<T>;

type LegacySize = 'small' | 'medium' | 'large';
type LegacyTheme = 'default' | 'primary' | 'danger' | 'warning' | 'success';
type LegacyVariant = 'base' | 'outline' | 'dashed' | 'text';

export interface ButtonProps extends Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, 'size' | 'onClick'> {
  theme?: LegacyTheme;
  variant?: LegacyVariant;
  shape?: 'rectangle' | 'square' | 'round' | 'circle';
  size?: LegacySize;
  icon?: React.ReactNode;
  loading?: boolean;
  block?: boolean;
  onClick?: (event: React.MouseEvent<HTMLButtonElement>) => void;
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>((props, ref) => {
  const {
    theme = 'default',
    variant = 'base',
    shape = 'rectangle',
    size = 'medium',
    icon,
    loading,
    block,
    children,
    disabled,
    style,
    ...rest
  } = props;
  const appearance = theme === 'primary' ? 'primary' : variant === 'text' ? 'subtle' : variant === 'outline' || variant === 'dashed' ? 'secondary' : 'secondary';
  const fluentSize = size === 'large' ? 'large' : size === 'small' ? 'small' : 'medium';
  const compact = shape === 'square' || shape === 'circle';
  const compactSize = size === 'large' ? 40 : size === 'small' ? 24 : 32;
  return (
    <FluentButton
      {...rest}
      ref={ref}
      appearance={appearance}
      shape={shape === 'circle' ? 'circular' : shape === 'square' ? 'square' : 'rounded'}
      size={fluentSize}
      icon={loading ? <Spinner size="tiny" /> : icon}
      disabled={disabled || loading}
      style={{
        width: block ? '100%' : compact ? compactSize : undefined,
        minWidth: block ? undefined : compact ? compactSize : undefined,
        maxWidth: block ? undefined : compact ? compactSize : undefined,
        paddingInline: compact ? 0 : undefined,
        ...(theme === 'danger' ? { color: '#b10e1c' } : {}),
        ...style,
      }}
    >
      {children}
    </FluentButton>
  );
});
Button.displayName = 'Button';

export interface InputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'size' | 'onChange' | 'prefix'> {
  size?: LegacySize;
  prefixIcon?: React.ReactNode;
  suffixIcon?: React.ReactNode;
  clearable?: boolean;
  readonly?: boolean;
  status?: 'default' | 'success' | 'warning' | 'error';
  onClear?: () => void;
  onChange?: (value: string, context?: any) => void;
  onEnter?: (value: string, context?: any) => void;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>((props, ref) => {
  const { prefixIcon, suffixIcon, readonly, onChange, onEnter, onClear, size, clearable, status, onKeyDown, value, defaultValue, className, ...rest } = props;
  const controlled = Object.prototype.hasOwnProperty.call(props, 'value');
  const [innerValue, setInnerValue] = React.useState(String(value ?? defaultValue ?? ''));

  React.useEffect(() => {
    if (controlled) setInnerValue(String(value ?? ''));
  }, [controlled, value]);

  return (
    <FluentInput
      {...rest}
      ref={ref}
      className={`${className || ''} ${status && status !== 'default' ? `fluent-input-status-${status}` : ''}`.trim()}
      value={innerValue}
      readOnly={readonly ?? rest.readOnly}
      size={size === 'large' ? 'large' : size === 'small' ? 'small' : 'medium'}
      contentBefore={prefixIcon}
      contentAfter={(suffixIcon || ((clearable || onClear) && innerValue)) ? (
        <span className="fluent-input-trailing">
          {suffixIcon}
          {(clearable || onClear) && innerValue && (
            <FluentButton
              appearance="transparent"
              aria-label="清空输入"
              icon={<Dismiss20Regular />}
              size="small"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => {
                setInnerValue('');
                onChange?.('', {});
                onClear?.();
              }}
            />
          )}
        </span>
      ) : undefined}
      appearance="outline"
      onChange={(event, data) => {
        setInnerValue(data.value);
        onChange?.(data.value, { e: event });
      }}
      onKeyDown={(event) => {
        onKeyDown?.(event);
        if (event.key === 'Enter') onEnter?.((event.currentTarget as HTMLInputElement).value, { e: event });
      }}
    />
  );
});
Input.displayName = 'Input';

export const InputNumber: React.FC<any> = ({ value, defaultValue, min, max, step = 1, onChange, disabled, readonly, suffix, className, style, placeholder, ...props }) => (
  <span className={`fluent-number-input ${className || ''}`} style={style}>
    <SpinButton
      {...props}
      value={value === undefined || value === '' ? null : Number(value)}
      defaultValue={defaultValue === undefined ? undefined : Number(defaultValue)}
      min={min}
      max={max}
      step={step}
      disabled={disabled}
      readOnly={readonly}
      input={{ readOnly: readonly, placeholder }}
      onChange={(event, data) => {
        const nextValue = data.value ?? (data.displayValue === '' || data.displayValue === undefined
          ? undefined
          : Number(data.displayValue));
        if (nextValue !== undefined && Number.isFinite(nextValue)) {
          onChange?.(nextValue, { e: event });
        }
      }}
    />
    {suffix && <span className="fluent-input-suffix">{suffix}</span>}
  </span>
);

export const InputAdornment: React.FC<any> = ({ prepend, append, children, className, style }) => (
  <span className={`fluent-input-adornment ${className || ''}`} style={style}>
    {prepend && <span>{prepend}</span>}
    {children}
    {append && <span>{append}</span>}
  </span>
);

export const TagInput: React.FC<any> = ({ value = [], onChange, placeholder, disabled, readonly, className, style, ...props }) => {
  const values = Array.isArray(value) ? value : [];
  const [draft, setDraft] = React.useState('');
  const commit = () => {
    const nextValue = draft.trim();
    if (!nextValue || values.includes(nextValue)) return setDraft('');
    onChange?.([...values, nextValue]);
    setDraft('');
  };
  return (
    <span className={`fluent-tag-input ${className || ''}`} style={style}>
      {values.map((item: string, index: number) => (
        <FluentTag key={`${item}-${index}`} dismissible={!disabled && !readonly} onDismiss={() => onChange?.(values.filter((_: string, itemIndex: number) => itemIndex !== index))}>
          {item}
        </FluentTag>
      ))}
      {!readonly && (
        <FluentInput
          {...props}
          appearance="underline"
          value={draft}
          disabled={disabled}
          placeholder={values.length ? '' : placeholder}
          onChange={(_, data) => setDraft(data.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ',') {
              event.preventDefault();
              commit();
            }
          }}
          onBlur={commit}
        />
      )}
    </span>
  );
};

export const RangeInput: React.FC<any> = ({ value = [], onChange, separator = '—', disabled, readonly, className, style, id, ...props }) => (
  <span id={id} role="group" aria-labelledby={props['aria-labelledby']} aria-describedby={props['aria-describedby']} className={`fluent-range-input ${className || ''}`} style={style}>
    <FluentInput {...props} aria-labelledby={undefined} aria-label="起始值" value={value?.[0] ?? ''} disabled={disabled} readOnly={readonly} onChange={(_, data) => onChange?.([data.value, value?.[1] ?? ''])} />
    <span>{separator}</span>
    <FluentInput {...props} aria-labelledby={undefined} aria-label="结束值" value={value?.[1] ?? ''} disabled={disabled} readOnly={readonly} onChange={(_, data) => onChange?.([value?.[0] ?? '', data.value])} />
  </span>
);

interface LegacyOption {
  label?: React.ReactNode;
  value?: any;
  disabled?: boolean;
  content?: React.ReactNode;
  [key: string]: any;
}

export const Select: React.FC<any> = ({
  options = [],
  value,
  defaultValue,
  onChange,
  multiple,
  disabled,
  readonly,
  filterable,
  creatable,
  placeholder,
  loading,
  label,
  className,
  style,
  clearable,
  onBlur,
  onFocus,
  ...props
}) => {
  const normalized = (options as LegacyOption[]).map((option) => typeof option === 'object' ? option : ({ label: String(option), value: option }));
  const values = multiple ? (Array.isArray(value) ? value : []) : value === undefined || value === null || value === '' ? [] : [value];
  const selectedOptions = values.map(String);
  const selectedLabels = values.map((selected) => normalized.find((option) => String(option.value) === String(selected))?.label ?? selected);
  const displayValue = multiple ? selectedLabels.map(String).join(', ') : selectedLabels.length ? String(selectedLabels[0]) : '';
  const resolveValue = (raw: string) => normalized.find((option) => String(option.value) === raw)?.value ?? raw;
  const canType = Boolean(filterable || creatable);
  const [inputValue, setInputValue] = React.useState(displayValue);
  const [searchQuery, setSearchQuery] = React.useState('');
  const filteredOptions = filterable && searchQuery.trim()
    ? normalized.filter((option) => `${String(option.label ?? '')} ${String(option.value ?? '')}`.toLocaleLowerCase().includes(searchQuery.trim().toLocaleLowerCase()))
    : normalized;

  React.useEffect(() => {
    setInputValue(displayValue);
    setSearchQuery('');
  }, [displayValue]);

  return (
    <Combobox
      {...props}
      className={className}
      style={style}
      aria-label={props['aria-label'] ?? (props['aria-labelledby'] ? undefined : typeof label === 'string' ? label : placeholder || '请选择')}
      placeholder={placeholder}
      disabled={disabled || readonly}
      multiselect={Boolean(multiple)}
      freeform={Boolean(creatable)}
      selectedOptions={selectedOptions}
      value={canType ? inputValue : displayValue}
      onFocus={(event) => {
        if (filterable) {
          setInputValue('');
          setSearchQuery('');
        }
        onFocus?.(event);
      }}
      onChange={(event) => {
        if (canType) {
          setInputValue(event.currentTarget.value);
          if (filterable) setSearchQuery(event.currentTarget.value);
        }
      }}
      onOptionSelect={(_, data) => {
        const next = multiple ? data.selectedOptions.map(resolveValue) : resolveValue(data.optionValue || '');
        const nextDisplayValue = multiple
          ? data.selectedOptions
              .map((selected) => normalized.find((option) => String(option.value) === selected)?.label ?? selected)
              .map(String)
              .join(', ')
          : String(data.optionText ?? data.optionValue ?? '');
        setInputValue(nextDisplayValue);
        setSearchQuery('');
        onChange?.(next, { selectedOptions: data.selectedOptions });
      }}
      onBlur={(event) => {
        if (creatable && !multiple && event.currentTarget.value && !selectedOptions.includes(event.currentTarget.value)) {
          onChange?.(event.currentTarget.value, { selectedOptions: [event.currentTarget.value] });
          setInputValue(event.currentTarget.value);
        } else {
          setInputValue(displayValue);
        }
        setSearchQuery('');
        onBlur?.(event);
      }}
    >
      {loading && <Option value="__loading" disabled>正在加载</Option>}
      {clearable && selectedOptions.length > 0 && <Option value="">清空选择</Option>}
      {filteredOptions.map((option, index) => (
        <Option key={`${String(option.value)}-${index}`} value={String(option.value)} disabled={option.disabled} text={typeof option.label === 'string' ? option.label : String(option.value)}>
          {option.content ?? option.label ?? String(option.value)}
        </Option>
      ))}
    </Combobox>
  );
};

export interface TextareaProps extends Omit<React.TextareaHTMLAttributes<HTMLTextAreaElement>, 'onChange'> {
  autosize?: boolean | { minRows?: number; maxRows?: number };
  onChange?: (value: string, context?: any) => void;
}

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>((inputProps, ref) => {
  const { autosize, onChange, value, defaultValue, ...props } = inputProps;
  const controlled = Object.prototype.hasOwnProperty.call(inputProps, 'value');
  const [innerValue, setInnerValue] = React.useState(String(value ?? defaultValue ?? ''));

  React.useEffect(() => {
    if (controlled) setInnerValue(String(value ?? ''));
  }, [controlled, value]);

  return (
    <FluentTextarea
      {...props}
      ref={ref}
      value={innerValue}
      resize={autosize ? 'vertical' : 'none'}
      onChange={(event, data) => {
        setInnerValue(data.value);
        onChange?.(data.value, { e: event });
      }}
    />
  );
});
Textarea.displayName = 'Textarea';

export const Switch: React.FC<any> = ({ value, checked, defaultValue, label, customValue, onChange, ...props }) => {
  const resolved = checked ?? value ?? defaultValue ?? false;
  const isChecked = Boolean(Array.isArray(customValue) ? resolved === customValue[0] : resolved);
  return (
    <FluentSwitch
      {...props}
      checked={isChecked}
      label={Array.isArray(label) ? label[isChecked ? 0 : 1] : label}
      onChange={(event, data) => onChange?.(Array.isArray(customValue) ? customValue[data.checked ? 0 : 1] : data.checked, { e: event })}
    />
  );
};

export const Checkbox: React.FC<any> = ({ value, checked, onChange, children, ...props }) => (
  <FluentCheckbox {...props} checked={Boolean(checked ?? value ?? false)} label={children} onChange={(event, data) => onChange?.(data.checked, { e: event })} />
);

export const Radio: any = ({ children, ...props }: any) => <FluentRadio {...props} label={children ?? props.label} />;
const RadioButton = ({ children, ...props }: any) => <FluentRadio {...props} label={children ?? props.label} />;
export const RadioGroup = ({ value, defaultValue, onChange, children, ...props }: any) => (
  <FluentRadioGroup {...props} value={String(value ?? defaultValue ?? '')} onChange={(event, data) => onChange?.(data.value, { e: event })}>
    {children}
  </FluentRadioGroup>
);
Radio.Group = RadioGroup;
Radio.Button = RadioButton;

export const Tag: React.FC<any> = ({ children, theme, variant, closable, onClose, icon, ...props }) => (
  <FluentTag
    {...props}
    appearance={variant === 'outline' ? 'outline' : 'filled'}
    dismissible={closable}
    icon={icon}
    onDismiss={onClose}
    style={{ color: theme === 'danger' ? '#b10e1c' : undefined, ...props.style }}
  >
    {children}
  </FluentTag>
);

const toFluentPositioning = (placement?: string) => {
  const placementMap: Record<string, string> = {
    top: 'above',
    'top-left': 'above-start',
    'top-right': 'above-end',
    bottom: 'below',
    'bottom-left': 'below-start',
    'bottom-right': 'below-end',
    left: 'before',
    'left-top': 'before',
    'left-bottom': 'before',
    right: 'after',
    'right-top': 'after',
    'right-bottom': 'after',
  };
  return placementMap[placement || ''] || 'below';
};

export const Tooltip: React.FC<any> = ({ content, placement, children, showArrow, ...props }) => (
  <FluentTooltip {...props} content={content} relationship="label" positioning={toFluentPositioning(placement)}>
    {React.isValidElement(children) ? children : <span>{children}</span>}
  </FluentTooltip>
);

export const Drawer: React.FC<any> = ({
  visible,
  onClose,
  header,
  footer,
  children,
  size = 'medium',
  placement = 'right',
  showOverlay = true,
  className,
  style,
  closeOnOverlayClick = true,
  closeOnEscKeydown = true,
  headerClassName,
  headerTitleClassName,
  bodyClassName,
  footerClassName,
  closeButtonClassName,
}) => {
  if (!visible) return null;
  const presetSizes: Record<string, string> = {
    compact: '560px',
    standard: '720px',
    wide: '960px',
    workspace: '1180px',
  };
  const resolvedSize = presetSizes[String(size)] || size;
  const builtInSizes = ['small', 'medium', 'large', 'full'];
  const hasBuiltInSize = builtInSizes.includes(String(resolvedSize));
  const fluentSize = hasBuiltInSize ? resolvedSize : 'large';
  const drawerStyle = hasBuiltInSize
    ? style
    : { ...style, width: resolvedSize, maxWidth: 'calc(100vw - 32px)' };
  return (
    <OverlayDrawer
      className={className}
      style={drawerStyle}
      open={Boolean(visible)}
      position={placement === 'left' ? 'start' : placement === 'bottom' || placement === 'top' ? 'end' : 'end'}
      size={fluentSize}
      modalType={showOverlay === false ? 'non-modal' : 'modal'}
      onOpenChange={(_, data) => {
        if (data.open) return;
        if (data.type === 'backdropClick' && !closeOnOverlayClick) return;
        if (data.type === 'escapeKeyDown' && !closeOnEscKeydown) return;
        onClose?.();
      }}
    >
      <DrawerHeader className={headerClassName}>
        <DrawerHeaderTitle
          className={headerTitleClassName}
          action={(
            <FluentButton
              className={closeButtonClassName}
              appearance="subtle"
              icon={<Dismiss20Regular />}
              aria-label="关闭"
              onClick={onClose}
            />
          )}
        >
          {header}
        </DrawerHeaderTitle>
      </DrawerHeader>
      <DrawerBody className={bodyClassName}>{children}</DrawerBody>
      {footer != null && footer !== false && <DrawerFooter className={footerClassName}>{footer}</DrawerFooter>}
    </OverlayDrawer>
  );
};

export const Dialog: React.FC<any> = ({
  visible,
  onClose,
  onConfirm,
  onCancel,
  header,
  body,
  footer,
  children,
  confirmBtn = '确认',
  cancelBtn = '取消',
  width,
  className,
  style,
}) => {
  const surfaceWidth = typeof width === 'number' ? `${width}px` : width;
  const showDefaultActions = footer === undefined && (cancelBtn != null || confirmBtn != null);
  const showActions = footer != null && footer !== false || showDefaultActions;
  return (
    <FluentDialog open={Boolean(visible)} onOpenChange={(_, data) => { if (!data.open) onClose?.(); }}>
      <DialogSurface
        className={className}
        style={{
          ...style,
          width: surfaceWidth || style?.width,
          maxWidth: surfaceWidth ? 'calc(100vw - 32px)' : style?.maxWidth,
        }}
      >
        <DialogBody>
          {header && <DialogTitle>{header}</DialogTitle>}
          <DialogContent>{body ?? children}</DialogContent>
          {showActions && (
            <DialogActions>
              {footer != null && footer !== false ? footer : (
                <>
                  {cancelBtn != null && (
                    <FluentButton appearance="secondary" onClick={() => { onCancel?.(); onClose?.(); }}>{cancelBtn}</FluentButton>
                  )}
                  {confirmBtn != null && <FluentButton appearance="primary" onClick={onConfirm}>{confirmBtn}</FluentButton>}
                </>
              )}
            </DialogActions>
          )}
        </DialogBody>
      </DialogSurface>
    </FluentDialog>
  );
};

export const Popconfirm: React.FC<any> = ({ content, children, onConfirm, onCancel, disabled, placement, ...props }) => {
  const [open, setOpen] = React.useState(false);
  return (
    <Popover open={open} positioning={toFluentPositioning(placement)} onOpenChange={(_, data) => !disabled && setOpen(data.open)}>
      <PopoverTrigger disableButtonEnhancement>{React.isValidElement(children) ? children : <span>{children}</span>}</PopoverTrigger>
      <PopoverSurface className="fluent-popconfirm">
        <div>{content}</div>
        <div className="fluent-popconfirm-actions">
          <FluentButton appearance="subtle" size="small" onClick={() => { setOpen(false); onCancel?.(); }}>取消</FluentButton>
          <FluentButton appearance="primary" size="small" onClick={() => { setOpen(false); onConfirm?.(); }}>确认</FluentButton>
        </div>
      </PopoverSurface>
    </Popover>
  );
};

interface TabPanelProps {
  value: string | number;
  label?: React.ReactNode;
  disabled?: boolean;
  children?: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
}

export const TabPanel: React.FC<TabPanelProps> = ({ children, className, style }) => <div className={className} style={style}>{children}</div>;

export const Tabs: any = ({ value, defaultValue, onChange, children, className, style }: any) => {
  const panels = React.Children.toArray(children).filter(React.isValidElement) as React.ReactElement<TabPanelProps>[];
  const fallbackValue = defaultValue ?? panels[0]?.props.value;
  const [internalValue, setInternalValue] = React.useState(fallbackValue);
  const selectedValue = value ?? internalValue;
  const activePanel = panels.find((panel) => String(panel.props.value) === String(selectedValue));

  return (
    <div className={`fluent-tabs ${className || ''}`} style={style}>
      <TabList
        selectedValue={String(selectedValue ?? '')}
        onTabSelect={(_, data) => {
          const originalValue = panels.find((panel) => String(panel.props.value) === String(data.value))?.props.value ?? data.value;
          setInternalValue(originalValue);
          onChange?.(originalValue);
        }}
      >
        {panels.map((panel) => (
          <Tab key={String(panel.props.value)} value={String(panel.props.value)} disabled={panel.props.disabled}>
            {panel.props.label}
          </Tab>
        ))}
      </TabList>
      <div className="fluent-tab-content">{activePanel}</div>
    </div>
  );
};
Tabs.TabPanel = TabPanel;

const getRowKey = (row: any, rowKey: string | ((row: any) => React.Key), index: number) => (
  typeof rowKey === 'function' ? rowKey(row) : row?.[rowKey] ?? index
);

const DEFAULT_COLUMN_MIN_WIDTH = 152;
const IDENTITY_COLUMN_MIN_WIDTH = 192;
const OPERATION_COLUMN_MIN_WIDTH = 112;
const SELECTION_COLUMN_WIDTH = 48;

const getPixelSize = (value: unknown) => {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string' && /^\d+(?:\.\d+)?px$/.test(value.trim())) return Number.parseFloat(value);
  return undefined;
};

const getCellAccessibleText = (node: React.ReactNode): string => {
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(getCellAccessibleText).filter(Boolean).join(' ').trim();
  if (React.isValidElement<{ children?: React.ReactNode }>(node)) {
    return getCellAccessibleText(node.props.children);
  }
  return '';
};

export const Pagination: React.FC<any> = ({
  current: controlledCurrent,
  defaultCurrent = 1,
  pageSize: controlledPageSize,
  defaultPageSize = 10,
  total = 0,
  onChange,
  showJumper,
  pageSizeOptions = [10, 20, 50, 100],
  className,
  disabled = false,
}) => {
  const [internalCurrent, setInternalCurrent] = React.useState(defaultCurrent);
  const [internalPageSize, setInternalPageSize] = React.useState(defaultPageSize);
  const current = controlledCurrent ?? internalCurrent;
  const pageSize = controlledPageSize ?? internalPageSize;
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const emit = (nextCurrent: number, nextPageSize = pageSize) => {
    const pageInfo = {
      current: Math.min(Math.max(1, nextCurrent), Math.max(1, Math.ceil(total / nextPageSize))),
      pageSize: nextPageSize,
    };
    if (controlledCurrent === undefined) setInternalCurrent(pageInfo.current);
    if (controlledPageSize === undefined) setInternalPageSize(pageInfo.pageSize);
    onChange?.(pageInfo);
  };
  const visiblePages = Array.from({ length: Math.min(pageCount, 5) }, (_, index) => {
    const start = Math.min(Math.max(1, current - 2), Math.max(1, pageCount - 4));
    return start + index;
  });
  return (
    <div className={`fluent-pagination ${className || ''}`}>
      <span className="fluent-pagination-total">共 {total} 条数据</span>
      <FluentDropdown
        aria-label="每页条数"
        className="fluent-pagination-size"
        disabled={disabled}
        size="small"
        value={`${pageSize} 条/页`}
        selectedOptions={[String(pageSize)]}
        onOptionSelect={(_, data) => emit(1, Number(data.optionValue))}
      >
        {pageSizeOptions.map((option: number) => <Option key={option} value={String(option)}>{option} 条/页</Option>)}
      </FluentDropdown>
      <FluentButton aria-label="上一页" appearance="subtle" size="small" disabled={disabled || current <= 1} onClick={() => emit(current - 1)}>‹</FluentButton>
      {visiblePages.map((page) => (
        <FluentButton key={page} appearance={page === current ? 'primary' : 'subtle'} size="small" disabled={disabled} onClick={() => emit(page)}>{page}</FluentButton>
      ))}
      <FluentButton aria-label="下一页" appearance="subtle" size="small" disabled={disabled || current >= pageCount} onClick={() => emit(current + 1)}>›</FluentButton>
      {showJumper && (
        <label className="fluent-pagination-jumper">
          跳至
          <FluentInput disabled={disabled} size="small" type="number" min={1} max={pageCount} value={String(current)} onChange={(_, data) => emit(Number(data.value || 1))} />
          / {pageCount} 页
        </label>
      )}
    </div>
  );
};

export const Table: React.FC<any> = ({
  data = [],
  columns = [],
  rowKey = 'id',
  loading,
  empty,
  cellEmptyContent = '-',
  pagination,
  onPageChange,
  selectedRowKeys = [],
  onSelectChange,
  onRowClick,
  className,
  tableLayout = 'fixed',
  ariaLabel,
  'aria-label': ariaLabelAttribute,
  ...rest
}) => {
  const [localPagination, setLocalPagination] = React.useState({
    current: pagination?.defaultCurrent ?? 1,
    pageSize: pagination?.defaultPageSize ?? 10,
  });
  const hasExternalPagination = Boolean(pagination?.onChange || onPageChange);
  const paginationCurrent = pagination?.current ?? localPagination.current;
  const paginationPageSize = pagination?.pageSize ?? localPagination.pageSize;
  React.useEffect(() => {
    if (!pagination || hasExternalPagination) return;
    const lastPage = Math.max(1, Math.ceil(data.length / paginationPageSize));
    if (paginationCurrent > lastPage) {
      setLocalPagination((previous) => ({ ...previous, current: lastPage }));
    }
  }, [data.length, hasExternalPagination, pagination, paginationCurrent, paginationPageSize]);
  const visibleData = pagination && !hasExternalPagination
    ? data.slice((paginationCurrent - 1) * paginationPageSize, paginationCurrent * paginationPageSize)
    : data;
  const selectableColumn = columns.find((column: any) => column.type === 'multiple');
  const firstDataColumnIndex = columns.findIndex((column: any) => column.type !== 'multiple');
  const lastDataColumnIndex = columns.reduce((lastIndex: number, column: any, columnIndex: number) => (
    column.type === 'multiple' ? lastIndex : columnIndex
  ), -1);
  const resolveColumnMinWidth = (column: any, columnIndex = columns.indexOf(column)) => {
    const widthPixels = getPixelSize(column.width);
    const minWidthPixels = getPixelSize(column.minWidth);
    if (widthPixels !== undefined && minWidthPixels !== undefined) return Math.max(widthPixels, minWidthPixels);
    if (column.type === 'multiple') return minWidthPixels ?? widthPixels ?? SELECTION_COLUMN_WIDTH;
    if (minWidthPixels !== undefined) return minWidthPixels;
    if (widthPixels !== undefined) return widthPixels;
    if (columnIndex === firstDataColumnIndex) return IDENTITY_COLUMN_MIN_WIDTH;
    if (columnIndex === lastDataColumnIndex) return OPERATION_COLUMN_MIN_WIDTH;
    return DEFAULT_COLUMN_MIN_WIDTH;
  };
  const resolveColumnPixelWidth = (column: any, columnIndex: number) => (
    getPixelSize(resolveColumnMinWidth(column, columnIndex)) ?? DEFAULT_COLUMN_MIN_WIDTH
  );
  const resolveFixedSide = (column: any, columnIndex: number): 'left' | 'right' | undefined => {
    if (column.fixed === 'left' || column.fixed === 'right') return column.fixed;
    if (column.type === 'multiple' || columnIndex === firstDataColumnIndex) return 'left';
    if (columnIndex === lastDataColumnIndex && lastDataColumnIndex !== firstDataColumnIndex) return 'right';
    return undefined;
  };
  const fixedSides = columns.map(resolveFixedSide);
  const leftOffsets = columns.map((_: any, columnIndex: number) => (
    columns
      .slice(0, columnIndex)
      .reduce((offset: number, column: any, index: number) => (
        fixedSides[index] === 'left' ? offset + resolveColumnPixelWidth(column, index) : offset
      ), 0)
  ));
  const rightOffsets = columns.map((_: any, columnIndex: number) => (
    columns
      .slice(columnIndex + 1)
      .reduce((offset: number, column: any, relativeIndex: number) => {
        const index = columnIndex + relativeIndex + 1;
        return fixedSides[index] === 'right' ? offset + resolveColumnPixelWidth(column, index) : offset;
      }, 0)
  ));
  const tableMinWidth = columns.reduce((width: number, column: any, columnIndex: number) => (
    width + resolveColumnPixelWidth(column, columnIndex)
  ), 0);
  const resolveFixedClassName = (column: any) => {
    const columnIndex = columns.indexOf(column);
    if (fixedSides[columnIndex] === 'left') {
      return 'fluent-table-fixed fluent-table-fixed-left';
    }
    if (fixedSides[columnIndex] === 'right') {
      return 'fluent-table-fixed fluent-table-fixed-right';
    }
    return undefined;
  };
  const resolveCellStyle = (column: any): React.CSSProperties => {
    const columnIndex = columns.indexOf(column);
    const fixedSide = fixedSides[columnIndex];
    return {
      width: getPixelSize(column.width) ?? resolveColumnMinWidth(column),
      minWidth: resolveColumnMinWidth(column),
      textAlign: column.align || 'left',
      ...(fixedSide === 'left' ? { position: 'sticky', left: leftOffsets[columnIndex] } : {}),
      ...(fixedSide === 'right' ? { position: 'sticky', right: rightOffsets[columnIndex] } : {}),
    };
  };
  const selectedSet = new Set(selectedRowKeys.map(String));
  const resolveRowCheckProps = (row: any) => (
    typeof selectableColumn?.checkProps === 'function'
      ? selectableColumn.checkProps({ row }) || {}
      : selectableColumn?.checkProps || {}
  );
  const selectableKeys = visibleData
    .map((row: any, index: number) => ({ key: getRowKey(row, rowKey, index), disabled: Boolean(resolveRowCheckProps(row).disabled) }))
    .filter(({ disabled }: { disabled: boolean }) => !disabled)
    .map(({ key }: { key: React.Key }) => key);
  const allSelected = selectableKeys.length > 0 && selectableKeys.every((key: React.Key) => selectedSet.has(String(key)));
  const toggleAll = (checked: boolean) => {
    const visible = new Set(selectableKeys.map(String));
    const next = checked
      ? Array.from(new Set([...selectedRowKeys, ...selectableKeys]))
      : selectedRowKeys.filter((key: React.Key) => !visible.has(String(key)));
    onSelectChange?.(next, { type: 'check', currentRowKey: 'CHECK_ALL_BOX' });
  };
  const toggleRow = (key: React.Key, checked: boolean) => {
    const next = checked ? Array.from(new Set([...selectedRowKeys, key])) : selectedRowKeys.filter((item: React.Key) => String(item) !== String(key));
    onSelectChange?.(next, { type: 'check', currentRowKey: key });
  };

  return (
    <div {...rest} aria-busy={loading || undefined} className={`fluent-table-shell ${className || ''}`}>
      <div className="fluent-table-scroll">
        <FluentTable aria-label={ariaLabelAttribute || ariaLabel || '数据表'} style={{ tableLayout, minWidth: Math.max(tableMinWidth, 1) }}>
          <TableHeader>
            <TableRow>
              {columns.map((column: any, columnIndex: number) => (
                <TableHeaderCell key={column.colKey || columnIndex} className={resolveFixedClassName(column)} style={resolveCellStyle(column)}>
                  {column.type === 'multiple' ? (
                    <FluentCheckbox aria-label="选择当前页全部数据" checked={allSelected ? true : selectedRowKeys.length ? 'mixed' : false} onChange={(_, data) => toggleAll(Boolean(data.checked))} />
                  ) : <span className="fluent-table-header-content" title={getCellAccessibleText(column.title) || undefined}>{column.title}</span>}
                </TableHeaderCell>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {!loading && visibleData.map((row: any, rowIndex: number) => {
              const key = getRowKey(row, rowKey, rowIndex);
              return (
                <TableRow
                  key={key}
                  aria-label={onRowClick ? `第 ${rowIndex + 1} 行，可按回车查看详情` : undefined}
                  tabIndex={onRowClick ? 0 : undefined}
                  onClick={(event) => onRowClick?.({ row, index: rowIndex, e: event })}
                  onKeyDown={onRowClick ? (event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      onRowClick({ row, index: rowIndex, e: event });
                    }
                  } : undefined}
                >
                  {columns.map((column: any, columnIndex: number) => {
                    let content: React.ReactNode;
                    if (column.type === 'multiple') {
                      const checkProps = resolveRowCheckProps(row);
                      content = (
                        <FluentCheckbox
                          {...checkProps}
                          aria-label={checkProps['aria-label'] || `选择第 ${rowIndex + 1} 行`}
                          checked={selectedSet.has(String(key))}
                          onChange={(_, value) => {
                            if (!checkProps.disabled) toggleRow(key, Boolean(value.checked));
                          }}
                        />
                      );
                    } else if (column.cell) {
                      content = column.cell({ row, rowIndex, col: column });
                    } else {
                      content = row?.[column.colKey];
                    }
                    const displayContent = content === undefined || content === null || content === '' ? cellEmptyContent : content;
                    const rawCellContent = column.colKey ? row?.[column.colKey] : undefined;
                    const cellText = getCellAccessibleText(displayContent) || getCellAccessibleText(rawCellContent);
                    const hasPlainText = (
                      typeof displayContent === 'string'
                      || typeof displayContent === 'number'
                      || typeof rawCellContent === 'string'
                      || typeof rawCellContent === 'number'
                    );
                    const shouldEllipsize = (
                      column.ellipsis !== false
                      && (column.ellipsis || hasPlainText || Boolean(cellText))
                    );
                    return (
                      <TableCell key={column.colKey || columnIndex} className={resolveFixedClassName(column)} style={resolveCellStyle(column)}>
                        <div
                          aria-label={shouldEllipsize && cellText ? cellText : undefined}
                          className={`fluent-table-cell-content${shouldEllipsize ? ' fluent-table-cell-content--ellipsis' : ''}`}
                          title={shouldEllipsize ? cellText || undefined : undefined}
                        >
                          {displayContent}
                        </div>
                      </TableCell>
                    );
                  })}
                </TableRow>
              );
            })}
          </TableBody>
        </FluentTable>
        {loading && <div className="fluent-table-loading-overlay" role="status"><Spinner label="正在加载" /></div>}
        {!loading && visibleData.length === 0 && <div className="fluent-table-state">{empty || <Empty description="暂无数据" />}</div>}
      </div>
      {pagination && (
        <Pagination
          {...pagination}
          disabled={loading || pagination.disabled}
          current={paginationCurrent}
          pageSize={paginationPageSize}
          onChange={(pageInfo: any) => {
            if (pagination.onChange) {
              pagination.onChange(pageInfo);
            } else if (onPageChange) {
              onPageChange(pageInfo);
            } else {
              setLocalPagination(pageInfo);
            }
          }}
        />
      )}
    </div>
  );
};

export const Space: React.FC<any> = ({ children, direction = 'horizontal', size = 'medium', breakLine, align, separator, style, ...props }) => {
  const items = React.Children.toArray(children);
  const gap = typeof size === 'number' ? size : size === 'small' ? 4 : size === 'large' ? 16 : 8;
  return (
    <div {...props} style={{ display: 'flex', flexDirection: direction === 'vertical' ? 'column' : 'row', flexWrap: breakLine ? 'wrap' : undefined, alignItems: align === 'center' ? 'center' : align, gap, ...style }}>
      {separator ? items.flatMap((item, index) => (index === 0 ? [item] : [React.cloneElement(separator, { key: `separator-${index}` }), item])) : items}
    </div>
  );
};

export const Loading: React.FC<any> = ({ loading = true, children, text, size = 'medium', className, style, ...props }) => {
  if (!children) return loading ? <Spinner {...props} size={size === 'small' ? 'tiny' : size === 'large' ? 'large' : 'medium'} label={text} /> : null;
  return <div className={`fluent-loading ${className || ''}`} style={{ position: 'relative', ...style }}>{children}{loading && <div className="fluent-loading-overlay"><Spinner label={text} /></div>}</div>;
};

export const Empty: React.FC<any> = ({ title, description, action, className, ...props }) => {
  const primary = title || description || '暂无数据';
  return (
    <div {...props} className={`fluent-empty ${className || ''}`}>
      <strong className="fluent-empty-title">{primary}</strong>
      {title && description && <span className="fluent-empty-description">{description}</span>}
      {action && <div className="fluent-empty-action">{action}</div>}
    </div>
  );
};

export const Link: React.FC<any> = ({ theme, hover, children, ...props }) => <FluentLink {...props}>{children}</FluentLink>;
export const Avatar: React.FC<any> = ({ image, children, ...props }) => <FluentAvatar {...props} image={image} name={typeof children === 'string' ? children : props.name} />;
export const Badge: React.FC<any> = ({ count, dot, children, ...props }) => <FluentBadge {...props}>{dot ? '' : count ?? children}</FluentBadge>;
export const Divider: React.FC<any> = ({ layout = 'horizontal', children, ...props }) => <FluentDivider {...props} vertical={layout === 'vertical'}>{children}</FluentDivider>;
export const Progress: React.FC<any> = ({ percentage = 0, ...props }) => <ProgressBar {...props} value={Number(percentage) / 100} />;
const LayoutRoot: React.FC<any> = ({ children, ...props }) => <div {...props}>{children}</div>;
const LayoutHeader: React.FC<any> = ({ children, ...props }) => <header {...props}>{children}</header>;
const LayoutContent: React.FC<any> = ({ children, ...props }) => <main {...props}>{children}</main>;
const LayoutFooter: React.FC<any> = ({ children, ...props }) => <footer {...props}>{children}</footer>;

export const Layout: any = LayoutRoot;
Layout.Header = LayoutHeader;
Layout.Content = LayoutContent;
Layout.Footer = LayoutFooter;
export const Card: React.FC<any> = ({ header, footer, children, bordered, hoverShadow, ...props }) => <section {...props} className={`fluent-card ${props.className || ''}`}>{header && <header>{header}</header>}{children}{footer && <footer>{footer}</footer>}</section>;
const justifyContentMap: Record<string, React.CSSProperties['justifyContent']> = {
  start: 'flex-start',
  end: 'flex-end',
  center: 'center',
  'space-around': 'space-around',
  'space-between': 'space-between',
  'space-evenly': 'space-evenly',
};
const alignItemsMap: Record<string, React.CSSProperties['alignItems']> = {
  top: 'flex-start',
  middle: 'center',
  bottom: 'flex-end',
  stretch: 'stretch',
  baseline: 'baseline',
};
const toGridPercent = (span: number) => `${(Math.min(12, Math.max(0, span)) / 12) * 100}%`;

export const Row: React.FC<any> = ({
  gutter = 0,
  justify,
  align,
  children,
  className,
  style,
  ...props
}) => {
  const horizontalGutter = Array.isArray(gutter) ? gutter[0] : gutter;
  const verticalGutter = Array.isArray(gutter) ? gutter[1] : gutter;
  return (
    <div
      {...props}
      className={`fluent-row ${className || ''}`.trim()}
      style={{
        '--fluent-row-gutter-x': `${Number(horizontalGutter) || 0}px`,
        '--fluent-row-gutter-y': `${Number(verticalGutter) || 0}px`,
        justifyContent: justifyContentMap[justify] ?? justify,
        alignItems: alignItemsMap[align] ?? align,
        ...style,
      } as React.CSSProperties}
    >
      {children}
    </div>
  );
};

export const Col: React.FC<any> = ({
  span,
  xs,
  sm,
  md,
  lg,
  xl,
  xxl,
  children,
  className,
  style,
  ...props
}) => {
  const responsiveSpans = { xs, sm, md, lg, xl, xxl };
  const responsiveClassNames = Object.entries(responsiveSpans)
    .filter(([, value]) => typeof value === 'number')
    .map(([breakpoint]) => `fluent-col--has-${breakpoint}`);
  return (
    <div
      {...props}
      className={[
        'fluent-col',
        typeof span === 'number' ? 'fluent-col--fixed' : '',
        ...responsiveClassNames,
        className || '',
      ].filter(Boolean).join(' ')}
      style={{
        ...(typeof span === 'number' ? { '--fluent-col-span': toGridPercent(span) } : {}),
        ...(typeof xs === 'number' ? { '--fluent-col-xs': toGridPercent(xs) } : {}),
        ...(typeof sm === 'number' ? { '--fluent-col-sm': toGridPercent(sm) } : {}),
        ...(typeof md === 'number' ? { '--fluent-col-md': toGridPercent(md) } : {}),
        ...(typeof lg === 'number' ? { '--fluent-col-lg': toGridPercent(lg) } : {}),
        ...(typeof xl === 'number' ? { '--fluent-col-xl': toGridPercent(xl) } : {}),
        ...(typeof xxl === 'number' ? { '--fluent-col-xxl': toGridPercent(xxl) } : {}),
        ...style,
      } as React.CSSProperties}
    >
      {children}
    </div>
  );
};
export { Label };

const message = (intent: 'success' | 'error' | 'warning' | 'info', content: React.ReactNode) => {
  showFluentToast({ title: content, intent });
  return Promise.resolve();
};

export const MessagePlugin = {
  success: (content: React.ReactNode) => message('success', content),
  error: (content: React.ReactNode) => message('error', content),
  warning: (content: React.ReactNode) => message('warning', content),
  info: (content: React.ReactNode) => message('info', content),
};

// FormItem links labels to each adapter's primary input, while compound controls use group semantics.
[Input, InputNumber, TagInput, Select, Textarea, Switch, Checkbox].forEach(control => Object.assign(control, { [formControlKind]: 'input' }));
[RangeInput, RadioGroup].forEach(control => Object.assign(control, { [formControlKind]: 'group' }));
