package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TemplateList(ctx context.Context, req *v1.TemplateListReq) (res *v1.TemplateListRes, err error) {
	return service.Admin().ListTemplates(ctx, req)
}
func (c *ControllerV1) TemplateGet(ctx context.Context, req *v1.TemplateGetReq) (res *v1.TemplateGetRes, err error) {
	return service.Admin().GetTemplate(ctx, req)
}
func (c *ControllerV1) TemplateUpdate(ctx context.Context, req *v1.TemplateUpdateReq) (res *v1.TemplateUpdateRes, err error) {
	return service.Admin().UpdateTemplate(ctx, req)
}
func (c *ControllerV1) TemplateTest(ctx context.Context, req *v1.TemplateTestReq) (res *v1.TemplateTestRes, err error) {
	return service.Admin().TestTemplate(ctx, req)
}
func (c *ControllerV1) MessageList(ctx context.Context, req *v1.MessageListReq) (res *v1.MessageListRes, err error) {
	return service.Admin().ListMessages(ctx, req)
}
func (c *ControllerV1) MessageSend(ctx context.Context, req *v1.MessageSendReq) (res *v1.MessageSendRes, err error) {
	return service.Admin().SendMessage(ctx, req)
}
func (c *ControllerV1) MessageBroadcast(ctx context.Context, req *v1.MessageBroadcastReq) (res *v1.MessageBroadcastRes, err error) {
	return service.Admin().SendBroadcast(ctx, req)
}
func (c *ControllerV1) AnnouncementList(ctx context.Context, req *v1.AnnouncementListReq) (res *v1.AnnouncementListRes, err error) {
	return service.Admin().ListAnnouncements(ctx, req)
}
func (c *ControllerV1) AnnouncementCreate(ctx context.Context, req *v1.AnnouncementCreateReq) (res *v1.AnnouncementCreateRes, err error) {
	return service.Admin().CreateAnnouncement(ctx, req)
}
func (c *ControllerV1) AnnouncementUpdate(ctx context.Context, req *v1.AnnouncementUpdateReq) (res *v1.AnnouncementUpdateRes, err error) {
	return service.Admin().UpdateAnnouncement(ctx, req)
}
func (c *ControllerV1) AnnouncementPublish(ctx context.Context, req *v1.AnnouncementPublishReq) (res *v1.AnnouncementPublishRes, err error) {
	return service.Admin().PublishAnnouncement(ctx, req)
}
func (c *ControllerV1) AnnouncementArchive(ctx context.Context, req *v1.AnnouncementArchiveReq) (res *v1.AnnouncementArchiveRes, err error) {
	return service.Admin().ArchiveAnnouncement(ctx, req)
}
