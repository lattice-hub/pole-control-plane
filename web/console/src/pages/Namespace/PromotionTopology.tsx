import React from 'react';

import { Button, Checkbox, Empty, Input, Select, Tag, Tooltip } from 'components/Fluent';
import { AddIcon, ArrowRightIcon, RefreshIcon, RocketIcon, RollbackIcon } from 'components/Fluent/icons';
import { ResourceHeader, ResourceToolbar } from 'components/ResourceLayout';
import { useNavigate } from 'components/Router';
import {
  describeEnvironmentPromotionTopology,
  EnvironmentPromotionEdge,
  EnvironmentPromotionTopology,
  publishEnvironmentPromotionTopology,
  saveEnvironmentPromotionTopology,
  validateEnvironmentPromotionTopology,
} from 'services/environment_promotion';
import { describeBusinessNamespaces } from 'services/namespace';
import { openErrNotification, openInfoNotification } from 'utils/notifition';
import style from './PromotionTopology.module.less';

const emptyTopology = (): EnvironmentPromotionTopology => ({
  draft_revision: 0,
  published_revision: 0,
  edges: [],
  lane_base_bindings: [],
});

const newEdge = (): EnvironmentPromotionEdge => ({
  id: `edge-${Date.now()}`,
  source: '',
  target: '',
  require_formal_release: true,
  require_approval: true,
  validation_gates: [],
  allowed_resource_domains: ['configuration'],
  conflict_policy: 'BLOCK',
});

