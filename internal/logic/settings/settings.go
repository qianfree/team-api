package settings

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/service"

	v1 "github.com/qianfree/team-api/api/settings/v1"
)

type sSettings struct{}

func New() *sSettings {
	return &sSettings{}
}

func init() {
	service.RegisterSettings(New())
}

func (s *sSettings) PublicSettingsGet(ctx context.Context, _ *v1.PublicSettingsGetReq) (*v1.PublicSettingsGetRes, error) {
	options, err := common.Config().GetPublicOptions(ctx)
	if err != nil {
		return nil, err
	}

	settings := make(map[string]any, len(options))
	for _, opt := range options {
		if def := common.GetSettingDef(opt.Key); def != nil {
			settings[opt.Key] = common.TypedValue(def.Type, opt.Value, def.Default)
		}
	}

	// Include public settings that may not yet exist in DB (use registry defaults)
	for _, def := range common.Registry {
		if def.IsPublic {
			if _, exists := settings[def.Key]; !exists {
				settings[def.Key] = common.TypedValue(def.Type, "", def.Default)
			}
		}
	}

	// Inject demo mode from config file (not database — avoids the self-lock bug)
	settings["demo_mode"] = g.Cfg().MustGet(ctx, "demo.enabled").Bool()
	demoMsg := g.Cfg().MustGet(ctx, "demo.message").String()
	if demoMsg == "" {
		demoMsg = "演示环境，数据不可修改"
	}
	settings["demo_message"] = demoMsg

	return &v1.PublicSettingsGetRes{Settings: settings}, nil
}

func (s *sSettings) PublicAnnouncements(ctx context.Context, req *v1.PublicAnnouncementsReq) (*v1.PublicAnnouncementsRes, error) {
	m := dao.NtfAnnouncements.Ctx(ctx).
		Where("status", "published").
		Where("(effective_at IS NULL OR effective_at <= NOW())").
		Where("(expires_at IS NULL OR expires_at > NOW())")

	if req.Position != "" {
		m = m.Where("display_position IN (?)", []string{req.Position, "both"})
	}

	var items []struct {
		Id              int64  `json:"id"`
		Title           string `json:"title"`
		Type            string `json:"type"`
		Content         string `json:"content"`
		IsPinned        int    `json:"is_pinned"`
		DisplayPosition string `json:"display_position"`
		CreatedAt       string `json:"created_at"`
	}
	err := m.OrderDesc("is_pinned").OrderDesc("created_at").
		Limit(20).
		Scan(&items)
	if err != nil {
		return nil, err
	}

	list := make([]v1.PublicAnnouncementItem, len(items))
	for i, item := range items {
		list[i] = v1.PublicAnnouncementItem{
			Id:              item.Id,
			Title:           item.Title,
			Type:            item.Type,
			Content:         item.Content,
			IsPinned:        item.IsPinned,
			DisplayPosition: item.DisplayPosition,
			CreatedAt:       item.CreatedAt,
		}
	}

	return &v1.PublicAnnouncementsRes{List: list}, nil
}
