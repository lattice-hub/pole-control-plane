import React from 'react';

export type NamePath = string | number | Array<string | number>;

export interface ValidationResult {
  result: boolean;
  message?: string;
  type?: 'error' | 'warning' | 'success';
}

export type CustomValidator = (value: unknown) => ValidationResult | boolean | void | Promise<ValidationResult | boolean | void>;

export interface FormRule {
  required?: boolean;
  min?: number;
  max?: number;
  pattern?: RegExp;
  message?: string;
  type?: string;
  validator?: CustomValidator;
}

export interface FieldData {
  name: NamePath;
  value?: unknown;
  [key: string]: unknown;
}

export interface SubmitContext<T = Record<string, unknown>> {
  e?: React.FormEvent<HTMLFormElement>;
  fields: T;
  validateResult: true | Record<string, string[]>;
  firstError?: string;
}

export interface FormInstanceFunctions<T = Record<string, any>> {
  getFieldValue(name: NamePath): unknown;
  getFieldsValue(names?: true | NamePath[]): T;
  setFieldsValue(values: Partial<T>): void;
  setFields(fields: FieldData[]): void;
  reset(): void;
  submit(options?: { showErrorMessage?: boolean }): Promise<SubmitContext<T>>;
  validate(names?: NamePath[]): Promise<true | Record<string, string[]>>;
}

export type InternalFormInstance<T = Record<string, any>> = FormInstanceFunctions<T>;

export interface FormProps<T = Record<string, any>> extends Omit<React.FormHTMLAttributes<HTMLFormElement>, 'onSubmit'> {
  form?: InternalFormInstance<T>;
  initialData?: Partial<T>;
  layout?: 'vertical' | 'inline';
  labelAlign?: 'left' | 'right' | 'top';
  labelWidth?: number | string;
  colon?: boolean;
  onSubmit?: (context: SubmitContext<T>) => void | Promise<void>;
  onValuesChange?: (changedValues: Partial<T>, allValues: T) => void;
}

interface RegisteredField {
  name: Array<string | number>;
  rules: FormRule[];
}

const normalizePath = (name: NamePath): Array<string | number> => Array.isArray(name) ? name : [name];
const pathKey = (name: NamePath) => JSON.stringify(normalizePath(name));

const getAt = (source: any, name: NamePath): any => normalizePath(name).reduce((value, key) => value?.[key], source);

const setAt = (source: any, name: NamePath, value: unknown): any => {
  const path = normalizePath(name);
  const root = Array.isArray(source) ? [...source] : { ...(source || {}) };
  let cursor = root;
  path.forEach((key, index) => {
    if (index === path.length - 1) {
      cursor[key] = value;
      return;
    }
    const current = cursor[key];
    const nextIsArray = typeof path[index + 1] === 'number';
    cursor[key] = nextIsArray ? (Array.isArray(current) ? [...current] : []) : { ...(current || {}) };
    cursor = cursor[key];
  });
  return root;
};

const hasAt = (source: any, name: NamePath) => {
  const path = normalizePath(name);
  let cursor = source;
  for (const key of path) {
    if (cursor == null || !Object.prototype.hasOwnProperty.call(cursor, key)) return false;
    cursor = cursor[key];
  }
  return true;
};

const emptyValue = (value: unknown) => value === undefined || value === null || value === '' || (Array.isArray(value) && value.length === 0);

class FormStore<T extends Record<string, any> = Record<string, any>> implements InternalFormInstance<T> {
  private values = {} as T;
  private initialValues = {} as T;
  private fields = new Map<string, RegisteredField>();
  private errors = new Map<string, string>();
  private listeners = new Set<() => void>();
  private submitHandler?: (context: SubmitContext<T>) => void | Promise<void>;
  private valuesChangeHandler?: (changed: Partial<T>, all: T) => void;

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  getSnapshot = () => this.values;

  configure(initialData?: Partial<T>, onSubmit?: FormProps<T>['onSubmit'], onValuesChange?: FormProps<T>['onValuesChange']) {
    this.submitHandler = onSubmit;
    this.valuesChangeHandler = onValuesChange;
    if (initialData && Object.keys(this.initialValues).length === 0) {
      this.initialValues = structuredCloneSafe(initialData) as T;
      this.values = structuredCloneSafe(initialData) as T;
    }
  }

  register(name: NamePath, rules: FormRule[] = [], initialData?: unknown) {
    const normalized = normalizePath(name);
    const key = pathKey(normalized);
    const registration = { name: normalized, rules };
    this.fields.set(key, registration);
    if (initialData !== undefined && !hasAt(this.values, normalized)) {
      this.initialValues = setAt(this.initialValues, normalized, structuredCloneSafe(initialData));
      this.values = setAt(this.values, normalized, structuredCloneSafe(initialData));
      this.emit();
    }
    return () => {
      if (this.fields.get(key) === registration) this.fields.delete(key);
    };
  }

