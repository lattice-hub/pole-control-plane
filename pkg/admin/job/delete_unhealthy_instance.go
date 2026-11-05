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
	"fmt"
	"time"

	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"

	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"

	svctypes "github.com/pole-io/pole-server/apis/pkg/types/service"
	"github.com/pole-io/pole-server/apis/store"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/service"
)

type DeleteUnHealthyInstanceJobConfig struct {
	InstanceDeleteTimeout time.Duration `mapstructure:"instanceDeleteTimeout"`
	SweepInterval         time.Duration `mapstructure:"sweepInterval"`
}

type deleteUnHealthyInstanceJob struct {
	cfg          *DeleteUnHealthyInstanceJobConfig
	namingServer service.DiscoverServer
	storage      store.Store
}

func (job *deleteUnHealthyInstanceJob) init(raw map[string]interface{}) error {
	cfg := &DeleteUnHealthyInstanceJobConfig{
		InstanceDeleteTimeout: 60 * time.Minute,
		SweepInterval:         time.Minute,
	}
	decodeConfig := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
		Result:     cfg,
	}
	decoder, err := mapstructure.NewDecoder(decodeConfig)
	if err != nil {
		jobLog().Error("[Maintain][Job][DeleteUnHealthyInstance] new config decoder failed", zap.Error(err))
		return err
	}
	err = decoder.Decode(raw)
	if err != nil {
		jobLog().Error("[Maintain][Job][DeleteUnHealthyInstance] parse config failed", zap.Error(err))
		return err
	}
	job.cfg = cfg
	if job.cfg.InstanceDeleteTimeout <= 0 || job.cfg.SweepInterval <= 0 {
		return fmt.Errorf("instanceDeleteTimeout and sweepInterval must be positive")
	}
	return nil
}

func (job *deleteUnHealthyInstanceJob) interval() time.Duration {
	return job.cfg.SweepInterval
}

func (job *deleteUnHealthyInstanceJob) execute() {
	batchSize := uint32(100)
	var count int = 0
	for {
		instanceIds, err := job.storage.GetUnHealthyInstances(job.cfg.InstanceDeleteTimeout, batchSize)
		if err != nil {
			jobLog().Error("[Maintain][Job][DeleteUnHealthyInstance] get unhealthy instances failed", zap.Error(err))
			break
		}
		if len(instanceIds) == 0 {
			break
		}

		var req []*apiservice.Instance
		for _, id := range instanceIds {
			instance, getErr := job.storage.GetInstance(id)
			if getErr != nil {
				jobLog().Error("[Maintain][Job][DeleteUnHealthyInstance] load instance failed",
					zap.String("id", id), zap.Error(getErr))
				return
			}
			if instance == nil {
				continue
			}
			request := unhealthyInstanceDeleteRequest(instance)
			if request.GetNamespace() == "" || request.GetService() == "" {
				svc, serviceErr := job.storage.GetServiceByID(instance.ServiceID)
				if serviceErr != nil {
					jobLog().Error("[Maintain][Job][DeleteUnHealthyInstance] load service failed",
						zap.String("service_id", instance.ServiceID), zap.Error(serviceErr))
					return
				}
				if svc == nil {
					jobLog().Warn("[Maintain][Job][DeleteUnHealthyInstance] skip instance without service",
						zap.String("id", id), zap.String("service_id", instance.ServiceID))
					continue
				}
				request.Namespace = svc.Namespace
				request.Service = svc.Name
			}
			req = append(req, request)
		}
		if len(req) == 0 {
			break
		}

		ctx, err := buildContext(job.storage)
		if err != nil {
			jobLog().Error("[Maintain][Job][DeleteUnHealthyInstance] build context failed", zap.Error(err))
			return
		}
		resp := job.namingServer.DeleteInstances(ctx, req)
		if api.CalcCode(resp) == 200 {
			jobLog().Info("[Maintain][Job][DeleteUnHealthyInstance] delete instances",
				zap.Int("count", len(req)), zap.Any("ids", instanceIds))
		} else {
			jobLog().Error("[Maintain][Job][DeleteUnHealthyInstance] delete instances failed",
				zap.Any("ids", instanceIds), zap.Uint32("code", resp.Code), zap.String("info", resp.Info))
			break
		}
		count += len(req)
	}

	jobLog().Info("[Maintain][Job][DeleteUnHealthyInstance] delete unhealthy instance done", zap.Int("count", count))

}

func unhealthyInstanceDeleteRequest(instance *svctypes.Instance) *apiservice.Instance {
	return &apiservice.Instance{
		Id:        instance.ID(),
		Namespace: instance.Namespace(),
		Service:   instance.Service(),
		Host:      instance.Host(),
		Port:      instance.Port(),
	}
}

func (job *deleteUnHealthyInstanceJob) clear() {
}
