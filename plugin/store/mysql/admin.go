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
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pole-io/pole-server/apis/pkg/types/admin"
	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/common/eventhub"
	"github.com/pole-io/pole-server/pkg/common/utils"
)

const (
	// 调整心跳频率，从2秒一次改为10秒一次
	TickTime = 10
	// 增加租约时长，避免频繁切换
	LeaseTime = 30
	// 心跳失败容错次数
	HeartbeatMaxFailures = 3
)

// adminStore implement adminStore interface
type adminStore struct {
	master  *BaseDB
	leStore LeaderElectionStore
	leMap   map[string]*CachedLeaderElection
	mutex   sync.Mutex
}

func newAdminStore(master *BaseDB) *adminStore {
	return &adminStore{
		master:  master,
		leStore: &leaderElectionStore{master: master},
		leMap:   make(map[string]*CachedLeaderElection),
	}
}

// LeaderElectionStore store inteface
type LeaderElectionStore interface {
	// CreateLeaderElection
	CreateLeaderElection(key string) error
	// GetVersion get current version
	GetVersion(key string) (int64, error)
	// CompareAndSwapVersion cas version
	CompareAndSwapVersion(key string, curVersion int64, newVersion int64, leader string) (bool, error)
	// CheckMtimeExpired check mtime expired
	CheckMtimeExpired(key string, leaseTime int32) (string, bool, error)
	// ListLeaderElections list all leaderelection
	ListLeaderElections() ([]*admin.LeaderElection, error)
}

// leaderElectionStore
type leaderElectionStore struct {
	master *BaseDB
}

// CreateLeaderElection insert election key into leader table
func (l *leaderElectionStore) CreateLeaderElection(key string) error {
	log.Debugf("[Store][database] create leader election (%s)", key)
	return l.master.processWithTransaction("createLeaderElection", func(tx *BaseTx) error {
		mainStr := "insert ignore into leader_election (elect_key, leader) values (?, ?)"
		if _, err := tx.Exec(mainStr, key, ""); err != nil {
			log.Errorf("[Store][database] create leader election (%s), err: %s", key, err.Error())
			return store.Error(err)
		}

		if err := tx.Commit(); err != nil {
			log.Errorf("[Store][database] create leader election (%s) commit tx err: %s", key, err.Error())
			return err
		}
		return nil
	})
}

// GetVersion get the version from election
func (l *leaderElectionStore) GetVersion(key string) (int64, error) {
	log.Debugf("[Store][database] get version (%s)", key)
	mainStr := "select version from leader_election where elect_key = ?"

	var count int64
	err := l.master.DB.QueryRow(mainStr, key).Scan(&count)
	if err != nil {
		log.Errorf("[Store][database] get version (%s), err: %s", key, err.Error())
	}
	return count, store.Error(err)
}

// CompareAndSwapVersion compare key version and update
func (l *leaderElectionStore) CompareAndSwapVersion(key string, curVersion int64, newVersion int64,
	leader string) (bool, error) {
	var rows int64
	err := l.master.processWithTransaction("compareAndSwapVersion", func(tx *BaseTx) error {
		log.Debugf("[Store][database] compare and swap version (%s, %d, %d, %s)", key, curVersion, newVersion, leader)
		mainStr := "update leader_election set leader = ?, version = ? where elect_key = ? and version = ?"
		result, err := tx.Exec(mainStr, leader, newVersion, key, curVersion)
		if err != nil {
			log.Errorf("[Store][database] compare and swap version (%s), err: %s", key, err.Error())
			return store.Error(err)
		}
		tRows, err := result.RowsAffected()
		if err != nil {
			log.Errorf("[Store][database] compare and swap version (%s), get RowsAffected err: %s", key, err.Error())
			return store.Error(err)
		}

		if err := tx.Commit(); err != nil {
			log.Errorf("[Store][database] create leader election (%s) commit tx err: %s", key, err.Error())
			return err
		}

		rows = tRows
		return nil
	})
	return rows > 0, err
}

// CheckMtimeExpired check last modify time expired
func (l *leaderElectionStore) CheckMtimeExpired(key string, leaseTime int32) (string, bool, error) {
	log.Debugf("[Store][database] check mtime expired (%s, %d)", key, leaseTime)
	mainStr := `select leader, UNIX_TIMESTAMP(SYSDATE()) - UNIX_TIMESTAMP(mtime) 
					from leader_election where elect_key = ?`

	var (
		leader   string
		diffTime int32
	)
	err := l.master.DB.QueryRow(mainStr, key).Scan(&leader, &diffTime)
	if err != nil {
		log.Errorf("[Store][database] check mtime expired (%s), err: %s", key, err.Error())
	}
	return leader, (diffTime > leaseTime), store.Error(err)
}

