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

package testsuit

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"

	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"
	apitraffic "github.com/pole-io/specification/source/go/api/v1/traffic_manage"

	"github.com/pole-io/pole-server/apis"
	authapi "github.com/pole-io/pole-server/apis/access_control/auth"
	cacheapi "github.com/pole-io/pole-server/apis/cache"
	"github.com/pole-io/pole-server/apis/pkg/types"
	storeapi "github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/cache"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/eventhub"
	"github.com/pole-io/pole-server/pkg/common/log"
	commonlog "github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/common/otel/metrics"
	"github.com/pole-io/pole-server/pkg/common/utils"
	"github.com/pole-io/pole-server/pkg/config"
	"github.com/pole-io/pole-server/pkg/goverrule"
	_ "github.com/pole-io/pole-server/pkg/goverrule/interceptor"
	"github.com/pole-io/pole-server/pkg/namespace"
	ns "github.com/pole-io/pole-server/pkg/namespace"
	_ "github.com/pole-io/pole-server/pkg/namespace/interceptor"
	"github.com/pole-io/pole-server/pkg/service"
	"github.com/pole-io/pole-server/pkg/service/batch"
	"github.com/pole-io/pole-server/pkg/service/healthcheck"
	_ "github.com/pole-io/pole-server/pkg/service/interceptor"
	"github.com/pole-io/pole-server/plugin/access_control/auth"
	storeplugin "github.com/pole-io/pole-server/plugin/store"
	sqldb "github.com/pole-io/pole-server/plugin/store/mysql"
	testdata "github.com/pole-io/pole-server/test/data"
)

func init() {
	go func() {
		http.ListenAndServe("0.0.0.0:16060", nil)
	}()
}

const (
	tblNameNamespace = "namespace"
	tblNameInstance  = "instance"
	tblNameService   = "service"
	tblNameRouting   = "routing"
	tblClient        = "client"
)

var (
	testNamespace = "testNamespace123qwe"
	testGroup     = "testGroup"
	testFile      = "testFile"
	testContent   = "testContent"
	operator      = "polaris"
	size          = 7
)

const (
	templateName1 = "t1"
	templateName2 = "t2"
)

type Bootstrap struct {
	Logger map[string]*commonlog.Options
}

type TestConfig struct {
	Bootstrap           Bootstrap      `yaml:"bootstrap"`
	Cache               cache.Config   `yaml:"cache"`
	Namespace           ns.Config      `yaml:"namespace"`
	Naming              service.Config `yaml:"naming"`
	DisableConfig       bool
	Config              config.Config      `yaml:"config"`
	HealthChecks        healthcheck.Config `yaml:"healthcheck"`
	Store               storeapi.Config    `yaml:"store"`
	DisableAuth         bool
	Auth                authapi.Config `yaml:"auth"`
	Plugin              apis.Config    `yaml:"plugin"`
	ReplaceStore        storeapi.Store
	ServiceCacheEntries []cacheapi.ConfigEntry
}

var InjectTestDataClean func() TestDataClean

func SetTestDataClean(callback func() TestDataClean) {
	InjectTestDataClean = callback
}

type DiscoverTestSuit struct {
	cfg                 *TestConfig
	server              service.DiscoverServer
	originSvr           service.DiscoverServer
	ruleSvr             goverrule.GoverRuleServer
	originRuleSvr       goverrule.GoverRuleServer
	healthCheckServer   *healthcheck.Server
	cacheMgr            *cache.CacheManager
	userMgn             authapi.UserServer
	strategyMgn         authapi.StrategyServer
	namespaceSvr        ns.NamespaceOperateServer
	cancelFlag          bool
	updateCacheInterval time.Duration
	DefaultCtx          context.Context
	cancel              context.CancelFunc
	Storage             storeapi.Store
	bc                  *batch.Controller
	cleanDataOp         TestDataClean
	caller              func() storeapi.Store
}

func (d *DiscoverTestSuit) InjectSuit(*DiscoverTestSuit) {

}

func (d *DiscoverTestSuit) GetBootstrapConfig() *TestConfig {
	return d.cfg
}

func (d *DiscoverTestSuit) CacheMgr() *cache.CacheManager {
	return d.cacheMgr
}

func (d *DiscoverTestSuit) GetTestDataClean() TestDataClean {
	return d.cleanDataOp
}

func (d *DiscoverTestSuit) DiscoverServer() service.DiscoverServer {
	return d.server
}

