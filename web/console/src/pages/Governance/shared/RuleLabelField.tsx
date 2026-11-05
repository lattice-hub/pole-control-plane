import React from 'react';
import { Button, Dialog, Input } from 'components/Fluent';
import { AddIcon, CloseIcon, Edit1Icon } from 'components/Fluent/icons';

import style from './governance.module.less';

interface RuleLabelFieldProps {
    /** 当前标签键值对 */
    metadata?: Record<string, string>;
    /** 是否可编辑（展示「编辑标签」入口） */
    editable?: boolean;
    /** 确认后写回 */
    onChange?: (next: Record<string, string>) => void;
}

const toRows = (metadata?: Record<string, string>) =>
    Object.entries(metadata || {}).map(([key, value]) => ({ key, value }));

const fromRows = (rows: Array<{ key: string; value: string }>) =>
    rows.reduce<Record<string, string>>((acc, row) => {
        if (row.key.trim()) acc[row.key.trim()] = row.value;
        return acc;
    }, {});

/**
 * 治理规则统一的「规则标签」字段：只读 chips 展示 + 编辑入口，点开独立弹窗集中管理键值对。
 */
const RuleLabelField: React.FC<RuleLabelFieldProps> = ({ metadata, editable = false, onChange }) => {
    const [visible, setVisible] = React.useState(false);
    const [draft, setDraft] = React.useState<Array<{ key: string; value: string }>>([]);
    const rows = toRows(metadata);

    const open = () => {
        setDraft(toRows(metadata));
        setVisible(true);
    };

    const confirm = () => {
        onChange?.(fromRows(draft));
        setVisible(false);
    };

    return (
        <div className={style.labelDisplay}>
            {rows.length ? rows.map((row) => (
                <span className={style.chip} key={row.key}>
                    <span className={style.chipKey}>{row.key}</span>
                    <span className={style.chipValue}>{row.value}</span>
                </span>
            )) : <span className={style.emptyLine}>暂无规则标签</span>}
            {editable && (
                <Button variant="text" type="button" className={style.editChipButton} onClick={open}>
                    <Edit1Icon />编辑标签
                </Button>
            )}
            <Dialog
                header="编辑规则标签"
                visible={visible}
                width={520}
                confirmBtn="确定"
                cancelBtn="取消"
                onConfirm={confirm}
                onClose={() => setVisible(false)}
            >
                <div className={style.labelDialogHint}>标签用于标记规则归属、来源等元信息，标签键唯一。</div>
                <div className={style.labelDialogHeader}>
                    <span>标签键</span>
                    <span>标签值</span>
                    <span />
                </div>
                <div className={style.labelDialogRows}>
                    {draft.length ? draft.map((row, index) => (
                        <div className={style.labelDialogRow} key={`label-${index}`}>
                            <Input
                                aria-label={`第 ${index + 1} 条规则标签键`}
                                value={row.key}
                                placeholder="标签键"
                                onChange={(value) => setDraft((prev) => prev.map((item, idx) => idx === index ? { ...item, key: value } : item))}
                            />
                            <Input
                                aria-label={`第 ${index + 1} 条规则标签值`}
                                value={row.value}
                                placeholder="标签值"
                                onChange={(value) => setDraft((prev) => prev.map((item, idx) => idx === index ? { ...item, value } : item))}
                            />
                            <Button shape="circle" variant="text" onClick={() => setDraft((prev) => prev.filter((_, idx) => idx !== index))}><CloseIcon /></Button>
                        </div>
                    )) : <div className={style.emptyLine}>暂无规则标签</div>}
                </div>
                <Button variant="text" icon={<AddIcon />} onClick={() => setDraft((prev) => [...prev, { key: '', value: '' }])}>添加规则标签</Button>
            </Dialog>
        </div>
    );
};

export default React.memo(RuleLabelField);