// ListLeaderElections list the election records
func (l *leaderElectionStore) ListLeaderElections() ([]*admin.LeaderElection, error) {
	log.Info("[Store][database] list leader election")
	mainStr := "select elect_key, leader, UNIX_TIMESTAMP(ctime), UNIX_TIMESTAMP(mtime) from leader_election"

	rows, err := l.master.Query(mainStr)
	if err != nil {
		log.Errorf("[Store][database] list leader election query err: %s", err.Error())
		return nil, store.Error(err)
	}

	return fetchLeaderElectionRows(rows)
}

func fetchLeaderElectionRows(rows *sql.Rows) ([]*admin.LeaderElection, error) {
	if rows == nil {
		return nil, nil
	}
	defer rows.Close()

	var out []*admin.LeaderElection

	for rows.Next() {
		space := &admin.LeaderElection{}
		if err := rows.Scan(&space.ElectKey, &space.Host, &space.Ctime, &space.Mtime); err != nil {
			log.Errorf("[Store][database] fetch leader election rows scan err: %s", err.Error())
			return nil, err
		}

		space.CreateTime = time.Unix(space.Ctime, 0)
		space.ModifyTime = time.Unix(space.Mtime, 0)
		space.Valid = checkLeaderValid(space.Mtime)
		out = append(out, space)
	}
	if err := rows.Err(); err != nil {
		log.Errorf("[Store][database] fetch leader election rows next err: %s", err.Error())
		return nil, err
	}

	return out, nil
}

func checkLeaderValid(mtime int64) bool {
	delta := time.Now().Unix() - mtime
	return delta <= LeaseTime
}

// isLeader
func isLeader(flag int32) bool {
	return flag > 0
}

// StopLeaderElections stop the election procedure
func (m *adminStore) StopLeaderElections() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var wg sync.WaitGroup
	for k, le := range m.leMap {
		wg.Add(1)
		go func(key string, election *CachedLeaderElection) {
			defer wg.Done()
			// 调用cancel让goroutine正常退出
			election.cancel()
			log.Infof("[Store][database] stopped leader election for key (%s)", key)
		}(k, le)
		delete(m.leMap, k)
	}

	// 等待所有goroutine优雅退出
	wg.Wait()
	log.Info("[Store][database] all leader elections stopped")
}

// IsLeader check leader
func (m *adminStore) IsLeader(key string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	le, ok := m.leMap[key]
	if !ok {
		return false
	}
	return le.isLeader()
}

// ListLeaderElections list election records
func (m *adminStore) ListLeaderElections() ([]*admin.LeaderElection, error) {
	return m.leStore.ListLeaderElections()
}

// ReleaseLeaderElection release election lock
func (m *adminStore) ReleaseLeaderElection(key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	le, ok := m.leMap[key]
	if !ok {
		return fmt.Errorf("LeaderElection(%s) not started", key)
	}

	le.setReleaseSignal()
	return nil
}

