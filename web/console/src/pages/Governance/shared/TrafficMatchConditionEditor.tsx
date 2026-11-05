import React from 'react';
import { Button, Input, Radio, RadioGroup, Select, Tag, TagInput, Tooltip } from 'components/Fluent';
import { AddIcon, DeleteIcon, InfoCircleIcon } from 'components/Fluent/icons';

import { MatchLogic, MatchType, MatchTypeMap, MatchTypeOption, MatchValueType, MatchValueTypeMap, MatchValueTypeOption } from 'services/types';
import { commaStringToTags, isTagInputMatchType, tagsToCommaString } from '../Router/routeEditorUtils';

import styles from './TrafficMatchConditionEditor.module.less';

export interface TrafficMatchConditionRow {
    paramType?: string;
    paramKey?: string;
    matchType?: string;
    valueType?: string;
    matchValue?: string;
}

interface OptionItem {
    label: string;
    value: string;
}

interface TrafficMatchConditionEditorProps {
    rows: TrafficMatchConditionRow[];
    contextLabel?: string;
    paramTypeOptions: OptionItem[];
    editable: boolean;
    showParamKey?: boolean;
    relation?: string;
    relationEditable?: boolean;
    title?: React.ReactNode;
    hint?: React.ReactNode;
    addText?: string;
    minRows?: number;
    className?: string;
    extraControl?: React.ReactNode;
    onRelationChange?: (relation: string) => void;
    onRowChange: (index: number, row: TrafficMatchConditionRow) => void;
    onAdd: () => void;
    onRemove: (index: number) => void;
}

const getOptionLabel = (options: OptionItem[], value?: string) => {
    const option = options.find(item => item.value === value);
    return option?.label || value || '-';
};

const renderReadonlyValue = (value?: string) => (
    <span className={styles.readonlyText}>{value || '-'}</span>
);