func (d *DiscoverTestSuit) OriginDiscoverServer() service.DiscoverServer {
	return d.originSvr
}

func (d *DiscoverTestSuit) GoverRuleServer() goverrule.GoverRuleServer {
	return d.ruleSvr
}

func (d *DiscoverTestSuit) OriginGoverRuleServer() goverrule.GoverRuleServer {
	return d.originRuleSvr
}

func (d *DiscoverTestSuit) HealthCheckServer() *healthcheck.Server {
	return d.healthCheckServer
}

func (d *DiscoverTestSuit) NamespaceServer() ns.NamespaceOperateServer {
	return d.namespaceSvr
}

func (d *DiscoverTestSuit) UserServer() authapi.UserServer {
	return d.userMgn
}

func (d *DiscoverTestSuit) StrategyServer() authapi.StrategyServer {
	return d.strategyMgn
}

func (d *DiscoverTestSuit) UpdateCacheInterval() time.Duration {
	return d.updateCacheInterval
}

func (d *DiscoverTestSuit) BatchController() *batch.Controller {
	return d.bc
}

// 加载配置
func (d *DiscoverTestSuit) loadConfig() error {

	d.cfg = new(TestConfig)

	confFileName := testdata.Path("service_test.yaml")
	if os.Getenv("STORE_MODE") == "sqldb" {
		fmt.Printf("run store mode : sqldb\n")
		confFileName = testdata.Path("service_test_sqldb.yaml")
		d.DefaultCtx = context.WithValue(d.DefaultCtx, types.ContextAuthTokenKey,
			"nu/0WRA4EqSR1FagrjRj0fZwPXuGlMpX+zCuWu4uMqy8xr1vRjisSbA25aAC3mtU8MeeRsKhQiDAynUR09I=")
	}
	// 如果有额外定制的配置文件，优先采用
	if val := os.Getenv("POLARIS_TEST_BOOTSTRAP_FILE"); val != "" {
		confFileName = val
	}
	buf, err := os.ReadFile(confFileName)
	if nil != err {
		return fmt.Errorf("read file %s error", confFileName)
	}

	if err = parseYamlContent(string(buf), d.cfg); err != nil {
		fmt.Printf("[ERROR] %v\n", err)
		return err
	}
	d.cfg.Naming.Interceptors = service.GetChainOrder()
	return err
}

func parseYamlContent(content string, conf *TestConfig) error {
	if err := yaml.Unmarshal([]byte(replaceEnv(content)), conf); nil != err {
		return fmt.Errorf("parse yaml %s error:%w", content, err)
	}
	return nil
}

// replaceEnv replace holder by env list
func replaceEnv(configContent string) string {
	return os.ExpandEnv(configContent)
}

// 判断一个resp是否执行成功
func RespSuccess(resp api.ResponseMessage) bool {
	ret := api.CalcCode(resp) == 200
	return ret
}

type options func(cfg *TestConfig)

func (d *DiscoverTestSuit) Initialize(opts ...options) error {
	return d.initialize(opts...)
}

func (d *DiscoverTestSuit) ReplaceStore(caller func() storeapi.Store) {
	d.caller = caller
}