// BatchCleanDeletedInstances batch clean soft deleted instances
func (m *adminStore) BatchCleanDeletedInstances(timeout time.Duration, batchSize uint32) (uint32, error) {
	log.Infof("[Store][database] batch clean soft deleted instances(%d)", batchSize)
	var rowsAffected int64
	err := m.master.processWithTransaction("batchCleanDeletedInstances", func(tx *BaseTx) error {
		// 查询出需要清理的实例 ID 信息
		loadWaitDel := "SELECT id FROM instance WHERE flag = 1 AND " +
			"mtime <= FROM_UNIXTIME(UNIX_TIMESTAMP(SYSDATE()) - ?) LIMIT ?"
		rows, err := tx.Query(loadWaitDel, int32(timeout.Seconds()), batchSize)
		if err != nil {
			log.Errorf("[Store][database] batch clean soft deleted instances(%d), err: %s", batchSize, err.Error())
			return store.Error(err)
		}
		waitDelIds := make([]interface{}, 0, batchSize)
		defer func() {
			_ = rows.Close()
		}()

		placeholders := make([]string, 0, batchSize)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				log.Errorf("[Store][database] scan deleted instances id, err: %s", err.Error())
				return store.Error(err)
			}
			waitDelIds = append(waitDelIds, id)
			placeholders = append(placeholders, "?")
		}

		if len(waitDelIds) == 0 {
			return nil
		}
		inSql := strings.Join(placeholders, ",")

		cleanCheckStr := fmt.Sprintf("delete from health_check where id in (%s)", inSql)
		if _, err = tx.Exec(cleanCheckStr, waitDelIds...); err != nil {
			log.Errorf("[Store][database] batch clean soft deleted instances(%d), err: %s", batchSize, err.Error())
			return store.Error(err)
		}

		cleanInsStr := fmt.Sprintf("delete from instance where flag = 1 and id in (%s)", inSql)
		result, err := tx.Exec(cleanInsStr, waitDelIds...)
		if err != nil {
			log.Errorf("[Store][database] batch clean soft deleted instances(%d), err: %s", batchSize, err.Error())
			return store.Error(err)
		}

		tRows, err := result.RowsAffected()
		if err != nil {
			log.Warnf("[Store][database] batch clean soft deleted instances(%d), get RowsAffected err: %s",
				batchSize, err.Error())
			return store.Error(err)
		}

		if err = tx.Commit(); err != nil {
			log.Errorf("[Store][database] batch clean soft deleted instances(%d) commit tx err: %s",
				batchSize, err.Error())
			return err
		}

		rowsAffected = tRows
		return nil
	})
	return uint32(rowsAffected), err
}

func (m *adminStore) GetUnHealthyInstances(timeout time.Duration, limit uint32) ([]string, error) {
	log.Infof("[Store][database] get unhealthy instances which mtime timeout %s (%d)", timeout, limit)
	queryStr := "select id from instance where flag=0 and enable_health_check=1 and health_status=0 " +
		"and mtime < FROM_UNIXTIME(UNIX_TIMESTAMP(SYSDATE()) - ?) limit ?"
	rows, err := m.master.Query(queryStr, int32(timeout.Seconds()), limit)
	if err != nil {
		log.Errorf("[Store][database] get unhealthy instances, err: %s", err.Error())
		return nil, store.Error(err)
	}

	var instanceIds []string
	defer rows.Close()
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			log.Errorf("[Store][database] fetch unhealthy instance rows, err: %s", err.Error())
			return nil, store.Error(err)
		}
		instanceIds = append(instanceIds, id)
	}
	if err := rows.Err(); err != nil {
		log.Errorf("[Store][database] fetch unhealthy instance rows next, err: %s", err.Error())
		return nil, store.Error(err)
	}

	return instanceIds, nil
}

// BatchCleanDeletedClients batch clean soft deleted clients
func (m *adminStore) BatchCleanDeletedClients(timeout time.Duration, batchSize uint32) (uint32, error) {
	log.Infof("[Store][database] batch clean soft deleted clients(%d)", batchSize)
	var rows int64
	err := m.master.processWithTransaction("batchCleanDeletedClients", func(tx *BaseTx) error {
		mainStr := "delete from client where flag = 1 and mtime <= FROM_UNIXTIME(UNIX_TIMESTAMP(SYSDATE()) - ?) limit ?"
		result, err := tx.Exec(mainStr, int32(timeout.Seconds()), batchSize)
		if err != nil {
			log.Errorf("[Store][database] batch clean soft deleted clients(%d), err: %s", batchSize, err.Error())
			return store.Error(err)
		}

		tRows, err := result.RowsAffected()
		if err != nil {
			log.Warnf("[Store][database] batch clean soft deleted clients(%d), get RowsAffected err: %s",
				batchSize, err.Error())
			return store.Error(err)
		}

		if err := tx.Commit(); err != nil {
			log.Errorf("[Store][database] batch clean soft deleted clients(%d) commit tx err: %s",
				batchSize, err.Error())
			return err
		}

		rows = tRows
		return nil
	})
	return uint32(rows), err
}

