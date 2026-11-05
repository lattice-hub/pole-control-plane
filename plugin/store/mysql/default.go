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
	"errors"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pluginapi"
)

const (
	// SystemNamespace system namespace
	SystemNamespace = "pole-system"
	// STORENAME database storage name
	STORENAME = "defaultStore"
	// DefaultConnMaxLifetime default maximum connection lifetime
	DefaultConnMaxLifetime = 60 * 30 // 默认是30分钟
	// emptyEnableTime 规则禁用时启用时间的默认值
	emptyEnableTime = "STR_TO_DATE('1980-01-01 00:00:01', '%Y-%m-%d %H:%i:%s')"
)

func Register(registry *pluginapi.Registry) error {
	return store.RegisterStoreFactory(registry, pluginapi.Descriptor{
		Kind:   pluginapi.KindStore,
		Name:   STORENAME,
		Origin: pluginapi.OriginBuiltin,
	}, func() (store.Store, error) {
		return &stableStore{}, nil
	})
}

// stableStore 实现了Store接口
type stableStore struct {
	*namespaceStore
	*environmentPromotionStore

	// 服务治理中心 stores
	*serviceStore
	*logicalServiceStore
	*instanceStore
	*routerRuleStore
	*rateLimitStore
	*circuitBreakerStore
	*faultDetectRuleStore
	*serviceContractStore
	*laneStore
	*losslessStore
	*trafficSecurityStore
	*trafficMirrorStore
	*trafficMockStore

	*clientStore
	*adminStore
	*toolStore
	*grayStore

	*userStore
	*groupStore
	*strategyStore
	*roleStore

	// MCP Server stores
	*mcpServerStore
	*a2aAgentStore
	*aiDefinitionStore
	*skillMarketplaceStore

	// 主数据库，可以进行读写
	master *BaseDB
	// 备数据库，提供只读
	slave *BaseDB
	start bool
}

// Name 实现Name函数
func (s *stableStore) Name() string {
	return STORENAME
}

func (s *stableStore) CountGovernanceRules(namespace string) (uint64, error) {
	return newGovernanceRuleRepository(s.master, s.slave).CountRulesByNamespace(namespace)
}

// Initialize 初始化函数
func (s *stableStore) Initialize(conf *store.Config) error {
	if s.start {
		return nil
	}

	masterConfig, slaveConfig, err := parseDatabaseConf(conf.Option)
	if err != nil {
		return err
	}
	master, err := NewBaseDB(masterConfig)
	if err != nil {
		return err
	}
	s.master = master

	if slaveConfig != nil {
		log.Infof("[Store][database] use slave database config: %+v", slaveConfig)
		slave, err := NewBaseDB(slaveConfig)
		if err != nil {
			return err
		}
		s.slave = slave
	}
	// 如果slave为空，意味着slaveConfig为空，用master数据库替代
	if s.slave == nil {
		s.slave = s.master
	}

	log.Infof("[Store][database] connect the database successfully")

	if err := ensureNamespaceKindSchema(s.master); err != nil {
		return err
	}
	if err := ensureServiceSubscribeGraphSchema(s.master); err != nil {
		return err
	}
	if err := ensureServiceIdentitySchema(s.master); err != nil {
		return err
	}
	if err := ensureLogicalServiceSchema(s.master); err != nil {
		return err
	}
	if err := ensureAIResourceDefinitionSchema(s.master); err != nil {
		return err
	}
	if err := ensureGovernanceRuleSchema(s.master); err != nil {
		return err
	}
	if err := ensureUnifiedConfigSchema(s.master); err != nil {
		return err
	}
	if err := ensureEnvironmentPromotionSchema(s.master); err != nil {
		return err
	}
	if err := ensureSystemConfigurationRuntimeSchema(s.master); err != nil {
		return err
	}
	if err := ensureSkillMarketplaceSchema(s.master); err != nil {
		return err
	}

	s.start = true
	s.newStore()
	return nil
}

// parseDatabaseConf return slave, master, error
func parseDatabaseConf(opt map[string]any) (*dbConfig, *dbConfig, error) {
	// 必填
	masterEnter, ok := opt["master"]
	if !ok || masterEnter == nil {
		return nil, nil, errors.New("database master db config is missing")
	}
	masterConfig, err := parseStoreConfig(masterEnter)
	if err != nil {
		return nil, nil, err
	}

	// 只读数据库可选
	slaveEntry, ok := opt["slave"]
	if !ok || slaveEntry == nil {
		return masterConfig, nil, nil
	}
	slaveConfig, err := parseStoreConfig(slaveEntry)
	if err != nil {
		return nil, nil, err
	}

	return masterConfig, slaveConfig, nil
}

