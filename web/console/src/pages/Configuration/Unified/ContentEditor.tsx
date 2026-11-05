import React from 'react';
import CodeEditor from 'components/CodeEditor';
import { ConfigDefinition } from 'services/unified_config';
import style from './workspace.module.less';
import { textFromBase64, textToBase64 } from 'services/unified_config_renderer';

export function fileFormat(name: string) {
  const extension = name.split('.').pop()?.toLowerCase() || '';
  return ({ yml: 'yaml', yaml: 'yaml', json: 'json', properties: 'properties', ini: 'ini', xml: 'xml' } as Record<string, string>)[extension] || 'text';
}
export default function ContentEditor({ definition, name, modelPath, onChange }: { definition: ConfigDefinition; name: string; modelPath: string; onChange: (d: ConfigDefinition) => void }) {
  let content = '';
  try { content = textFromBase64(definition.content || ''); } catch { return <p role="alert">正文不是有效 UTF-8，请使用高级 JSON 编辑。</p>; }
  return <div className={style.editor}><CodeEditor validate={definition.engine !== 'pole-mustache-v1'} modelPath={modelPath} ariaLabel="配置正文" height="100%" value={content} language={fileFormat(name)} onChange={value => onChange({ ...definition, content: textToBase64(value || ''), format: fileFormat(name) })} /></div>;
}