// BatchCleanDeletedServices batch clean soft deleted clients
func (m *adminStore) BatchCleanDeletedServices(timeout time.Duration, batchSize uint32) (uint32, error) {
	log.Infof("[Store][database] batch clean soft deleted services(%d)", batchSize)
	var rowsAffected int64
	err := m.master.processWithTransaction("batchCleanDeletedServices", func(tx *BaseTx) error {
		// 查询出需要清理的实例 ID 信息
		loadWaitDel := "SELECT id FROM service WHERE flag = 1 AND " +
			"mtime <= FROM_UNIXTIME(UNIX_TIMESTAMP(SYSDATE()) - ?) LIMIT ?"
		rows, err := tx.Query(loadWaitDel, int32(timeout.Seconds()), batchSize)
		if err != nil {
			log.Errorf("[Store][database] batch clean soft deleted services(%d), err: %s", batchSize, err.Error())
			return store.Error(err)
		}
		waitDelIds := make([]interface{}, 0, batchSize)
		defer func() {
			_ = rows.Close()
		}()

		placeholders := make([]string, 0, batchSize)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				log.Errorf("[Store][database] scan deleted services id, err: %s", err.Error())
				return store.Error(err)
			}
			waitDelIds = append(waitDelIds, id)
			placeholders = append(placeholders, "?")
		}

		if len(waitDelIds) == 0 {
			return nil
		}
		inSql := strings.Join(placeholders, ",")

		cleanMetaStr := fmt.Sprintf("delete from service_metadata where id in (%s)", inSql)
		if _, err := tx.Exec(cleanMetaStr, waitDelIds...); err != nil {
			log.Errorf("[Store][database] batch clean soft deleted services(%d), err: %s", batchSize, err.Error())
			return store.Error(err)
		}

		cleanInsStr := fmt.Sprintf("delete from service where flag = 1 and id in (%s)", inSql)
		result, err := tx.Exec(cleanInsStr, waitDelIds...)
		if err != nil {
			log.Errorf("[Store][database] batch clean soft deleted services(%d), err: %s", batchSize, err.Error())
			return store.Error(err)
		}

		tRows, err := result.RowsAffected()
		if err != nil {
			log.Warnf("[Store][database] batch clean soft deleted services(%d), get RowsAffected err: %s",
				batchSize, err.Error())
			return store.Error(err)
		}

		if err = tx.Commit(); err != nil {
			log.Errorf("[Store][database] batch clean soft deleted services(%d) commit tx err: %s",
				batchSize, err.Error())
			return err
		}

		rowsAffected = tRows
		return nil
	})
	return uint32(rowsAffected), err
}

// BatchCleanDeletedRules batch clean soft deleted clients
func (m *adminStore) BatchCleanDeletedRules(rule string, timeout time.Duration, batchSize uint32) (uint32, error) {
	log.Infof("[Store][database] batch clean soft deleted %s(%d)", rule, batchSize)
	var rows int64

	// 验证表名，防止SQL注入（与 pole_server.sql 及 clean_deleted_resource job 一致）
	validTables := map[string]bool{
		"governance_rule":         true,
		"governance_rule_release": true,
	}

	if !validTables[rule] {
		log.Errorf("[Store][database] invalid table name: %s", rule)
		return 0, fmt.Errorf("invalid table name: %s", rule)
	}

	err := m.master.processWithTransaction("batchCleanDeleted"+rule, func(tx *BaseTx) error {
		mainStr := "delete from " + rule + " where flag = 1 and mtime <= FROM_UNIXTIME(UNIX_TIMESTAMP(SYSDATE()) - ?) limit ?"
		result, err := tx.Exec(mainStr, int32(timeout.Seconds()), batchSize)
		if err != nil {
			log.Errorf("[Store][database] batch clean soft deleted %s(%d), err: %s", rule, batchSize, err.Error())
			return store.Error(err)
		}

		tRows, err := result.RowsAffected()
		if err != nil {
			log.Warnf("[Store][database] batch clean soft deleted %s(%d), get RowsAffected err: %s",
				rule, batchSize, err.Error())
			return store.Error(err)
		}

		if err := tx.Commit(); err != nil {
			log.Errorf("[Store][database] batch clean soft deleted %s(%d) commit tx err: %s",
				rule, batchSize, err.Error())
			return err
		}

		rows = tRows
		return nil
	})
	return uint32(rows), err
}

