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

package job

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/store"
	"github.com/pole-io/pole-server/pkg/cache"
	commonlog "github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/service"
)

// jobLog 惰性获取 job 专用 scope（OnceValue：首次调用时解析并缓存，避免包初始化时日志配置尚未加载）
var jobLog = sync.OnceValue(func() *commonlog.Scope {
	return commonlog.GetScopeOrDefaultByName(commonlog.JobLoggerName)
})

// MaintainJobs
type MaintainJobs struct {
	jobs        map[string]maintainJob
	startedJobs map[string]maintainJob
	storage     store.Store
	cancel      context.CancelFunc
}

// NewMaintainJobs
func NewMaintainJobs(namingServer service.DiscoverServer, cacheMgn *cache.CacheManager,
	storage store.Store) *MaintainJobs {
	return &MaintainJobs{
		jobs: map[string]maintainJob{
			"DeleteUnHealthyInstance": &deleteUnHealthyInstanceJob{
				namingServer: namingServer, storage: storage},
			"DeleteEmptyService": &deleteEmptyServiceJob{
				namingServer: namingServer, cacheMgn: cacheMgn, storage: storage},
			"CleanDeletedResources": &cleanDeletedResourceJob{
				storage: storage},
		},
		startedJobs: map[string]maintainJob{},
		storage:     storage,
	}
}

// StartMaintainJobs
func (mj *MaintainJobs) StartMaintianJobs(configs []JobConfig) error {
	if err := mj.storage.StartLeaderElection(store.ElectionKeyMaintainJob); err != nil {
		jobLog().Error("[Maintain][Job] start leader election failed", zap.Error(err))
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	mj.cancel = cancel
	for _, cfg := range configs {
		if !cfg.Enable {
			jobLog().Info("[Maintain][Job] job not enable", zap.String("name", cfg.Name))
			continue
		}
		jobName := parseJobName(cfg.Name)
		job, ok := mj.findAdminJob(jobName)
		if !ok {
			jobLog().Warn("[Maintain][Job] job not exist", zap.String("job", jobName))
			continue
		}
		if _, ok := mj.startedJobs[jobName]; ok {
			return fmt.Errorf("[Maintain][Job] job (%s) duplicated", jobName)
		}
		if err := job.init(cfg.Option); err != nil {
			jobLog().Error("[Maintain][Job] job fail to init", zap.String("job", jobName), zap.Error(err))
			return fmt.Errorf("[Maintain][Job] job (%s) fail to init", jobName)
		}
		runAdminJob(ctx, jobName, job.interval(), job, mj.storage)
		mj.startedJobs[jobName] = job
	}
	return nil
}

func parseJobName(name string) string {
	// 兼容老配置
	if name == "DeleteEmptyAutoCreatedService" {
		name = "DeleteEmptyService"
	}
	return name
}

func (mj *MaintainJobs) findAdminJob(name string) (maintainJob, bool) {
	job, ok := mj.jobs[name]
	if !ok {
		return nil, false
	}

	return job, true
}

// StopMaintainJobs
func (mj *MaintainJobs) StopMaintainJobs() {
	if mj.cancel != nil {
		mj.cancel()
	}
	mj.startedJobs = map[string]maintainJob{}
}

func runAdminJob(ctx context.Context, name string, interval time.Duration, job maintainJob, storage store.Store) {
	safeExec := func() {
		if !storage.IsLeader(store.ElectionKeyMaintainJob) {
			jobLog().Info("[Maintain][Job] I am follower", zap.String("job", name))
			job.clear()
			return
		}
		jobLog().Info("[Maintain][Job] I am leader, job start", zap.String("job", name))
		job.execute()
		jobLog().Info("[Maintain][Job] I am leader, job end", zap.String("job", name))
	}

	ticker := time.NewTicker(interval)
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				safeExec()
			}
		}
	}(ctx)
}

type maintainJob interface {
	init(cfg map[string]interface{}) error
	execute()
	clear()
	interval() time.Duration
}

func getMasterAccountToken(storage store.Store) (string, error) {
	user, err := storage.GetMainUser()
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", fmt.Errorf("main user not found")
	}
	return user.Token, nil
}

func buildContext(storage store.Store) (context.Context, error) {
	token, err := getMasterAccountToken(storage)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.ContextAuthTokenKey, token)
	ctx = context.WithValue(ctx, types.ContextOperator, "maintain-job")
	return ctx, nil
}
