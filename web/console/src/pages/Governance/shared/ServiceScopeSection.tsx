import React from 'react';
import { Button, Popup, Select, Tag } from 'components/Fluent';
import { ChevronRightIcon, SendIcon } from 'components/Fluent/icons';

import styles from './ServiceScopeSection.module.less';
import { GovernanceServiceRole } from './serviceContext';

export interface ServiceScopeValue {
    namespace?: string;
    service?: string;
}

export interface ServiceScopeOption {
    label: React.ReactNode;
    value: string;
    namespace?: string;
}

interface ServiceScopeSectionProps {
    order?: number;
    title?: string;
    editable: boolean;
    collapsed: boolean;
    caller: ServiceScopeValue;
    callee: ServiceScopeValue;
    namespaceOptions?: ServiceScopeOption[];
    callerNamespaceOptions?: ServiceScopeOption[];
    calleeNamespaceOptions?: ServiceScopeOption[];
    callerServiceOptions: ServiceScopeOption[];
    calleeServiceOptions: ServiceScopeOption[];
    onCollapsedChange: (collapsed: boolean) => void;
    onCallerNamespaceChange: (value: string) => void;
    onCallerServiceChange: (value: string) => void;
    onCalleeNamespaceChange: (value: string) => void;
    onCalleeServiceChange: (value: string) => void;
    extraContent?: React.ReactNode;
    fixedRole?: GovernanceServiceRole;
}

const textFromOption = (options: ServiceScopeOption[], value?: string, fallback = '-') => {
    if (!value) return fallback;
    const label = options.find((item) => item.value === value)?.label;
    if (typeof label === 'string' || typeof label === 'number') return String(label);
    return value;
};

const renderPopupText = (value: string) => (
    <Popup trigger="hover" content={value || '-'}>
        <span className={styles.ellipsisText}>{value || '-'}</span>
    </Popup>
);

const ServiceScopeSection: React.FC<ServiceScopeSectionProps> = ({
    order,
    title = '服务范围',
    editable,
    collapsed,
    caller,
    callee,
    namespaceOptions,
    callerNamespaceOptions,
    calleeNamespaceOptions,
    callerServiceOptions,
    calleeServiceOptions,
    onCollapsedChange,
    onCallerNamespaceChange,
    onCallerServiceChange,
    onCalleeNamespaceChange,
    onCalleeServiceChange,
    extraContent,
    fixedRole,
}) => {
    const callerNamespaces = callerNamespaceOptions || namespaceOptions || [];
    const calleeNamespaces = calleeNamespaceOptions || namespaceOptions || [];
    const callerNamespaceText = textFromOption(callerNamespaces, caller.namespace, '*');
    const callerServiceText = textFromOption(callerServiceOptions, caller.service, '-');
    const calleeNamespaceText = textFromOption(calleeNamespaces, callee.namespace, '*');
    const calleeServiceText = textFromOption(calleeServiceOptions, callee.service, '-');
    const summary = `${callerNamespaceText}/${callerServiceText} -> ${calleeNamespaceText}/${calleeServiceText}`;

    const renderCard = (type: 'caller' | 'callee') => {
        const isCaller = type === 'caller';
        const namespaceValue = isCaller ? caller.namespace : callee.namespace;
        const serviceValue = isCaller ? caller.service : callee.service;
        const serviceOptions = isCaller ? callerServiceOptions : calleeServiceOptions;
        const currentNamespaceOptions = isCaller ? callerNamespaces : calleeNamespaces;
        const namespaceText = isCaller ? callerNamespaceText : calleeNamespaceText;
        const serviceText = isCaller ? callerServiceText : calleeServiceText;
        const isFixed = fixedRole === type;
        return (
            <div className={`${styles.serviceCard} ${isCaller ? styles.serviceCaller : styles.serviceCallee}`}>
                <div className={styles.serviceCardHead}>
                    <span className={styles.serviceDot} />
                    <div className={styles.serviceCardTitleGroup}>
                        <div className={styles.serviceTitle}>{isCaller ? '主调' : '被调'}</div>
                        <div className={styles.serviceSubtitle}>{isCaller ? '发起调用方' : '目标服务方'}</div>
                    </div>
                    {isFixed && <Tag className={styles.fixedTag} theme="primary" variant="light">当前服务</Tag>}
                </div>
                {editable && !isFixed ? (
                    <div className={styles.serviceFields}>
                        <div>
                            <div className={styles.fieldLabel}>命名空间</div>
                            <Select
                                aria-label={isCaller ? "主调命名空间" : "被调命名空间"}
                                filterable
                                creatable
                                options={currentNamespaceOptions}
                                value={namespaceValue || ''}
                                onChange={(value) => (isCaller ? onCallerNamespaceChange(value as string) : onCalleeNamespaceChange(value as string))}
                            />
                        </div>
                        <div>
                            <div className={styles.fieldLabel}>服务</div>
                            <Select
                                aria-label={isCaller ? "主调服务" : "被调服务"}
                                filterable
                                creatable
                                options={serviceOptions}
                                value={serviceValue || ''}
                                onChange={(value) => (isCaller ? onCallerServiceChange(value as string) : onCalleeServiceChange(value as string))}
                            />
                        </div>
                    </div>
                ) : (
                    <div className={styles.serviceReadonly}>
                        {renderPopupText(namespaceText)}
                        <span>/</span>
                        {renderPopupText(serviceText)}
                    </div>
                )}
            </div>
        );
    };

    return (
        <div className={styles.scopeSection}>
            <div
                className={`${styles.scopeHeader} ${collapsed ? styles.scopeCollapsedHeader : ''}`}
                onClick={() => onCollapsedChange(!collapsed)}
            >
                <div className={styles.scopeTitleWrap}>
                    {order !== undefined && <span className={styles.scopeNumber}>{order}</span>}
                    <div className={styles.scopeTitleLine}>
                        <span className={styles.scopeTitle}>{title}</span>
                        <Popup trigger="hover" content={collapsed ? summary : '主调方 -> 被调方'}>
                            <span className={`${styles.scopeHint} ${collapsed ? styles.scopeSummary : ''}`}>
                                {collapsed ? summary : '主调方 -> 被调方'}
                            </span>
                        </Popup>
                    </div>
                </div>
                <Button
                    aria-label={collapsed ? `展开${title}` : `收起${title}`}
                    variant="text"
                    type="button"
                    className={`${styles.caretButton} ${collapsed ? '' : styles.caretButtonOpen}`}
                    onClick={(event) => {
                        event.stopPropagation();
                        onCollapsedChange(!collapsed);
                    }}
                >
                    <ChevronRightIcon />
                </Button>
            </div>
            {!collapsed && (
                <div className={styles.scopeBody}>
                    <div className={styles.serviceFlowGrid}>
                        {renderCard('caller')}
                        <div className={styles.flowConnector}><SendIcon /></div>
                        {renderCard('callee')}
                    </div>
                    {extraContent && <div className={styles.extraContent}>{extraContent}</div>}
                </div>
            )}
        </div>
    );
};

export default ServiceScopeSection;