// 内部初始化函数
func (d *DiscoverTestSuit) initialize(opts ...options) error {
	// 初始化defaultCtx
	d.DefaultCtx = context.WithValue(context.Background(), types.ContextRequestId, "test-1")
	d.DefaultCtx = context.WithValue(d.DefaultCtx, types.ContextAuthTokenKey,
		"nu/0WRA4EqSR1FagrjRj0fZwPXuGlMpX+zCuWu4uMqy8xr1vRjisSbA25aAC3mtU8MeeRsKhQiDAynUR09I=")

	if err := d.loadConfig(); err != nil {
		panic(err)
	}

	for i := range opts {
		opts[i](d.cfg)
	}

	d.cleanDataOp = d
	if InjectTestDataClean != nil {
		d.cleanDataOp = InjectTestDataClean()
	}
	// 注入测试套件相关数据信息
	d.cleanDataOp.InjectSuit(d)

	_ = commonlog.Configure(d.cfg.Bootstrap.Logger)

	metrics.InitMetrics()
	eventhub.InitEventHub()

	// 初始化存储层
	if d.caller != nil {
		d.Storage = d.caller()
	} else {
		storeapi.SetStoreConfig(&d.cfg.Store)
		s, _ := storeplugin.TestGetStore()
		d.Storage = s
	}

	apis.SetPluginConfig(&d.cfg.Plugin)

	ctx, cancel := context.WithCancel(context.Background())

	d.cancel = cancel

	// 初始化缓存模块
	cacheMgn, err := cache.TestCacheInitialize(ctx, &d.cfg.Cache, d.Storage)
	if err != nil {
		panic(err)
	}
	d.cacheMgr = cacheMgn
	_ = d.cacheMgr.OpenResourceCache(cacheapi.ConfigEntry{
		Name: cacheapi.GrayName,
	})

	if !d.cfg.DisableAuth {
		// 初始化鉴权层
		userMgn, strategyMgn, err := auth.TestInitialize(ctx, &d.cfg.Auth, d.Storage, cacheMgn)
		if err != nil {
			panic(err)
		}
		d.userMgn = userMgn
		d.strategyMgn = strategyMgn
	}

	// 初始化命名空间模块
	namespaceSvr, err := TestNamespaceInitialize(ctx, &d.cfg.Namespace, d.Storage, cacheMgn, d.userMgn, d.strategyMgn)
	if err != nil {
		panic(err)
	}
	d.namespaceSvr = namespaceSvr

	// 批量控制器
	namingBatchConfig, err := batch.ParseBatchConfig(d.cfg.Naming.Batch)
	if err != nil {
		panic(err)
	}
	healthBatchConfig, err := batch.ParseBatchConfig(d.cfg.HealthChecks.Batch)
	if err != nil {
		panic(err)
	}

	batchConfig := &batch.Config{
		Register:         namingBatchConfig.Register,
		Deregister:       namingBatchConfig.Register,
		ClientRegister:   namingBatchConfig.ClientRegister,
		ClientDeregister: namingBatchConfig.ClientDeregister,
		Heartbeat:        healthBatchConfig.Heartbeat,
	}

	bc, err := batch.NewBatchCtrlWithConfig(ctx, d.Storage, cacheMgn, batchConfig)
	if err != nil {
		log.Errorf("new batch ctrl with config err: %s", err.Error())
		panic(err)
	}
	d.bc = bc

	if len(d.cfg.HealthChecks.LocalHost) == 0 {
		d.cfg.HealthChecks.LocalHost = utils.LocalHost // 补充healthCheck的配置
	}
	healthCheckServer, err := healthcheck.TestInitialize(ctx, &d.cfg.HealthChecks, bc, d.Storage)
	if err != nil {
		panic(err)
	}
	healthcheck.SetServer(healthCheckServer)
	d.healthCheckServer = healthCheckServer
	healthCheckServer.SetServiceCache(cacheMgn.Service())
	healthCheckServer.SetInstanceCache(cacheMgn.Instance())

	val, originVal, err := service.TestInitialize(ctx, &d.cfg.Naming, &d.cfg.Cache, d.cfg.ServiceCacheEntries,
		bc, cacheMgn, d.Storage, namespaceSvr, healthCheckServer, d.userMgn, d.strategyMgn)
	if err != nil {
		panic(err)
	}
	d.server = val
	d.originSvr = originVal

	// 多等待一会
	d.updateCacheInterval = d.cacheMgr.GetUpdateCacheInterval() + time.Millisecond*500
	if err := cache.TestRun(ctx, d.cacheMgr); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	return nil
}

func TestNamespaceInitialize(ctx context.Context, nsOpt *namespace.Config, storage storeapi.Store, cacheMgr *cache.CacheManager,
	userMgn authapi.UserServer, strategyMgn authapi.StrategyServer) (namespace.NamespaceOperateServer, error) {

	ctx = context.WithValue(ctx, authapi.ContextKeyUserSvr, userMgn)
	ctx = context.WithValue(ctx, authapi.ContextKeyPolicySvr, strategyMgn)

	_, proxySvr, err := namespace.InitServer(ctx, nsOpt, storage, cacheMgr)
	if err != nil {
		return nil, err
	}
	return proxySvr, nil
}

func (d *DiscoverTestSuit) Destroy() {
	d.cancel()
	d.healthCheckServer.Destroy()
	_ = d.cacheMgr.Close()
	_ = d.Storage.Destroy()
}

func (d *DiscoverTestSuit) CleanReportClient() {
	if d.Storage.Name() == sqldb.STORENAME {
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)
			defer rollbackDbTx(dbTx)

			if _, err := dbTx.Exec("delete from client"); err != nil {
				panic(err)
			}
			if _, err := dbTx.Exec("delete from client_stat"); err != nil {
				panic(err)
			}

			commitDbTx(dbTx)
		}()
	}
}