export default React.memo(() => {
  const navigate = useNavigate();
  const [topology, setTopology] = React.useState<EnvironmentPromotionTopology>(emptyTopology());
  const [namespaces, setNamespaces] = React.useState<string[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [comment, setComment] = React.useState('');
  const [savedSnapshot, setSavedSnapshot] = React.useState<string | null>(null);
  const topologyRef = React.useRef(topology);
  topologyRef.current = topology;
  const snapshot = (value: EnvironmentPromotionTopology) => JSON.stringify({ edges: value.edges, lane_base_bindings: value.lane_base_bindings });
  const dirty = savedSnapshot === null || snapshot(topology) !== savedSnapshot;

  const refresh = React.useCallback(async (preserveEdits = false) => {
    const initial = topologyRef.current;
    setLoading(true);
    setSavedSnapshot(null);
    try {
      const [current, spaces] = await Promise.all([
        describeEnvironmentPromotionTopology(),
        describeBusinessNamespaces(),
      ]);
      setTopology(previous => !preserveEdits && previous === initial ? current : { ...previous, published_revision: current.published_revision });
      setSavedSnapshot(snapshot(current));
      setNamespaces(spaces.map((item) => item.name));
    } catch (error) {
      openErrNotification('加载晋升拓扑失败', error instanceof Error ? error.message : String(error));
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => { void refresh(); }, [refresh]);

  const patchEdge = (index: number, patch: Partial<EnvironmentPromotionEdge>) => {
    setTopology((current) => ({
      ...current,
      edges: current.edges.map((edge, row) => row === index ? { ...edge, ...patch } : edge),
    }));
  };

  const validate = async (candidate = topology) => {
    const result = await validateEnvironmentPromotionTopology(candidate);
    if (result.valid) {
      openInfoNotification('校验通过', '当前基线关系无环，泳道绑定与环境引用有效');
      return true;
    }
    openErrNotification('拓扑校验未通过', result.issues.map((issue) => issue.message).join('；'));
    return false;
  };

  const save = async () => {
    setSaving(true);
    try {
      const candidate = topology;
      if (!await validate(candidate)) return;
      const saved = await saveEnvironmentPromotionTopology(candidate);
      setSavedSnapshot(snapshot(saved));
      setTopology((current) => current === candidate ? saved : { ...current, draft_revision: saved.draft_revision, published_revision: saved.published_revision });
      openInfoNotification('草稿已保存', `草稿 Revision ${saved.draft_revision}`);
    } catch (error) {
      openErrNotification('保存拓扑失败', error instanceof Error ? error.message : String(error));
    } finally {
      setSaving(false);
    }
  };

  const publish = async () => {
    if (loading || saving || dirty || !topology.draft_revision) {
      openErrNotification('无法发布拓扑', '请先加载并保存当前草稿，再发布。');
      return;
    }
    const candidate = topology;
    setSaving(true);
    try {
      if (!await validate(candidate)) return;
      if (snapshot(topologyRef.current) !== savedSnapshot || topologyRef.current.draft_revision !== candidate.draft_revision) {
        openErrNotification('无法发布拓扑', '草稿已更改，请先保存再发布。');
        return;
      }
      await publishEnvironmentPromotionTopology(candidate.draft_revision, comment);
      setComment('');
      await refresh(snapshot(topologyRef.current) !== savedSnapshot);
      openInfoNotification('拓扑已发布', '新创建的晋升变更集将固定使用该 Revision');
    } catch (error) {
      openErrNotification('发布拓扑失败', error instanceof Error ? error.message : String(error));
    } finally {
      setSaving(false);
    }
  };

  const namespaceOptions = namespaces.map((value) => ({ label: value, value }));

  return (
    <div className={style.page}>
      <ResourceHeader
        density="compact"
        placement="app-header"
        eyebrow="Environment / Promotion"
        title="环境晋升拓扑"
        description="基线环境组成有向无环图；泳道通过独立绑定回归基线，不作为 DAG 中间节点。"
      />
      <ResourceToolbar
        density="compact"
        title="全局拓扑"
        count={`草稿 r${topology.draft_revision} · 已发布 r${topology.published_revision}`}
        filters={(
          <div className={style.toolbarActions}>
            <Button variant="text" icon={<RollbackIcon />} onClick={() => navigate('/namespace')}>返回环境空间</Button>
            <Tooltip content="刷新拓扑"><Button shape="square" variant="outline" icon={<RefreshIcon />} disabled={loading || saving} onClick={() => void refresh()} /></Tooltip>
            <Button variant="outline" onClick={() => void validate()}>校验</Button>
            <Button variant="outline" loading={saving} disabled={loading || savedSnapshot === null} onClick={save}>保存草稿</Button>
          </div>
        )}
      />

      <section className={style.summary}>
        <div><span>基线晋升边</span><strong>{topology.edges.length}</strong></div>
        <div><span>泳道绑定</span><strong>{topology.lane_base_bindings.length}</strong></div>
        <div><span>默认冲突策略</span><strong>BLOCK</strong></div>
        <div><span>发布语义</span><strong>候选 → 审批 → 生效</strong></div>
      </section>

      <section className={style.graph} aria-label="环境晋升拓扑预览">
        {topology.edges.length ? topology.edges.map((edge) => (
          <div className={style.graphEdge} key={edge.id}>
            <span>{edge.source || '未选择'}</span><ArrowRightIcon /><span>{edge.target || '未选择'}</span>
          </div>
        )) : <Empty title="尚未定义基线晋升关系" description="添加第一条晋升边开始设计环境流转。" />}
      </section>

      <section className={style.editorCard}>
        <div className={style.sectionHeading}>
          <div><strong>基线晋升关系</strong><span>每条边独立定义门禁；目标始终先生成候选版本。</span></div>
          <Button icon={<AddIcon />} onClick={() => setTopology((current) => ({ ...current, edges: [...current.edges, newEdge()] }))}>添加晋升边</Button>
        </div>
        <div className={style.edgeTable}>
          {topology.edges.map((edge, index) => (
            <div className={style.edgeRow} key={edge.id}>
              <Select value={edge.source} options={namespaceOptions} placeholder="来源环境" onChange={(source: string) => patchEdge(index, { source })} />
              <ArrowRightIcon />
              <Select value={edge.target} options={namespaceOptions} placeholder="目标环境" onChange={(target: string) => patchEdge(index, { target })} />
              <Checkbox checked={edge.require_formal_release} onChange={(checked: boolean) => patchEdge(index, { require_formal_release: checked })}>正式版本</Checkbox>
              <Checkbox checked={edge.require_approval} onChange={(checked: boolean) => patchEdge(index, { require_approval: checked })}>需要审批</Checkbox>
              <Tag size="small" variant="light">冲突阻断</Tag>
              <Button variant="text" onClick={() => setTopology((current) => ({ ...current, edges: current.edges.filter((_, row) => row !== index) }))}>移除</Button>
            </div>
          ))}
        </div>
      </section>

      <section className={style.editorCard}>
        <div className={style.sectionHeading}>
          <div><strong>泳道回归绑定</strong><span>一个泳道只绑定一个 base；变更按资源三方比较后回归。</span></div>
          <Button icon={<AddIcon />} onClick={() => setTopology((current) => ({
            ...current, lane_base_bindings: [...current.lane_base_bindings, { lane: '', base: '' }],
          }))}>添加泳道绑定</Button>
        </div>
        {topology.lane_base_bindings.map((binding, index) => (
          <div className={style.bindingRow} key={`${index}-${binding.lane}`}>
            <Select value={binding.lane} options={namespaceOptions} placeholder="泳道环境" onChange={(lane: string) => setTopology((current) => ({
              ...current, lane_base_bindings: current.lane_base_bindings.map((item, row) => row === index ? { ...item, lane } : item),
            }))} />
            <ArrowRightIcon />
            <Select value={binding.base} options={namespaceOptions} placeholder="Base 基线" onChange={(base: string) => setTopology((current) => ({
              ...current, lane_base_bindings: current.lane_base_bindings.map((item, row) => row === index ? { ...item, base } : item),
            }))} />
            <Button variant="text" onClick={() => setTopology((current) => ({
              ...current, lane_base_bindings: current.lane_base_bindings.filter((_, row) => row !== index),
            }))}>移除</Button>
          </div>
        ))}
      </section>

      <section className={style.publishBar}>
        <Input value={comment} placeholder="填写本次拓扑发布说明" onChange={setComment} />
        {dirty && <span>请先加载并保存当前草稿，再发布。</span>}
        <Button theme="primary" icon={<RocketIcon />} loading={saving} disabled={loading || dirty || !topology.draft_revision} onClick={publish}>发布拓扑 Revision</Button>
      </section>
    </div>
  );
});
