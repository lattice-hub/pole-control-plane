import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Input,
  Select,
  TabPanel,
  Table,
  Tabs,
  Textarea,
  Tooltip,
} from 'components/Fluent'
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer'
import { AddIcon, RefreshIcon } from 'components/Fluent/icons'
import { useNavigate } from 'components/Router'

import { ResourceToolbar } from 'components/ResourceLayout'
import ResourceNameLink from 'components/ResourceNameLink'
import QueryComposer from 'components/QueryComposer'
import { ConfirmOperationButton, OperationButtonGroup } from 'components/OperationButton'
import {
  bindServiceEnvironment,
  createLogicalService,
  deleteLogicalService,
  describeAllLogicalServices,
  describeLogicalServices,
  describeUnboundServiceEnvironments,
  LogicalServiceView,
} from 'services/logical_service'
import { deleteServices, describeServices, ServiceView } from 'services/service'
import { openErrNotification, openInfoNotification } from 'utils/notifition'
import SystemNamespaceServices from './SystemNamespaceServices'
import ServiceEditor from './ServiceEditor'
import style from './index.module.less'

const pageSize = 10
type ServiceWorkspaceTab = 'logical' | 'unbound' | 'system'

interface ServicesTableProps {
  initialTab?: ServiceWorkspaceTab
  systemNamespace?: string
}

