import React, { useCallback, useEffect, useState } from 'react'
import { Button, Input, Select, Table } from 'components/Fluent'
import EditorDrawer, { EditorDrawerActions } from 'components/EditorDrawer'
import { useNavigate, useSearchParams } from 'components/Router'

import { ResourceHeader, ResourceToolbar } from 'components/ResourceLayout'
import ResourceNameLink from 'components/ResourceNameLink'
import { ConfirmOperationButton } from 'components/OperationButton'
import {
  bindServiceEnvironment,
  deleteLogicalService,
  describeLogicalServiceEnvironments,
  describeLogicalServices,
  describeUnboundServiceEnvironments,
  LogicalServiceView,
  ServiceEnvironmentBindingView,
  unbindServiceEnvironment,
} from 'services/logical_service'
import { ServiceView } from 'services/service'
import { openErrNotification, openInfoNotification } from 'utils/notifition'
import style from './index.module.less'

export default function LogicalServiceDetail() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const logicalServiceId = searchParams.get('id') || ''
  const [logicalService, setLogicalService] = useState<LogicalServiceView | null>(null)
  const [environments, setEnvironments] = useState<ServiceEnvironmentBindingView[]>([])
  const [unbound, setUnbound] = useState<ServiceView[]>([])
  const [selectedServiceId, setSelectedServiceId] = useState('')
  const [bindingKeyword, setBindingKeyword] = useState('')
  const [bindingVisible, setBindingVisible] = useState(false)
  const [loading, setLoading] = useState(false)
  const businessEnvironments = environments.filter(item => item.namespace !== 'pole-system')
  const systemBindings = environments.filter(item => item.namespace === 'pole-system')

  const load = useCallback(async () => {
    if (!logicalServiceId) return
    setLoading(true)
    try {
      const [logical, bindings] = await Promise.all([
        describeLogicalServices({ id: logicalServiceId, offset: 0, limit: 1 }),
        describeLogicalServiceEnvironments(logicalServiceId),
      ])
      setLogicalService(logical.list.find(item => item.id === logicalServiceId) || null)
      setEnvironments(bindings.list)
    } catch (error) {
      openErrNotification('加载逻辑服务失败', error instanceof Error ? error.message : String(error))
    } finally {
      setLoading(false)
    }
  }, [logicalServiceId])

  useEffect(() => {
    void load()
  }, [load])

  const openBinding = async (keyword = '') => {
    try {
      const result = await describeUnboundServiceEnvironments({
        name: keyword || undefined,
        offset: 0,
        limit: 100,
      })
      setUnbound(result.list)
      setSelectedServiceId('')
      setBindingVisible(true)
    } catch (error) {
      openErrNotification('获取未关联服务失败', error instanceof Error ? error.message : String(error))
    }
  }

  const bind = async () => {
    if (!selectedServiceId) return
    try {
      await bindServiceEnvironment(logicalServiceId, selectedServiceId)
      openInfoNotification('关联成功', '环境服务已加入逻辑服务')
      setBindingVisible(false)
      await load()
    } catch (error) {
      openErrNotification('关联失败', error instanceof Error ? error.message : String(error))
    }
  }

  const unbind = async (binding: ServiceEnvironmentBindingView) => {
    try {
      await unbindServiceEnvironment(logicalServiceId, binding.service_id)
      openInfoNotification('已解除关联', '环境服务仍保留并继续正常注册发现')
      await load()
    } catch (error) {
      openErrNotification('解除关联失败', error instanceof Error ? error.message : String(error))
    }
  }

  const remove = async () => {
    try {
      await deleteLogicalService(logicalServiceId)
      openInfoNotification('删除成功', '逻辑服务已删除，环境服务未受影响')
      navigate('/discovery/service')
    } catch (error) {
      openErrNotification('删除失败', error instanceof Error ? error.message : String(error))
    }
  }

  const columns = [
    { colKey: 'namespace', title: 'Namespace', width: 180 },
    {
      colKey: 'service_name',
      title: '运行时服务名',
      width: 260,
      cell: ({ row }: any) => (
        <ResourceNameLink
          name={row.service_name}
          onClick={row.service ? () => navigate(
            `/discovery/service/instance?logicalServiceId=${encodeURIComponent(logicalServiceId)}`
            + `&namespace=${encodeURIComponent(row.namespace)}&service=${encodeURIComponent(row.service_name)}`,
          ) : undefined}
        />
      ),
    },
    {
      colKey: 'health',
      title: '健康实例',
      width: 140,
      cell: ({ row }: any) => row.service
        ? `${row.service.healthy_instance_count || 0}/${row.service.total_instance_count || 0}`
        : '服务已缺失',
    },
    { colKey: 'mtime', title: '关联时间', width: 180 },
    {
      colKey: 'action',
      title: '操作',
      width: 190,
      cell: ({ row }: any) => (
        <>
          <Button variant="text" disabled={!row.service} onClick={() => navigate(
            `/discovery/service/instance?logicalServiceId=${encodeURIComponent(logicalServiceId)}`
            + `&namespace=${encodeURIComponent(row.namespace)}&service=${encodeURIComponent(row.service_name)}`,
          )}>进入环境</Button>
          <Button variant="text" theme="danger" onClick={() => void unbind(row)}>解除关联</Button>
        </>
      ),
    },
  ]

  return (
    <div className={`${style.page} ${style.logicalDetailPage}`}>
      <ResourceHeader
        density="compact"
        placement="app-header"
        eyebrow="注册发现 / 逻辑服务"
        title={logicalService?.name || '逻辑服务详情'}
        description={logicalService?.comment || '显式维护该逻辑服务在各 Namespace 中对应的运行时服务。'}
      />
      <section className={style.metricRail}>
        <div className={style.metricItem}><span>业务环境</span><strong>{businessEnvironments.length}</strong></div>
        <div className={style.metricItem}><span>健康实例</span><strong>{logicalService?.healthy_instance_count || 0}</strong></div>
        <div className={style.metricItem}><span>实例总数</span><strong>{logicalService?.total_instance_count || 0}</strong></div>
      </section>
      <section className={style.listSection}>
        <ResourceToolbar
          density="compact"
          title="环境服务"
          count={`共 ${businessEnvironments.length} 个显式关联`}
          filters={(
            <>
              <Button variant="outline" onClick={() => navigate('/discovery/service')}>返回列表</Button>
              <ConfirmOperationButton
                action="delete"
                label="删除逻辑服务"
                shape="rectangle"
                variant="outline"
                theme="danger"
                disabled={logicalService?.deleteable === false || environments.length > 0}
                disabledLabel={logicalService?.deleteable === false ? '无权限操作' : '请先解除环境关联'}
                confirmContent={`确认删除逻辑服务 ${logicalService?.name || ''} 吗？环境服务不会被删除。`}
                onConfirm={() => void remove()}
              />
              <Button theme="primary" onClick={() => void openBinding(bindingKeyword)}>关联环境服务</Button>
            </>
          )}
        />
        <section className={`${style.tableSurface} ${style.serviceTableSurface}`}>
          <Table
            data={businessEnvironments}
            columns={columns}
            rowKey="service_id"
            loading={loading}
            empty="尚未关联环境服务。逻辑服务会保留，关联操作必须由管理员显式完成。"
          />
        </section>
      </section>
      {systemBindings.length > 0 && (
        <section className={style.listSection}>
          <ResourceToolbar
            density="compact"
            title="历史系统空间关联（仅清理）"
            count={`发现 ${systemBindings.length} 个不参与业务聚合的关联，请解除后再删除逻辑服务`}
          />
          <section className={`${style.tableSurface} ${style.serviceTableSurface}`}>
            <Table
              data={systemBindings}
              columns={columns}
              rowKey="service_id"
              loading={loading}
            />
          </section>
        </section>
      )}
      <EditorDrawer
        visible={bindingVisible}
        bodyClassName={style.serviceOperationBody}
        title="关联已有环境服务"
        description="一个逻辑服务在同一环境空间只能关联一个运行时服务。"
        width="compact"
        onClose={() => setBindingVisible(false)}
        footer={<EditorDrawerActions onCancel={() => setBindingVisible(false)} onSubmit={() => void bind()} submitText="确认关联" submitDisabled={!selectedServiceId} />}
      >
        <div className={style.logicalForm}>
          <p>一个逻辑服务在同一 Namespace 只能关联一个运行时服务。</p>
          <label>
            <span>按运行时服务名筛选</span>
            <div>
              <Input value={bindingKeyword} onChange={setBindingKeyword} />
              <Button variant="outline" onClick={() => void openBinding(bindingKeyword)}>查询</Button>
            </div>
          </label>
          <label>
            <span>未关联环境服务</span>
            <Select
              value={selectedServiceId}
              placeholder="选择 Namespace / 运行时服务"
              options={unbound.map(item => ({
                label: `${item.namespace} / ${item.name}`,
                value: item.id,
              }))}
              onChange={setSelectedServiceId}
            />
          </label>
        </div>
      </EditorDrawer>
    </div>
  )
}
