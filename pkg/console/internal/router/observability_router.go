package router

import (
	"github.com/gin-gonic/gin"
	bootstrap "github.com/pole-io/pole-server/pkg/console/config"
	"github.com/pole-io/pole-server/pkg/console/internal/handlers"
)

func ObservabilityRouter(webSvr *gin.Engine, config *bootstrap.Config) {
	v1 := webSvr.Group("/observability/v1")
	v1.GET("/platform/overview", handlers.DescribeObservabilityPlatformOverview(config))
	v1.GET("/platform/traces", handlers.DescribeObservabilityPlatformTraces(config))
	v1.GET("/traces/:trace_id", handlers.DescribeObservabilityTrace(config))
	v1.GET("/events", handlers.DescribeObservabilityEvents(config))
	v1.GET("/operations", handlers.DescribeObservabilityOperations(config))
}