func rollbackDbTx(dbTx *sqldb.BaseTx) {
	if err := dbTx.Rollback(); err != nil {
		log.Errorf("fail to rollback db tx, err %v", err)
	}
}

func commitDbTx(dbTx *sqldb.BaseTx) {
	if err := dbTx.Commit(); err != nil {
		log.Errorf("fail to commit db tx, err %v", err)
	}
}

// 从数据库彻底删除命名空间
func (d *DiscoverTestSuit) CleanNamespace(name string) {
	if name == "" {
		panic("name is empty")
	}

	log.Infof("clean namespace: %s", name)

	if d.Storage.Name() == sqldb.STORENAME {
		str := "delete from namespace where name = ?"
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)
			defer rollbackDbTx(dbTx)

			if _, err := dbTx.Exec(str, name); err != nil {
				panic(err)
			}

			commitDbTx(dbTx)
		}()
	}
}

// 从数据库彻底删除全部服务
func (d *DiscoverTestSuit) CleanAllService() {

	if d.Storage.Name() == sqldb.STORENAME {
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)

			defer rollbackDbTx(dbTx)

			if _, err := dbTx.Exec("delete from service_metadata"); err != nil {
				rollbackDbTx(dbTx)
				panic(err)
			}

			if _, err := dbTx.Exec("delete from service"); err != nil {
				rollbackDbTx(dbTx)
				panic(err)
			}
			commitDbTx(dbTx)
		}()
	}
}

// 从数据库彻底删除服务
func (d *DiscoverTestSuit) CleanService(name, namespace string) {

	if d.Storage.Name() == sqldb.STORENAME {
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)

			defer rollbackDbTx(dbTx)

			str := "select id from service where name = ? and namespace = ?"
			var id string
			err = dbTx.QueryRow(str, name, namespace).Scan(&id)
			switch {
			case err == sql.ErrNoRows:
				return
			case err != nil:
				panic(err)
			}

			if _, err := dbTx.Exec("delete from service_metadata where id = ?", id); err != nil {
				rollbackDbTx(dbTx)
				panic(err)
			}

			if _, err := dbTx.Exec("delete from service where id = ?", id); err != nil {
				rollbackDbTx(dbTx)
				panic(err)
			}
			commitDbTx(dbTx)
		}()
	}
}

// clean services
func (d *DiscoverTestSuit) CleanServices(services []*apiservice.Service) {

	if d.Storage.Name() == sqldb.STORENAME {
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)

			defer rollbackDbTx(dbTx)

			str := "delete from service where name = ? and namespace = ?"
			for _, service := range services {
				if _, err := dbTx.Exec(
					str, service.GetName(), service.GetNamespace()); err != nil {
					panic(err)
				}
			}
			commitDbTx(dbTx)
		}()
	}

}

// 从数据库彻底删除实例
func (d *DiscoverTestSuit) CleanInstance(instanceID string) {
	if instanceID == "" {
		panic("instanceID is empty")
	}
	log.Infof("clean instance: %s", instanceID)

	if d.Storage.Name() == sqldb.STORENAME {
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)

			defer rollbackDbTx(dbTx)

			str := "delete from instance where id = ?"
			if _, err := dbTx.Exec(str, instanceID); err != nil {
				rollbackDbTx(dbTx)
				panic(err)
			}

			commitDbTx(dbTx)
		}()
	}
}

// 彻底删除一个路由配置
func (d *DiscoverTestSuit) CleanCommonRoutingConfig(service string, namespace string) {

	if d.Storage.Name() == sqldb.STORENAME {
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)

			defer rollbackDbTx(dbTx)

			str := "delete from routing_config where id in (select id from service where name = ? and namespace = ?)"
			// fmt.Printf("%s %s %s\n", str, service, namespace)
			if _, err := dbTx.Exec(str, service, namespace); err != nil {
				panic(err)
			}
			str = "delete from governance_rule where rule_type = ?"
			// fmt.Printf("%s %s %s\n", str, service, namespace)
			if _, err := dbTx.Exec(str, "route"); err != nil {
				panic(err)
			}

			commitDbTx(dbTx)
		}()
	}
}

func (d *DiscoverTestSuit) TruncateCommonRoutingConfigV2() {
	if d.Storage.Name() == sqldb.STORENAME {
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)
			defer rollbackDbTx(dbTx)

			str := "delete from governance_rule where rule_type = ?"
			if _, err := dbTx.Exec(str, "route"); err != nil {
				panic(err)
			}

			commitDbTx(dbTx)
		}()
	}
}

