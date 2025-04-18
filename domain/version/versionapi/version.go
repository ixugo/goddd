package versionapi

import (
	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/domain/version"
	"github.com/ixugo/goddd/pkg/web"
)

type API struct {
	versionCore version.Core
}

func New(ver version.Core) API {
	return API{versionCore: ver}
}

func Register(r gin.IRouter, verAPI API, handler ...gin.HandlerFunc) {
	{
		group := r.Group("/version", handler...)
		group.GET("", web.WarpH(verAPI.getVersion))
	}
}

func (v API) getVersion(_ *gin.Context, _ *struct{}) (any, error) {
	return gin.H{"version": DBVersion, "remark": DBRemark}, nil
}