  getFieldValue = (name: NamePath) => getAt(this.values, name);

  getFieldsValue = (names?: true | NamePath[]) => {
    if (names === true || names === undefined) return this.values;
    return names.reduce((result, name) => setAt(result, name, this.getFieldValue(name)), {}) as T;
  };

  setFieldsValue = (values: Partial<T>) => {
    let next: any = this.values;
    Object.entries(values).forEach(([key, value]) => { next = setAt(next, key, value); });
    this.update(next, values);
  };

  setFields = (fields: FieldData[]) => {
    let next: any = this.values;
    let changed: any = {};
    fields.forEach(({ name, value }) => {
      next = setAt(next, name, value);
      changed = setAt(changed, name, value);
    });
    this.update(next, changed);
  };

  setFieldValue = (name: NamePath, value: unknown) => {
    this.update(setAt(this.values, name, value), setAt({}, name, value));
  };

  reset = () => {
    this.values = structuredCloneSafe(this.initialValues) as T;
    this.errors.clear();
    this.emit();
    this.valuesChangeHandler?.({} as Partial<T>, this.values);
  };

  validate = async (names?: NamePath[]) => {
    const requested = names ? new Set(names.map(pathKey)) : undefined;
    const errors: Record<string, string[]> = {};
    for (const [key, field] of this.fields) {
      if (requested && !requested.has(key)) continue;
      const value = this.getFieldValue(field.name);
      for (const rule of field.rules) {
        const message = await validateRule(value, rule);
        if (message) (errors[key] ||= []).push(message);
      }
      if (errors[key]?.[0]) this.errors.set(key, errors[key][0]);
      else this.errors.delete(key);
    }
    this.values = { ...this.values };
    this.emit();
    return Object.keys(errors).length === 0 ? true : errors;
  };

  validateField = async (name: NamePath, rules: FormRule[]) => {
    const key = pathKey(name);
    const error = await validateSingle(this.getFieldValue(name), rules);
    if (error) this.errors.set(key, error);
    else this.errors.delete(key);
    this.values = { ...this.values };
    this.emit();
  };

  getFieldError = (name: NamePath) => this.errors.get(pathKey(name));

  submit = async (_options?: { showErrorMessage?: boolean }) => {
    const validateResult = await this.validate();
    const context: SubmitContext<T> = {
      fields: this.values,
      validateResult,
      firstError: validateResult === true ? undefined : Object.values(validateResult)[0]?.[0],
    };
    await this.submitHandler?.(context);
    return context;
  };

  private update(next: T, changed: Partial<T>) {
    this.values = next;
    this.emit();
    this.valuesChangeHandler?.(changed, this.values);
  }

  private emit() { this.listeners.forEach(listener => listener()); }
}

const structuredCloneSafe = <T,>(value: T): T => {
  if (value === undefined) return value;
  if (typeof structuredClone === 'function') return structuredClone(value);
  return JSON.parse(JSON.stringify(value));
};

const validateRule = async (value: unknown, rule: FormRule): Promise<string | undefined> => {
  if (rule.required && emptyValue(value)) return rule.message || '此项为必填项';
  if (emptyValue(value)) return undefined;
  const length = typeof value === 'number' ? value : (value as any)?.length;
  if (rule.min !== undefined && typeof length === 'number' && length < rule.min) return rule.message || `不能少于 ${rule.min}`;
  if (rule.max !== undefined && typeof length === 'number' && length > rule.max) return rule.message || `不能超过 ${rule.max}`;
  if (rule.pattern && !rule.pattern.test(String(value))) return rule.message || '格式不正确';
  if (rule.validator) {
    const result = await rule.validator(value);
    if (result === false) return rule.message || '校验失败';
    if (result && typeof result === 'object' && result.result === false) return result.message || rule.message || '校验失败';
  }
  return undefined;
};

interface FormContextValue {
  form: FormStore<any>;
  layout: 'vertical' | 'inline';
  labelAlign: 'left' | 'right' | 'top';
  labelWidth?: number | string;
  colon: boolean;
}

const FormContext = React.createContext<FormContextValue | null>(null);
const ListPrefixContext = React.createContext<Array<string | number>>([]);

const useStoreSnapshot = (form: FormStore<any>) => React.useSyncExternalStore(form.subscribe, form.getSnapshot, form.getSnapshot);