// BatchCleanDeletedServiceContracts batch clean soft deleted service_contract
func (m *adminStore) BatchCleanDeletedServiceContracts(timeout time.Duration, batchSize uint32) (uint32, error) {
	log.Infof("[Store][database] batch clean soft deleted service_contract(%d)", batchSize)
	var affectRows int64
	err := m.master.processWithTransaction("batchCleanDeletedServiceContracts", func(tx *BaseTx) error {
		// 查询出需要清理的服务契约 ID 信息
		fetchSql := "select id from service_contract where flag = 1 and mtime <= FROM_UNIXTIME(UNIX_TIMESTAMP(SYSDATE()) - ?) order by mtime asc limit ?"
		rows, err := tx.Query(fetchSql, int32(timeout.Seconds()), batchSize)
		if err != nil {
			log.Errorf("[Store][database] batch clean soft deleted service_contract(%d), err: %s", batchSize, err.Error())
			return store.Error(err)
		}
		waitDelIds := make([]interface{}, 0, batchSize)
		defer func() {
			_ = rows.Close()
		}()
		placeholders := make([]string, 0, batchSize)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				log.Errorf("[Store][database] scan deleted service_contract id, err: %s", err.Error())
				return store.Error(err)
			}
			waitDelIds = append(waitDelIds, id)
			placeholders = append(placeholders, "?")
		}
		if len(waitDelIds) == 0 {
			return nil
		}
		inSql := strings.Join(placeholders, ",")
		// 删除服务契约的健康检查
		cleanMetaStr := fmt.Sprintf("delete from service_contract_detail where contract_id in (%s)", inSql)
		if _, err := tx.Exec(cleanMetaStr, waitDelIds...); err != nil {
			log.Errorf("[Store][database] batch clean soft deleted service_contract(%d), err: %s", batchSize, err.Error())
			return store.Error(err)
		}
		// 删除服务契约
		cleanInsStr := fmt.Sprintf("delete from service_contract where flag = 1 and id in (%s)", inSql)
		result, err := tx.Exec(cleanInsStr, waitDelIds...)
		if err != nil {
			log.Errorf("[Store][database] batch clean soft deleted service_contract(%d), err: %s", batchSize, err.Error())
			return store.Error(err)
		}
		tRows, err := result.RowsAffected()
		if err != nil {
			log.Warnf("[Store][database] batch clean soft deleted service_contract(%d), get RowsAffected err: %s",
				batchSize, err.Error())
			return store.Error(err)
		}

		if err := tx.Commit(); err != nil {
			log.Errorf("[Store][database] batch clean soft deleted service_contract(%d) commit tx err: %s",
				batchSize, err.Error())
			return err
		}

		affectRows = tRows
		return nil
	})
	return uint32(affectRows), err
}

// CachedLeaderElection 带缓存的领导选举实现
type CachedLeaderElection struct {
	electKey         string
	leStore          LeaderElectionStore
	leaderFlag       int32
	version          int64
	ctx              context.Context
	cancel           context.CancelFunc
	releaseSignal    int32
	releaseTickLimit int32
	leader           string
	mutex            sync.RWMutex

	// 缓存相关
	lastHeartbeatTime time.Time     // 上次心跳时间
	heartbeatFailures int32         // 心跳失败次数计数
	cacheExpireTime   time.Duration // 缓存过期时间
}

// NewCachedLeaderElection 创建带缓存的领导选举实例
func NewCachedLeaderElection(key string, leStore LeaderElectionStore) *CachedLeaderElection {
	ctx, cancel := context.WithCancel(context.TODO())
	return &CachedLeaderElection{
		electKey:          key,
		leStore:           leStore,
		leaderFlag:        0,
		version:           0,
		ctx:               ctx,
		cancel:            cancel,
		releaseSignal:     0,
		releaseTickLimit:  0,
		lastHeartbeatTime: time.Time{},
		heartbeatFailures: 0,
		cacheExpireTime:   time.Duration(LeaseTime/2) * time.Second,
	}
}

// mainLoop 带缓存的领导选举主循环
func (cle *CachedLeaderElection) mainLoop() {
	cle.changeToFollower("")
	log.Infof("[Store][database] cached leader election started (%s)", cle.electKey)
	ticker := time.NewTicker(TickTime * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cle.tick()
		case <-cle.ctx.Done():
			log.Infof("[Store][database] cached leader election stopped (%s)", cle.electKey)
			cle.changeToFollower("")
			return
		}
	}
}

