> ## Documentation Index
> Fetch the complete documentation index at: https://docs.bigmodel.cn/llms.txt
> Use this file to discover all available pages before exploring further.

# 对话补全(异步)

> 和 [指定模型](/cn/guide/start/model-overview) 对话，通过查询异步结果获取模型响应。支持多种模型，支持多模态（文本、图片、音频、视频、文件），可配置采样，温度，最大令牌数，工具调用等。注意此为异步接口，通过 [查询异步结果](/api-reference/%E6%A8%A1%E5%9E%8B-api/%E6%9F%A5%E8%AF%A2%E5%BC%82%E6%AD%A5%E7%BB%93%E6%9E%9C) 获取生成结果。



## OpenAPI

````yaml /openapi/openapi.json post /paas/v4/async/chat/completions
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
  /paas/v4/async/chat/completions:
    post:
      tags:
        - 模型 API
      summary: 对话补全(异步)
      description: >-
        和 [指定模型](/cn/guide/start/model-overview)
        对话，通过查询异步结果获取模型响应。支持多种模型，支持多模态（文本、图片、音频、视频、文件），可配置采样，温度，最大令牌数，工具调用等。注意此为异步接口，通过
        [查询异步结果](/api-reference/%E6%A8%A1%E5%9E%8B-api/%E6%9F%A5%E8%AF%A2%E5%BC%82%E6%AD%A5%E7%BB%93%E6%9E%9C)
        获取生成结果。
      requestBody:
        content:
          application/json:
            schema:
              oneOf:
                - $ref: '#/components/schemas/AsyncChatCompletionTextRequest'
                  title: 文本模型
                - $ref: '#/components/schemas/AsyncChatCompletionVisionRequest'
                  title: 视觉模型
                - $ref: '#/components/schemas/AsyncChatCompletionAudioRequest'
                  title: 音频模型
                - $ref: '#/components/schemas/AsyncChatCompletionHumanOidRequest'
                  title: 角色模型
            examples:
              基础调用示例:
                value:
                  model: glm-5.1
                  messages:
                    - role: system
                      content: 你是一个有用的AI助手。
                    - role: user
                      content: 请介绍一下人工智能的发展历程。
                  temperature: 1
              深度思考示例:
                value:
                  model: glm-5.1
                  messages:
                    - role: user
                      content: 写一首关于春天的诗。
                  thinking:
                    type: enabled
              多轮对话示例:
                value:
                  model: glm-5.1
                  messages:
                    - role: system
                      content: 你是一个专业的编程助手
                    - role: user
                      content: 什么是递归？
                    - role: assistant
                      content: 递归是一种编程技术，函数调用自身来解决问题...
                    - role: user
                      content: 能给我一个 Python 递归的例子吗？
              图片理解示例:
                value:
                  model: glm-5v-turbo
                  messages:
                    - role: user
                      content:
                        - type: image_url
                          image_url:
                            url: https://cdn.bigmodel.cn/static/logo/register.png
                        - type: image_url
                          image_url:
                            url: https://cdn.bigmodel.cn/static/logo/api-key.png
                        - type: text
                          text: What are the pics talk about?
              视频理解示例:
                value:
                  model: glm-5v-turbo
                  messages:
                    - role: user
                      content:
                        - type: video_url
                          video_url:
                            url: >-
                              https://cdn.bigmodel.cn/agent-demos/lark/113123.mov
                        - type: text
                          text: What are the video show about?
              文件理解示例:
                value:
                  model: glm-5v-turbo
                  messages:
                    - role: user
                      content:
                        - type: file_url
                          file_url:
                            url: https://cdn.bigmodel.cn/static/demo/demo2.txt
                        - type: file_url
                          file_url:
                            url: https://cdn.bigmodel.cn/static/demo/demo1.pdf
                        - type: text
                          text: What are the files show about?
              音频对话示例:
                value:
                  model: glm-4-voice
                  messages:
                    - role: user
                      content:
                        - type: text
                          text: 你好，这是我的语音输入测试，请慢速复述一遍
                        - type: input_audio
                          input_audio:
                            data: base64_voice_xxx
                            format: wav
              Function Call 示例:
                value:
                  model: glm-5.1
                  messages:
                    - role: user
                      content: 今天北京的天气怎么样？
                  tools:
                    - type: function
                      function:
                        name: get_weather
                        description: 获取指定城市的天气信息
                        parameters:
                          type: object
                          properties:
                            city:
                              type: string
                              description: 城市名称
                          required:
                            - city
                  tool_choice: auto
                  temperature: 0.3
        required: true
      responses:
        '200':
          description: 业务处理成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AsyncResponse'
        default:
          description: 请求失败
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
components:
  schemas:
    AsyncChatCompletionTextRequest:
      required:
        - model
        - messages
      type: object
      description: 普通对话模型请求，支持纯文本对话和工具调用
      properties:
        model:
          type: string
          description: >-
            调用的普通对话模型代码。`GLM-5.1` 是最新的旗舰模型系列。`GLM-5`
            系列提供了复杂推理、超长上下文、极快推理速度等多款模型。
          example: glm-5.1
          default: glm-5.1
          enum:
            - glm-5.1
            - glm-5-turbo
            - glm-5
            - glm-4.7
            - glm-4.6
            - glm-4.5-air
            - glm-4.5-airx
            - glm-4.5-flash
            - glm-4-flash-250414
            - glm-4-flashx-250414
        messages:
          type: array
          description: >-
            对话消息列表，包含当前对话的完整上下文信息。每条消息都有特定的角色和内容，模型会根据这些消息生成回复。消息按时间顺序排列，支持四种角色：`system`（系统消息，用于设定`AI`的行为和角色）、`user`（用户消息，来自用户的输入）、`assistant`（助手消息，来自`AI`的回复）、`tool`（工具消息，工具调用的结果）。普通对话模型主要支持纯文本内容。注意不能只包含系统消息或助手消息。
          items:
            oneOf:
              - title: 用户消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - user
                    description: 消息作者的角色
                    default: user
                  content:
                    type: string
                    description: 文本消息内容
                    example: >-
                      What opportunities and challenges will the Chinese large
                      model industry face in 2025?
                required:
                  - role
                  - content
              - title: 系统消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - system
                    description: 消息作者的角色
                    default: system
                  content:
                    type: string
                    description: 消息文本内容
                    example: You are a helpful assistant.
                required:
                  - role
                  - content
              - title: 助手消息
                type: object
                description: 可包含工具调用
                properties:
                  role:
                    type: string
                    enum:
                      - assistant
                    description: 消息作者的角色
                    default: assistant
                  content:
                    type: string
                    description: 文本消息内容
                    example: I'll help you with that analysis.
                  tool_calls:
                    type: array
                    description: 模型生成的工具调用消息。当提供此字段时，`content`通常为空。
                    items:
                      type: object
                      properties:
                        id:
                          type: string
                          description: 工具调用ID
                        type:
                          type: string
                          description: 工具类型，支持 `web_search、retrieval、function`
                          enum:
                            - function
                            - web_search
                            - retrieval
                        function:
                          type: object
                          description: 函数调用信息，当`type`为`function`时不为空
                          properties:
                            name:
                              type: string
                              description: 函数名称
                            arguments:
                              type: string
                              description: 函数参数，`JSON`格式字符串
                          required:
                            - name
                            - arguments
                      required:
                        - id
                        - type
                required:
                  - role
              - title: 工具消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - tool
                    description: 消息作者的角色
                    default: tool
                  content:
                    type: string
                    description: 消息文本内容
                    example: 'Function executed successfully with result: ...'
                  tool_call_id:
                    type: string
                    description: 指示此消息对应的工具调用 `ID`
                required:
                  - role
                  - content
          minItems: 1
        thinking:
          $ref: '#/components/schemas/ChatThinking'
        do_sample:
          type: boolean
          example: true
          default: true
          description: >-
            是否启用采样策略来生成文本。默认值为 `true`。当设置为 `true` 时，模型会使用 `temperature、top_p`
            等参数进行随机采样，生成更多样化的输出；当设置为 `false` 时，模型总是选择概率最高的词汇，生成更确定性的输出，此时
            `temperature` 和 `top_p` 参数将被忽略。对于需要一致性和可重复性的任务（如代码生成、翻译），建议设置为
            `false`。
        temperature:
          type: number
          description: >-
            采样温度，控制输出的随机性和创造性，取值范围为 `(0.0, 1.0]`，限两位小数。对于`GLM-5.1` `GLM-5`
            `GLM-5-Turbo` `GLM-4.7` `GLM-4.6`系列默认值为 `1.0`，`GLM-4.5`系列默认值为
            `0.6`，`GLM-4`系列默认值为
            `0.75`。较高的值（如`0.8`）会使输出更随机、更具创造性，适合创意写作和头脑风暴；较低的值（如`0.2`）会使输出更稳定、更确定，适合事实性问答和代码生成。建议根据应用场景调整
            `top_p` 或 `temperature` 参数，但不要同时调整两个参数。
          format: float
          example: 1
          default: 1
          minimum: 0
          maximum: 1
        top_p:
          type: number
          description: >-
            核采样（`nucleus sampling`）参数，是`temperature`采样的替代方法，取值范围为 `[0.01,
            1.0]`，限两位小数。对于`GLM-5.1` `GLM-5` `GLM-5-Turbo` `GLM-4.7` `GLM-4.6`
            `GLM-4.5`系列默认值为 `0.95`，`GLM-4`系列默认值为
            `0.9`。模型只考虑累积概率达到`top_p`的候选词汇。例如：`0.1`表示只考虑前`10%`概率的词汇，`0.9`表示考虑前`90%`概率的词汇。较小的值会产生更集中、更一致的输出；较大的值会增加输出的多样性。建议根据应用场景调整
            `top_p` 或 `temperature` 参数，但不建议同时调整两个参数。
          format: float
          example: 0.95
          default: 0.95
          minimum: 0.01
          maximum: 1
        max_tokens:
          type: integer
          description: >-
            模型输出的最大令牌`token`数量限制。`GLM-5.1` `GLM-5` `GLM-5-Turbo` `GLM-4.7`
            `GLM-4.6`系列最大支持`128K`输出长度，`GLM-4.5`最大支持`96K`输出长度，建议设置不小于`1024`。令牌是文本的基本单位，通常`1`个令牌约等于`0.75`个英文单词或`1.5`个中文字符。设置合适的`max_tokens`可以控制响应长度和成本，避免过长的输出。如果模型在达到`max_tokens`限制前完成回答，会自然结束；如果达到限制，输出可能被截断。

            默认值和最大值等更多详见 [max_tokens
            文档](/cn/guide/start/concept-param#max_tokens)
          example: 1024
          minimum: 1
          maximum: 131072
        tools:
          type: array
          description: >-
            模型可以调用的工具列表。支持函数调用、知识库检索和网络搜索。使用此参数提供模型可以生成 `JSON`
            输入的函数列表或配置其他工具。最多支持 `128` 个函数。目前 `GLM-4` 系列已支持所有 `tools`，`GLM-4.5`
            已支持 `web search` 和 `retrieval`。
          anyOf:
            - items:
                $ref: '#/components/schemas/FunctionToolSchema'
            - items:
                $ref: '#/components/schemas/RetrievalToolSchema'
            - items:
                $ref: '#/components/schemas/WebSearchToolSchema'
            - items:
                $ref: '#/components/schemas/MCPToolSchema'
        tool_choice:
          oneOf:
            - type: string
              enum:
                - auto
              description: 用于控制模型选择调用哪个函数的方式，仅在工具类型为`function`时补充。默认`auto`且仅支持`auto`。
          description: 控制模型如何选择工具。
        stop:
          type: array
          description: >-
            停止词列表，当模型生成的文本中遇到这些指定的字符串时会立即停止生成。目前仅支持单个停止词，格式为["stop_word1"]。停止词不会包含在返回的文本中。这对于控制输出格式、防止模型生成不需要的内容非常有用，例如在对话场景中可以设置["Human:"]来防止模型模拟用户发言。
          items:
            type: string
          maxItems: 1
        response_format:
          type: object
          description: >-
            指定模型的响应输出格式，默认为`text`，仅文本模型支持此字段。支持两种格式：{ "type": "text" }
            表示普通文本输出模式，模型返回自然语言文本；{ "type": "json_object" }
            表示`JSON`输出模式，模型会返回有效的`JSON`格式数据，适用于结构化数据提取、`API`响应生成等场景。使用`JSON`模式时，建议在提示词中明确说明需要`JSON`格式输出。
          properties:
            type:
              type: string
              enum:
                - text
                - json_object
              default: text
              description: 输出格式类型：`text`表示普通文本输出，`json_object`表示`JSON`格式输出
          required:
            - type
        request_id:
          type: string
          description: 请求唯一标识符。由用户端传递，建议使用`UUID`格式确保唯一性，若未提供平台将自动生成。
        user_id:
          type: string
          description: 终端用户的唯一标识符。`ID`长度要求：最少`6`个字符，最多`128`个字符，建议使用不包含敏感信息的唯一标识。
          minLength: 6
          maxLength: 128
    AsyncChatCompletionVisionRequest:
      required:
        - model
        - messages
      type: object
      description: 视觉模型请求，支持多模态内容（文本、图片、视频、文件）
      properties:
        model:
          type: string
          description: 调用的视觉模型代码。`GLM-5V-Turbo` 系列支持视觉理解，具备卓越的多模态理解能力和工具调用能力。
          example: glm-5v-turbo
          default: glm-5v-turbo
          enum:
            - glm-5v-turbo
            - glm-4.6v
            - glm-4.6v-flash
            - glm-4.6v-flashx
            - glm-4v-flash
            - glm-4.1v-thinking-flashx
            - glm-4.1v-thinking-flash
        messages:
          type: array
          description: >-
            对话消息列表，包含当前对话的完整上下文信息。每条消息都有特定的角色和内容，模型会根据这些消息生成回复。消息按时间顺序排列，支持角色：`system`（系统消息，用于设定`AI`的行为和角色）、`user`（用户消息，来自用户的输入）、`assistant`（助手消息，来自`AI`的回复）。视觉模型支持纯文本和多模态内容（文本、图片、视频、文件）。注意不能只包含系统或助手消息。
          items:
            oneOf:
              - title: 用户消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - user
                    description: 消息作者的角色
                    default: user
                  content:
                    oneOf:
                      - type: array
                        description: 多模态消息内容，支持文本、图片、文件、视频（可从上方切换至文本消息）
                        items:
                          $ref: '#/components/schemas/VisionMultimodalContentItem'
                      - type: string
                        description: 文本消息内容（可从上方切换至多模态消息）
                        example: >-
                          What opportunities and challenges will the Chinese
                          large model industry face in 2025?
                required:
                  - role
                  - content
              - title: 系统消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - system
                    description: 消息作者的角色
                    default: system
                  content:
                    oneOf:
                      - type: string
                        description: 消息文本内容
                        example: You are a helpful assistant.
                required:
                  - role
                  - content
              - title: 助手消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - assistant
                    description: 消息作者的角色
                    default: assistant
                  content:
                    oneOf:
                      - type: string
                        description: 文本消息内容
                        example: I'll help you with that analysis.
                required:
                  - role
          minItems: 1
        thinking:
          $ref: '#/components/schemas/ChatThinking'
        do_sample:
          type: boolean
          example: true
          default: true
          description: >-
            是否启用采样策略来生成文本。默认值为 `true`。当设置为 `true` 时，模型会使用 `temperature、top_p`
            等参数进行随机采样，生成更多样化的输出；当设置为 `false` 时，模型总是选择概率最高的词汇，生成更确定性的输出，此时
            `temperature` 和 `top_p` 参数将被忽略。对于需要一致性和可重复性的任务（如代码生成、翻译），建议设置为
            `false`。
        temperature:
          type: number
          description: >-
            采样温度，控制输出的随机性和创造性，取值范围为 `[0.0,
            1.0]`，限两位小数。对于`GLM-5V-Turbo`，`GLM-4.6V`，`GLM-4.5V`系列默认值为
            `0.8`，`GLM-4.1v`系列默认值为
            `0.8`。较高的值（如`0.8`）会使输出更随机、更具创造性，适合创意写作和头脑风暴；较低的值（如`0.2`）会使输出更稳定、更确定，适合事实性问答和代码生成。建议根据应用场景调整
            `top_p` 或 `temperature` 参数，但不要同时调整两个参数。
          format: float
          example: 0.8
          default: 0.8
          minimum: 0
          maximum: 1
        top_p:
          type: number
          description: >-
            核采样（`nucleus sampling`）参数，是`temperature`采样的替代方法，取值范围为 `[0.01,
            1.0]`，限两位小数。对于`GLM-5V-Turbo`，`GLM-4.6V`，`GLM-4.5V`系列默认值为
            `0.6`，`GLM-4.1v`系列默认值为
            `0.6`。模型只考虑累积概率达到`top_p`的候选词汇。例如：`0.1`表示只考虑前`10%`概率的词汇，`0.9`表示考虑前`90%`概率的词汇。较小的值会产生更集中、更一致的输出；较大的值会增加输出的多样性。建议根据应用场景调整
            `top_p` 或 `temperature` 参数，但不要同时调整两个参数。
          format: float
          example: 0.6
          default: 0.6
          minimum: 0.01
          maximum: 1
        max_tokens:
          type: integer
          description: >-
            模型输出的最大令牌`token`数量限制。`GLM-5V-Turbo`最大支持`128K`输出长度，`GLM-4.6V`最大支持`32K`输出长度，`GLM-4.5V`最大支持`16K`输出长度，`GLM-4.1v`系列最大支持`16K`输出长度，建议设置不小于`1024`。令牌是文本的基本单位，通常`1`个令牌约等于`0.75`个英文单词或`1.5`个中文字符。设置合适的`max_tokens`可以控制响应长度和成本，避免过长的输出。如果模型在达到`max_tokens`限制前完成回答，会自然结束；如果达到限制，输出可能被截断。

            默认值和最大值等更多详见 [max_tokens
            文档](/cn/guide/start/concept-param#max_tokens)
          example: 1024
          minimum: 1
          maximum: 131072
        tools:
          type: array
          description: >-
            模型可以调用的工具列表。仅限`GLM-5V-Turbo`, `GLM-4.6V`支持。使用此参数提供模型可以生成 `JSON`
            输入的函数列表或配置其他工具。最多支持 `128` 个函数。
          anyOf:
            - items:
                $ref: '#/components/schemas/FunctionToolSchema'
        tool_choice:
          oneOf:
            - type: string
              enum:
                - auto
              description: 用于控制模型选择调用哪个函数的方式，仅在工具类型为`function`时补充。默认`auto`且仅支持`auto`。
          description: 控制模型如何选择工具。
        stop:
          type: array
          description: >-
            停止词列表，当模型生成的文本中遇到这些指定的字符串时会立即停止生成。目前仅支持单个停止词，格式为["stop_word1"]。停止词不会包含在返回的文本中。这对于控制输出格式、防止模型生成不需要的内容非常有用，例如在对话场景中可以设置["Human:"]来防止模型模拟用户发言。
          items:
            type: string
          maxItems: 1
        request_id:
          type: string
          description: 请求唯一标识符。由用户端传递，建议使用`UUID`格式确保唯一性，若未提供平台将自动生成。
        user_id:
          type: string
          description: 终端用户的唯一标识符。`ID`长度要求：最少`6`个字符，最多`128`个字符，建议使用不包含敏感信息的唯一标识。
          minLength: 6
          maxLength: 128
    AsyncChatCompletionAudioRequest:
      required:
        - model
        - messages
      type: object
      description: 音频模型请求，支持语音理解、生成和识别功能
      properties:
        model:
          type: string
          description: 调用的音频模型代码。`GLM-4-Voice` 支持语音理解和生成。
          example: glm-4-voice
          default: glm-4-voice
          enum:
            - glm-4-voice
            - 禁用仅占位
        messages:
          type: array
          description: >-
            对话消息列表，包含当前对话的完整上下文信息。每条消息都有特定的角色和内容，模型会根据这些消息生成回复。消息按时间顺序排列，支持角色：`system`（系统消息，用于设定`AI`的行为和角色）、`user`（用户消息，来自用户的输入）、`assistant`（助手消息，来自`AI`的回复）。音频模型支持文本和音频内容。注意不能只包含系统或助手消息。
          items:
            oneOf:
              - title: 用户消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - user
                    description: 消息作者的角色
                    default: user
                  content:
                    oneOf:
                      - type: array
                        description: 多模态消息内容，支持文本、音频
                        items:
                          $ref: '#/components/schemas/AudioMultimodalContentItem'
                      - type: string
                        description: 消息文本内容
                        example: You are a helpful assistant.
                required:
                  - role
                  - content
              - title: 系统消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - system
                    description: 消息作者的角色
                    default: system
                  content:
                    type: string
                    description: 消息文本内容
                    example: 你是一个专业的语音助手，能够理解和生成自然语音。
                required:
                  - role
                  - content
              - title: 助手消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - assistant
                    description: 消息作者的角色
                    default: assistant
                  content:
                    oneOf:
                      - type: string
                        description: 文本消息内容
                        example: I'll help you with that analysis.
                  audio:
                    type: object
                    description: 语音消息
                    properties:
                      id:
                        type: string
                        description: 语音消息`id`，用于多轮对话
                required:
                  - role
          minItems: 1
        do_sample:
          type: boolean
          example: true
          default: true
          description: >-
            是否启用采样策略来生成文本。默认值为 `true`。当设置为 `true` 时，模型会使用 `temperature、top_p`
            等参数进行随机采样，生成更多样化的输出；当设置为 `false` 时，模型总是选择概率最高的词汇，生成更确定性的输出，此时
            `temperature` 和 `top_p` 参数将被忽略。对于需要一致性和可重复性的任务（如语音识别、转录），建议设置为
            `false`。
        temperature:
          type: number
          description: >-
            采样温度，控制输出的随机性和创造性，取值范围为 `[0.0, 1.0]`，限两位小数。对于`GLM-4-Voice`默认值为
            `0.8`。较高的值（如`0.8`）会使输出更随机、更具创造性，适合语音生成和对话；较低的值（如`0.1`）会使输出更稳定、更确定，适合语音识别和转录。建议根据应用场景调整
            `top_p` 或 `temperature` 参数，但不要同时调整两个参数。
          format: float
          example: 0.8
          default: 0.8
          minimum: 0
          maximum: 1
        top_p:
          type: number
          description: >-
            核采样（`nucleus sampling`）参数，是`temperature`采样的替代方法，取值范围为 `[0.01,
            1.0]`，限两位小数。对于`GLM-4-Voice`默认值为
            `0.6`。模型只考虑累积概率达到`top_p`的候选词汇。例如：`0.1`表示只考虑前`10%`概率的词汇，`0.9`表示考虑前`90%`概率的词汇。较小的值会产生更集中、更一致的输出；较大的值会增加输出的多样性。建议根据应用场景调整
            `top_p` 或 `temperature` 参数，但不要同时调整两个参数。
          format: float
          example: 0.6
          default: 0.6
          minimum: 0.01
          maximum: 1
        max_tokens:
          type: integer
          description: 模型输出的最大令牌`token`数量限制。`GLM-4-Voice`最大支持`4K`输出长度，默认`1024`。令牌是文本的基本单位。
          example: 1024
          minimum: 1
          maximum: 4096
        watermark_enabled:
          type: boolean
          description: |-
            控制`AI`生成图片时是否添加水印。
             - `true`: 默认启用`AI`生成的显式水印及隐式数字水印，符合政策要求。
             - `false`: 关闭所有水印，仅允许已签署免责声明的客户使用，签署路径：个人中心-安全管理-去水印管理
          example: true
        stop:
          type: array
          description: >-
            停止词列表，当模型生成的文本中遇到这些指定的字符串时会立即停止生成。目前仅支持单个停止词，格式为["stop_word1"]。停止词不会包含在返回的文本中。这对于控制输出格式、防止模型生成不需要的内容非常有用。
          items:
            type: string
          maxItems: 1
        request_id:
          type: string
          description: 请求唯一标识符。由用户端传递，建议使用`UUID`格式确保唯一性，若未提供平台将自动生成。
        user_id:
          type: string
          description: 终端用户的唯一标识符。`ID`长度要求：最少`6`个字符，最多`128`个字符，建议使用不包含敏感信息的唯一标识。
          minLength: 6
          maxLength: 128
    AsyncChatCompletionHumanOidRequest:
      required:
        - model
        - messages
      type: object
      description: 角色扮演，专业心理咨询专用模型
      properties:
        model:
          type: string
          description: 调用的专用模型代码。`CharGLM-4` 是角色扮演专用模型，`Emohaa` 是专业心理咨询模型。
          example: charglm-4
          default: charglm-4
          enum:
            - charglm-4
            - emohaa
        meta:
          type: object
          description: 角色及用户信息数据(仅限 `Emohaa` 支持此参数)
          required:
            - user_info
            - bot_info
            - bot_name
            - user_name
          properties:
            user_info:
              type: string
              description: 用户信息描述
            bot_info:
              type: string
              description: 角色信息描述
            bot_name:
              type: string
              description: 角色名称
            user_name:
              type: string
              description: 用户名称
        messages:
          type: array
          description: >-
            对话消息列表，包含当前对话的完整上下文信息。每条消息都有特定的角色和内容，模型会根据这些消息生成回复。消息按时间顺序排列，支持角色：`system`（系统消息，用于设定`AI`的行为和角色）、`user`（用户消息，来自用户的输入）、`assistant`（助手消息，来自`AI`的回复）。注意不能只包含系统消息或助手消息。
          items:
            oneOf:
              - title: 用户消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - user
                    description: 消息作者的角色
                    default: user
                  content:
                    type: string
                    description: 文本消息内容
                    example: 我最近工作压力很大，经常感到焦虑，不知道该怎么办
                required:
                  - role
                  - content
              - title: 系统消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - system
                    description: 消息作者的角色
                    default: system
                  content:
                    type: string
                    description: 消息文本内容
                    example: >-
                      你乃苏东坡。人生如梦，何不活得潇洒一些？在这忙碌纷繁的现代生活中，帮助大家找到那份属于自己的自在与豁达，共赏人生之美好
                required:
                  - role
                  - content
              - title: 助手消息
                type: object
                properties:
                  role:
                    type: string
                    enum:
                      - assistant
                    description: 消息作者的角色
                    default: assistant
                  content:
                    type: string
                    description: 文本消息内容
                    example: I'll help you with that analysis.
                required:
                  - role
                  - content
          minItems: 1
        do_sample:
          type: boolean
          example: true
          default: true
          description: >-
            是否启用采样策略来生成文本。默认值为 `true`。当设置为 `true` 时，模型会使用 `temperature、top_p`
            等参数进行随机采样，生成更多样化的输出；当设置为 `false` 时，模型总是选择概率最高的词汇，生成更确定性的输出，此时
            `temperatur`e 和 `top_p` 参数将被忽略。对于需要一致性和可重复性的任务（如语音识别、转录），建议设置为
            `false`。
        temperature:
          type: number
          description: >-
            采样温度，控制输出的随机性和创造性，取值范围为 `[0.0, 1.0]`，限两位小数。`Charglm-4` 和 `Emohaa`
            默认值为 `0.95`。建议根据应用场景调整 `top_p` 或 `temperature` 参数，但不要同时调整两个参数。
          format: float
          example: 0.8
          default: 0.8
          minimum: 0
          maximum: 1
        top_p:
          type: number
          description: >-
            核采样（`nucleus sampling`）参数，是`temperature`采样的替代方法，取值范围为 `[0.01,
            1.0]`，限两位小数。`Charglm-4` 和 `Emohaa` 默认值为 `0.7`。建议根据应用场景调整 `top_p` 或
            `temperature` 参数，但不要同时调整两个参数。
          format: float
          example: 0.6
          default: 0.6
          minimum: 0.01
          maximum: 1
        max_tokens:
          type: integer
          description: >-
            模型输出的最大令牌`token`数量限制。`Charglm-4` 和 `Emohaa`
            最大支持`4K`输出长度，默认`1024`。令牌是文本的基本单位。
          example: 1024
          minimum: 1
          maximum: 4096
        stop:
          type: array
          description: >-
            停止词列表，当模型生成的文本中遇到这些指定的字符串时会立即停止生成。目前仅支持单个停止词，格式为["stop_word1"]。停止词不会包含在返回的文本中。这对于控制输出格式、防止模型生成不需要的内容非常有用。
          items:
            type: string
          maxItems: 1
        request_id:
          type: string
          description: 请求唯一标识符。由用户端传递，建议使用`UUID`格式确保唯一性，若未提供平台将自动生成。
        user_id:
          type: string
          description: 终端用户的唯一标识符。`ID`长度要求：最少`6`个字符，最多`128`个字符，建议使用不包含敏感信息的唯一标识。
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
    ChatThinking:
      type: object
      description: 仅 `GLM-4.5` 及以上模型支持此参数配置. 控制大模型是否开启思维链。
      properties:
        type:
          type: string
          description: >-
            是否开启思维链(当开启后 `GLM-5.1` `GLM-5`  `GLM-5v-Turbo` `GLM-4.7` `GLM-4.5V`
            为强制思考，`GLM-4.6` `GLM-4.6V` `GLM-4.5` 为模型自动判断是否思考), 默认: `enabled`.
          default: enabled
          enum:
            - enabled
            - disabled
        clear_thinking:
          type: boolean
          description: >-
            默认为 `True`。用于控制是否清除历史对话轮次（`previous turns`）中的 `reasoning_content`。详见
            [思考模式](/cn/guide/capabilities/thinking-mode) 
             - `true`（默认）：在本次请求中，系统会忽略/移除历史 `turns` 的 `reasoning_content`，仅使用非推理内容（如用户/助手可见文本、工具调用与结果等）作为上下文输入。适用于普通对话与轻量任务，可降低上下文长度与成本 
             - `false`：保留历史 `turns` 的 `reasoning_content` 并随上下文一同提供给模型。若你希望启用 `Preserved Thinking`，必须在 `messages` 中完整、未修改、按原顺序透传历史 `reasoning_content`；缺失、裁剪、改写或重排会导致效果下降或无法生效。
             - 注意：该参数只影响跨 `turn` 的历史 `thinking blocks`；不改变模型在当前 `turn` 内是否产生/输出 `thinking`
          default: true
          example: true
    FunctionToolSchema:
      type: object
      title: Function Call
      properties:
        type:
          type: string
          default: function
          enum:
            - function
        function:
          $ref: '#/components/schemas/FunctionObject'
      required:
        - type
        - function
      additionalProperties: false
    RetrievalToolSchema:
      type: object
      title: Retrieval
      properties:
        type:
          type: string
          default: retrieval
          enum:
            - retrieval
        retrieval:
          $ref: '#/components/schemas/RetrievalObject'
      required:
        - type
        - retrieval
      additionalProperties: false
    WebSearchToolSchema:
      type: object
      title: Web Search
      properties:
        type:
          type: string
          default: web_search
          enum:
            - web_search
        web_search:
          $ref: '#/components/schemas/WebSearchObject'
      required:
        - type
        - web_search
      additionalProperties: false
    MCPToolSchema:
      type: object
      title: MCP
      properties:
        type:
          type: string
          default: mcp
          enum:
            - mcp
        mcp:
          $ref: '#/components/schemas/MCPObject'
      required:
        - type
        - mcp
      additionalProperties: false
    VisionMultimodalContentItem:
      oneOf:
        - title: 文本
          type: object
          properties:
            type:
              type: string
              enum:
                - text
              description: 内容类型为文本
              default: text
            text:
              type: string
              description: 文本内容
          required:
            - type
            - text
          additionalProperties: false
        - title: 图片
          type: object
          properties:
            type:
              type: string
              enum:
                - image_url
              description: 内容类型为图片`URL`
              default: image_url
            image_url:
              type: object
              description: 图片信息
              properties:
                url:
                  type: string
                  description: >-
                    图片的`URL`地址或`Base64`编码。图像大小上传限制为每张图像`5M`以下，且像素不超过`6000*6000`。支持`jpg、png、jpeg`格式。`GLM-5V-Turbo`
                    `GLM4.6V` `GLM4.5V` 系列限制`50`张。`GLM-4V-Plus-0111`
                    限制`5`张。`GLM-4V-Flash`限制`1`张图像且不支持`Base64`编码。
              required:
                - url
              additionalProperties: false
          required:
            - type
            - image_url
          additionalProperties: false
        - title: 视频
          type: object
          properties:
            type:
              type: string
              enum:
                - video_url
              description: 内容类型为视频输入
              default: video_url
            video_url:
              type: object
              description: 视频信息。注意：`GLM-4V-Plus-0111` 的 `video_url` 参数必须在 `content` 数组的第一位。
              properties:
                url:
                  type: string
                  description: >-
                    视频的`URL`地址。`GLM-5V-Turbo` `GLM-4.6V` `GLM-4.5V` 系列视频大小限制为
                    `200M`
                    以内。`GLM-4V-Plus`视频大小限制为`20M`以内，视频时长不超过`30s`。对于其他多模态模型，视频大小限制为`200M`以内。视频类型：`mp4`，`mkv`，`mov`。
              required:
                - url
              additionalProperties: false
          required:
            - type
            - video_url
          additionalProperties: false
        - title: 文件
          type: object
          properties:
            type:
              type: string
              enum:
                - file_url
              description: >-
                内容类型为文件输入(仅`GLM-5V-Turbo` `GLM-4.6V` `GLM-4.5V`支持，且不支持同时传入
                `file_url` 和 `image_url` 或 `video_url` 参数)
              default: file_url
            file_url:
              type: object
              description: 文件信息。
              properties:
                url:
                  type: string
                  description: >-
                    文件的`URL`地址，不支持`Base64`编码。支持`pdf、txt、word、jsonl、xlsx、pptx`等格式，最多支持`50`个。
              required:
                - url
              additionalProperties: false
          required:
            - type
            - file_url
          additionalProperties: false
    AudioMultimodalContentItem:
      oneOf:
        - title: 文本
          type: object
          properties:
            type:
              type: string
              enum:
                - text
              description: 内容类型为文本
              default: text
            text:
              type: string
              description: 文本内容
          required:
            - type
            - text
          additionalProperties: false
        - title: 音频
          type: object
          properties:
            type:
              type: string
              enum:
                - input_audio
              description: 内容类型为音频输入
              default: input_audio
            input_audio:
              type: object
              description: 音频信息，仅`glm-4-voice`支持音频输入
              properties:
                data:
                  type: string
                  description: 语音文件的`base64`编码。音频最长不超过 `10` 分钟。`1s`音频=`12.5 Tokens`，向上取整。
                format:
                  type: string
                  description: 语音文件的格式，支持`wav`和`mp3`
                  enum:
                    - wav
                    - mp3
              required:
                - data
                - format
              additionalProperties: false
          required:
            - type
            - input_audio
          additionalProperties: false
    FunctionObject:
      type: object
      properties:
        name:
          type: string
          description: 要调用的函数名称。必须是 `a-z、A-Z、0-9`，或包含下划线和破折号，最大长度为 `64`。
          minLength: 1
          maxLength: 64
          pattern: ^[a-zA-Z0-9_-]+$
        description:
          type: string
          description: 函数功能的描述，供模型选择何时以及如何调用函数。
        parameters:
          $ref: '#/components/schemas/FunctionParameters'
      required:
        - name
        - description
        - parameters
    RetrievalObject:
      type: object
      properties:
        knowledge_id:
          type: string
          description: 知识库 `ID`，从平台创建或获取
        prompt_template:
          type: string
          description: >-
            请求模型的提示模板，包含占位符 `{{ knowledge }}` 和 `{{ question }}`
            的自定义请求模板。默认模板：`在文档 `{{ knowledge }}` 中搜索问题 `{{question}}`
            的答案。如果找到答案，仅使用文档中的陈述进行回应；如果没有找到答案，使用你自己的知识回答并告知用户信息不来自文档。不要重复问题，直接开始答案。`
      required:
        - knowledge_id
    WebSearchObject:
      type: object
      properties:
        enable:
          type: boolean
          description: 是否启用搜索功能，默认值为 `false`，启用时设置为 `true`
        search_engine:
          type: string
          description: >-
            搜索引擎类型，默认为
            `search_std`；支持`search_std、search_pro、search_pro_sogou、search_pro_quark`。
          enum:
            - search_std
            - search_pro
            - search_pro_sogou
            - search_pro_quark
        search_query:
          type: string
          description: 强制触发搜索
        search_intent:
          type: string
          description: >-
            是否进行搜索意图识别，默认执行搜索意图识别。`true`：执行搜索意图识别，有搜索意图后执行搜索；`false`：跳过搜索意图识别，直接执行搜索
        count:
          type: integer
          description: >-
            返回结果的条数。可填范围：`1-50`，最大单次搜索返回`50`条，默认为`10`。支持的搜索引擎：`search_std、search_pro、search_pro_sogou`。对于`search_pro_sogou`:
            可选枚举值，`10、20、30、40、50`
          minimum: 1
          maximum: 50
        search_domain_filter:
          type: string
          description: |-
            用于限定搜索结果的范围，仅返回指定白名单域名的内容。
            白名单域名:（如 `www.example.com`）。
            支持的搜索引擎：`search_std、search_pro、search_pro_sogou`
        search_recency_filter:
          type: string
          description: >-
            搜索指定时间范围内的网页。默认为`noLimit`。可填值：`oneDay`（一天内）、`oneWeek`（一周内）、`oneMonth`（一个月内）、`oneYear`（一年内）、`noLimit`（不限，默认）。支持的搜索引擎：`search_std、search_pro、search_pro_sogou、search_pro_quark`
          enum:
            - oneDay
            - oneWeek
            - oneMonth
            - oneYear
            - noLimit
        content_size:
          type: string
          description: >-
            控制网页摘要的字数。默认值为`medium`。`medium`：返回摘要信息，满足大模型的基础推理需求。`high`：最大化上下文，信息量较大但内容详细，适合需要信息细节的场景。
          enum:
            - medium
            - high
        result_sequence:
          type: string
          description: 指定搜索结果返回的顺序是在模型回复结果之前还是之后，可选值：`before`、`after`，默认 `after`
          enum:
            - before
            - after
        search_result:
          type: boolean
          description: 是否返回搜索来源的详细信息，默认值 `false`
        require_search:
          type: boolean
          description: 是否强制搜索结果才返回回答，默认值 `false`
        search_prompt:
          type: string
          description: |-
            用于定制搜索结果处理的`Prompt`，默认`Prompt`：

            你是一位智能问答专家，具备整合信息的能力，能够进行时间识别、语义理解与矛盾信息清洗处理。
            当前日期是{{current_date}}，请以此时间为唯一基准，参考以下信息，全面、准确地回答用户问题。
            仅提炼有价值的内容用于回答，确保答案具有实时性与权威性，直接陈述答案，无需说明数据来源或内部处理过程。
      required:
        - search_engine
    MCPObject:
      type: object
      properties:
        server_label:
          description: >-
            `mcp server`标识，如果连接智谱的`mcp server`，以`mcp
            code`填充该字段，且无需填写`server_url`
          type: string
        server_url:
          description: '`mcp server`地址'
          type: string
        transport_type:
          description: 传输类型
          type: string
          default: streamable-http
          enum:
            - sse
            - streamable-http
        allowed_tools:
          description: 允许的工具集合
          type: array
          items:
            type: string
        headers:
          description: '`mcp server` 需要的鉴权信息'
          type: object
      required:
        - server_label
    FunctionParameters:
      type: object
      description: 使用 `JSON Schema` 定义的参数。必须传递 `JSON Schema` 对象以准确定义接受的参数。如果调用函数时不需要参数，则省略。
      additionalProperties: true
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      description: >-
        使用以下格式进行身份验证：Bearer [<your api
        key>](https://bigmodel.cn/usercenter/proj-mgmt/apikeys)

````