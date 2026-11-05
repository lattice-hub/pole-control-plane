package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bootstrap "github.com/pole-io/pole-server/pkg/console/config"
	"github.com/pole-io/pole-server/pkg/console/internal/common/api"
	"github.com/pole-io/pole-server/pkg/console/internal/common/model"
	"github.com/pole-io/pole-server/pkg/console/internal/observabilityquery"
)

func DescribeObservabilityPlatformOverview(config *bootstrap.Config) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		provider := observabilityquery.NewProvider(config.ObservabilityQuery)
		overview, err := provider.PlatformOverview(ctx.Request.Context(), observabilityquery.PlatformOverviewQuery{
			StartTime: ctx.Query("start_time"),
			EndTime:   ctx.Query("end_time"),
			Step:      ctx.Query("step"),
			Category:  ctx.Query("category"),
			API:       ctx.Query("api"),
		})
		if err != nil {
			response := model.NewResponse(int32(api.ExecuteException))
			response.Data = err.Error()
			ctx.JSON(http.StatusInternalServerError, response)
			return
		}

		response := model.NewResponse(int32(api.ExecuteSuccess))
		response.Data = overview
		ctx.JSON(http.StatusOK, response)
	}
}

func DescribeObservabilityPlatformTraces(config *bootstrap.Config) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		provider := observabilityquery.NewProvider(config.ObservabilityQuery)
		traces, err := provider.PlatformTraces(ctx.Request.Context(), observabilityquery.TraceQuery{
			StartTime:     ctx.Query("start_time"),
			EndTime:       ctx.Query("end_time"),
			Service:       ctx.Query("service"),
			API:           ctx.Query("api"),
			Limit:         ctx.Query("limit"),
			ErrorOnly:     ctx.Query("error_only"),
			MinDurationMS: ctx.Query("min_duration_ms"),
		})
		if err != nil {
			response := model.NewResponse(int32(api.ExecuteException))
			response.Data = err.Error()
			ctx.JSON(http.StatusInternalServerError, response)
			return
		}
		response := model.NewResponse(int32(api.ExecuteSuccess))
		response.Data = traces
		ctx.JSON(http.StatusOK, response)
	}
}

func DescribeObservabilityTrace(config *bootstrap.Config) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		provider := observabilityquery.NewProvider(config.ObservabilityQuery)
		trace, err := provider.TraceDetail(ctx.Request.Context(), ctx.Param("trace_id"))
		if err != nil {
			response := model.NewResponse(int32(api.ExecuteException))
			response.Data = err.Error()
			ctx.JSON(http.StatusInternalServerError, response)
			return
		}
		response := model.NewResponse(int32(api.ExecuteSuccess))
		response.Data = trace
		ctx.JSON(http.StatusOK, response)
	}
}

func DescribeObservabilityEvents(config *bootstrap.Config) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		provider := observabilityquery.NewProvider(config.ObservabilityQuery)
		events, err := provider.ServiceEvents(ctx.Request.Context(), observabilityquery.EventLogQuery{
			Namespace: ctx.Query("namespace"),
			Service:   ctx.Query("service"),
			Resource:  ctx.Query("resource"),
			EventType: ctx.Query("event_type"),
			StartTime: ctx.Query("start_time"),
			EndTime:   ctx.Query("end_time"),
			Limit:     ctx.Query("limit"),
			Cursor:    ctx.Query("cursor"),
			Direction: ctx.Query("direction"),
		})
		if err != nil {
			response := model.NewResponse(int32(api.ExecuteException))
			response.Data = err.Error()
			ctx.JSON(http.StatusInternalServerError, response)
			return
		}
		ctx.JSON(http.StatusOK, events)
	}
}

func DescribeObservabilityOperations(config *bootstrap.Config) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		provider := observabilityquery.NewProvider(config.ObservabilityQuery)
		operations, err := provider.Operations(ctx.Request.Context(), observabilityquery.OperationLogQuery{
			Namespace:     ctx.Query("namespace"),
			ResourceType:  ctx.Query("resource_type"),
			ResourceName:  ctx.Query("resource_name"),
			OperationType: ctx.Query("operation_type"),
			Operator:      ctx.Query("operator"),
			StartTime:     ctx.Query("start_time"),
			EndTime:       ctx.Query("end_time"),
			Limit:         ctx.Query("limit"),
			Cursor:        ctx.Query("cursor"),
			Direction:     ctx.Query("direction"),
		})
		if err != nil {
			response := model.NewResponse(int32(api.ExecuteException))
			response.Data = err.Error()
			ctx.JSON(http.StatusInternalServerError, response)
			return
		}
		ctx.JSON(http.StatusOK, operations)
	}
}
