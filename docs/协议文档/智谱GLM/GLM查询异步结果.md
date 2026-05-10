> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 查询异步结果

> 查询对话补全和视频生成异步请求的处理结果和状态。



## OpenAPI

````yaml /openapi/openapi.json get /paas/v4/async-result/{id}
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
  /paas/v4/async-result/{id}:
    get:
      tags:
        - 模型 API
      summary: 查询异步结果
      description: 查询对话补全和视频生成异步请求的处理结果和状态。
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            description: 任务 ID。
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                oneOf:
                  - $ref: '#/components/schemas/ChatCompletionResponse'
                    title: 对话补全
                  - $ref: '#/components/schemas/AsyncVideoGenerationResponse'
                    title: 视频生成
                  - $ref: '#/components/schemas/AsyncImageGenerationResponse'
                    title: 图像生成
            text/event-stream:
              schema:
                $ref: '#/components/schemas/ChatCompletionChunk'
        default:
          description: 请求失败。
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    ChatCompletionResponse:
      type: object
      properties:
        id:
          description: 任务 `ID`
          type: string
        request_id:
          description: 请求 `ID`
          type: string
        created:
          description: 请求创建时间，`Unix` 时间戳（秒）
          type: integer
        model:
          description: 模型名称
          type: string
        choices:
          type: array
          description: 模型响应列表
          items:
            type: object
            properties:
              index:
                type: integer
                description: 结果索引
              message:
                $ref: '#/components/schemas/ChatCompletionResponseMessage'
              finish_reason:
                type: string
                description: >-
                  推理终止原因。'stop’表示自然结束或触发stop词，'tool_calls’表示模型命中函数，'length’表示达到token长度限制，'sensitive’表示内容被安全审核接口拦截（用户应判断并决定是否撤回公开内容），'network_error’表示模型推理异常，'model_context_window_exceeded'表示超出模型上下文窗口。
        usage:
          type: object
          description: 调用结束时返回的 `Token` 使用统计。
          properties:
            prompt_tokens:
              type: number
              description: 用户输入的 `Token` 数量。
            completion_tokens:
              type: number
              description: 输出的 `Token` 数量
            prompt_tokens_details:
              type: object
              properties:
                cached_tokens:
                  type: number
                  description: 命中的缓存 `Token` 数量
            total_tokens:
              type: integer
              description: '`Token` 总数，对于 `glm-4-voice` 模型，`1`秒音频=`12.5 Tokens`，向上取整'
        video_result:
          type: array
          description: 视频生成结果。
          items:
            type: object
            properties:
              url:
                type: string
                description: 视频链接。
              cover_image_url:
                type: string
                description: 视频封面链接。
        web_search:
          type: array
          description: 返回与网页搜索相关的信息，使用`WebSearchToolSchema`时返回
          items:
            type: object
            properties:
              icon:
                type: string
                description: 来源网站的图标
              title:
                type: string
                description: 搜索结果的标题
              link:
                type: string
                description: 搜索结果的网页链接
              media:
                type: string
                description: 搜索结果网页的媒体来源名称
              publish_date:
                type: string
                description: 网站发布时间
              content:
                type: string
                description: 搜索结果网页引用的文本内容
              refer:
                type: string
                description: 角标序号
        content_filter:
          type: array
          description: 返回内容安全的相关信息
          items:
            type: object
            properties:
              role:
                type: string
                description: >-
                  安全生效环节，包括 `role = assistant` 模型推理，`role = user` 用户输入，`role =
                  history` 历史上下文
              level:
                type: integer
                description: 严重程度 `level 0-3`，`level 0`表示最严重，`3`表示轻微
    AsyncVideoGenerationResponse:
      type: object
      properties:
        model:
          type: string
          description: 模型名称。
        task_status:
          type: string
          description: 任务处理状态，`PROCESSING`（处理中），`SUCCESS`（成功），`FAIL`（失败） 注：处理中状态需通过查询获取结果
        video_result:
          type: array
          description: 数组，包含生成的视频`URL`。
          items:
            type: object
            properties:
              url:
                type: string
                description: 视频链接。
              cover_image_url:
                type: string
                description: 视频封面链接。
        request_id:
          type: string
          description: 标识此次请求的唯一`ID`，可由用户在客户端请求时提交或平台自动生成。
    AsyncImageGenerationResponse:
      type: object
      properties:
        model:
          type: string
          description: 模型名称。
        task_status:
          type: string
          description: 任务处理状态，`PROCESSING`（处理中），`SUCCESS`（成功），`FAIL`（失败） 注：处理中状态需通过查询获取结果
        image_result:
          type: array
          description: 数组，包含生成的图片`URL`。
          items:
            type: object
            properties:
              url:
                type: string
                description: 图片链接。图片的临时链接有效期为`30`天，请及时转存图片。
        request_id:
          type: string
          description: 标识此次请求的唯一`ID`，可由用户在客户端请求时提交或平台自动生成。
    ChatCompletionChunk:
      type: object
      properties:
        id:
          type: string
          description: 任务 ID
        created:
          description: 请求创建时间，`Unix` 时间戳（秒）
          type: integer
        model:
          description: 模型名称
          type: string
        choices:
          type: array
          description: 模型响应列表
          items:
            type: object
            properties:
              index:
                type: integer
                description: 结果索引
              delta:
                type: object
                description: 模型增量返回的文本信息
                properties:
                  role:
                    type: string
                    description: 当前对话的角色，目前默认为 `assistant`（模型）
                  content:
                    oneOf:
                      - type: string
                        description: >-
                          当前对话文本内容。如果调用函数则为 `null`，否则返回推理结果。

                          对于`GLM-4.5V`系列模型，返回内容可能包含思考过程标签 `<think>
                          </think>`，文本边界标签 `<|begin_of_box|> <|end_of_box|>`。
                      - type: array
                        description: 当前对话的多模态内容（适用于`GLM-4V`系列）
                        items:
                          type: object
                          properties:
                            type:
                              type: string
                              enum:
                                - text
                              description: 内容类型，目前为文本
                            text:
                              type: string
                              description: 文本内容
                      - type: string
                        nullable: true
                        description: 当使用`tool_calls`时，`content`可能为`null`
                  audio:
                    type: object
                    description: 当使用 `glm-4-voice` 模型时返回的音频内容
                    properties:
                      id:
                        type: string
                        description: 当前对话的音频内容`id`，可用于多轮对话输入
                      data:
                        type: string
                        description: 当前对话的音频内容`base64`编码
                      expires_at:
                        type: string
                        description: 当前对话的音频内容过期时间
                  reasoning_content:
                    type: string
                    description: 思维链内容, 仅 `glm-4.5` 系列支持
                  tool_calls:
                    type: array
                    description: 生成的应该被调用的工具信息，流式返回时会逐步生成
                    items:
                      type: object
                      properties:
                        index:
                          type: integer
                          description: 工具调用索引
                        id:
                          type: string
                          description: 工具调用的唯一标识符
                        type:
                          type: string
                          description: 工具类型，目前支持`function`
                          enum:
                            - function
                        function:
                          type: object
                          properties:
                            name:
                              type: string
                              description: 函数名称
                            arguments:
                              type: string
                              description: 函数参数，`JSON`格式字符串
              finish_reason:
                type: string
                description: >-
                  模型推理终止的原因。`stop` 表示自然结束或触发stop词，`tool_calls` 表示模型命中函数，`length`
                  表示达到 `token` 长度限制，`sensitive`
                  表示内容被安全审核接口拦截（用户应判断并决定是否撤回公开内容），`network_error`
                  表示模型推理异常，'model_context_window_exceeded'表示超出模型上下文窗口。
                enum:
                  - stop
                  - length
                  - tool_calls
                  - sensitive
                  - network_error
        usage:
          type: object
          description: 本次模型调用的 `tokens` 数量统计
          properties:
            prompt_tokens:
              type: integer
              description: 用户输入的 `tokens` 数量。对于 `glm-4-voice`，`1`秒音频=`12.5 Tokens`，向上取整。
            completion_tokens:
              type: integer
              description: 模型输出的 `tokens` 数量
            total_tokens:
              type: integer
              description: 总 `tokens` 数量，对于 `glm-4-voice` 模型，`1`秒音频=`12.5 Tokens`，向上取整
        content_filter:
          type: array
          description: 返回内容安全的相关信息
          items:
            type: object
            properties:
              role:
                type: string
                description: >-
                  安全生效环节，包括：`role = assistant` 模型推理，`role = user` 用户输入，`role =
                  history` 历史上下文
              level:
                type: integer
                description: 严重程度 `level 0-3`，`level 0` 表示最严重，`3` 表示轻微
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
    ChatCompletionResponseMessage:
      type: object
      properties:
        role:
          type: string
          description: 当前对话角色，默认为 `assistant`
          example: assistant
        content:
          oneOf:
            - type: string
              description: >-
                当前对话文本内容。如果调用函数则为 `null`，否则返回推理结果。

                对于`GLM-4.5V`系列模型，返回内容可能包含思考过程标签 `<think> </think>`，文本边界标签
                `<|begin_of_box|> <|end_of_box|>`。
            - type: array
              description: 多模态回复内容，适用于`GLM-4V`系列模型
              items:
                type: object
                properties:
                  type:
                    type: string
                    enum:
                      - text
                    description: 回复内容类型，目前为文本
                  text:
                    type: string
                    description: 文本内容
            - type: string
              nullable: true
              description: 当使用`tool_calls`时，`content`可能为`null`
        reasoning_content:
          type: string
          description: 思维链内容，仅在使用 `glm-4.5` 系列, `glm-4.1v-thinking` 系列模型时返回。
        audio:
          type: object
          description: 当使用 `glm-4-voice` 模型时返回的音频内容
          properties:
            id:
              type: string
              description: 当前对话的音频内容`id`，可用于多轮对话输入
            data:
              type: string
              description: 当前对话的音频内容`base64`编码
            expires_at:
              type: string
              description: 当前对话的音频内容过期时间
        tool_calls:
          type: array
          description: 生成的应该被调用的函数名称和参数。
          items:
            $ref: '#/components/schemas/ChatCompletionResponseMessageToolCall'
    ChatCompletionResponseMessageToolCall:
      type: object
      properties:
        function:
          type: object
          description: 包含生成的函数名称和 `JSON` 格式参数。
          properties:
            name:
              type: string
              description: 生成的函数名称。
            arguments:
              type: string
              description: 生成的函数调用参数的 `JSON` 格式字符串。调用函数前请验证参数。
          required:
            - name
            - arguments
        mcp:
          type: object
          description: '`MCP` 工具调用参数'
          properties:
            id:
              description: '`mcp` 工具调用唯一标识'
              type: string
            type:
              description: 工具调用类型, 例如 `mcp_list_tools, mcp_call`
              type: string
              enum:
                - mcp_list_tools
                - mcp_call
            server_label:
              description: '`MCP`服务器标签'
              type: string
            error:
              description: 错误信息
              type: string
            tools:
              description: '`type = mcp_list_tools` 时的工具列表'
              type: array
              items:
                type: object
                properties:
                  name:
                    description: 工具名称
                    type: string
                  description:
                    description: 工具描述
                    type: string
                  annotations:
                    description: 工具注解
                    type: object
                  input_schema:
                    description: 工具输入参数规范
                    type: object
                    properties:
                      type:
                        description: 固定值 'object'
                        type: string
                        default: object
                        enum:
                          - object
                      properties:
                        description: 参数属性定义
                        type: object
                      required:
                        description: 必填属性列表
                        type: array
                        items:
                          type: string
                      additionalProperties:
                        description: 是否允许额外参数
                        type: boolean
            arguments:
              description: 工具调用参数，参数为 `json` 字符串
              type: string
            name:
              description: 工具名称
              type: string
            output:
              description: 工具返回的结果输出
              type: object
        id:
          type: string
          description: 命中函数的唯一标识符。
        type:
          type: string
          description: 调用的工具类型，目前仅支持 'function', 'mcp'。
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````