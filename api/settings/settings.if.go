// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package settings

import (
	"context"

	"github.com/qianfree/team-api/api/settings/v1"
)

type ISettingsV1 interface {
	PublicSettingsGet(ctx context.Context, req *v1.PublicSettingsGetReq) (res *v1.PublicSettingsGetRes, err error)
	PublicAnnouncements(ctx context.Context, req *v1.PublicAnnouncementsReq) (res *v1.PublicAnnouncementsRes, err error)
}
