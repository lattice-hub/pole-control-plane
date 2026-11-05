import React, { useEffect, useMemo, useState } from 'react';
import { Button, Input, InputNumber, Select, Switch, TagInput } from 'components/Fluent';
import EditorDrawer from 'components/EditorDrawer';
import { EffectiveSystemSetting, ManagedSystemDomain, saveManagedSystemDraft } from 'services/system_configuration';
import { toRequestErrorPayload } from 'utils/request';

import style from '../index.module.less';

interface Props {
  visible: boolean;
  settings: EffectiveSystemSetting[];
  domain?: ManagedSystemDomain;
  onClose: () => void;
  onSaved: (domain: ManagedSystemDomain) => void;
}

const errorText = (error: unknown) => {
  const payload = toRequestErrorPayload(error);
  return typeof payload === 'string' ? payload : payload.message;
};

const initialValues = (settings: EffectiveSystemSetting[], domain?: ManagedSystemDomain) => {
  const saved = domain?.draft?.values || domain?.active?.values || {};
  return settings.reduce<Record<string, unknown>>((result, setting) => {
    if (!setting.editable) return result;
    result[setting.key] = saved[setting.key] ?? setting.desired_value ?? setting.value
      ?? (setting.value_type === 'boolean' ? false : setting.value_type === 'list' ? [] : '');
    return result;
  }, {});
};

const constraintText = (setting: EffectiveSystemSetting) => {
  const rule = setting.validation || {};
  if (rule.options?.length) return `可选值：${rule.options.join('、')}`;
  if (setting.value_type === 'duration' && (rule.min_duration || rule.max_duration)) {
    return `范围：${rule.min_duration || '不限'} ～ ${rule.max_duration || '不限'}`;
  }
  if (setting.value_type === 'integer' && (rule.min_integer !== undefined || rule.max_integer !== undefined)) {
    return `范围：${rule.min_integer ?? '不限'} ～ ${rule.max_integer ?? '不限'}`;
  }
  return '';
};

export default function ManagedDomainDraftPanel({ visible, settings, domain, onClose, onSaved }: Props) {
  const editable = useMemo(() => settings.filter((setting) => setting.editable), [settings]);
  const initial = useMemo(() => initialValues(settings, domain), [domain, settings]);
  const [values, setValues] = useState<Record<string, unknown>>(initial);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!visible) return;
    setValues(initial);
    setError('');
  }, [initial, visible]);

  const update = (key: string, value: unknown) => {
    setValues((current) => ({ ...current, [key]: value }));
    setError('');
  };

  const save = async () => {
    if (!settings[0]) return;
    setSaving(true);
    setError('');
    try {
      const result = await saveManagedSystemDraft(settings[0].component, settings[0].domain, {
        expectedDraftRevision: domain?.draft?.revision || 0,
        values,
      });
      onSaved(result);
      onClose();
    } catch (requestError) {
      setError(errorText(requestError));
    } finally {
      setSaving(false);
    }
  };

  return (
    <EditorDrawer
      visible={visible}
      width="standard"
      title={`${settings[0]?.domain || ''} 配置草稿`}
      description="保存仅生成待发布草稿，不会立即改变当前实例。"
      onClose={onClose}
      footer={(
        <div className={style.managedDrawerFooter}>
          <span>保存只生成草稿，不改变当前实例</span>
          <Button variant="outline" onClick={onClose}>取消</Button>
          <Button theme="primary" loading={saving} disabled={!editable.length} onClick={save}>保存草稿</Button>
        </div>
      )}
    >
      <div className={style.managedDrawer}>
        <header>
          <strong>编辑当前领域</strong>
          <p>已逐项过滤自举、Secret 和部署拓扑配置；这里仅展示通过字段级评审的设置。发布后，重启级设置会进入“待重启”，不会被误报为已生效。</p>
        </header>
        {error && <div className={style.managedFormError} role="alert">{error}</div>}
        <div className={style.managedFields}>
          {editable.map((setting) => {
            const rule = setting.validation || {};
            const help = constraintText(setting);
            return (
              <label key={setting.key}>
                <span>
                  <strong>{setting.label}</strong>
                  <code>{setting.key}</code>
                </span>
                {setting.value_type === 'boolean' && (
                  <div className={style.managedBoolean}>
                    <Switch
                      checked={Boolean(values[setting.key])}
                      value={Boolean(values[setting.key])}
                      label={['开启', '关闭']}
                      onChange={(value: boolean) => update(setting.key, Boolean(value))}
                    />
                  </div>
                )}
                {setting.value_type === 'integer' && (
                  <InputNumber
                    value={values[setting.key]}
                    min={rule.min_integer}
                    max={rule.max_integer}
                    onChange={(value: number) => update(setting.key, Number(value))}
                  />
                )}
                {setting.value_type === 'list' && (
                  <TagInput
                    value={Array.isArray(values[setting.key]) ? values[setting.key] : []}
                    onChange={(value: string[]) => update(setting.key, value)}
                    placeholder="输入后按 Enter 添加"
                  />
                )}
                {setting.value_type === 'string' && rule.options?.length ? (
                  <Select
                    value={String(values[setting.key] ?? '')}
                    options={rule.options.map((value) => ({ label: value, value }))}
                    onChange={(value) => update(setting.key, String(value))}
                  />
                ) : null}
                {(setting.value_type === 'duration' || (setting.value_type === 'string' && !rule.options?.length)) && (
                  <Input
                    value={String(values[setting.key] ?? '')}
                    onChange={(value) => update(setting.key, value)}
                    placeholder={setting.value_type === 'duration' ? '例如 30s、5m' : ''}
                  />
                )}
                <small>{setting.description}{help ? ` · ${help}` : ''}</small>
              </label>
            );
          })}
        </div>
      </div>
    </EditorDrawer>
  );
}
