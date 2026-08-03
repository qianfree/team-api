package relayconvert

import (
	"context"

	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/types"
)

// noopReqConvert / noopRespConvert 是注册表测试用的空操作转换函数（仅满足函数类型签名）。
// 定义在单独文件中，供 text/request/response 注册表测试共享，避免同包重复声明。
var noopReqConvert = func(_ context.Context, _ convmeta.Meta, request any) (any, error) {
	return request, nil
}

var noopRespConvert = func(_ context.Context, _ convmeta.Meta, response any) (any, *dto.Usage, error) {
	return response, nil, nil
}

// makeRoute 由 ID 派生唯一 (from, to) 协议格式，保证跨测试不发生路由冲突。
func makeRoute(id string, suffix string) (types.RelayFormat, types.RelayFormat) {
	return types.RelayFormat(id + "_" + suffix + "_from"),
		types.RelayFormat(id + "_" + suffix + "_to")
}
