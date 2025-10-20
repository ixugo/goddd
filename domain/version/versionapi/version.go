package versionapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/domain/version"
	"github.com/ixugo/goddd/pkg/orm"
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
		group.GET("", web.WrapH(verAPI.getVersion))
	}
}

func (v API) getVersion(_ *gin.Context, _ *struct{}) (any, error) {
	return gin.H{"version": DBVersion, "remark": DBRemark}, nil
}

// RecordVersion 更新版本号，错误仅记录日志，不建议上层处理
func (v API) RecordVersion() {
	// 如果没有执行表迁移，则不需要更新版本号
	if !orm.GetEnabledAutoMigrate() {
		return
	}

	if err := v.versionCore.RecordVersion(DBVersion, DBRemark); err != nil {
		slog.Error("RecordVersion", "err", err)
	}
}
