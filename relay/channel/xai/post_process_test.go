package xai

import (
	"github.com/qianfree/team-api/relay/common"
)

// relaykit 接管转换后，私有请求适配靠这个可选接口接回；断言实现不掉。
var _ common.RequestPostProcessor = (*Adaptor)(nil)
