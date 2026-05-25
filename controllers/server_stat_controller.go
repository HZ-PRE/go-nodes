package controllers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"nodes/models"
)

const timeLayout = "2006-01-02 15:04"

func parseTimeRange(ctx *gin.Context) (start, end time.Time) {
	now := time.Now()
	start = now.Add(-1 * time.Minute)
	end = now

	hasStart := false
	hasEnd := false

	if s := ctx.Query("start_time"); s != "" {
		if t, err := time.Parse(timeLayout, s); err == nil {
			start = t
			hasStart = true
		}
	}
	if s := ctx.Query("end_time"); s != "" {
		if t, err := time.Parse(timeLayout, s); err == nil {
			end = t
			hasEnd = true
		}
	}
	// only one provided: derive the other with 1-minute offset
	if hasStart && !hasEnd {
		end = start.Add(1 * time.Minute)
	}
	if hasEnd && !hasStart {
		start = end.Add(-1 * time.Minute)
	}
	return
}

func (c *ServerControllers) CreateServersNode(ctx *gin.Context) {
	var req models.ServerNode
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.respond(ctx, nil, err)
		return
	}
	c.respond(ctx, "created", c.svc.CreateServersNode(req))
}

func (c *ServerControllers) GetServerNodes(ctx *gin.Context) {
	data, err := c.svc.GetServerNodes()
	c.respond(ctx, data, err)
}
func (c *ServerControllers) GetServerNodesV1(ctx *gin.Context) {
	data, err := c.svc.GetServerNodesV1()
	c.respond(ctx, data, err)
}
func (c *ServerControllers) SubLinux(ctx *gin.Context) {
	id := ctx.Param("id")
	t, err := strconv.ParseUint(ctx.Param("t"), 10, 32)
	if err != nil {
		c.respond(ctx, nil, err)
		return
	}
	if t == 1 || t == 3 || t == 4 {
		ss, err := c.svc.GetServersByPort()
		if err != nil {
			c.respond(ctx, nil, fmt.Errorf("节点检测失效"))
			return
		}
		if len(ss) > 0 {
			var names []string
			for _, v := range ss {
				names = append(names, fmt.Sprintf("%s%s:%d", v.App, v.Name, v.Port))
			}
			str := strings.Join(names, ",")
			c.respond(ctx, nil, fmt.Errorf("节点端口有冲突：%s", str))
			return
		}
	}
	out := "started"
	if t == 1 {
		out, err = c.svc.XrayInitCore(id)
	} else if t == 2 {
		out, err = c.svc.NodeExporterInit(id)
	} else if t == 3 {
		out, err = c.svc.XrayCon(id)
	} else if t == 4 {
		app := ctx.Query("app")
		if app == "" {
			c.respond(ctx, nil, fmt.Errorf("app parameter is required"))
			return
		}
		out, err = c.svc.GostCon(id, app)
	} else if t == 5 {
		out, err = c.svc.LinuxCMD(id, "cat /tmp/XrayConInstall.log")
	} else if t == 6 {
		out, err = c.svc.LinuxCMD(id, "cat /tmp/NodeExporterInstall.log")
	} else if t == 7 {
		out, err = c.svc.LinuxCMD(id, "ip addr")
	} else if t == 8 {
		out, err = c.svc.LinuxCMD(id, "systemctl status XrayR")
	} else if t == 9 {
		out, err = c.svc.LinuxCMD(id, "ps -ef|grep gost")
	} else if t == 10 {
		out, err = c.svc.LinuxCMD(id, "ps -ef|grep node_exporter")
	} else if t == 11 {
		out, err = c.svc.LinuxCMD(id, "lscpu")
	} else if t == 12 {
		out, err = c.svc.LinuxCMD(id, "uptime")
	} else if t == 13 {
		out, err = c.svc.LinuxCMD(id, "free -h")
	} else if t == 14 {
		out, err = c.svc.LinuxCMD(id, "df -h")
	}
	c.respond(ctx, out, err)
}
func (c *ServerControllers) GetServerNodesById(ctx *gin.Context) {
	out, err := c.svc.GetServerNodesById(ctx.Param("id"))
	c.respond(ctx, out, err)
}
func (c *ServerControllers) InitNodeMonitor(ctx *gin.Context) {
	out, err := c.svc.InitNodeMonitor()
	c.respond(ctx, out, err)
}
func (c *ServerControllers) GetServersByApp(ctx *gin.Context) {
	out, err := c.svc.GetServersByApp(ctx.Param("app"))
	c.respond(ctx, out, err)
}
func (c *ServerControllers) GetServersByPort(ctx *gin.Context) {
	out, err := c.svc.GetServersByPort()
	c.respond(ctx, out, err)
}
func (c *ServerControllers) CreateServers(ctx *gin.Context) {
	var req []models.Server
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.respond(ctx, nil, err)
		return
	}
	app := ctx.Param("app")
	c.respond(ctx, "created", c.svc.CreateServers(req, app))
}

