# OpenAPI 规范与 LLM API

**官方文档**: https://spec.openapis.org/oas/v3.1.0

## 概述

OpenAPI 规范（原 Swagger 规范）是一种描述 RESTful API 的标准语言无关描述格式。它允许人类和机器发现 API 的功能，是 LLM 代理理解 API 能力的重要参考。

## OpenAI OpenAPI 规范

OpenAI 官方提供了 OpenAPI 规范文件：

- **GitHub 仓库**: https://github.com/openai/openai-openapi
- **在线规范**: https://app.stainless.com/api/spec/documented/openai

### 关键端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/v1/chat/completions` | POST | 聊天完成 |
| `/v1/embeddings` | POST | 嵌入向量 |
| `/v1/models` | GET | 列出模型 |
| `/v1/models/{model}` | GET | 获取模型详情 |

## OpenAPI 3.1.0 核心概念

### 文档结构

```yaml
openapi: 3.1.0
info:
  title: LLM Proxy API
  version: 1.0.0
  description: 高性能 LLM API 代理服务
servers:
  - url: https://api.example.com/v1
    description: 生产环境
paths:
  /chat/completions:
    post:
      summary: 创建聊天完成
      description: 为给定对话创建模型响应
      operationId: createChatCompletion
      tags:
        - Chat Completion
      parameters: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ChatCompletionRequest'
      responses:
        '200':
          description: 成功响应
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ChatCompletionResponse'
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '429':
          $ref: '#/components/responses/RateLimit'
        '500':
          $ref: '#/components/responses/InternalError'
components:
  schemas:
    ChatCompletionRequest:
      type: object
      required:
        - model
        - messages
      properties:
        model:
          type: string
          description: 模型 ID
        messages:
          type: array
          items:
            $ref: '#/components/schemas/Message'
        temperature:
          type: number
          default: 1
    ChatCompletionResponse:
      type: object
      properties:
        id:
          type: string
        object:
          type: string
        created:
          type: integer
        model:
          type: string
        choices:
          type: array
          items:
            $ref: '#/components/schemas/Choice'
  responses:
    BadRequest:
      description: 请求参数错误
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
    RateLimit:
      description: 速率限制
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
```

## LLM API 工具调用规范

### OpenAI 函数调用格式

```yaml
components:
  schemas:
    FunctionDeclaration:
      type: object
      required:
        - name
        - description
        - parameters
      properties:
        name:
          type: string
          description: 函数名称
        description:
          type: string
          description: 函数功能描述
        parameters:
          type: object
          description: JSON Schema 格式的参数定义
          properties:
            type:
              type: string
              const: object
            properties:
              type: object
              additionalProperties:
                type: object
                properties:
                  type:
                    type: string
                  description:
                    type: string
                  enum:
                    type: array
                  default:
                    type: string
            required:
              type: array
              items:
                type: string
    ToolCall:
      type: object
      properties:
        id:
          type: string
          description: 工具调用 ID
        type:
          type: string
          const: function
        function:
          type: object
          properties:
            name:
              type: string
            arguments:
              type: string
          required:
            - name
            - arguments
```

### 工具调用示例

```yaml
components:
  schemas:
    getWeatherParams:
      type: object
      properties:
        location:
          type: string
          description: 城市名称
        unit:
          type: string
          enum:
            - celsius
            - fahrenheit
          default: celsius
      required:
        - location
    getWeatherTool:
      type: object
      properties:
        type:
          type: string
          const: function
        function:
          type: object
          properties:
            name:
              type: string
              const: get_weather
            description:
              type: string
              Get the current weather in a given location
            parameters:
              $ref: '#/components/schemas/getWeatherParams'
          required:
            - name
            - description
            - parameters
```

## 参考资料

- [OpenAPI 3.1.0 规范](https://spec.openapis.org/oas/v3.1.0)
- [OpenAI OpenAPI 规范](https://github.com/openai/openai-openapi)
- [Swagger Editor](https://editor.swagger.io/)
