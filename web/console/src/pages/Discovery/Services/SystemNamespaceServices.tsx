import React from 'react'
import { Button, Input, Table } from 'components/Fluent'
import { useNavigate } from 'components/Router'

import { ResourceToolbar } from 'components/ResourceLayout'
import ResourceNameLink from 'components/ResourceNameLink'
import { ConfirmOperationButton, OperationButtonGroup } from 'components/OperationButton'
import { deleteServices, describeServices, ServiceView } from 'services/service'
import { openErrNotification, openInfoNotification } from 'utils/notifition'
import ServiceEditor from './ServiceEditor'
import style from './index.module.less'

const pageSize = 10

export default function SystemNamespaceServices({ namespace, system = true }: { namespace: string, system?: boolean }) {
  const navigate = useNavigate()
  const [services, setServices] = React.useState<ServiceView[]>([])
  const [total, setTotal] = React.useState(0)
  const [page, setPage] = React.useState(1)
  const [keyword, setKeyword] = React.useState('')
  const [loading, setLoading] = React.useState(true)
  const [error, setError] = React.useState('')
  const [appliedKeyword, setAppliedKeyword] = React.useState('')
  const [creatorVisit, setCreatorVisit] = React.useState<number | null>(null)
  const [created, setCreated] = React.useState<{ visit: number; name: string } | null>(null)
  const [loadedVisit, setLoadedVisit] = React.useState<number | null>(null)
  const requestRef = React.useRef(0)
  const visitRef = React.useRef({ namespace, system, generation: 0 })
  if (visitRef.current.namespace !== namespace || visitRef.current.system !== system) {
    visitRef.current = { namespace, system, generation: visitRef.current.generation + 1 }
  }
  const visit = visitRef.current.generation
  const isCurrentVisit = () => visitRef.current.generation === visit
  const createdName = created?.visit === visit ? created.name : ''

  const refresh = React.useCallback(async (nextPage = page) => {
    if (!isCurrentVisit()) return
    const request = ++requestRef.current
    setLoading(true)
    setError('')
    try {
      const result = await describeServices({
        namespace,
        name: keyword || undefined,
        offset: (nextPage - 1) * pageSize,
        limit: pageSize,
      })
      if (!isCurrentVisit() || request !== requestRef.current) return
      setServices(result.list)
      setTotal(result.totalCount)
      setAppliedKeyword(keyword)
      setLoadedVisit(visit)
    } catch (reason) {
      if (!isCurrentVisit() || request !== requestRef.current) return
      setLoadedVisit(visit)
      setError(reason instanceof Error ? reason.message : String(reason))
      openErrNotification(`获取${system ? '系统' : '环境'}服务失败`, reason instanceof Error ? reason.message : String(reason))
    } finally {
      if (isCurrentVisit() && request === requestRef.current) setLoading(false)
    }
  }, [keyword, namespace, page, system, visit])

  React.useEffect(() => {
    setCreated(null)
    setCreatorVisit(null)
    setPage(1)
    void refresh(1)
    return () => { requestRef.current += 1 }
  }, [namespace, system])

  const remove = async (row: ServiceView) => {
    if (!isCurrentVisit() || row.namespace !== namespace) return
    try {
      await deleteServices([{ id: row.id, namespace: row.namespace, name: row.name }])
      if (!isCurrentVisit()) return
      openInfoNotification('删除成功', `${system ? '系统' : '环境'}服务 ${row.namespace}/${row.name} 已删除`)
      await refresh()
    } catch (error) {
      if (!isCurrentVisit()) return
      openErrNotification(`删除${system ? '系统' : '环境'}服务失败`, error instanceof Error ? error.message : String(error))
    }
  }

  const columns = [
    {
      colKey: 'name',
      title: system ? '系统服务' : '环境服务',
      width: 280,
      cell: ({ row }: any) => (
        <ResourceNameLink
          name={row.name}
          onClick={() => navigate(
            `/discovery/service/instance?namespace=${encodeURIComponent(namespace)}`
            + `&service=${encodeURIComponent(row.name)}`,
          )}
        />
      ),
    },
    { colKey: 'namespace', title: system ? '系统空间' : '业务环境', width: 180 },
    {
      colKey: 'health',
      title: '健康实例',
      width: 150,
      cell: ({ row }: any) => `${row.healthy_instance_count || 0}/${row.total_instance_count || 0}`,
    },
    { colKey: 'mtime', title: '更新时间', width: 180 },
    {
      colKey: 'action',
      title: '操作',
      width: 160,
      cell: ({ row }: any) => (
        <OperationButtonGroup>
          <Button variant="text" onClick={() => navigate(
            `/discovery/service/instance?namespace=${encodeURIComponent(namespace)}`
            + `&service=${encodeURIComponent(row.name)}&tab=instances`,
          )}>查看实例</Button>
          <ConfirmOperationButton
            action="delete"
            label={`删除${system ? '系统' : '环境'}服务`}
            disabled={row.deleteable === false}
            disabledLabel="无权限操作"
            confirmContent={`确认删除${system ? '系统' : '环境'}服务 ${row.namespace}/${row.name} 吗？`}
            onConfirm={() => void remove(row)}
          />
        </OperationButtonGroup>
      ),
    },
  ]

  return (
    <section className={style.listSection}>
      <ResourceToolbar
        density="compact"
        title={`${namespace} 服务`}
        count={loading || loadedVisit !== visit ? '正在同步列表' : error ? '加载失败' : `共 ${total} 条`}
        filters={(
          <>
            <Input value={keyword} placeholder={`搜索${system ? '系统' : '环境'}服务`} onChange={setKeyword} />
            <Button variant="outline" onClick={() => { setPage(1); void refresh(1) }}>查询</Button>
            {!system && <Button theme="primary" onClick={() => setCreatorVisit(visit)}>新建环境服务</Button>}
          </>
        )}
      />
      {createdName && !system && <div className={style.onboarding}><div><strong>环境服务 {createdName} 已创建</strong><p>下一步在实例列表选择“新建实例”，注册后查看健康状态。</p></div><Button variant="outline" onClick={() => navigate(`/discovery/service/instance?namespace=${encodeURIComponent(namespace)}&service=${encodeURIComponent(createdName)}&tab=instances`)}>查看并注册实例</Button></div>}
      {loadedVisit === visit && error && <p role="alert">服务列表加载失败：{error}。请重新查询。</p>}
      <section className={`${style.tableSurface} ${style.serviceTableSurface}`}>
        <Table
          data={loadedVisit !== visit || error ? [] : services}
          columns={columns}
          loading={loading || loadedVisit !== visit}
          rowKey="id"
          pagination={{
            current: page,
            pageSize,
            total,
            onChange: (info: any) => {
              const nextPage = info.current || 1
              setPage(nextPage)
              void refresh(nextPage)
            },
          }}
          empty={error ? '无法获取服务列表' : appliedKeyword ? '没有匹配的服务。' : `当前${system ? '系统空间' : '业务环境'}没有可见服务。`}
        />
      </section>
      {!system && <ServiceEditor key={visit} op="create" visible={creatorVisit === visit} fixedNamespace={namespace}
        onCreated={(service) => { if (!isCurrentVisit()) return; setCreated({ visit, name: service.name }); setPage(1); void refresh(1) }}
        closeDrawer={() => { if (isCurrentVisit()) setCreatorVisit(null) }} />}
    </section>
  )
}
