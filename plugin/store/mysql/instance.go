/**
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package sqldb

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	svctypes "github.com/pole-io/pole-server/apis/pkg/types/service"
	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/common/utils"
)

// instanceStore 实现了InstanceStore接口
type instanceStore struct {
	master *BaseDB // 大部分操作都用主数据库
	slave  *BaseDB // 缓存相关的读取，请求到slave
}

// AddInstance 添加实例
func (ins *instanceStore) AddInstance(instance *svctypes.Instance) error {
	err := RetryTransaction("addInstance", func() error {
		return ins.addInstance(instance)
	})
	return store.Error(err)
}

// addInstance
func (ins *instanceStore) addInstance(instance *svctypes.Instance) error {
	tx, err := ins.master.Begin()
	if err != nil {
		log.Errorf("[Store][database] add instance tx begin err: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 新增数据之前，必须先清理老数据
	if err := cleanInstance(tx, instance.ID()); err != nil {
		return err
	}

	if err := addMainInstance(tx, instance); err != nil {
		log.Errorf("[Store][database] add instance main insert err: %s", err.Error())
		return err
	}

	if err := addInstanceCheck(tx, instance); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		log.Errorf("[Store][database] add instance commit tx err: %s", err.Error())
		return err
	}

	return nil
}

// BatchAddInstances 批量增加实例
func (ins *instanceStore) BatchAddInstances(instances []*svctypes.Instance) error {

	err := RetryTransaction("batchAddInstances", func() error {
		return ins.batchAddInstances(instances)
	})
	return store.Error(err)
}

// batchAddInstances batch add instances
func (ins *instanceStore) batchAddInstances(instances []*svctypes.Instance) error {
	tx, err := ins.master.Begin()
	if err != nil {
		log.Errorf("[Store][database] batch add instances begin tx err: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := batchAddMainInstances(tx, instances); err != nil {
		log.Errorf("[Store][database] batch add main instances err: %s", err.Error())
		return err
	}
	if err := batchAddInstanceCheck(tx, instances); err != nil {
		log.Errorf("[Store][database] batch add instance check err: %s", err.Error())
		return err
	}
	if err := tx.Commit(); err != nil {
		log.Errorf("[Store][database] batch add instance commit tx err: %s", err.Error())
		return err
	}

	return nil
}

// UpdateInstance 更新实例
func (ins *instanceStore) UpdateInstance(instance *svctypes.Instance) error {
	err := RetryTransaction("updateInstance", func() error {
		return ins.updateInstance(instance)
	})
	if err == nil {
		return nil
	}

	serr := store.Error(err)
	if store.Code(serr) == store.DuplicateEntryErr {
		serr = store.NewStatusError(store.DataConflictErr, err.Error())
	}
	return serr
}

// updateInstance update instance
func (ins *instanceStore) updateInstance(instance *svctypes.Instance) error {
	tx, err := ins.master.Begin()
	if err != nil {
		log.Errorf("[Store][database] update instance tx begin err: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 更新main表
	if err := updateInstanceMain(tx, instance); err != nil {
		log.Errorf("[Store][database] update instance main err: %s", err.Error())
		return err
	}
	// 更新health check表
	if err := updateInstanceCheck(tx, instance); err != nil {
		log.Errorf("[Store][database] update instance check err: %s", err.Error())
		return err
	}
	if err := tx.Commit(); err != nil {
		log.Errorf("[Store][database] update instance commit tx err: %s", err.Error())
		return err
	}

	return nil
}

// CleanInstance 清理数据
func (ins *instanceStore) CleanInstance(instanceID string) error {
	return RetryTransaction("cleanInstance", func() error {
		return ins.master.processWithTransaction("cleanInstance", func(tx *BaseTx) error {
			if err := cleanInstance(tx, instanceID); err != nil {
				return err
			}

			if err := tx.Commit(); err != nil {
				log.Errorf("[Store][database] clean instance commit tx err: %s", err.Error())
				return err
			}

			return nil
		})
	})
}

// cleanInstance 清理数据
func cleanInstance(tx *BaseTx, instanceID string) error {
	log.Infof("[Store][database] clean instance(%s)", instanceID)
	cleanIns := "delete from instance where id = ? and flag = 1"
	if _, err := tx.Exec(cleanIns, instanceID); err != nil {
		log.Errorf("[Store][database] clean instance(%s), err: %s", instanceID, err.Error())
		return store.Error(err)
	}
	cleanCheck := "delete from health_check where id = ?"
	if _, err := tx.Exec(cleanCheck, instanceID); err != nil {
		log.Errorf("[Store][database] clean health_check(%s), err: %s", instanceID, err.Error())
		return store.Error(err)
	}
	return nil
}

// DeleteInstance 删除一个实例，删除实例实际上是把flag置为1
func (ins *instanceStore) DeleteInstance(instanceID string) error {
	if instanceID == "" {
		return errors.New("delete Instance Missing instance id")
	}
	return RetryTransaction("deleteInstance", func() error {
		return ins.master.processWithTransaction("deleteInstance", func(tx *BaseTx) error {
			str := "update instance set flag = 1, mtime = sysdate() where `id` = ?"
			if _, err := tx.Exec(str, instanceID); err != nil {
				return store.Error(err)
			}

			if err := tx.Commit(); err != nil {
				log.Errorf("[Store][database] delete instance commit tx err: %s", err.Error())
				return err
			}

			return nil
		})
	})
}

// BatchDeleteInstances 批量删除实例
func (ins *instanceStore) BatchDeleteInstances(ids []interface{}) error {
	return RetryTransaction("batchDeleteInstance", func() error {
		return ins.master.processWithTransaction("batchDeleteInstance", func(tx *BaseTx) error {
			if err := BatchOperation("delete-instance", ids, func(objects []interface{}) error {
				if len(objects) == 0 {
					return nil
				}
				str := `update instance set flag = 1, mtime = sysdate() where id in ( ` + PlaceholdersN(len(objects)) + `)`
				_, err := tx.Exec(str, objects...)
				return store.Error(err)
			}); err != nil {
				return err
			}

			if err := tx.Commit(); err != nil {
				log.Errorf("[Store][database] batch delete instance commit tx err: %s", err.Error())
				return err
			}

			return nil
		})
	})
}

// GetInstance 获取单个实例详情，只返回有效的数据
func (ins *instanceStore) GetInstance(instanceID string) (*svctypes.Instance, error) {
	instance, err := ins.getInstance(instanceID)
	if err != nil {
		return nil, err
	}

	// 如果实例无效，则不返回
	if instance != nil && !instance.Valid {
		return nil, nil
	}

	return instance, nil
}

// BatchGetInstanceIsolate 检查实例是否存在
func (ins *instanceStore) BatchGetInstanceIsolate(ids map[string]bool) (map[string]bool, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	str := "select id, isolate from instance where flag = 0 and id in(" + PlaceholdersN(len(ids)) + ")"
	args := make([]interface{}, 0, len(ids))
	for key := range ids {
		args = append(args, key)
	}
	instanceIsolate := make(map[string]bool, len(ids))
	rows, err := ins.master.Query(str, args...)
	if err != nil {
		log.Errorf("[Store][database] check instances existed query err: %s", err.Error())
		return nil, err
	}
	var idx string
	var isolate int
	for rows.Next() {
		if err := rows.Scan(&idx, &isolate); err != nil {
			log.Errorf("[Store][database] check instances existed scan err: %s", err.Error())
			return nil, err
		}
		instanceIsolate[idx] = utils.Int2bool(isolate)
	}

	return instanceIsolate, nil
}

// GetInstancesBrief 批量获取实例的serviceID
func (ins *instanceStore) GetInstancesBrief(ids map[string]bool) (map[string]*svctypes.Instance, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	str := `select instance.id, host, port, name, namespace, token, IFNULL(platform_id,"") from service, instance
		 where instance.flag = 0 and service.flag = 0
		 and service.id = instance.service_id and instance.id in (` + PlaceholdersN(len(ids)) + ")"
	args := make([]interface{}, 0, len(ids))
	for key := range ids {
		args = append(args, key)
	}

	rows, err := ins.master.Query(str, args...)
	if err != nil {
		log.Errorf("[Store][database] get instances service token query err: %s", err.Error())
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]*svctypes.Instance, len(ids))
	var item svctypes.ExpandInstanceStore
	var instance svctypes.InstanceStore
	item.ServiceInstance = &instance
	for rows.Next() {
		if err := rows.Scan(&instance.ID, &instance.Host, &instance.Port,
			&item.ServiceName, &item.Namespace, &item.ServiceToken, &item.ServicePlatformID); err != nil {
			log.Errorf("[Store][database] get instances service token scan err: %s", err.Error())
			return nil, err
		}

		out[instance.ID] = svctypes.ExpandStore2Instance(&item)
	}

	return out, nil
}

// GetInstancesCount 获取有效的实例总数
func (ins *instanceStore) GetInstancesCount() (uint32, error) {
	countStr := "select count(*) from instance where flag = 0"
	var count uint32
	var err error
	Retry("query-instance-row", func() error {
		err = ins.master.QueryRow(countStr).Scan(&count)
		return err
	})
	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		log.Errorf("[Store][database] get instances count scan err: %s", err.Error())
		return 0, err
	default:
	}

	return count, nil
}

// GetInstancesCountTx .
func (ins *instanceStore) GetInstancesCountTx(tx store.Tx) (uint32, error) {
	dbTx, _ := tx.GetDelegateTx().(*BaseTx)
	countStr := "select count(*) from instance where flag = 0"
	var count uint32
	var err error
	Retry("query-instance-row", func() error {
		err = dbTx.QueryRow(countStr).Scan(&count)
		return err
	})
	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		log.Errorf("[Store][database] get instances count scan err: %s", err.Error())
		return 0, err
	default:
	}

	return count, nil
}

// GetInstancesMainByService 根据服务和host获取实例
// @note 不包括metadata
func (ins *instanceStore) GetInstancesMainByService(serviceID, host string) ([]*svctypes.Instance, error) {
	// 只查询有效的服务实例
	str := genInstanceSelectSQL() + " where service_id = ? and host = ? and flag = 0"
	rows, err := ins.master.Query(str, serviceID, host)
	if err != nil {
		log.Errorf("[Store][database] get instances main query err: %s", err.Error())
		return nil, err
	}

	var out []*svctypes.Instance
	err = callFetchInstanceRows(rows, func(entry *svctypes.InstanceStore) (b bool, e error) {
		out = append(out, svctypes.Store2Instance(entry))
		return true, nil
	})
	if err != nil {
		log.Errorf("[Store][database] call fetch instance rows err: %s", err.Error())
		return nil, err
	}

	return out, nil
}

// GetExpandInstances 根据过滤条件查看对应服务实例及数目
func (ins *instanceStore) GetExpandInstances(filter, metaFilter map[string]string, offset uint32,
	limit uint32) (uint32, []*svctypes.Instance, error) {
	// 只查询有效的实例列表
	filter["instance.flag"] = "0"

	out, err := ins.getExpandInstances(filter, metaFilter, offset, limit)
	if err != nil {
		return 0, nil, err
	}

	num, err := ins.getExpandInstancesCount(filter, metaFilter)
	if err != nil {
		return 0, nil, err
	}
	return num, out, err
}

// getExpandInstances 根据过滤条件查看对应服务实例
func (ins *instanceStore) getExpandInstances(filter, metaFilter map[string]string, offset uint32,
	limit uint32) ([]*svctypes.Instance, error) {
	// 这种情况意味着，不需要详细的数据，可以不用query了
	if limit == 0 {
		return make([]*svctypes.Instance, 0), nil
	}
	_, isName := filter["name"]
	_, isNamespace := filter["namespace"]
	_, isHost := filter["host"]
	needForceIndex := isName || isNamespace || isHost

	str := genExpandInstanceSelectSQL(needForceIndex)
	order := &Order{"instance.mtime", "desc"}
	str, args := genWhereSQLAndArgs(str, filter, metaFilter, order, offset, limit)

	rows, err := ins.master.Query(str, args...)
	if err != nil {
		log.Errorf("[Store][database] get instance by filters query err: %s, str: %s, args: %v", err.Error(), str, args)
		return nil, err
	}

	out, err := fetchExpandInstanceRows(rows)
	if err != nil {
		log.Errorf("[Store][database] get row instances err: %s", err.Error())
		return nil, err
	}
	return out, nil
}

// getExpandInstancesCount 根据过滤条件查看对应服务实例的数目
func (ins *instanceStore) getExpandInstancesCount(filter, metaFilter map[string]string) (uint32, error) {
	str := `select count(*) from instance `
	// 查询条件是否有service表中的字段
	_, isName := filter["name"]
	_, isNamespace := filter["namespace"]
	if isName || isNamespace {
		str += `inner join service on instance.service_id = service.id `
	}
	str, args := genWhereSQLAndArgs(str, filter, metaFilter, nil, 0, 1)

	var count uint32
	var err error
	Retry("query-instance-row", func() error {
		err = ins.master.QueryRow(str, args...).Scan(&count)
		return err
	})
	switch {
	case err == sql.ErrNoRows:
		log.Errorf("[Store][database] no row with this expand instance filter")
		return count, err
	case err != nil:
		log.Errorf("[Store][database] get expand instance count by filter err: %s", err.Error())
		return count, err
	default:
		return count, nil
	}
}

// GetMoreInstances 根据mtime获取增量修改数据
// 这里会返回所有的数据的，包括valid=false的数据
// 对于首次拉取，firstUpdate=true，只会拉取flag!=1的数据
func (ins *instanceStore) GetMoreInstances(tx store.Tx, mtime time.Time, firstUpdate, needMeta bool,
	serviceID []string) (map[string]*svctypes.Instance, error) {
	dbTx, _ := tx.GetDelegateTx().(*BaseTx)
	fetchSql := genCompleteInstanceSelectSQL()
	args := make([]interface{}, 0, len(serviceID)+1)

	// 首次拉取
	if firstUpdate {
		fetchSql += " where instance.flag = 0"
	} else {
		fetchSql += " where instance.mtime >= FROM_UNIXTIME(?)"
		args = append(args, timeToTimestamp(mtime))
	}

	if len(serviceID) > 0 {
		fetchSql += " and service_id in (" + PlaceholdersN(len(serviceID)) + ")"
	}
	for _, id := range serviceID {
		args = append(args, id)
	}

	rows, err := dbTx.Query(fetchSql, args...)
	if err != nil {
		log.Errorf("[Store][database] get more instance query err: %s", err.Error())
		return nil, err
	}
	return fetchInstanceWithMetaRows(rows)
}

// SetInstanceHealthStatus 设置实例健康状态
func (ins *instanceStore) SetInstanceHealthStatus(instanceID string, flag int, revision string) error {
	return RetryTransaction("setInstanceHealthStatus", func() error {
		return ins.master.processWithTransaction("setInstanceHealthStatus", func(tx *BaseTx) error {
			str := "update instance set health_status = ?, revision = ?, mtime = sysdate() where `id` = ?"
			if _, err := tx.Exec(str, flag, revision, instanceID); err != nil {
				return store.Error(err)
			}

			if err := tx.Commit(); err != nil {
				log.Errorf("[Store][database] set instance health status commit tx err: %s", err.Error())
				return err
			}

			return nil
		})
	})
}

// BatchSetInstanceHealthStatus 批量设置健康状态
func (ins *instanceStore) BatchSetInstanceHealthStatus(ids []interface{}, isolate int, revision string) error {
	return RetryTransaction("batchSetInstanceHealthStatus", func() error {
		return ins.master.processWithTransaction("batchSetInstanceHealthStatus", func(tx *BaseTx) error {
			if err := BatchOperation("set-instance-healthy", ids, func(objects []interface{}) error {
				if len(objects) == 0 {
					return nil
				}
				str := "update instance set health_status = ?, revision = ?, mtime = sysdate() where id in "
				str += "(" + PlaceholdersN(len(objects)) + ")"
				args := make([]interface{}, 0, len(objects)+2)
				args = append(args, isolate)
				args = append(args, revision)
				args = append(args, objects...)
				_, err := tx.Exec(str, args...)
				return store.Error(err)
			}); err != nil {
				return err
			}

			if err := tx.Commit(); err != nil {
				log.Errorf("[Store][database] batch set instance health status commit tx err: %s", err.Error())
				return err
			}

			return nil
		})
	})
}

// BatchSetInstanceIsolate 批量设置实例隔离状态
func (ins *instanceStore) BatchSetInstanceIsolate(ids []interface{}, isolate int, revision string) error {
	return RetryTransaction("batchSetInstanceIsolate", func() error {
		return ins.master.processWithTransaction("batchSetInstanceIsolate", func(tx *BaseTx) error {
			if err := BatchOperation("set-instance-isolate", ids, func(objects []interface{}) error {
				if len(objects) == 0 {
					return nil
				}
				str := "update instance set isolate = ?, revision = ?, mtime = sysdate() where id in "
				str += "(" + PlaceholdersN(len(objects)) + ")"
				args := make([]interface{}, 0, len(objects)+2)
				args = append(args, isolate)
				args = append(args, revision)
				args = append(args, objects...)
				_, err := tx.Exec(str, args...)
				return store.Error(err)
			}); err != nil {
				return err
			}

			if err := tx.Commit(); err != nil {
				log.Errorf("[Store][database] batch set instance isolate commit tx err: %s", err.Error())
				return err
			}

			return nil
		})
	})
}

// BatchAppendInstanceMetadata 追加实例 metadata
func (ins *instanceStore) BatchAppendInstanceMetadata(requests []*store.InstanceMetadataRequest) error {
	if len(requests) == 0 {
		return nil
	}
	return ins.master.processWithTransaction("AppendInstanceMetadata", func(tx *BaseTx) error {
		for i := range requests {
			id := requests[i].InstanceID
			revision := requests[i].Revision
			metadata := requests[i].Metadata
			str := "replace into instance_manual_metadata(`id`, `mkey`, `mvalue`, `ctime`, `mtime`) values"
			values := make([]string, 0, len(metadata))
			args := make([]interface{}, 0, len(metadata)*3)
			for k, v := range metadata {
				values = append(values, "(?, ?, ?, sysdate(), sysdate())")
				args = append(args, id, k, v)
			}
			str += strings.Join(values, ",")
			if log.DebugEnabled() {
				log.Debug("[Store][database] append instance metadata", zap.String("sql", str), zap.Any("args", args))
			}
			if _, err := tx.Exec(str, args...); err != nil {
				log.Errorf("[Store][database] append instance metadata err: %s", err.Error())
				return err
			}

			str = "update instance set revision = ?, mtime = sysdate() where id = ?"
			if _, err := tx.Exec(str, revision, id); err != nil {
				log.Errorf("[Store][database] append instance metadata update revision err: %s", err.Error())
				return err
			}
		}
		return tx.Commit()
	})
}

// BatchRemoveInstanceMetadata 删除实例指定的 metadata
func (ins *instanceStore) BatchRemoveInstanceMetadata(requests []*store.InstanceMetadataRequest) error {
	if len(requests) == 0 {
		return nil
	}
	return ins.master.processWithTransaction("RemoveInstanceMetadata", func(tx *BaseTx) error {
		for i := range requests {
			id := requests[i].InstanceID
			revision := requests[i].Revision
			keys := requests[i].Keys
			str := "delete from instance_manual_metadata where id = ? and mkey in (%s)"
			values := make([]string, 0, len(keys))
			args := make([]interface{}, 0, 1+len(keys))
			args = append(args, id)
			for i := range keys {
				key := keys[i]
				values = append(values, "?")
				args = append(args, key)
			}
			str = fmt.Sprintf(str, strings.Join(values, ","))

			if _, err := tx.Exec(str, args...); err != nil {
				log.Errorf("[Store][database] remove instance metadata by keys err: %s", err.Error())
				return err
			}

			str = "update instance set revision = ?, mtime = sysdate() where id = ?"
			if _, err := tx.Exec(str, revision, id); err != nil {
				log.Errorf("[Store][database] remove instance metadata by keys update revision err: %s", err.Error())
				return err
			}
		}
		return tx.Commit()
	})
}

// getInstance 内部获取instance函数，根据instanceID，直接读取元数据，不做其他过滤
func (ins *instanceStore) getInstance(instanceID string) (*svctypes.Instance, error) {
	str := genInstanceSelectSQL() + " where instance.id = ?"
	rows, err := ins.master.Query(str, instanceID)
	if err != nil {
		log.Errorf("[Store][database] get instance query err: %s", err.Error())
		return nil, err
	}

	out, err := fetchInstanceRows(rows)
	if err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return nil, err
	}
	out[0].MallocProto()
	return out[0], nil
}

// fetchInstanceWithMetaRows 获取instance main+health_check rows里面的数据
func fetchInstanceWithMetaRows(rows *sql.Rows) (map[string]*svctypes.Instance, error) {
	if rows == nil {
		return nil, nil
	}
	defer rows.Close()

	out := make(map[string]*svctypes.Instance)
	var item svctypes.InstanceStore
	var metadataStr string
	progress := 0
	for rows.Next() {
		progress++
		if progress%100000 == 0 {
			log.Infof("[Store][database] instance+meta fetch rows progress: %d", progress)
		}
		if err := rows.Scan(&item.ID, &item.ServiceID, &item.VpcID, &item.Host, &item.Port, &item.Protocol,
			&item.Version, &item.HealthStatus, &item.Isolate, &item.Weight, &item.EnableHealthCheck,
			&item.LogicSet, &item.Region, &item.Zone, &item.Campus, &item.Priority, &item.Revision,
			&item.Flag, &item.CheckType, &item.TTL, &item.CreateTime, &item.ModifyTime, &metadataStr); err != nil {
			log.Errorf("[Store][database] fetch instance+meta rows err: %s", err.Error())
			return nil, err
		}

		item.Meta = make(map[string]string)
		_ = json.Unmarshal([]byte(metadataStr), &item.Meta)
		out[item.ID] = svctypes.Store2Instance(&item)
	}
	if err := rows.Err(); err != nil {
		log.Errorf("[Store][database] fetch instance+metadata rows next err: %s", err.Error())
		return nil, err
	}
	return out, nil
}

// addMainInstance 往instance主表中增加数据
func addMainInstance(tx *BaseTx, instance *svctypes.Instance) error {
	// #lizard forgives
	str := `replace into instance(id, service_id, vpc_id, host, port, protocol, version, health_status, isolate,
		 weight, enable_health_check, logic_set, cmdb_region, cmdb_zone, cmdb_idc, priority, metadata, revision, ctime, mtime)
			 values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, sysdate(), sysdate())`
	_, err := tx.Exec(str, instance.ID(), instance.ServiceID, "", instance.Host(), instance.Port(),
		instance.Protocol(), instance.Version(), instance.Healthy(), instance.Isolate(), instance.Weight(),
		instance.EnableHealthCheck(), "", instance.Location().GetRegion(),
		instance.Location().GetZone(), instance.Location().GetCampus(),
		instance.Priority(), utils.MustJson(instance.Proto.GetMetadata()), instance.Revision())
	return err
}

// batchAddMainInstances 批量增加main instance数据
func batchAddMainInstances(tx *BaseTx, instances []*svctypes.Instance) error {
	str := `replace into instance(id, service_id, vpc_id, host, port, protocol, version, health_status, isolate,
		 weight, enable_health_check, logic_set, cmdb_region, cmdb_zone, cmdb_idc, priority, metadata, revision, ctime, mtime)
		 values`
	first := true
	args := make([]interface{}, 0)
	for _, entry := range instances {
		if !first {
			str += ","
		}
		str += "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, sysdate(), sysdate())"
		first = false
		args = append(args, entry.ID(), entry.ServiceID, "", entry.Host(), entry.Port())
		args = append(args, entry.Protocol(), entry.Version(), entry.Healthy(), entry.Isolate(),
			entry.Weight())
		args = append(args, entry.EnableHealthCheck(), "",
			entry.Location().GetRegion(), entry.Location().GetZone(),
			entry.Location().GetCampus(), entry.Priority(), utils.MustJson(entry.Proto.GetMetadata()), entry.Revision())
	}
	_, err := tx.Exec(str, args...)
	return err
}

// addInstanceCheck 往health_check加入健康检查信息
func addInstanceCheck(tx *BaseTx, instance *svctypes.Instance) error {
	check := instance.HealthCheck()
	if check == nil {
		return nil
	}

	str := "replace into health_check(`id`, `type`, `ttl`) values(?, ?, ?)"
	_, err := tx.Exec(str, instance.ID(), check.GetType(),
		svctypes.HealthCheckInterval(check))
	return err
}

// batchAddInstanceCheck 批量增加healthCheck数据
func batchAddInstanceCheck(tx *BaseTx, instances []*svctypes.Instance) error {
	str := "replace into health_check(`id`, `type`, `ttl`) values"
	first := true
	args := make([]interface{}, 0)
	for _, entry := range instances {
		if entry.HealthCheck() == nil {
			continue
		}
		if !first {
			str += ","
		}
		str += "(?,?,?)"
		first = false
		args = append(args, entry.ID(), entry.HealthCheck().GetType(),
			svctypes.HealthCheckInterval(entry.HealthCheck()))
	}
	// 不存在健康检查信息，直接返回
	if first {
		return nil
	}
	_, err := tx.Exec(str, args...)
	return err
}

// updateInstanceCheck 更新instance的check表
func updateInstanceCheck(tx *BaseTx, instance *svctypes.Instance) error {
	// healthCheck为空，则删除
	check := instance.HealthCheck()
	if check == nil {
		return deleteInstanceCheck(tx, instance.ID())
	}

	str := "replace into health_check(id, type, ttl) values(?, ?, ?)"
	_, err := tx.Exec(str, instance.ID(), check.GetType(),
		svctypes.HealthCheckInterval(check))
	return err
}

// updateInstanceMain 更新instance主表
func updateInstanceMain(tx *BaseTx, instance *svctypes.Instance) error {
	str := `update instance set protocol = ?,
	 version = ?, health_status = ?, isolate = ?, weight = ?, enable_health_check = ?, logic_set = ?,
	 cmdb_region = ?, cmdb_zone = ?, cmdb_idc = ?, priority = ?, metadata = ?, revision = ?, mtime = sysdate() where id = ?`

	_, err := tx.Exec(str, instance.Protocol(), instance.Version(), instance.Healthy(), instance.Isolate(),
		instance.Weight(), instance.EnableHealthCheck(), "",
		instance.Location().GetRegion(), instance.Location().GetZone(),
		instance.Location().GetCampus(), instance.Priority(), utils.MustJson(instance.Proto.GetMetadata()),
		instance.Revision(), instance.ID())

	return err
}

// deleteInstanceCheck 删除healthCheck数据
func deleteInstanceCheck(tx *BaseTx, id string) error {
	str := "delete from health_check where id = ?"
	_, err := tx.Exec(str, id)
	return err
}

// fetchInstanceRows 获取instance rows的内容
func fetchInstanceRows(rows *sql.Rows) ([]*svctypes.Instance, error) {
	var out []*svctypes.Instance
	err := callFetchInstanceRows(rows, func(entry *svctypes.InstanceStore) (b bool, e error) {
		out = append(out, svctypes.Store2Instance(entry))
		return true, nil
	})
	if err != nil {
		log.Errorf("[Store][database] call fetch instance rows err: %s", err.Error())
		return nil, err
	}

	return out, nil
}

// callFetchInstanceRows 带回调的fetch instance
func callFetchInstanceRows(rows *sql.Rows, callback func(entry *svctypes.InstanceStore) (bool, error)) error {
	if rows == nil {
		return nil
	}
	defer rows.Close()
	progress := 0
	for rows.Next() {
		progress++
		if progress%100000 == 0 {
			log.Infof("[Store][database] instance fetch rows progress: %d", progress)
		}
		var item svctypes.InstanceStore
		var labels string
		if err := rows.Scan(&item.ID, &item.ServiceID, &item.VpcID, &item.Host, &item.Port, &item.Protocol,
			&item.Version, &item.HealthStatus, &item.Isolate, &item.Weight, &item.EnableHealthCheck,
			&item.LogicSet, &item.Region, &item.Zone, &item.Campus, &item.Priority, &item.Revision,
			&item.Flag, &item.CheckType, &item.TTL, &item.CreateTime, &item.ModifyTime, &labels); err != nil {
			log.Errorf("[Store][database] fetch instance rows err: %s", err.Error())
			return err
		}
		item.Meta = make(map[string]string)
		_ = json.Unmarshal([]byte(labels), &item.Meta)
		ok, err := callback(&item)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		log.Errorf("[Store][database] instance rows catch err: %s", err.Error())
		return err
	}

	return nil
}

// fetchExpandInstanceRows 获取expandInstance rows的内容
func fetchExpandInstanceRows(rows *sql.Rows) ([]*svctypes.Instance, error) {
	if rows == nil {
		return nil, nil
	}
	defer rows.Close()

	var out []*svctypes.Instance
	var item svctypes.ExpandInstanceStore
	var instance svctypes.InstanceStore
	var metadataStr string
	item.ServiceInstance = &instance
	progress := 0
	for rows.Next() {
		progress++
		if progress%50000 == 0 {
			log.Infof("[Store][database] expand instance fetch rows progress: %d", progress)
		}
		err := rows.Scan(&instance.ID, &instance.ServiceID, &instance.VpcID, &instance.Host, &instance.Port,
			&instance.Protocol, &instance.Version, &instance.HealthStatus, &instance.Isolate,
			&instance.Weight, &instance.EnableHealthCheck, &instance.LogicSet, &instance.Region,
			&instance.Zone, &instance.Campus, &instance.Priority, &instance.Revision, &instance.Flag,
			&instance.CheckType, &instance.TTL, &item.ServiceName, &item.Namespace,
			&instance.CreateTime, &instance.ModifyTime, &metadataStr)
		if err != nil {
			log.Errorf("[Store][database] fetch instance rows err: %s", err.Error())
			return nil, err
		}
		instance.Meta = make(map[string]string)
		_ = json.Unmarshal([]byte(metadataStr), &instance.Meta)
		out = append(out, svctypes.ExpandStore2Instance(&item))
	}

	if err := rows.Err(); err != nil {
		log.Errorf("[Store][database] instance rows catch err: %s", err.Error())
		return nil, err
	}

	return out, nil
}

// genInstanceSelectSQL 生成instance的select sql语句
func genInstanceSelectSQL() string {
	str := `select instance.id, service_id, IFNULL(vpc_id,""), host, port, IFNULL(protocol, ""), IFNULL(version, ""),
			 health_status, isolate, weight, enable_health_check, IFNULL(logic_set, ""), IFNULL(cmdb_region, ""),
			 IFNULL(cmdb_zone, ""), IFNULL(cmdb_idc, ""), priority, revision, flag, IFNULL(health_check.type, -1),
			 IFNULL(health_check.ttl, 0), UNIX_TIMESTAMP(instance.ctime), UNIX_TIMESTAMP(instance.mtime), IFNULL(metadata, "{}")
			 from instance left join health_check
			 on instance.id = health_check.id `
	return str
}

// genCompleteInstanceSelectSQL 生成完整instance(主表+health_check+metadata)的sql语句
func genCompleteInstanceSelectSQL() string {
	str := `select instance.id, service_id, IFNULL(vpc_id,""), host, port, IFNULL(protocol, ""), IFNULL(version, ""),
		 health_status, isolate, weight, enable_health_check, IFNULL(logic_set, ""), IFNULL(cmdb_region, ""),
		 IFNULL(cmdb_zone, ""), IFNULL(cmdb_idc, ""), priority, revision, flag, IFNULL(health_check.type, -1),
		 IFNULL(health_check.ttl, 0), UNIX_TIMESTAMP(instance.ctime), UNIX_TIMESTAMP(instance.mtime), IFNULL(instance.metadata, "{}")
		 from instance
		 left join health_check on instance.id = health_check.id `
	return str
}

// genExpandInstanceSelectSQL 生成expandInstance的select sql语句
func genExpandInstanceSelectSQL(needForceIndex bool) string {
	str := `select instance.id, service_id, IFNULL(vpc_id,""), host, port, IFNULL(protocol, ""), IFNULL(version, ""),
					 health_status, isolate, weight, enable_health_check, IFNULL(logic_set, ""), IFNULL(cmdb_region, ""),
					 IFNULL(cmdb_zone, ""), IFNULL(cmdb_idc, ""), priority, instance.revision, instance.flag,
					 IFNULL(health_check.type, -1), IFNULL(health_check.ttl, 0), service.name, service.namespace,
					 UNIX_TIMESTAMP(instance.ctime), UNIX_TIMESTAMP(instance.mtime), IFNULL(instance.metadata, "{}")
					 from (service inner join instance `
	if needForceIndex {
		str += `force index(service_id, host) `
	}
	str += `on service.id = instance.service_id) left join health_check on instance.id = health_check.id `
	return str
}
