package routes

import (
	"github.com/gin-gonic/gin"

	"nodes/config"
	"nodes/controllers"
	"nodes/middleware"
)

func SetupRouter(task *middleware.ServerTask, ctl *controllers.ServerControllers) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	cfg := config.AppConfig
	if cfg.Log.Level != "none" {
		r.Use(middleware.Logger())
	}

	r.Use(config.CORSMiddleware())

	api := r.Group("/api")
	api.POST("/open/createServerLog/:app", ctl.CreateServerLog)
	api.POST("/open/api/createServers/:app", ctl.CreateServers)
	api.GET("/open/api/serverStats", ctl.GetServerStats)
	api.POST("/open/api/serverStat", ctl.CreateServerStat)
	api.GET("/getOnUserByTime", ctl.GetOnUserByTime)
	api.GET("/getMaxOnUserByTime/:h", ctl.GetMaxOnUserByTime)
	api.GET("/getAppOnUser", ctl.GetAppOnUser)
	api.GET("/getServersNode", ctl.GetServerNodes)
	api.GET("/GetServerNodesV1", ctl.GetServerNodesV1)
	api.POST("/createServersNode", ctl.CreateServersNode)
	api.POST("/createServers/:app", ctl.CreateServers)
	api.POST("/serverStat", ctl.CreateServerStat)
	api.GET("/serverStats", ctl.GetServerStats)
	api.GET("/serverStatsInfo/:ip", ctl.GetServerStatsInfo)
	api.GET("/serverStat/:ip", ctl.GetServerStatsByDay)
	api.GET("/serverStatsByAppDay", ctl.GetServerStatsByAppDay)
	api.GET("/getNodeNelNet", ctl.GetNodeNelNet)
	api.GET("/getNodeNelNet/:ip/:prot", ctl.SelNodeNelNet)
	api.GET("/getServerLog", ctl.GetServerLog)
	api.GET("/getServerUrl", ctl.GetServersUrl)
	api.POST("/createServersUrl", ctl.CreateServersUrl)
	api.GET("/SubLinux/:t/:id", ctl.SubLinux)
	api.GET("/GetServersByPort", ctl.GetServersByPort)
	api.GET("/GetServerNodesById/:id", ctl.GetServerNodesById)
	api.GET("/GetServersByApp/:app", ctl.GetServersByApp)
	api.GET("/InitNodeMonitor", ctl.InitNodeMonitor)
	api.GET("/ServerUrlDel/:id", ctl.ServerUrlDel)
	api.POST("/CreateServerDicts", ctl.CreateServerDicts)
	api.GET("/GetServerDictsByType/:t", ctl.GetServerDictsByType)
	api.DELETE("/ServerDictsDel/:id", ctl.ServerDictsDel)
	api.GET("/getServerHost", ctl.GetServersHost)
	api.GET("/getServersHostByApp/:app", ctl.GetServersHostByApp)
	api.POST("/createServersHost", ctl.CreateServersHost)
	api.PUT("/putServerHostByDomain", ctl.PutServerHostByDomain)
	api.GET("/ServerHostDel/:id", ctl.ServerHostDel)
	api.GET("/getServerSupplierApiBySupplier/:supplier/:cdn", ctl.GetServerSupplierApiBySupplier)
	api.GET("/getServerSupplierApi", ctl.GetServerSupplierApi)
	api.DELETE("/serverSupplierApiDel/:id", ctl.ServerSupplierApiDel)
	api.POST("/postServerSupplierApi", ctl.PostServerSupplierApi)

	if task != nil {
		task.Start()
	}
	return r
}
