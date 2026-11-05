package router

import (
	"github.com/gin-gonic/gin"

	bootstrap "github.com/pole-io/pole-server/pkg/console/config"
	"github.com/pole-io/pole-server/pkg/console/internal/handlers"
)

func LimiterRouter(webSvr *gin.Engine, config *bootstrap.Config) {
	proxy := handlers.ReverseProxyForServer(&config.LimiterServer, config)
	v1 := webSvr.Group("/limiter/v1")
	v1.GET("/quota-leases", func(c *gin.Context) {
		c.Request.URL.Path = "/maintain/quota-leases"
		proxy(c)
	})
}
