package main

import (
	_ "github.com/qianfree/team-api/internal/packed"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"

	_ "github.com/qianfree/team-api/internal/logic"
	_ "github.com/qianfree/team-api/plugins"

	"github.com/gogf/gf/v2/os/gctx"

	"github.com/qianfree/team-api/internal/cmd"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
