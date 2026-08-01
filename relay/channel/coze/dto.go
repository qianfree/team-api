package coze

import relaykitdto "github.com/qianfree/team-api/relaykit/dto"

// CozeCreateRequest / CozeMessage 与 relaykit/dto 中定义字节相同的协议结构，
// 别名到 relaykit 统一权威定义，消除本地重复。
//
// 阶段 7 清理：此前本地还定义了 CozeCreateResponse / CozeRetrieveResponse /
// CozeMessageListResponse，但全仓无任何引用，已删除。
type CozeCreateRequest = relaykitdto.CozeCreateRequest
type CozeMessage = relaykitdto.CozeMessage
