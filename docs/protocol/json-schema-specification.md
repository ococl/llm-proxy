# JSON Schema 规范参考

**当前版本**: 2020-12  
**官方文档**: https://json-schema.org/

## 概述

JSON Schema 是一种用于描述 JSON 数据结构的声明性格式。它可以用于：
- 验证 JSON 数据的格式
- 生成 API 文档
- 创建测试用例
- 配置数据验证规则

## 主要版本

| 版本 | 发布时间 | 状态 |
|------|----------|------|
| Draft-07 | 2018年3月 | 已稳定 |
| 2019-09 | 2019年9月 | 已稳定 |
| 2020-12 | 2020年12月 | 当前版本 |

## 核心概念

### Schema

所有 JSON Schema 都以对象形式表示：

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object"
}
```

### 元 Schema

元 Schema 用于验证 Schema 本身的格式：

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#"
}
```

## 基本类型

### 类型定义

```json
{
  "type": "string",
  "type": "number",
  "type": "integer",
  "type": "boolean",
  "type": "array",
  "type": "object",
  "type": "null"
}
```

### 多类型联合

```json
{
  "type": ["string", "number"]
}
```

## 字符串验证

```json
{
  "type": "string",
  "minLength": 1,
  "maxLength": 100,
  "pattern": "^[a-zA-Z]+$",
  "format": "email",
  "format": "uri",
  "format": "date-time",
  "format": "ipv4",
  "format": "ipv6"
}
```

## 数值验证

```json
{
  "type": "number",
  "minimum": 0,
  "maximum": 100,
  "exclusiveMinimum": 0,
  "exclusiveMaximum": 100,
  "multipleOf": 0.5
}
```

## 数组验证

```json
{
  "type": "array",
  "items": {
    "type": "string"
  },
  "minItems": 1,
  "maxItems": 10,
  "uniqueItems": true,
  "contains": {
    "type": "number"
  }
}
```

### 元组类型

```json
{
  "type": "array",
  "prefixItems": [
    {"type": "string"},
    {"type": "number"}
  ],
  "items": {
    "type": "string"
  }
}
```

## 对象验证

```json
{
  "type": "object",
  "properties": {
    "name": {
      "type": "string"
    },
    "age": {
      "type": "integer",
      "minimum": 0
    }
  },
  "required": ["name", "age"],
  "minProperties": 1,
  "maxProperties": 10,
  "additionalProperties": false,
  "propertyNames": {
    "type": "string",
    "pattern": "^[a-z]+$"
  },
  "dependentRequired": {
    "billing_address": ["street"]
  }
}
```

## 复杂验证

### 枚举

```json
{
  "enum": ["red", "green", "blue"]
}
```

### 常量

```json
{
  "const": "fixed_value"
}
```

### 条件组合

```json
{
  "allOf": [
    {"type": "string"},
    {"minLength": 5}
  ],
  "anyOf": [
    {"type": "string"},
    {"type": "number"}
  ],
  "oneOf": [
    {"type": "string"},
    {"type": "number"}
  ],
  "not": {
    "type": "boolean"
  }
}
```

### 条件逻辑

```json
{
  "type": "object",
  "properties": {
    "isPremium": {
      "type": "boolean"
    },
    "discount": {
      "type": "number"
    }
  },
  "if": {
    "properties": {
      "isPremium": {"const": true}
    }
  },
  "then": {
    "properties": {
      "discount": {
        "minimum": 0.1,
        "maximum": 0.5
      }
    }
  },
  "else": {
    "properties": {
      "discount": {
        "maximum": 0.1
      }
    }
  }
}
```

## 引用 (References)

### 内部引用

```json
{
  "definitions": {
    "address": {
      "type": "object",
      "properties": {
        "street": {"type": "string"},
        "city": {"type": "string"}
      }
    }
  },
  "type": "object",
  "properties": {
    "billing_address": {"$ref": "#/definitions/address"},
    "shipping_address": {"$ref": "#/definitions/address"}
  }
}
```