const ServicesTable = ({ initialTab = 'logical', systemNamespace = 'pole-system' }: ServicesTableProps) => {
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<ServiceWorkspaceTab>(initialTab)
  const [logicalServices, setLogicalServices] = useState<LogicalServiceView[]>([])
  const [logicalServiceOptions, setLogicalServiceOptions] = useState<LogicalServiceView[]>([])
  const [logicalTotal, setLogicalTotal] = useState(0)
  const [unboundServices, setUnboundServices] = useState<ServiceView[]>([])
  const [unboundTotal, setUnboundTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [loading, setLoading] = useState(true)
  const [hasLoaded, setHasLoaded] = useState(false)
  const [loadError, setLoadError] = useState('')
  const [appliedKeyword, setAppliedKeyword] = useState('')
  const [creatorVisible, setCreatorVisible] = useState(false)
  const [environmentCreatorVisible, setEnvironmentCreatorVisible] = useState(false)
  const [createdEnvironment, setCreatedEnvironment] = useState<{ namespace: string; name: string } | null>(null)
  const [syncState, setSyncState] = useState<'waiting' | 'ready' | 'retry'>('waiting')
  const [syncAttempt, setSyncAttempt] = useState(0)
  const syncEpoch = React.useRef(0)
  const [syncError, setSyncError] = useState('')
  const listContext = React.useRef({ activeTab, page, keyword, appliedKeyword })
  listContext.current = { activeTab, page, keyword, appliedKeyword }
  const [bindingTarget, setBindingTarget] = useState<ServiceView | null>(null)
  const [selectedLogicalId, setSelectedLogicalId] = useState('')
  const requestRef = React.useRef(0)
  const [draft, setDraft] = useState({ name: '', comment: '', business: '', department: '' })

  const loadLogicalServices = (nextPage: number, nextKeyword: string) => describeLogicalServices({
    name: nextKeyword || undefined, offset: (nextPage - 1) * pageSize, limit: pageSize,
  })

  const refresh = useCallback(async (nextPage = page, nextKeyword = keyword, isCurrent: () => boolean = () => true) => {
    if (activeTab === 'system') return
    const request = ++requestRef.current
    setLoading(true)
    setLoadError('')
    try {
      if (activeTab === 'logical') {
        const result = await loadLogicalServices(nextPage, nextKeyword)
        if (request !== requestRef.current || !isCurrent()) return
        setLogicalServices(result.list)
        setLogicalTotal(result.totalCount)
      } else {
        const [unbound, options, logical] = await Promise.all([
          describeUnboundServiceEnvironments({ name: nextKeyword || undefined, offset: (nextPage - 1) * pageSize, limit: pageSize }),
          describeAllLogicalServices(),
          loadLogicalServices(1, ''),
        ])
        if (request !== requestRef.current || !isCurrent()) return
        setUnboundServices(unbound.list)
        setUnboundTotal(unbound.totalCount)
        setLogicalServiceOptions(options)
        setLogicalServices(logical.list)
        setLogicalTotal(logical.totalCount)
      }
      setAppliedKeyword(nextKeyword)
      setHasLoaded(true)
    } catch (error) {
      if (request !== requestRef.current || !isCurrent()) return
      setLoadError(error instanceof Error ? error.message : String(error))
      openErrNotification('获取服务失败', error instanceof Error ? error.message : String(error))
    } finally {
      if (request === requestRef.current) setLoading(false)
    }
  }, [activeTab, keyword, page])

  useEffect(() => {
    setActiveTab(initialTab)
  }, [initialTab])

  useEffect(() => {
    setPage(1)
    setHasLoaded(false)
    if (activeTab !== 'system') void refresh(1)
    return () => { requestRef.current += 1 }
  }, [activeTab])

  useEffect(() => {
    if (!createdEnvironment) return
    const epoch = ++syncEpoch.current
    let timer: ReturnType<typeof setTimeout> | undefined
    let attempts = 0
    let lastError = ''
    const snapshot = listContext.current
    setSyncError('')
    setSyncState('waiting')
    const check = async () => {
      if (epoch !== syncEpoch.current) return
      attempts += 1
      try {
        const result = await describeServices({ namespace: createdEnvironment.namespace, name: createdEnvironment.name, offset: 0, limit: 1 })
        if (epoch !== syncEpoch.current) return
        if (result.list.some(item => item.namespace === createdEnvironment.namespace && item.name === createdEnvironment.name)) {
          setSyncState('ready')
          const isCurrent = () => epoch === syncEpoch.current && listContext.current.activeTab === 'unbound' && listContext.current.page === snapshot.page && listContext.current.keyword === snapshot.keyword && listContext.current.appliedKeyword === snapshot.appliedKeyword
          if (isCurrent()) void refresh(snapshot.page, snapshot.appliedKeyword, isCurrent)
          return
        }
        lastError = ''
      } catch (error) {
        if (epoch !== syncEpoch.current) return
        lastError = error instanceof Error ? error.message : String(error)
      }
      if (attempts >= 4) { setSyncError(lastError); setSyncState('retry') }
      else timer = setTimeout(() => void check(), 1000)
    }
    void check()
    return () => { syncEpoch.current += 1; if (timer) clearTimeout(timer) }
  }, [createdEnvironment, syncAttempt])

  const cancelSync = () => {
    syncEpoch.current += 1
    setSyncError('')
    setSyncState(current => current === 'waiting' ? 'retry' : current)
  }

  const changeTab = (value: string | number) => {
    cancelSync()
    const nextTab = String(value) as ServiceWorkspaceTab
    setActiveTab(nextTab)
    setKeyword('')
    setPage(1)
    if (nextTab === 'system') {
      navigate(`/discovery/service?scope=system&namespace=${encodeURIComponent(systemNamespace)}`)
      return
    }
    navigate(`/discovery/service?view=${nextTab}`)
  }

  const metrics = useMemo(() => logicalServices.reduce((result, item) => ({
    environments: result.environments + Number(item.environment_count || 0),
    healthy: result.healthy + Number(item.healthy_instance_count || 0),
    total: result.total + Number(item.total_instance_count || 0),
  }), { environments: 0, healthy: 0, total: 0 }), [logicalServices])

  const create = async () => {
    if (!draft.name.trim()) {
      openErrNotification('无法创建', '请输入逻辑服务名称')
      return
    }
    try {
      await createLogicalService({ ...draft, name: draft.name.trim() })
      openInfoNotification('创建成功', '逻辑服务已创建')
      setCreatorVisible(false)
      setDraft({ name: '', comment: '', business: '', department: '' })
      await refresh(1)
    } catch (error) {
      openErrNotification('创建失败', error instanceof Error ? error.message : String(error))
    }
  }

  const bind = async () => {
    if (!bindingTarget?.id || !selectedLogicalId) return
    try {
      await bindServiceEnvironment(selectedLogicalId, bindingTarget.id)
      openInfoNotification('关联成功', `${bindingTarget.namespace}/${bindingTarget.name} 已加入逻辑服务`)
      setBindingTarget(null)
      setSelectedLogicalId('')
      await refresh()
    } catch (error) {
      openErrNotification('关联失败', error instanceof Error ? error.message : String(error))
    }
  }

  const removeLogicalService = async (row: LogicalServiceView) => {
    try {
      await deleteLogicalService(row.id)
      openInfoNotification('删除成功', `逻辑服务 ${row.name} 已删除，环境服务未受影响`)
      await refresh()
    } catch (error) {
      openErrNotification('删除逻辑服务失败', error instanceof Error ? error.message : String(error))
    }
  }

  const removeEnvironmentService = async (row: ServiceView) => {
    try {
      await deleteServices([{ id: row.id, namespace: row.namespace, name: row.name }])
      openInfoNotification('删除成功', `环境服务 ${row.namespace}/${row.name} 已删除`)
      await refresh()
    } catch (error) {
      openErrNotification('删除环境服务失败', error instanceof Error ? error.message : String(error))
    }
  }

  const logicalColumns = [
    {
      colKey: 'name',
      title: '逻辑服务',
      width: 260,
      cell: ({ row }: any) => (
        <ResourceNameLink name={row.name} onClick={() => navigate(`detail?id=${encodeURIComponent(row.id)}`)} />
      ),
    },
    {
      colKey: 'environment_count',
      title: '可访问环境',
      width: 130,
      cell: ({ row }: any) => `${row.environment_count || 0} 个`,
    },
    {
      colKey: 'health',
      title: '健康实例',
      width: 150,
      cell: ({ row }: any) => `${row.healthy_instance_count || 0}/${row.total_instance_count || 0}`,
    },
    {
      colKey: 'owner',
      title: '归属',
      width: 180,
      cell: ({ row }: any) => row.department || row.business || row.owners || '-',
    },
    {
      colKey: 'mtime',
      title: '更新时间',
      width: 180,
      cell: ({ row }: any) => row.mtime || '-',
    },
    {
      colKey: 'action',
      title: '操作',
      width: 140,
      cell: ({ row }: any) => (
        <OperationButtonGroup>
          <Button variant="text" onClick={() => navigate(`detail?id=${encodeURIComponent(row.id)}`)}>查看</Button>
          <ConfirmOperationButton
            action="delete"
            disabled={row.deleteable === false || Number(row.environment_count || 0) > 0}
            disabledLabel={row.deleteable === false ? '无权限操作' : '请先解除环境关联'}
            confirmContent={`确认删除逻辑服务 ${row.name} 吗？环境服务不会被删除。`}
            onConfirm={() => void removeLogicalService(row)}
          />
        </OperationButtonGroup>
      ),
    },
  ]

  const unboundColumns = [
    {
      colKey: 'name',
      title: '运行时服务',
      width: 260,
      cell: ({ row }: any) => (
        <ResourceNameLink
          name={row.name}
          onClick={() => navigate(`instance?namespace=${encodeURIComponent(row.namespace)}&service=${encodeURIComponent(row.name)}`)}
        />
      ),
    },
    { colKey: 'namespace', title: '命名空间', width: 180 },
    {
      colKey: 'health',
      title: '健康实例',
      width: 150,
      cell: ({ row }: any) => `${row.healthy_instance_count || 0}/${row.total_instance_count || 0}`,
    },
    {
      colKey: 'action',
      title: '操作',
      width: 220,
      cell: ({ row }: any) => (
        <OperationButtonGroup>
          <Button variant="text" onClick={() => navigate(`instance?namespace=${encodeURIComponent(row.namespace)}&service=${encodeURIComponent(row.name)}`)}>进入环境</Button>
          <Button variant="text" onClick={() => setBindingTarget(row)}>关联</Button>
          <ConfirmOperationButton
            action="delete"
            disabled={row.deleteable === false}
            disabledLabel="无权限操作"
            confirmContent={`确认删除环境服务 ${row.namespace}/${row.name} 吗？`}
            onConfirm={() => void removeEnvironmentService(row)}
          />
        </OperationButtonGroup>
      ),
    },
  ]

  const currentTotal = activeTab === 'logical' ? logicalTotal : unboundTotal
  const currentData = activeTab === 'logical' ? logicalServices : unboundServices

  return (
    <div className={style.workspace}>
      {activeTab !== 'system' && (
        <section className={style.metricRail}>
          <div className={style.metricItem}><span>逻辑服务</span><strong>{!hasLoaded || loadError ? '—' : logicalTotal}</strong></div>
          <div className={style.metricItem}><span>环境服务</span><strong>{!hasLoaded || loadError ? '—' : metrics.environments}</strong></div>
          <div className={style.metricItem}><span>健康实例</span><strong>{!hasLoaded || loadError ? '—' : `${metrics.healthy}/${metrics.total}`}</strong></div>
        </section>
      )}
      <Tabs className={style.workspaceTabs} value={activeTab} onChange={changeTab}>
        <TabPanel value="logical" label="逻辑服务" />
        <TabPanel value="unbound" label={`未关联环境服务${unboundTotal ? ` (${unboundTotal})` : ''}`} />
        <TabPanel value="system" label="系统服务" />
      </Tabs>
      {activeTab === 'system' ? (
        <SystemNamespaceServices namespace={systemNamespace} />
      ) : <section className={style.listSection}>
        <ResourceToolbar
          density="compact"
          title={activeTab === 'logical' ? '逻辑服务清单' : '未关联环境服务'}
          count={loading ? '正在同步列表' : loadError ? '加载失败' : `共 ${currentTotal} 条`}
          filters={(
            <>
              <QueryComposer
                keyword={keyword}
                keywordPlaceholder={activeTab === 'logical' ? '搜索逻辑服务' : '搜索运行时服务'}
                suggestions={currentData.map((item) => String(item.name || '')).filter(Boolean)}
                onKeywordChange={(value) => { cancelSync(); setKeyword(value) }}
                onSubmit={({ keyword: nextKeyword }) => {
                  cancelSync()
                  setPage(1)
                  void refresh(1, nextKeyword)
                }}
                onReset={() => {
                  cancelSync()
                  setKeyword('')
                  setPage(1)
                  void refresh(1, '')
                }}
                actions={(
                  <>
                    <Tooltip content="刷新服务列表">
                      <Button shape="square" variant="outline" onClick={() => void refresh(page, keyword)}>
                        <RefreshIcon />
                      </Button>
                    </Tooltip>
                    {activeTab === 'logical' && (
                      <Button theme="primary" icon={<AddIcon />} onClick={() => setCreatorVisible(true)}>新建逻辑服务</Button>
                    )}
                    {activeTab === 'unbound' && (
                      <Button theme="primary" icon={<AddIcon />} onClick={() => { cancelSync(); setCreatedEnvironment(null); setEnvironmentCreatorVisible(true) }}>新建环境服务</Button>
                    )}
                  </>
                )}
              />
            </>
          )}
        />
        {activeTab === 'unbound' && createdEnvironment && (
          <div className={style.onboarding}>
            <div>
              <strong>环境服务 {createdEnvironment.namespace}/{createdEnvironment.name} 已创建</strong>
              <p>下一步在实例列表选择“新建实例”，注册后查看健康状态。</p>
              {syncState === 'waiting' && <p role="status">正在等待服务同步…</p>}
              {syncState === 'retry' && <p role="status">{syncError ? `服务查询失败：${syncError}。请重试。` : '服务尚未同步或同步检查已停止，请重试。'}</p>}
            </div>
            {syncState === 'retry' ? (
              <Button variant="outline" onClick={() => setSyncAttempt(value => value + 1)}>重试同步</Button>
            ) : (
              <Button variant="outline" disabled={syncState !== 'ready'} onClick={() => navigate(`/discovery/service/instance?namespace=${encodeURIComponent(createdEnvironment.namespace)}&service=${encodeURIComponent(createdEnvironment.name)}&tab=instances`)}>查看并注册实例</Button>
            )}
          </div>
        )}
        <section className={`${style.tableSurface} ${style.serviceTableSurface}`}>
          {loadError && <p role="alert">服务列表加载失败：{loadError}。请刷新重试。</p>}
          <Table
            data={loadError ? [] : currentData}
            columns={activeTab === 'logical' ? logicalColumns : unboundColumns}
            loading={loading}
            rowKey="id"
            pagination={{
              current: page,
              pageSize,
              total: currentTotal,
              onChange: (info: any) => {
                cancelSync()
                const nextPage = info.current || 1
                setPage(nextPage)
                void refresh(nextPage)
              },
            }}
            empty={loadError ? '无法获取服务列表' : appliedKeyword ? '没有匹配的服务。' : activeTab === 'logical'
              ? '暂无逻辑服务。首次接入可先选择环境并创建环境服务；跨环境聚合时再创建逻辑服务。'
              : '暂无未关联的环境服务。可选择环境接入服务，或调整查询条件。'}
          />
        </section>
      </section>}

      <EditorDrawer
        visible={creatorVisible}
        bodyClassName={style.serviceOperationBody}
        title="新建逻辑服务"
        description="建立跨环境聚合身份，后续再显式关联各环境运行时服务。"
        width="compact"
        onClose={() => setCreatorVisible(false)}
        footer={<EditorDrawerActions onCancel={() => setCreatorVisible(false)} onSubmit={() => void create()} submitText="创建逻辑服务" />}
      >
        <div className={style.logicalForm}>
          <label><span>逻辑服务名称</span><Input value={draft.name} onChange={name => setDraft(current => ({ ...current, name }))} /></label>
          <label><span>业务</span><Input value={draft.business} onChange={business => setDraft(current => ({ ...current, business }))} /></label>
          <label><span>部门</span><Input value={draft.department} onChange={department => setDraft(current => ({ ...current, department }))} /></label>
          <label><span>说明</span><Textarea value={draft.comment} onChange={comment => setDraft(current => ({ ...current, comment }))} /></label>
        </div>
      </EditorDrawer>

      <ServiceEditor
        op="create"
        visible={environmentCreatorVisible}
        onCreated={(service) => {
          setSyncState('waiting')
          setSyncError('')
          setCreatedEnvironment({ namespace: service.namespace, name: service.name })
          setPage(1)
          setKeyword('')
          setAppliedKeyword('')
          void refresh(1, '')
        }}
        closeDrawer={() => setEnvironmentCreatorVisible(false)}
      />

      <EditorDrawer
        visible={Boolean(bindingTarget)}
        bodyClassName={style.serviceOperationBody}
        title="关联到逻辑服务"
        description="把当前环境服务纳入一个跨环境逻辑身份。"
        width="compact"
        onClose={() => setBindingTarget(null)}
        footer={<EditorDrawerActions onCancel={() => setBindingTarget(null)} onSubmit={() => void bind()} submitText="确认关联" submitDisabled={!selectedLogicalId} />}
      >
        <div className={style.logicalForm}>
          <p>环境服务：{bindingTarget?.namespace}/{bindingTarget?.name}</p>
          <label>
            <span>目标逻辑服务</span>
            <Select
              value={selectedLogicalId}
              placeholder="选择逻辑服务"
              options={logicalServiceOptions.map(item => ({ label: item.name, value: item.id }))}
              onChange={setSelectedLogicalId}
            />
          </label>
        </div>
      </EditorDrawer>
    </div>
  )
}

export default React.memo(ServicesTable)