// tick 带缓存的心跳检查
func (cle *CachedLeaderElection) tick() {
	if cle.checkReleaseTickLimit() {
		log.Infof("[Store][database] abandon leader election in this tick (%s)", cle.electKey)
		return
	}
	shouldRelease := cle.checkAndClearReleaseSignal()

	cle.mutex.RLock()
	isLeader := cle.isLeader()
	cle.mutex.RUnlock()

	// 如果是leader，处理leader相关逻辑
	if isLeader {
		if shouldRelease {
			log.Infof("[Store][database] release leader election (%s)", cle.electKey)
			cle.changeToFollower("")
			cle.setReleaseTickLimit()
			return
		}

		// 检查是否需要执行心跳
		now := time.Now()
		cle.mutex.RLock()
		timeSinceLastHeartbeat := now.Sub(cle.lastHeartbeatTime)
		cle.mutex.RUnlock()

		// 如果距离上次心跳时间未超过缓存过期时间的一半，直接返回
		// 这样可以大幅减少数据库心跳频率
		if timeSinceLastHeartbeat < cle.cacheExpireTime/2 {
			return
		}

		// 执行心跳
		success, err := cle.heartbeat()
		if err == nil && success {
			cle.mutex.Lock()
			cle.lastHeartbeatTime = now
			cle.heartbeatFailures = 0
			cle.mutex.Unlock()
			return
		}

		// 心跳失败
		cle.mutex.Lock()
		cle.heartbeatFailures++
		failures := cle.heartbeatFailures
		cle.mutex.Unlock()

		// 只有连续失败超过阈值才放弃leader身份
		if failures > HeartbeatMaxFailures {
			if err != nil {
				log.Errorf("[Store][database] leader heartbeat err (%v), change to follower state (%s)",
					err, cle.electKey)
			} else {
				log.Infof("[Store][database] leader heartbeat abort, change to follower state (%s)",
					cle.electKey)
			}
			cle.changeToFollower("")
		} else {
			log.Warnf("[Store][database] leader heartbeat failed (attempt %d/%d) for (%s)",
				failures, HeartbeatMaxFailures, cle.electKey)
		}
		return
	}

	// 如果不是leader，检查当前leader是否存活
	leader, dead, err := cle.checkLeaderDead()
	if err != nil {
		log.Errorf("[Store][database] check leader dead err (%s), stay follower state (%s)",
			err.Error(), cle.electKey)
		return
	}

	if !dead {
		// 有活跃的leader
		if leader == utils.LocalHost {
			cle.changeToLeader()
			cle.mutex.Lock()
			cle.lastHeartbeatTime = time.Now()
			cle.mutex.Unlock()
		} else if leader != cle.leader {
			cle.changeToFollower(leader)
		}
		return
	}

	// 没有活跃的leader，尝试竞选
	success, err := cle.elect()
	if err != nil {
		log.Errorf("[Store][database] elect leader err (%s), stay follower state (%s)",
			err.Error(), cle.electKey)
		return
	}

	if success {
		cle.mutex.Lock()
		cle.lastHeartbeatTime = time.Now()
		cle.mutex.Unlock()
		cle.changeToLeader()
	}
}

// isLeader
func (cle *CachedLeaderElection) isLeader() bool {
	// isLeader
	return cle.leaderFlag > 0
}

// checkReleaseTickLimit
func (cle *CachedLeaderElection) checkReleaseTickLimit() bool {
	if cle.releaseTickLimit > 0 {
		cle.releaseTickLimit = cle.releaseTickLimit - 1
		return true
	}
	return false
}

// checkAndClearReleaseSignal
func (cle *CachedLeaderElection) checkAndClearReleaseSignal() bool {
	return atomic.CompareAndSwapInt32(&cle.releaseSignal, 1, 0)
}

// setReleaseSignal
func (cle *CachedLeaderElection) setReleaseSignal() {
	atomic.StoreInt32(&cle.releaseSignal, 1)
}

// setReleaseTickLimit
func (cle *CachedLeaderElection) setReleaseTickLimit() {
	cle.releaseTickLimit = LeaseTime / TickTime * 3
}

// elect 竞选leader
func (cle *CachedLeaderElection) elect() (bool, error) {
	curVersion, err := cle.leStore.GetVersion(cle.electKey)
	if err != nil {
		return false, err
	}
	cle.version = curVersion + 1
	return cle.leStore.CompareAndSwapVersion(cle.electKey, curVersion, cle.version, utils.LocalHost)
}

// heartbeat 发送心跳
func (cle *CachedLeaderElection) heartbeat() (bool, error) {
	curVersion := cle.version
	cle.version = curVersion + 1
	return cle.leStore.CompareAndSwapVersion(cle.electKey, curVersion, cle.version, utils.LocalHost)
}