### 外部引用

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "https://example.com/schemas/user.json",
  "type": "object",
  "properties": {
    "name": {"type": "string"}
  }
}
```

```json
{
  "$ref": "https://example.com/schemas/user.json"
}
```

## 字符串格式

### 预定义格式

| 格式 | 描述 | 正则示例 |
|------|------|----------|
| `date-time` | ISO 8601 日期时间 | `2024-01-24T12:00:00Z` |
| `date` | ISO 8601 日期 | `2024-01-24` |
| `time` | ISO 8601 时间 | `12:00:00` |
| `email` | 电子邮件地址 | |
| `hostname` | 主机名 | |
| `ipv4` | IPv4 地址 | |
| `ipv6` | IPv6 地址 | |
| `uri` | URI | |
| `uri-reference` | URI 引用 | |
| `uuid` | UUID | |

### 2020-12 版本新增格式

| 格式 | 描述 |
|------|------|
| `duration` | ISO 8601 持续时间 |
| `json-pointer` | JSON Pointer |
| `relative-json-pointer` | 相对 JSON Pointer |
| `regex` | 正则表达式 |

## 常用关键词速查

| 关键词 | 描述 |
|--------|------|
| `$schema` | 声明 JSON Schema 版本 |
| `$id` | Schema 标识符 |
| `$ref` | 引用其他 Schema |
| `type` | 数据类型 |
| `enum` | 枚举值 |
| `const` | 常量值 |
| `properties` | 对象属性定义 |
| `items` | 数组元素定义 |
| `required` | 必需属性 |
| `minimum/maximum` | 数值范围 |
| `minLength/maxLength` | 字符串长度 |
| `pattern` | 正则匹配 |
| `format` | 字符串格式 |
| `allOf/anyOf/oneOf/not` | 逻辑组合 |
| `if/then/else` | 条件验证 |

## 在 LLM 代理中的应用

### 请求验证 Schema 示例

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Chat Completion Request",
  "type": "object",
  "required": ["model", "messages"],
  "properties": {
    "model": {
      "type": "string",
      "description": "模型 ID"
    },
    "messages": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["role", "content"],
        "properties": {
          "role": {
            "type": "string",
            "enum": ["system", "user", "assistant"]
          },
          "content": {
            "type": ["string", "array"]
          }
        }
      }
    },
    "temperature": {
      "type": "number",
      "minimum": 0,
      "maximum": 2,
      "default": 1
    },
    "max_tokens": {
      "type": "integer",
      "minimum": 1
    },
    "stream": {
      "type": "boolean",
      "default": false
    }
  }
}
```

### 响应验证 Schema 示例

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Chat Completion Response",
  "type": "object",
  "required": ["id", "object", "created", "model", "choices"],
  "properties": {
    "id": {
      "type": "string",
      "pattern": "^chatcmpl-[A-Za-z0-9]+$"
    },
    "object": {
      "type": "string",
      "const": "chat.completion"
    },
    "created": {
      "type": "integer",
      "minimum": 0
    },
    "model": {
      "type": "string"
    },
    "choices": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["index", "message", "finish_reason"],
        "properties": {
          "index": {
            "type": "integer"
          },
          "message": {
            "type": "object",
            "required": ["role", "content"],
            "properties": {
              "role": {
                "type": "string",
                "const": "assistant"
              },
              "content": {
                "type": ["string", "null"]
              }
            }
          },
          "finish_reason": {
            "type": "string",
            "enum": ["stop", "length", "tool_calls"]
          }
        }
      }
    },
    "usage": {
      "type": "object",
      "required": ["prompt_tokens", "completion_tokens", "total_tokens"]
    }
  }
}
```

## 元 Schema

### Draft-07 元 Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "http://json-schema.org/draft-07/schema#"
}
```

### 2020-12 元 Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema#",
  "$id": "https://json-schema.org/draft/2020-12/schema"
}
```

## 验证工具

### 在线验证器

- [JSON Schema Validator](https://www.jsonschemavalidator.net/)
- [JSON Schema Lint](https://jsonschemalint.com/)

### Go 库

```go
import (
    "github.com/xeipuuv/gojsonschema"
)

func ValidateJSON(schemaPath string, data interface{}) (*gojsonschema.Result, error) {
    schemaLoader := gojsonschema.NewReferenceLoader(schemaPath)
    documentLoader := gojsonschema.NewGoLoader(data)
    
    return gojsonschema.Validate(schemaLoader, documentLoader)
}
```

### JavaScript/TypeScript 库

```typescript
import Ajv from 'ajv';

const ajv = new Ajv();
const validate = ajv.compile(schema);
const valid = validate(data);
```

## 最佳实践

1. **使用最新版本**: 推荐使用 2020-12 版本
2. **明确类型**: 始终指定 `type` 字段
3. **添加描述**: 使用 `description` 提供文档
4. **定义引用**: 使用 `$ref` 避免重复
5. **合理约束**: 不要过度限制，给 API 调用方灵活性
6. **版本控制**: 在 `$schema` 中声明版本
7. **使用枚举**: 对有限取值使用 `enum`

## 参考资料

- [JSON Schema 官方文档](https://json-schema.org/)
- [JSON Schema 规范 (2020-12)](https://json-schema.org/draft/2020-12/json-schema-core.html)
- [JSON Schema Validation (2020-12)](https://json-schema.org/draft/2020-12/json-schema-validation.html)
- [Understanding JSON Schema](https://json-schema.org/understanding-json-schema/)
