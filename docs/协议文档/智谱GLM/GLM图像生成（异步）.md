> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 图像生成(异步)

> 使用 [GLM-Image](/cn/guide/models/image-generation/glm-image) 系列模型从文本提示生成高质量图像。通过对用户文字描述快速、精准的理解，让 `AI` 的图像表达更加精确和个性化。仅支持 `GLM-Image` 模型。



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/async/images/generations
openapi: 3.0.1
info:
  title: ZHIPU AI API
  description: ZHIPU AI 接口提供强大的 AI 能力，包括聊天对话、工具调用和视频生成。
  license:
    name: ZHIPU AI 开发者协议和政策
    url: https://chat.z.ai/legal-agreement/terms-of-service
  version: 1.0.0
  contact:
    name: Z.AI 开发者
    url: https://chat.z.ai/legal-agreement/privacy-policy
    email: user_feedback@z.ai
servers:
  - url: https://open.bigmodel.cn/api/
    description: 开放平台服务
security:
  - bearerAuth: []
tags:
  - name: 模型 API
    description: Chat API
  - name: 工具 API
    description: Web Search API
  - name: Agent API
    description: Agent API
  - name: 文件 API
    description: File API
  - name: 知识库 API
    description: Knowledge API
  - name: 实时 API
    description: Realtime API
  - name: 批处理 API
    description: Batch API
  - name: 助理 API
    description: Assistant API
  - name: 智能体 API（旧）
    description: QingLiu Agent API
paths:
  /paas/v4/async/images/generations:
    post:
      tags:
        - 模型 API
      summary: 图像生成(异步)
      description: >-
        使用 [GLM-Image](/cn/guide/models/image-generation/glm-image)
        系列模型从文本提示生成高质量图像。通过对用户文字描述快速、精准的理解，让 `AI` 的图像表达更加精确和个性化。仅支持 `GLM-Image`
        模型。
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/AsyncCreateImageRequest'
            examples:
              图像生成示例:
                value:
                  model: glm-image
                  prompt: 一只可爱的小猫咪，坐在阳光明媚的窗台上，背景是蓝天白云.
                  size: 1280x1280
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AsyncResponse'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    AsyncCreateImageRequest:
      type: object
      required:
        - model
        - prompt
      properties:
        model:
          type: string
          description: 模型编码
          enum:
            - glm-image
          example: glm-image
        prompt:
          type: string
          description: 所需图像的文本描述
          example: 一只可爱的小猫咪
        quality:
          type: string
          description: '生成图像的质量。`hd`: 生成更精细、细节更丰富的图像，整体一致性更高，耗时约`20`秒；'
          enum:
            - hd
          default: hd
        size:
          type: string
          description: >-
            图片尺寸，推荐枚举值：`1280x1280` (默认), `1568×1056`, `1056×1568`, `1472×1088`,
            `1088×1472`, `1728×960`, `960×1728`。

            自定义参数:长宽推荐设置在`1024px-2048px`范围内,并保证最大像素数不超过`2^22px`;长宽均需为`32`的整数倍。
          default: 1280x1280
          example: 1280x1280
        watermark_enabled:
          type: boolean
          description: |-
            控制`AI`生成图片时是否添加水印。
             - `true`: 默认启用`AI`生成的显式水印及隐式数字水印，符合政策要求。
             - `false`: 关闭所有水印，仅允许已签署免责声明的客户使用，签署路径：个人中心-安全管理-去水印管理
          example: true
        user_id:
          type: string
          description: >-
            终端用户的唯一`ID`，协助平台对终端用户的违规行为、生成违法及不良信息或其他滥用行为进行干预。`ID`长度要求：最少`6`个字符，最多`128`个字符。
          minLength: 6
          maxLength: 128
    AsyncResponse:
      type: object
      properties:
        model:
          description: 此次调用使用的名称。
          type: string
        id:
          description: 生成的任务`ID`，调用请求结果接口时使用此`ID`。
          type: string
        request_id:
          description: 用户在客户端请求期间提交的任务编号或平台生成的任务编号。
          type: string
        task_status:
          description: 处理状态，`PROCESSING (处理中)`、`SUCCESS (成功)`、`FAIL (失败)`。结果需要通过查询获取。
          type: string
    Error:
      type: object
      properties:
        error:
          required:
            - code
            - message
          type: object
          properties:
            code:
              type: string
            message:
              type: string
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````