// checkLeaderDead 检查当前leader是否已经失效
func (cle *CachedLeaderElection) checkLeaderDead() (string, bool, error) {
	return cle.leStore.CheckMtimeExpired(cle.electKey, LeaseTime)
}

// changeToLeader 转变为leader角色
func (cle *CachedLeaderElection) changeToLeader() {
	log.Infof("[Store][database] cached election: change from follower to leader (%s)", cle.electKey)
	atomic.StoreInt32(&cle.leaderFlag, 1)
	cle.leader = utils.LocalHost
	cle.publishLeaderChangeEvent()
}

// changeToFollower 转变为follower角色
func (cle *CachedLeaderElection) changeToFollower(leader string) {
	log.Infof("[Store][database] cached election: change from leader(%s) to follower for election key (%s)",
		cle.leader, cle.electKey)
	atomic.StoreInt32(&cle.leaderFlag, 0)
	cle.leader = leader
	cle.publishLeaderChangeEvent()
}

// publishLeaderChangeEvent 发布leader变更事件
func (cle *CachedLeaderElection) publishLeaderChangeEvent() {
	_ = eventhub.Publish(eventhub.LeaderChangeEventTopic, store.LeaderChangeEvent{
		Key:        cle.electKey,
		Leader:     cle.isLeader(),
		LeaderHost: cle.leader,
	})
}

// StartLeaderElection 启动带缓存的领导选举
func (m *adminStore) StartLeaderElection(key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	_, ok := m.leMap[key]
	if ok {
		return nil
	}

	// 创建带缓存的领导选举实例
	cle := NewCachedLeaderElection(key, m.leStore)

	// 先创建选举记录
	err := m.leStore.CreateLeaderElection(key)
	if err != nil {
		return store.Error(err)
	}

	// 启动选举循环
	go cle.mainLoop()

	// 保存到选举映射中
	m.leMap[key] = cle

	log.Infof("[Store][database] started cached leader election for key (%s)", key)
	return nil
}

// BatchCleanWithChunks 使用分块处理方式进行批量清理，减少MySQL压力
func (m *adminStore) BatchCleanWithChunks(tableName string, timeout time.Duration, batchSize uint32, chunkSize uint32) (uint32, error) {
	if chunkSize == 0 {
		chunkSize = 100 // 默认每次处理100条记录
	}

	if chunkSize > batchSize {
		chunkSize = batchSize
	}

	log.Infof("[Store][database] batch clean with chunks for %s (batch:%d, chunk:%d)",
		tableName, batchSize, chunkSize)

	// 验证表名（与 pole_server.sql 及 clean_deleted_resource job 一致）
	validTables := map[string]bool{
		"instance":                true,
		"service":                 true,
		"client":                  true,
		"service_contract":        true,
		"governance_rule":         true,
		"governance_rule_release": true,
	}

	if !validTables[tableName] {
		return 0, fmt.Errorf("invalid table name: %s", tableName)
	}

	var totalCleaned uint32
	var remainingToClean uint32 = batchSize

	for remainingToClean > 0 {
		currentChunkSize := chunkSize
		if remainingToClean < chunkSize {
			currentChunkSize = remainingToClean
		}

		// 根据表名调用相应的清理方法
		var cleanedCount uint32
		var err error

		switch tableName {
		case "instance":
			cleanedCount, err = m.BatchCleanDeletedInstances(timeout, currentChunkSize)
		case "service":
			cleanedCount, err = m.BatchCleanDeletedServices(timeout, currentChunkSize)
		case "client":
			cleanedCount, err = m.BatchCleanDeletedClients(timeout, currentChunkSize)
		case "service_contract":
			cleanedCount, err = m.BatchCleanDeletedServiceContracts(timeout, currentChunkSize)
		case "governance_rule", "governance_rule_release":
			cleanedCount, err = m.BatchCleanDeletedRules(tableName, timeout, currentChunkSize)
		}

		if err != nil {
			log.Errorf("[Store][database] batch clean with chunks for %s error: %v", tableName, err)
			return totalCleaned, err
		}

		totalCleaned += cleanedCount
		remainingToClean -= currentChunkSize

		// 如果本次没有清理到任何记录，说明已经没有符合条件的记录了，退出循环
		if cleanedCount == 0 {
			break
		}

		// 让出CPU执行权，避免长时间占用数据库连接
		time.Sleep(100 * time.Millisecond)
	}

	return totalCleaned, nil
}
