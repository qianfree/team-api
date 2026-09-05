# 第三方开源组件声明 / Third-Party Notices

版权所有 © 2026 qianfree。

Team-API 使用了以下开源组件，感谢各项目的作者与贡献者。
本文件用于满足各组件许可证的署名要求，商业授权交付时需随产品附带。
本文件不改变 Team-API 的主许可证，也不授予任何额外的软件使用、复制、修改、分发、SaaS、OEM 或转售权利；各第三方组件仍受其各自许可证约束。

> 本文件由脚本扫描 `go.mod`（Go 模块缓存）与 `web/*/node_modules`（package.json 的 license 字段）生成。
> 重新生成方式见文末。

## 许可证分布概览

- **Go 依赖**：以本文件表格为准；发布前应重新扫描并复核许可证，重点关注 GPL/AGPL/LGPL 等 copyleft 许可证
- **前端依赖**：以本文件表格为准；发布前应重新扫描并复核生产依赖与构建期依赖

## Go 依赖（含间接依赖）

> 下表覆盖生成时已缓存的 122 个模块；另有 66 个模块（均为其他平台专用的间接依赖，如 Windows/macOS 专属包）本次未下载，发布前用文末方法重新扫描补全。

| 模块 | 版本 | 许可证 |
|------|------|--------|
| `github.com/aws/aws-sdk-go-v2` | v1.41.6 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream` | v1.7.9 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/config` | v1.32.16 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/credentials` | v1.19.15 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/feature/ec2/imds` | v1.18.22 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/internal/configsources` | v1.4.22 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/internal/endpoints/v2` | v2.7.22 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/internal/v4a` | v1.4.23 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding` | v1.13.8 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/internal/checksum` | v1.9.14 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/internal/presigned-url` | v1.13.22 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/internal/s3shared` | v1.19.22 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/s3` | v1.99.1 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/signin` | v1.0.10 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/sso` | v1.30.16 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/ssooidc` | v1.35.20 | Apache-2.0 |
| `github.com/aws/aws-sdk-go-v2/service/sts` | v1.42.0 | Apache-2.0 |
| `github.com/aws/smithy-go` | v1.25.0 | Apache-2.0 |
| `github.com/go-logr/logr` | v1.4.3 | Apache-2.0 |
| `github.com/go-logr/stdr` | v1.2.2 | Apache-2.0 |
| `github.com/pquerna/otp` | v1.5.0 | Apache-2.0 |
| `github.com/richardlehane/mscfb` | v1.0.6 | Apache-2.0 |
| `github.com/richardlehane/msoleps` | v1.0.6 | Apache-2.0 |
| `github.com/sethvargo/go-retry` | v0.3.0 | Apache-2.0 |
| `github.com/tklauser/numcpus` | v0.6.1 | Apache-2.0 |
| `github.com/wenlng/go-captcha-assets` | v1.0.7 | Apache-2.0 |
| `github.com/wenlng/go-captcha/v2` | v2.0.5 | Apache-2.0 |
| `go.opentelemetry.io/auto/sdk` | v1.2.1 | Apache-2.0 |
| `go.opentelemetry.io/otel` | v1.43.0 | Apache-2.0 |
| `go.opentelemetry.io/otel/metric` | v1.43.0 | Apache-2.0 |
| `go.opentelemetry.io/otel/sdk` | v1.39.0 | Apache-2.0 |
| `go.opentelemetry.io/otel/sdk/metric` | v1.39.0 | Apache-2.0 |
| `go.opentelemetry.io/otel/trace` | v1.43.0 | Apache-2.0 |
| `gopkg.in/yaml.v3` | v3.0.1 | Apache-2.0 |
| `github.com/emirpasic/gods/v2` | v2.0.0-alpha | BSD-2-Clause |
| `github.com/gorilla/websocket` | v1.5.3 | BSD-2-Clause |
| `github.com/magiconair/properties` | v1.8.10 | BSD-2-Clause |
| `github.com/pmezard/go-difflib` | v1.0.0 | BSD-2-Clause |
| `github.com/redis/go-redis/v9` | v9.19.0 | BSD-2-Clause |
| `github.com/zeebo/xxh3` | v1.1.0 | BSD-2-Clause |
| `gopkg.in/check.v1` | v1.0.0-20201130134442-10cb98267c6c | BSD-2-Clause |
| `github.com/cloudflare/ahocorasick` | v0.0.0-20240916140611-054963ec9396 | BSD-3-Clause |
| `github.com/fsnotify/fsnotify` | v1.9.0 | BSD-3-Clause |
| `github.com/google/go-cmp` | v0.7.0 | BSD-3-Clause |
| `github.com/google/go-querystring` | v1.0.0 | BSD-3-Clause |
| `github.com/google/uuid` | v1.6.0 | BSD-3-Clause |
| `github.com/grokify/html-strip-tags-go` | v0.1.0 | BSD-3-Clause |
| `github.com/lufia/plan9stats` | v0.0.0-20211012122336-39d0f177ccd0 | BSD-3-Clause |
| `github.com/remyoudompheng/bigfft` | v0.0.0-20230129092748-24d4a6f8daec | BSD-3-Clause |
| `github.com/rogpeppe/go-internal` | v1.14.1 | BSD-3-Clause |
| `github.com/shirou/gopsutil/v3` | v3.24.5 | BSD-3-Clause |
| `github.com/tklauser/go-sysconf` | v0.3.12 | BSD-3-Clause |
| `github.com/xuri/efp` | v0.0.1 | BSD-3-Clause |
| `github.com/xuri/excelize/v2` | v2.10.1 | BSD-3-Clause |
| `github.com/xuri/nfp` | v0.0.2-0.20250530014748-2ddeb826f9a9 | BSD-3-Clause |
| `golang.org/x/crypto` | v0.53.0 | BSD-3-Clause |
| `golang.org/x/image` | v0.25.0 | BSD-3-Clause |
| `golang.org/x/net` | v0.56.0 | BSD-3-Clause |
| `golang.org/x/sync` | v0.22.0 | BSD-3-Clause |
| `golang.org/x/sys` | v0.47.0 | BSD-3-Clause |
| `golang.org/x/text` | v0.40.0 | BSD-3-Clause |
| `golang.org/x/time` | v0.15.0 | BSD-3-Clause |
| `modernc.org/libc` | v1.72.1 | BSD-3-Clause |
| `modernc.org/mathutil` | v1.7.1 | BSD-3-Clause |
| `modernc.org/memory` | v1.11.0 | BSD-3-Clause |
| `modernc.org/sqlite` | v1.49.1 | BSD-3-Clause |
| `github.com/golang/freetype` | v0.0.0-20170609003504-e2365dfdc4a0 | FTL / GPL-2.0-or-later（本项目选择 FTL） |
| `github.com/shoenig/go-m1cpu` | v0.2.1 | MPL-2.0 |
| `github.com/shoenig/test` | v1.7.0 | MPL-2.0 |
| `github.com/davecgh/go-spew` | v1.1.1 | ISC |
| `github.com/BurntSushi/toml` | v1.5.0 | MIT |
| `github.com/Masterminds/semver/v3` | v3.5.0 | MIT |
| `github.com/alicebob/miniredis/v2` | v2.38.0 | MIT |
| `github.com/aliyun/aliyun-oss-go-sdk` | v3.0.2+incompatible | MIT |
| `github.com/boombuler/barcode` | v1.0.1-0.20190219062509-6c824513bacc | MIT |
| `github.com/bsm/ginkgo/v2` | v2.12.0 | MIT |
| `github.com/bsm/gomega` | v1.27.10 | MIT |
| `github.com/cespare/xxhash/v2` | v2.3.0 | MIT |
| `github.com/clbanning/mxj` | v1.8.4 | MIT |
| `github.com/clbanning/mxj/v2` | v2.7.0 | MIT |
| `github.com/dustin/go-humanize` | v1.0.1 | MIT |
| `github.com/fatih/color` | v1.18.0 | MIT |
| `github.com/go-ole/go-ole` | v1.2.6 | MIT |
| `github.com/gogf/gf/contrib/drivers/pgsql/v2` | v2.10.2 | MIT |
| `github.com/gogf/gf/contrib/nosql/redis/v2` | v2.10.2 | MIT |
| `github.com/gogf/gf/v2` | v2.10.2 | MIT |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | MIT |
| `github.com/klauspost/cpuid/v2` | v2.2.10 | MIT |
| `github.com/kr/pretty` | v0.3.1 | MIT |
| `github.com/kr/text` | v0.2.0 | MIT |
| `github.com/lib/pq` | v1.10.9 | MIT |
| `github.com/mattn/go-colorable` | v0.1.13 | MIT |
| `github.com/mattn/go-isatty` | v0.0.21 | MIT |
| `github.com/mattn/go-runewidth` | v0.0.16 | MIT |
| `github.com/mfridman/interpolate` | v0.0.2 | MIT |
| `github.com/mitchellh/mapstructure` | v1.4.3 | MIT |
| `github.com/mozillazg/go-httpheader` | v0.2.1 | MIT |
| `github.com/ncruces/go-strftime` | v1.0.0 | MIT |
| `github.com/olekukonko/errors` | v1.1.0 | MIT |
| `github.com/olekukonko/ll` | v0.0.9 | MIT |
| `github.com/olekukonko/tablewriter` | v1.1.0 | MIT |
| `github.com/power-devops/perfstat` | v0.0.0-20210106213030-5aafc221ea8c | MIT |
| `github.com/pressly/goose/v3` | v3.27.1 | MIT |
| `github.com/rivo/uniseg` | v0.2.0 | MIT |
| `github.com/robfig/cron/v3` | v3.0.1 | MIT |
| `github.com/samber/lo` | v1.53.0 | MIT |
| `github.com/shopspring/decimal` | v1.4.0 | MIT |
| `github.com/stretchr/testify` | v1.11.1 | MIT |
| `github.com/tencentyun/cos-go-sdk-v5` | v0.7.73 | MIT |
| `github.com/tidwall/gjson` | v1.18.0 | MIT |
| `github.com/tidwall/match` | v1.1.1 | MIT |
| `github.com/tidwall/pretty` | v1.2.0 | MIT |
| `github.com/tidwall/sjson` | v1.2.5 | MIT |
| `github.com/tiendc/go-deepcopy` | v1.7.2 | MIT |
| `github.com/yuin/gopher-lua` | v1.1.1 | MIT |
| `github.com/yusufpapurcu/wmi` | v1.2.4 | MIT |
| `go.uber.org/atomic` | v1.11.0 | MIT |
| `go.uber.org/goleak` | v1.3.0 | MIT |
| `go.uber.org/multierr` | v1.11.0 | MIT |
| `gopkg.in/alexcesaro/quotedprintable.v3` | v3.0.0-20150716171945-2caba252f4dc | MIT |
| `gopkg.in/gomail.v2` | v2.0.0-20160411212932-81ebce5c23df | MIT |
| `github.com/qianfree/team-api/relaykit` | v0.0.0-00010101000000-000000000000 | NO_LICENSE_FILE |

### Go 依赖特别说明

- `github.com/golang/freetype`：双许可证（FTL / GPL-2.0+），本项目**选择 FTL**（类 BSD 带广告条款），不触发 GPL 义务
- `github.com/shoenig/go-m1cpu`、`github.com/shoenig/test`：MPL-2.0（文件级弱传染），均为间接依赖（前者仅 macOS 平台编译、后者仅测试用），商业分发合规，保留其 LICENSE 即可
- `github.com/qianfree/team-api/relaykit`：本项目自有子模块，非第三方

## 前端依赖（web/admin + web/tenant，含开发依赖）

- **0BSD**（1 个）： `tslib`
- **Apache-2.0**（3 个）： `detect-libc`, `echarts`, `typescript`
- **BSD-2-Clause**（1 个）： `entities`
- **BSD-3-Clause**（4 个）： `highlight.js`, `source-map-js`, `speakingurl`, `zrender`
- **ISC**（2 个）： `graceful-fs`, `picocolors`
- **MIT**（89 个）： `alien-signals`, `async-validator`, `asynckit`, `axios`, `b-tween`, `b-validate`, `birpc`, `call-bind-apply-helpers`, `chart.js`, `color-convert`, `color-name`, `color-string`, `color`, `combined-stream`, `compute-scroll-into-view`, `copy-anything`, `css-render`, `csstype`, `date-fns-tz`, `date-fns`, `dayjs`, `delayed-stream`, `dunder-proto`, `enhanced-resolve`, `es-define-property`, `es-errors`, `es-object-atoms`, `es-set-tostringtag`, `estree-walker`, `evtd`, `fdir`, `follow-redirects`, `form-data`, `function-bind`, `get-intrinsic`, `get-proto`, `gopd`, `has-symbols`, `has-tostringtag`, `hasown`, `hookable`, `is-arrayish`, `is-what`, `jiti`, `lodash-es`, `lodash`, `magic-string`, `marked`, `marked`, `math-intrinsics`, `mime-db`, `mime-types`, `mitt`, `muggle-string`, `naive-ui`, `nanoid`, `number-precision`, `packrup`, `path-browserify`, `perfect-debounce`, `picomatch`, `pinia`, `postcss`, `proxy-from-env`, `resize-observer-polyfill`, `rfdc`, `rolldown`, `scroll-into-view-if-needed`, `seemly`, `simple-swizzle`, `superjson`, `tailwindcss`, `tapable`, `tinyglobby`, `treemate`, `undici-types`, `unhead`, `vdirs`, `vite`, `vooks`, `vscode-uri`, `vue-chartjs`, `vue-demi`, `vue-echarts`, `vue-router`, `vue-tsc`, `vueuc`, `vue`, `zhead`
- **MPL-2.0**（2 个）： `lightningcss-win32-x64-msvc`, `lightningcss`

### 前端依赖特别说明

- `lightningcss`（MPL-2.0）：仅构建期 CSS 处理工具，不进入运行时产物

---

## 重新生成方法

```bash
# Go 依赖：列出全部模块并在模块缓存中检查 LICENSE 文件
go list -m -f "{{if not .Main}}{{.Path}}|{{.Version}}|{{.Dir}}{{end}}" all
# 推荐工具：go install github.com/google/go-licenses@latest
# go-licenses report ./... 可生成带许可证文本链接的完整报告

# 前端依赖（在各 web/* 目录下）：
# bunx license-checker --summary  /  npx license-checker --json
```

> ⚠️ 本文件列出版本为生成时快照。每次发布商业版本前应重新扫描，重点核对**新增依赖**是否引入 copyleft 类许可证。
