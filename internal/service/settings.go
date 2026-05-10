// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"

	v1 "github.com/qianfree/team-api/api/settings/v1"
)

type (
	ISettings interface {
		PublicSettingsGet(ctx context.Context, _ *v1.PublicSettingsGetReq) (*v1.PublicSettingsGetRes, error)
		PublicAnnouncements(ctx context.Context, req *v1.PublicAnnouncementsReq) (*v1.PublicAnnouncementsRes, error)
	}
)

var (
	localSettings ISettings
)

func Settings() ISettings {
	if localSettings == nil {
		panic("implement not found for interface ISettings, forgot register?")
	}
	return localSettings
}

func RegisterSettings(i ISettings) {
	localSettings = i
}
