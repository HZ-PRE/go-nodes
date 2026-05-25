package controllers

import (
	"fmt"
	"nodes/models"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (c *ServerControllers) CreateServerDicts(ctx *gin.Context) {
	var req models.ServerDict
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.respond(ctx, nil, err)
		return
	}
	c.respond(ctx, "created", c.svc.PostServerDicts(req))
}

func (c *ServerControllers) GetServerDictsByType(ctx *gin.Context) {
	t, err := strconv.ParseUint(ctx.Param("t"), 10, 32)
	if err != nil {
		c.respond(ctx, nil, err)
		return
	}
	ret, err := c.svc.GetServerDictsByType(uint8(t))
	c.respond(ctx, ret, err)
}
func (c *ServerControllers) ServerDictsDel(ctx *gin.Context) {
	err := c.svc.ServerDictsDel(ctx.Param("id"))
	c.respond(ctx, "删除成功", err)
}
func (c *ServerControllers) CreateServersUrl(ctx *gin.Context) {
	var req models.ServerUrl
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.respond(ctx, nil, err)
		return
	}
	c.respond(ctx, "created", c.svc.PostServerUrl(req))
}

func (c *ServerControllers) GetServersUrl(ctx *gin.Context) {
	ret, err := c.svc.GetServerUrl()
	c.respond(ctx, ret, err)
}
func (c *ServerControllers) ServerUrlDel(ctx *gin.Context) {
	err := c.svc.ServerUrlDel(ctx.Param("id"))
	c.respond(ctx, "删除成功", err)
}
func (c *ServerControllers) CreateServersHost(ctx *gin.Context) {
	var req models.ServerHost
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.respond(ctx, nil, err)
		return
	}

	if req.Domain == "" {
		c.respond(ctx, nil, fmt.Errorf("请填写域名"))
		return
	}
	if 0 < req.Scope && 1 != req.Beian {
		c.respond(ctx, nil, fmt.Errorf("此域名未备案，请先备案"))
		return
	}
	if 1 == req.IsSelf && 0 == req.ID {
		if -1 == req.Scope {
			c.respond(ctx, nil, fmt.Errorf("必须选择加速类型"))
			return
		}
		if req.OriginDomain == "" {
			c.respond(ctx, nil, fmt.Errorf("请填写源域名"))
			return
		}
		c.respond(ctx, "created", c.svc.PostServerDomain(req))
		return
	}
	c.respond(ctx, "created", c.svc.PostServerHost(req))
}

func (c *ServerControllers) GetServersHost(ctx *gin.Context) {
	ret, err := c.svc.GetServerHost()
	c.respond(ctx, ret, err)
}
func (c *ServerControllers) PutServerHostByDomain(ctx *gin.Context) {
	var domain []string
	if err := ctx.ShouldBindJSON(&domain); err != nil {
		c.respond(ctx, nil, err)
		return
	}
	if len(domain) == 0 {
		c.respond(ctx, nil, fmt.Errorf("请填写域名列表"))
		return
	}
	err := c.svc.PutServerHostByDomain(domain)
	c.respond(ctx, "更新成功", err)
}
func (c *ServerControllers) GetServersHostByApp(ctx *gin.Context) {
	app := ctx.Param("app")
	ret, err := c.svc.GetServerHostByApp(app)
	c.respond(ctx, ret, err)
}
func (c *ServerControllers) ServerHostDel(ctx *gin.Context) {
	err := c.svc.ServerHostDel(ctx.Param("id"))
	c.respond(ctx, "删除成功", err)
}
func (c *ServerControllers) GetServerSupplierApiBySupplier(ctx *gin.Context) {
	supplier := ctx.Param("supplier")
	cdn, err := strconv.Atoi(ctx.Param("cdn"))
	if err != nil {
		c.respond(ctx, nil, err)
		return
	}
	ret, err := c.svc.GetServerSupplierApiBySupplier(supplier, cdn)
	c.respond(ctx, ret, err)
}
func (c *ServerControllers) GetServerSupplierApi(ctx *gin.Context) {
	ret, err := c.svc.GetServerSupplierApi()
	c.respond(ctx, ret, err)
}
func (c *ServerControllers) ServerSupplierApiDel(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		c.respond(ctx, nil, err)
		return
	}
	err = c.svc.ServerSupplierApiDel(id)
	c.respond(ctx, "隐藏成功", err)
}
func (c *ServerControllers) PostServerSupplierApi(ctx *gin.Context) {
	var api models.ServerSupplierApi
	if err := ctx.ShouldBindJSON(&api); err != nil {
		c.respond(ctx, nil, err)
		return
	}
	err := c.svc.PostServerSupplierApi(api)
	c.respond(ctx, "成功", err)
}
