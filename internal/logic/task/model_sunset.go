package task

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	do "github.com/qianfree/team-api/internal/model/do"
)

// CheckModelSunset 检查并处理已过 sunset 日期的弃用模型
func CheckModelSunset(ctx context.Context) error {
	g.Log().Debug(ctx, "[Cron] starting model sunset check")

	// 查询所有 sunset 日期已到的弃用模型
	type sunsetModel struct {
		ID               int64  `json:"id"`
		ModelId          string `json:"model_id"`
		SunsetDate       string `json:"sunset_date"`
		ReplacementModel string `json:"replacement_model"`
	}

	var models []sunsetModel
	err := g.DB().Ctx(ctx).
		Model("mdl_models").
		Fields("id, model_id, sunset_date, replacement_model").
		Where("status", "deprecated").
		Where("sunset_date IS NOT NULL").
		Where("sunset_date <= CURRENT_DATE").
		Scan(&models)
	if err != nil {
		g.Log().Errorf(ctx, "[Cron] query sunset models failed: %v", err)
		return err
	}

	if len(models) == 0 {
		g.Log().Debug(ctx, "[Cron] no models to sunset")
		return nil
	}

	sunsetCount := 0
	for _, m := range models {
		// 设置为 offline 并清除弃用字段
		_, err := dao.MdlModels.Ctx(ctx).
			Where("id", m.ID).
			Data(do.MdlModels{
				Status:           "offline",
				DeprecatedAt:     nil,
				SunsetDate:       nil,
				ReplacementModel: "",
			}).Update()
		if err != nil {
			g.Log().Errorf(ctx, "[Cron] sunset model %s (%d) failed: %v", m.ModelId, m.ID, err)
			continue
		}

		// 清除模型缓存
		relay.NewDataProvider().InvalidateModelCache(m.ModelId)

		sunsetCount++

		// 发送下线通知
		go func(name, sunsetDate, replacement string) {
			bgCtx := context.Background()
			engine := common.NewNotificationEngine()
			if err := engine.SendToAllTenants(bgCtx, "model_sunset", g.Map{
				"model_name":        name,
				"sunset_date":       sunsetDate,
				"replacement_model": replacement,
			}, ""); err != nil {
				g.Log().Errorf(bgCtx, "[Cron] send sunset notification for %s failed: %v", name, err)
			}
		}(m.ModelId, m.SunsetDate, m.ReplacementModel)
	}

	g.Log().Debugf(ctx, "[Cron] model sunset check completed: %d models transitioned to offline", sunsetCount)
	return nil
}