export interface FormItemProps extends Omit<React.HTMLAttributes<HTMLDivElement>, 'children'> {
  name?: NamePath;
  label?: React.ReactNode;
  rules?: FormRule[];
  initialData?: unknown;
  help?: React.ReactNode;
  shouldUpdate?: (previous: Record<string, any>, current: Record<string, any>) => boolean;
  children?: React.ReactNode | ((form: InternalFormInstance) => React.ReactNode);
}

// Shared adapters declare whether their primary slot is a labelable input or a group.
export const formControlKind = Symbol('fluentFormControlKind');

export const FormItem: React.FC<FormItemProps> = ({ name, label, rules = [], initialData, help, shouldUpdate, children, className, style, ...rest }) => {
  const context = React.useContext(FormContext);
  const prefix = React.useContext(ListPrefixContext);
  if (!context) throw new Error('FormItem 必须在 Form 内使用');
  const fullName = name === undefined ? undefined : [...prefix, ...normalizePath(name)];
  const values = useStoreSnapshot(context.form);
  const previous = React.useRef(values);
  const [, forceRender] = React.useReducer(value => value + 1, 0);

  React.useEffect(() => {
    if (!fullName) return;
    return context.form.register(fullName, rules, initialData);
  }, [context.form, pathKey(fullName || []), rules, initialData]);

  React.useEffect(() => {
    if (shouldUpdate?.(previous.current, values)) forceRender();
    previous.current = values;
  }, [values, shouldUpdate]);

  const fieldId = React.useId();
  const labelId = `${fieldId}-label`;
  const helpId = `${fieldId}-help`;
  const value = fullName ? getAt(values, fullName) : undefined;
  const error = fullName ? context.form.getFieldError(fullName) : undefined;
  const child = React.isValidElement(children) ? children as React.ReactElement<any> : undefined;
  const kind = child && (typeof child.type === 'string'
    ? (['input', 'select', 'textarea'].includes(child.type) ? 'input' : undefined)
    : (child.type as any)[formControlKind]);
  const controlId = child?.props.id || `${fieldId}-control`;
  const joinIds = (...ids: Array<string | undefined | false>) => ids.filter(Boolean).join(' ') || undefined;
  const fieldAria = {
    'aria-labelledby': joinIds(child?.props['aria-labelledby'], label !== undefined && labelId),
    'aria-describedby': joinIds(child?.props['aria-describedby'], Boolean(error || help) && helpId),
    'aria-invalid': error ? true : child?.props['aria-invalid'],
    'aria-required': rules.some(rule => rule.required) || child?.props['aria-required'] || undefined,
  };
  const renderChild = () => {
    if (typeof children === 'function') return children(context.form);
    if (!child) return children;
    return React.cloneElement(child, {
      ...(kind ? { ...fieldAria, id: controlId } : {}),
      ...(fullName ? { value } : {}),
      onChange: (nextValue: unknown, changeContext?: unknown) => {
        child.props.onChange?.(nextValue, changeContext);
        if (fullName) {
          context.form.setFieldValue(fullName, nextValue);
          void context.form.validateField(fullName, rules);
        }
      },
    });
  };

  const width = typeof context.labelWidth === 'number' ? `${context.labelWidth}px` : context.labelWidth;
  return (
    <div {...rest} className={`fluent-form-item${className ? ` ${className}` : ''}`} style={{ display: context.layout === 'vertical' ? 'block' : 'flex', alignItems: context.layout === 'vertical' ? undefined : 'flex-start', gap: context.layout === 'vertical' ? undefined : 12, marginBottom: 16, ...style }}>
      {label !== undefined && (
        <label
          id={labelId}
          htmlFor={kind === 'input' ? controlId : undefined}
          className="fluent-form-item__label"
          style={{ display: 'block', width: context.layout === 'vertical' ? undefined : width, textAlign: context.labelAlign === 'right' ? 'right' : 'left', marginBottom: context.layout === 'vertical' ? 6 : undefined }}
        >
          {rules.some(rule => rule.required) && <span aria-hidden="true" style={{ color: 'var(--app-danger)', marginRight: 4 }}>*</span>}
          {label}{context.colon && label ? '：' : null}
        </label>
      )}
      <div className="fluent-form-item__control" role={!kind && label !== undefined ? 'group' : undefined} {...(!kind ? fieldAria : {})} style={{ flex: 1, minWidth: 0 }}>
        {renderChild()}
        {(error || help) && <div id={helpId} className="fluent-form-item__help" role={error ? 'alert' : undefined} style={{ color: error ? 'var(--app-danger)' : undefined, fontSize: 12, marginTop: 4 }}>{error || help}</div>}
      </div>
    </div>
  );
};