// parseStoreConfig 解析store的配置
func parseStoreConfig(opts any) (*dbConfig, error) {
	obj, _ := opts.(map[any]any)
	needCheckFields := map[string]string{"dbType": "", "dns": ""}
	for key := range needCheckFields {
		val, ok := obj[key]
		if !ok {
			return nil, fmt.Errorf("config Plugin %s:%s type must be string", STORENAME, key)
		}
		if val != nil {
			needCheckFields[key] = fmt.Sprintf("%v", val)
		} else {
			log.Warnf("[Store][database] config field is empty: %s", key)
		}
	}

	c := &dbConfig{
		dbType: needCheckFields["dbType"],
		dns:    needCheckFields["dns"],
	}
	if maxOpenConns, _ := obj["maxOpenConns"].(int); maxOpenConns > 0 {
		c.maxOpenConns = maxOpenConns
	}
	if maxIdleConns, _ := obj["maxIdleConns"].(int); maxIdleConns > 0 {
		c.maxIdleConns = maxIdleConns
	}
	c.connMaxLifetime = DefaultConnMaxLifetime
	if connMaxLifetime, _ := obj["connMaxLifetime"].(int); connMaxLifetime > 0 {
		c.connMaxLifetime = connMaxLifetime
	}
	return c, nil
}

// Destroy 退出函数
func (s *stableStore) Destroy() error {
	s.start = false
	if s.master != nil {
		_ = s.master.Close()
	}
	if s.slave != nil {
		_ = s.slave.Close()
	}

	if s.adminStore != nil {
		s.adminStore.StopLeaderElections()
	}

	s.master = nil
	s.slave = nil

	return nil
}

// CreateTransaction 创建一个事务
func (s *stableStore) CreateTransaction() (store.Transaction, error) {
	// 每次创建事务前，还是需要ping一下
	_ = s.master.Ping()

	nt := &transaction{}
	tx, err := s.master.Begin()
	if err != nil {
		log.Errorf("[Store][database] database begin err: %s", err.Error())
		return nil, err
	}

	nt.tx = tx
	return nt, nil
}

func (s *stableStore) StartTx() (store.Tx, error) {
	// 每次创建事务前，还是需要ping一下
	_ = s.master.Ping()

	tx, err := s.master.Begin()
	if err != nil {
		return nil, err
	}
	return NewSqlDBTx(tx), nil
}

func (s *stableStore) StartReadTx() (store.Tx, error) {
	// 每次创建事务前，还是需要ping一下
	_ = s.slave.Ping()

	// 由于是 Read，为了确保 Read 过程中的可重复读，这里主动设置为 RR 级别
	tx, err := s.slave.BeginTx(&sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
	})
	if err != nil {
		return nil, err
	}
	return NewSqlDBTx(tx), nil
}

// newStore 初始化子类
func (s *stableStore) newStore() {
	s.namespaceStore = &namespaceStore{master: s.master, slave: s.slave}
	s.environmentPromotionStore = &environmentPromotionStore{master: s.master}

	s.serviceStore = &serviceStore{master: s.master, slave: s.slave}
	s.logicalServiceStore = &logicalServiceStore{master: s.master, slave: s.slave}
	s.instanceStore = &instanceStore{master: s.master, slave: s.slave}
	governanceRepo := newGovernanceRuleRepository(s.master, s.slave)
	s.routerRuleStore = &routerRuleStore{master: s.master, slave: s.slave, governanceRuleRepository: governanceRepo}
	s.rateLimitStore = &rateLimitStore{master: s.master, slave: s.slave, governanceRuleRepository: governanceRepo}
	s.circuitBreakerStore = &circuitBreakerStore{master: s.master, slave: s.slave, governanceRuleRepository: governanceRepo}
	s.faultDetectRuleStore = &faultDetectRuleStore{master: s.master, slave: s.slave, governanceRuleRepository: governanceRepo}
	s.serviceContractStore = &serviceContractStore{master: s.master, slave: s.slave}
	s.laneStore = &laneStore{master: s.master, slave: s.slave, governanceRuleRepository: governanceRepo}
	s.losslessStore = &losslessStore{master: s.master, slave: s.slave, governanceRuleRepository: governanceRepo}
	s.trafficSecurityStore = &trafficSecurityStore{trafficGovernanceStore{master: s.master, slave: s.slave, governanceRuleRepository: governanceRepo}}
	s.trafficMirrorStore = &trafficMirrorStore{trafficGovernanceStore{master: s.master, slave: s.slave, governanceRuleRepository: governanceRepo}}
	s.trafficMockStore = &trafficMockStore{trafficGovernanceStore{master: s.master, slave: s.slave, governanceRuleRepository: governanceRepo}}

	s.grayStore = &grayStore{master: s.master, slave: s.slave}

	s.adminStore = newAdminStore(s.master)
	s.toolStore = &toolStore{db: s.master}

	s.userStore = &userStore{master: s.master, slave: s.slave}
	s.groupStore = &groupStore{master: s.master, slave: s.slave}
	s.strategyStore = &strategyStore{master: s.master, slave: s.slave}
	s.roleStore = &roleStore{master: s.master, slave: s.slave}

	// Initialize AI module stores
	s.mcpServerStore = newMCPServerStore(s.master, s.slave)
	s.a2aAgentStore = newA2AAgentStore(s.master, s.slave)
	s.aiDefinitionStore = newAIResourceDefinitionStore(s.master, s.slave)
	s.skillMarketplaceStore = newSkillMarketplaceStore(s.master, s.slave)
}

func buildEtimeStr(enable bool) string {
	etimeStr := "sysdate()"
	if !enable {
		etimeStr = emptyEnableTime
	}
	return etimeStr
}