// 彻底删除一个路由配置
func (d *DiscoverTestSuit) CleanCommonRoutingConfigV2(rules []*apitraffic.RouteRule) {

	if d.Storage.Name() == sqldb.STORENAME {
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)
			defer rollbackDbTx(dbTx)

			str := "delete from governance_rule where rule_type = ? and id in (%s)"

			places := []string{}
			args := []interface{}{"route"}
			for i := range rules {
				places = append(places, "?")
				args = append(args, rules[i].Id)
			}

			str = fmt.Sprintf(str, strings.Join(places, ","))
			// fmt.Printf("%s %s %s\n", str, service, namespace)
			if _, err := dbTx.Exec(str, args...); err != nil {
				panic(err)
			}

			commitDbTx(dbTx)
		}()
	}
}

// 彻底删除限流规则
func (d *DiscoverTestSuit) CleanRateLimit(id string) {

	if d.Storage.Name() == sqldb.STORENAME {
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)

			defer rollbackDbTx(dbTx)

			str := `delete from governance_rule where rule_type = ? and id = ?`
			if _, err := dbTx.Exec(str, "ratelimit", id); err != nil {
				panic(err)
			}

			commitDbTx(dbTx)
		}()
	}
}

func buildCircuitBreakerKey(id, version string) string {
	return fmt.Sprintf("%s_%s", id, version)
}

// 彻底删除熔断规则
func (d *DiscoverTestSuit) CleanCircuitBreaker(id, version string) {
	log.Infof("clean circuit breaker, id: %s, version: %s", id, version)

	if d.Storage.Name() == sqldb.STORENAME {
		func() {
			tx, err := d.Storage.StartTx()
			if err != nil {
				panic(err)
			}

			dbTx := tx.GetDelegateTx().(*sqldb.BaseTx)

			defer rollbackDbTx(dbTx)

			str := `delete from governance_rule where rule_type = ? and id = ?`
			if _, err := dbTx.Exec(str, "circuitbreaker", id); err != nil {
				panic(err)
			}

			commitDbTx(dbTx)
		}()
	}
}

// 彻底删除熔断规则发布记录
func (d *DiscoverTestSuit) CleanServiceContract() error {
	if d.Storage.Name() == sqldb.STORENAME {
		proxyTx, err := d.Storage.StartTx()
		if err != nil {
			return err
		}

		tx := proxyTx.GetDelegateTx().(*sqldb.BaseTx)

		defer tx.Rollback()
		_, err = tx.Exec("delete from service_contract")
		if err != nil {
			return err
		}
		_, err = tx.Exec("delete from service_contract_detail")
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	return nil
}

// CleanCluster 清理集群测试数据
func (d *DiscoverTestSuit) CleanCluster(name string) {
	log.Infof("clean cluster: %s", name)
	// TODO: 清理集群相关数据
}

// CleanTopologyData 清理拓扑测试数据
func (d *DiscoverTestSuit) CleanTopologyData(serviceID string) {
	log.Infof("clean topology data for service: %s", serviceID)
	// TODO: 清理拓扑相关数据
}

// CleanServiceSubscriber 清理服务订阅者测试数据
func (d *DiscoverTestSuit) CleanServiceSubscriber(serviceID string, subscriberServiceID string) {
	log.Infof("clean subscriber: %s for service: %s", subscriberServiceID, serviceID)
	// TODO: 清理服务订阅者相关数据
}

// CleanConfigListener 清理配置监听者测试数据
func (d *DiscoverTestSuit) CleanConfigListener(fileID string, listenerID string) {
	log.Infof("clean config listener: %s for file: %s", listenerID, fileID)
	// TODO: 清理配置监听者相关数据
}

func (d *DiscoverTestSuit) ClearTestDataWhenUseRDS() error {
	if d.Storage.Name() == sqldb.STORENAME {
		proxyTx, err := d.Storage.StartTx()
		if err != nil {
			return err
		}

		tx := proxyTx.GetDelegateTx().(*sqldb.BaseTx)

		defer tx.Rollback()
		_, err = tx.Exec("delete from namespace where name = ? ", testNamespace)
		if err != nil {
			return err
		}
		_, err = tx.Exec("delete from service_contract")
		if err != nil {
			return err
		}
		_, err = tx.Exec("delete from service_contract_detail")
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	return nil
}