const validateSingle = async (value: unknown, rules: FormRule[]) => {
  for (const rule of rules) {
    const message = await validateRule(value, rule);
    if (message) return message;
  }
  return undefined;
};

interface FormListField { key: number; name: number; }
interface FormListProps {
  name: NamePath;
  initialData?: unknown[];
  rules?: FormRule[];
  children: (fields: FormListField[], operations: { add(value?: unknown, index?: number): void; remove(index: number | number[]): void }) => React.ReactNode;
}

export const FormList: React.FC<FormListProps> = ({ name, initialData = [], rules = [], children }) => {
  const context = React.useContext(FormContext);
  const parentPrefix = React.useContext(ListPrefixContext);
  if (!context) throw new Error('FormList 必须在 Form 内使用');
  const fullName = [...parentPrefix, ...normalizePath(name)];
  const values = useStoreSnapshot(context.form);
  const list = (getAt(values, fullName) as unknown[]) || [];
  const keys = React.useRef<number[]>([]);
  const nextKey = React.useRef(0);
  while (keys.current.length < list.length) keys.current.push(nextKey.current++);
  if (keys.current.length > list.length) keys.current.length = list.length;

  React.useEffect(() => context.form.register(fullName, rules, initialData), [context.form, pathKey(fullName), rules, initialData]);

  const add = (value: unknown = undefined, index = list.length) => {
    const next = [...list];
    next.splice(index, 0, value);
    keys.current.splice(index, 0, nextKey.current++);
    context.form.setFieldValue(fullName, next);
  };
  const remove = (indexes: number | number[]) => {
    const removed = new Set(Array.isArray(indexes) ? indexes : [indexes]);
    context.form.setFieldValue(fullName, list.filter((_, index) => !removed.has(index)));
    keys.current = keys.current.filter((_, index) => !removed.has(index));
  };

  return (
    <ListPrefixContext.Provider value={fullName}>
      {children(list.map((_, index) => ({ key: keys.current[index], name: index })), { add, remove })}
    </ListPrefixContext.Provider>
  );
};

type FormComponent = React.ForwardRefExoticComponent<FormProps & React.RefAttributes<FormInstanceFunctions>> & {
  FormItem: typeof FormItem;
  FormList: typeof FormList;
  useForm: <T extends Record<string, any> = Record<string, any>>() => [InternalFormInstance<T>];
  useWatch: (name: NamePath, form?: InternalFormInstance) => unknown;
};

const FormBase = React.forwardRef<FormInstanceFunctions, FormProps>((props, ref) => {
  const { form: suppliedForm, initialData, layout = 'vertical', labelAlign = 'left', labelWidth, colon = false, onSubmit, onReset, onValuesChange, children, className, style, ...rest } = props;
  const fallback = React.useRef<FormStore | null>(null);
  if (!fallback.current) fallback.current = new FormStore();
  const form = (suppliedForm || fallback.current) as FormStore;
  form.configure(initialData, onSubmit, onValuesChange);
  React.useImperativeHandle(ref, () => form, [form]);

  return (
    <FormContext.Provider value={{ form, layout, labelAlign, labelWidth, colon }}>
      <form
        {...rest}
        className={`fluent-form fluent-form--${layout}${className ? ` ${className}` : ''}`}
        style={style}
        onSubmit={(event) => {
          event.preventDefault();
          void form.validate().then(async validateResult => {
            const context: SubmitContext = { e: event, fields: form.getFieldsValue(true), validateResult, firstError: validateResult === true ? undefined : Object.values(validateResult)[0]?.[0] };
            await onSubmit?.(context);
          });
        }}
        onReset={(event) => {
          event.preventDefault();
          if (onReset) {
            onReset(event);
            return;
          }
          form.reset();
        }}
      >
        {children}
      </form>
    </FormContext.Provider>
  );
});
FormBase.displayName = 'Form';

export const Form = FormBase as FormComponent;
Form.FormItem = FormItem;
Form.FormList = FormList;
Form.useForm = <T extends Record<string, any> = Record<string, any>>() => {
  const ref = React.useRef<FormStore<T> | null>(null);
  if (!ref.current) ref.current = new FormStore<T>();
  return [ref.current];
};
Form.useWatch = (name: NamePath, suppliedForm?: InternalFormInstance) => {
  const context = React.useContext(FormContext);
  const fallback = React.useRef<FormStore | null>(null);
  if (!fallback.current) fallback.current = new FormStore();
  const form = (suppliedForm || context?.form || fallback.current) as FormStore;
  return React.useSyncExternalStore(form.subscribe, () => form.getFieldValue(name), () => form.getFieldValue(name));
};
