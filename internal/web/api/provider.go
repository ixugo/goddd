package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/ixugo/goddd/domain/uniqueid"
	"github.com/ixugo/goddd/domain/uniqueid/store/uniqueiddb"
	"github.com/ixugo/goddd/domain/version/versionapi"
	"github.com/ixugo/goddd/internal/conf"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/web"
	"gorm.io/gorm"
)

var (
	ProviderVersionSet = wire.NewSet(versionapi.NewVersionCore)
	ProviderSet        = wire.NewSet(
		wire.Struct(new(Usecase), "*"),
		NewHTTPHandler,
		versionapi.New,
	)
)

type Usecase struct {
	Conf    *conf.Bootstrap
	DB      *gorm.DB
	Version versionapi.API
}

// NewHTTPHandler 生成Gin框架路由内容
func NewHTTPHandler(uc *Usecase) http.Handler {
	cfg := uc.Conf
	// 如果不处于调试模式，将 Gin 设置为发布模式
	if !uc.Conf.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	g := gin.New()
	// 处理未找到路由的情况，返回 JSON 格式的 404 错误信息
	g.NoRoute(func(c *gin.Context) {
		c.JSON(404, "来到了无人的荒漠")
	})
	// 如果启用了 Pprof，设置 Pprof 监控
	if cfg.Server.HTTP.PProf.Enabled {
		web.SetupPProf(g, &cfg.Server.HTTP.PProf.AccessIps)
	}

	setupRouter(g, uc)
	// 在确认所有表迁移完成后，再更新版本记录
	// 防止更新中断的情况，后续启动中无法更新版本号
	uc.Version.RecordVersion()
	return g
}

// NewUniqueID 生成唯一 id
func NewUniqueID(db *gorm.DB) uniqueid.Core {
	store := uniqueiddb.NewDB(db).AutoMigrate(orm.GetEnabledAutoMigrate())
	return uniqueid.NewCore(store, 6)
}