const TrafficMatchConditionEditor: React.FC<TrafficMatchConditionEditorProps> = ({
    rows,
    contextLabel = '匹配条件',
    paramTypeOptions,
    editable,
    showParamKey = true,
    relation = MatchLogic.AND,
    relationEditable = true,
    title = '匹配条件',
    hint,
    addText = '添加匹配条件',
    minRows = 1,
    className,
    extraControl,
    onRelationChange,
    onRowChange,
    onAdd,
    onRemove,
}) => {
    const currentRows = rows.length ? rows : [];
    const relationText = relation === MatchLogic.OR ? '同一规则内条件按 OR 关系计算' : '同一规则内条件按 AND 关系计算';
    const relationSummary = relation === MatchLogic.OR ? '任一满足（OR）' : '全部满足（AND）';

    const updateRow = (index: number, patch: Partial<TrafficMatchConditionRow>) => {
        const row = currentRows[index] || {};
        onRowChange(index, { ...row, ...patch });
    };

    const isRequestParameter = (row: TrafficMatchConditionRow) =>
        (row.valueType || MatchValueType.TEXT) === MatchValueType.PARAMETER;
    const hasRequestParameter = currentRows.some(isRequestParameter);

    return (
        <div className={`${styles.conditionEditor} ${className || ''}`}>
            <div className={styles.conditionHead}>
                <div>
                    <div className={styles.conditionTitle}>{title}</div>
                    <div className={styles.conditionHint}>{hint || relationText}</div>
                </div>
                <div className={styles.headActions}>
                    {extraControl}
                    {editable && relationEditable ? (
                        <div className={styles.relationControl}>
                            <span className={styles.relationLabel}>条件关系</span>
                            <RadioGroup
                                aria-label={`${contextLabel}条件关系`}
                                layout="horizontal"
                                value={relation}
                                onChange={(value) => onRelationChange?.(value as string)}
                            >
                                <Radio value={MatchLogic.AND}>全部满足</Radio>
                                <Radio value={MatchLogic.OR}>任一满足</Radio>
                            </RadioGroup>
                        </div>
                    ) : (
                        <Tag variant="outline">{relationSummary}</Tag>
                    )}
                </div>
            </div>

            <div className={styles.conditionSurface}>
                <div className={`${styles.conditionTable} ${showParamKey ? '' : styles.conditionTableNoKey}`} role="table" aria-label={`${contextLabel}匹配条件`}>
                    <div className={styles.tableHeaderRow} role="row">
                        <div className={styles.tableHeader} role="columnheader">参数类型</div>
                        {showParamKey && <div className={styles.tableHeader} role="columnheader">参数键</div>}
                        <div className={styles.tableHeader} role="columnheader">匹配类型</div>
                        <div className={styles.tableHeader} role="columnheader">值来源</div>
                        <div className={styles.tableHeader} role="columnheader">匹配值</div>
                        <div className={`${styles.tableHeader} ${styles.actionHeader}`} role="columnheader">操作</div>
                    </div>
                    {currentRows.map((row, index) => {
                        const matchType = row.matchType || MatchType.EXACT;
                        const valueType = row.valueType || MatchValueType.TEXT;
                        const requestParameter = isRequestParameter(row);
                        return (
                            <div
                                className={styles.conditionRow}
                                role="row"
                                aria-label={`${contextLabel}第 ${index + 1} 条匹配条件`}
                                key={`condition-${index}`}
                            >
                            <div className={styles.tableField} role="cell">
                                <span className={styles.fieldLabel}>参数类型</span>
                                {editable ? (
                                    <Select
                                        aria-label={`${contextLabel}第 ${index + 1} 条匹配条件：参数类型`}
                                        options={paramTypeOptions}
                                        value={row.paramType}
                                        onChange={(value) => updateRow(index, { paramType: value as string })}
                                    />
                                ) : renderReadonlyValue(getOptionLabel(paramTypeOptions, row.paramType))}
                            </div>
                            {showParamKey && (
                                <div className={styles.tableField} role="cell">
                                    <span className={styles.fieldLabel}>参数键</span>
                                    {editable ? (
                                        <Input
                                            aria-label={`${contextLabel}第 ${index + 1} 条匹配条件：参数键`}
                                            value={row.paramKey || ''}
                                            placeholder="请输入参数键"
                                            onChange={(value) => updateRow(index, { paramKey: value })}
                                        />
                                    ) : renderReadonlyValue(row.paramKey)}
                                </div>
                            )}
                            <div className={styles.tableField} role="cell">
                                <span className={styles.fieldLabel}>匹配类型</span>
                                {requestParameter ? (
                                    <span className={styles.captureHint}>采集参数</span>
                                ) : editable ? (
                                    <Select
                                        aria-label={`${contextLabel}第 ${index + 1} 条匹配条件：匹配类型`}
                                        options={MatchTypeOption}
                                        value={matchType}
                                        onChange={(value) => updateRow(index, { matchType: value as string })}
                                    />
                                ) : renderReadonlyValue(MatchTypeMap[matchType as MatchType] || matchType)}
                            </div>
                            <div className={styles.tableField} role="cell">
                                <span className={styles.fieldLabel}>值来源</span>
                                {editable ? (
                                    <Select
                                        aria-label={`${contextLabel}第 ${index + 1} 条匹配条件：值来源`}
                                        options={MatchValueTypeOption}
                                        value={valueType}
                                        onChange={(value) => {
                                            const nextValueType = value as string;
                                            updateRow(index, {
                                                valueType: nextValueType,
                                                matchType: nextValueType === MatchValueType.PARAMETER ? MatchType.EXACT : matchType,
                                                matchValue: nextValueType === MatchValueType.PARAMETER ? '' : row.matchValue,
                                            });
                                        }}
                                    />
                                ) : renderReadonlyValue(MatchValueTypeMap[valueType as MatchValueType] || valueType)}
                            </div>
                            <div className={`${styles.tableField} ${styles.valueField}`} role="cell">
                                <span className={styles.fieldLabel}>匹配值</span>
                                {requestParameter ? (
                                    <span className={styles.captureHint}>采集该键的请求值</span>
                                ) : editable ? (
                                    isTagInputMatchType(matchType) ? (
                                        <TagInput
                                            aria-label={`${contextLabel}第 ${index + 1} 条匹配条件：匹配值`}
                                            value={commaStringToTags(row.matchValue)}
                                            placeholder="请输入匹配值，回车分隔"
                                            onChange={(value) => updateRow(index, { matchValue: tagsToCommaString(value as Array<string | number>) })}
                                        />
                                    ) : (
                                        <Input
                                            aria-label={`${contextLabel}第 ${index + 1} 条匹配条件：匹配值`}
                                            value={row.matchValue || ''}
                                            placeholder="请输入匹配值"
                                            onChange={(value) => updateRow(index, { matchValue: value })}
                                        />
                                    )
                                ) : renderReadonlyValue(row.matchValue)}
                            </div>
                            <div className={styles.actionCell} role="cell">
                                {editable && (
                                    <Tooltip content={currentRows.length <= minRows ? `至少保留 ${minRows} 条条件` : '删除匹配条件'} placement="top">
                                        <span>
                                            <Button
                                                aria-label={`${contextLabel}删除第 ${index + 1} 条匹配条件`}
                                                shape="circle"
                                                variant="text"
                                                disabled={currentRows.length <= minRows}
                                                icon={<DeleteIcon />}
                                                onClick={() => onRemove(index)}
                                            />
                                        </span>
                                    </Tooltip>
                                )}
                            </div>
                            </div>
                        );
                    })}
                    {!currentRows.length && <div className={styles.emptyState}>暂无匹配条件</div>}
                </div>

                {editable && (
                    <div className={styles.addBar}>
                        <Button aria-label={`${contextLabel}${addText}`} variant="text" icon={<AddIcon />} onClick={onAdd}>
                            {addText}
                        </Button>
                    </div>
                )}
            </div>

            {hasRequestParameter && (
                <div className={styles.capabilityHint}>
                    <InfoCircleIcon />
                    请求参数会采集当前参数键的实际请求值并交给治理策略消费；Proxyless SDK 已支持，xDS 当前不消费该动态语义。
                </div>
            )}
        </div>
    );
};

export default React.memo(TrafficMatchConditionEditor);
