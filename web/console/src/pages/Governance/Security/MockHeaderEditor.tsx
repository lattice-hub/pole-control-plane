import React from 'react';
import { Button } from 'components/Fluent';
import { Input } from '@fluentui/react-components';
import { AddIcon, CloseIcon } from 'components/Fluent/icons';

import shared from '../shared/governance.module.less';
import style from './index.module.less';

interface MockHeaderEditorProps {
    ruleIndex: number;
    record?: Record<string, string>;
    onChange: (next: Record<string, string>) => void;
}

interface HeaderRow {
    key: string;
    value: string;
}

const toRows = (record?: Record<string, string>): HeaderRow[] =>
    Object.entries(record || {}).map(([key, value]) => ({ key, value }));

const signature = (record?: Record<string, string>) => JSON.stringify(Object.entries(record || {}).sort(([a], [b]) => a.localeCompare(b)));

const MockHeaderEditor: React.FC<MockHeaderEditorProps> = ({ ruleIndex, record, onChange }) => {
    const [rows, setRows] = React.useState<HeaderRow[]>(() => toRows(record));
    const [nameError, setNameError] = React.useState<{ index: number; message: string }>();
    const nameErrorId = React.useId();
    const lastEmitted = React.useRef<string>();
    const recordSignature = signature(record);

    React.useEffect(() => {
        if (lastEmitted.current === recordSignature) {
            lastEmitted.current = undefined;
            return;
        }
        setRows(toRows(record));
        setNameError(undefined);
    }, [recordSignature]);

    const commitRows = (nextRows: HeaderRow[]) => {
        setRows(nextRows);
        const nextRecord = nextRows.reduce<Record<string, string>>((result, row) => {
            if (row.key.trim()) result[row.key] = row.value;
            return result;
        }, {});
        lastEmitted.current = signature(nextRecord);
        onChange(nextRecord);
    };

    const updateName = (index: number, next: string) => {
        const normalized = next.trim().toLowerCase();
        if (normalized && rows.some((row, rowIndex) => rowIndex !== index && row.key.trim().toLowerCase() === normalized)) {
            setNameError({ index, message: `第 ${index + 1} 个响应头名称“${next}”已存在，不区分大小写。已保留原名称，请使用不同的名称。` });
            return;
        }
        setNameError(undefined);
        commitRows(rows.map((item, rowIndex) => rowIndex === index ? { ...item, key: next } : item));
    };

    return (
        <div className={style.repeatEditor}>
            {rows.map((row, index) => (
                <div className={style.kvRow} key={`mock-header-${index}`}>
                    <Input aria-label={`第 ${ruleIndex + 1} 条 Mock 子规则第 ${index + 1} 个响应头名称`} aria-describedby={nameError?.index === index ? nameErrorId : undefined} value={row.key} placeholder="Header 名" onChange={(_event, data) => updateName(index, data.value)} />
                    <Input aria-label={`第 ${ruleIndex + 1} 条 Mock 子规则第 ${index + 1} 个响应头值`} value={row.value} placeholder="Header 值" onChange={(_event, data) => commitRows(rows.map((item, idx) => idx === index ? { ...item, value: data.value } : item))} />
                    <Button aria-label={`删除第 ${ruleIndex + 1} 条 Mock 子规则第 ${index + 1} 个响应头`} shape="circle" variant="text" onClick={() => { setNameError(undefined); commitRows(rows.filter((_, idx) => idx !== index)); }}><CloseIcon /></Button>
                </div>
            ))}
            {nameError && <p id={nameErrorId} role="alert">{nameError.message}</p>}
            <Button aria-label={`第 ${ruleIndex + 1} 条 Mock 子规则添加响应头`} className={shared.addRuleButton} variant="dashed" icon={<AddIcon />} onClick={() => commitRows([...rows, { key: '', value: '' }])}>添加响应头</Button>
        </div>
    );
};

export default MockHeaderEditor;
