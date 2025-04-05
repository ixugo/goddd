package api

import (
	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/internal/core/version"
	"github.com/ixugo/goddd/pkg/web"
)

type VersionAPI struct {
	versionCore version.Core
}

func NewVersionAPI(ver version.Core) VersionAPI {
	return VersionAPI{versionCore: ver}
}

func registerVersion(r gin.IRouter, verAPI VersionAPI, handler ...gin.HandlerFunc) {
	{
		group := r.Group("/version", handler...)
		group.GET("", web.WarpH(verAPI.getVersion))
	}
}

func (v VersionAPI) getVersion(_ *gin.Context, _ *struct{}) (any, error) {
	return gin.H{"version": dbVersion, "remark": dbRemark}, nil
}