func (c *ServerControllers) CreateServerStat(ctx *gin.Context) {
	var req models.ServerStatRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.respond(ctx, nil, err)
		return
	}
	stats := req.ToServerStats()
	c.respond(ctx, "created", c.svc.CreateServerStat(stats))
}

func (c *ServerControllers) GetServerStats(ctx *gin.Context) {
	start, end := parseTimeRange(ctx)
	flow, _ := strconv.ParseUint(ctx.Query("flow"), 10, 32)
	t, _ := strconv.ParseUint(ctx.Query("t"), 10, 32)
	data, err := c.svc.GetServerStats(start, end, uint(flow), uint(t))
	c.respond(ctx, data, err)
}

func (c *ServerControllers) GetAppOnUser(ctx *gin.Context) {
	start, end := parseTimeRange(ctx)
	flow, _ := strconv.ParseUint(ctx.Query("flow"), 10, 32)
	data, err := c.svc.GetAppOnUser(start, end, uint(flow))
	c.respond(ctx, data, err)
}

func (c *ServerControllers) GetMaxOnUserByTime(ctx *gin.Context) {
	h, _ := strconv.Atoi(ctx.Param("h"))
	if h <= 0 {
		h = 24
	}
	data, err := c.svc.GetMaxOnUserByTime(h)
	c.respond(ctx, data, err)
}

func (c *ServerControllers) GetOnUserByTime(ctx *gin.Context) {
	h, _ := strconv.Atoi(ctx.DefaultQuery("h", "24"))
	app := ctx.Query("app")
	data, err := c.svc.GetOnUserByTime(h, app)
	c.respond(ctx, data, err)
}

func (c *ServerControllers) GetServerStatsInfo(ctx *gin.Context) {
	ip := ctx.Param("ip")
	start, end := parseTimeRange(ctx)
	flow, _ := strconv.ParseUint(ctx.Query("flow"), 10, 32)
	data, err := c.svc.GetServerStatsInfo(ip, start, end, uint(flow))
	c.respond(ctx, data, err)
}

func (c *ServerControllers) GetServerStatsByDay(ctx *gin.Context) {
	ip := ctx.Param("ip")
	data, err := c.svc.GetServerStatsByDay(ip)
	c.respond(ctx, data, err)
}

func (c *ServerControllers) GetServerStatsByAppDay(ctx *gin.Context) {
	app := ctx.Query("app")
	data, err := c.svc.GetServerStatsByAppDay(app)
	c.respond(ctx, data, err)
}

func (c *ServerControllers) GetNodeNelNet(ctx *gin.Context) {
	data, err := c.svc.GetNodeNelNet()
	c.respond(ctx, data, err)
}

func (c *ServerControllers) SelNodeNelNet(ctx *gin.Context) {
	data, err := c.svc.SelNodeNelNet(ctx.Param("ip"), ctx.Param("prot"))
	c.respond(ctx, data, err)
}

func (c *ServerControllers) CreateServerLog(ctx *gin.Context) {
	var req models.ServerLog
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.respond(ctx, nil, err)
		return
	}
	if app := ctx.Param("app"); app != "" {
		req.App = app
	}
	c.respond(ctx, "created", c.svc.CreateServerLog(req))
}

func (c *ServerControllers) GetServerLog(ctx *gin.Context) {
	data, err := c.svc.GetServerLog(ctx.Query("app"))
	c.respond(ctx, data, err)
}
