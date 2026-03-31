# tpclaw-components-im

IM（即时通讯）平台适配器组件库，为 [tpclaw](../tpclaw) 提供统一的多平台 IM 消息处理能力。

## 功能特性

- **统一消息格式** - 将不同 IM 平台的消息转换为统一的 `IMMessage` 格式
- **多平台支持** - 支持飞书、钉钉、企业微信
- **可扩展架构** - 通过适配器模式轻松扩展新平台
- **处理器注册机制** - 灵活的消息处理器注册与调用

## 目录结构

```
tpclaw-components-im/
├── api/            # 核心接口定义
│   ├── adapter.go  # 适配器接口 (IMAdapter)
│   ├── message.go  # 统一消息格式 (IMMessage)
│   └── metadata.go # 元数据常量定义
├── adapter/        # 平台适配器实现
│   ├── feishu/     # 飞书适配器
│   ├── dingtalk/   # 钉钉适配器
│   └── wecom/      # 企业微信适配器
├── client/         # 平台客户端（消息发送）
├── endpoint/       # 端点实现（WebSocket 等）
├── processor/      # 消息处理器
└── internal/       # 内部工具包
```

## 支持的平台

| 平台 | 适配器路径 | 功能 |
|------|-----------|------|
| 飞书 | `adapter/feishu` | 消息接收、发送、卡片消息、WebSocket |
| 钉钉 | `adapter/dingtalk` | 消息接收、发送、媒体下载 |
| 企业微信 | `adapter/wecom` | 消息接收、发送、加密响应 |

## 快速开始

### 安装

```bash
go get github.com/teambuf/tpclaw-components-im
```

### 使用适配器

```go
import (
    "github.com/teambuf/tpclaw-components-im/adapter/feishu"
    "github.com/teambuf/tpclaw-components-im/api"
)

// 创建飞书适配器
adapter := feishu.NewAdapter(
    feishu.WithAppID("your-app-id"),
    feishu.WithAppSecret("your-app-secret"),
)

// 解析消息
msg, err := adapter.ParseMessage(ctx, body, headers, params)

// 获取平台标识
platform := adapter.Platform() // "feishu"
```

### 使用处理器

```go
import (
    "github.com/teambuf/tpclaw-components-im/processor"
    "github.com/teambuf/tpclaw-components-im/api"
)

// 从全局注册表创建处理器
p, err := processor.CreateProcessor(api.ProcessorMessageTransform, nil)

// 或使用预定义的处理器名称
p, err := processor.CreateProcessor("im/feishu/decrypt", config)
```

## 核心接口

### IMAdapter

适配器接口定义了 IM 平台需要实现的核心方法：

```go
type IMAdapter interface {
    Platform() string
    ParseMessage(ctx context.Context, body []byte, headers, params map[string]string) (*IMMessage, error)
    VerifySignature(body []byte, headers, params map[string]string) error
    HandleChallenge(body []byte) (response []byte, handled bool, err error)
    FormatResponse(msg *IMMessage, responseType ResponseType) ([]byte, error)
    CreateProcessor(processorType ProcessorType, config interface{}) (endpoint.Process, error)
}
```

### IMMessage

统一的消息格式：

```go
type IMMessage struct {
    ID        string                 // 消息 ID
    Platform  string                 // 平台标识
    Timestamp time.Time               // 时间戳
    ChatID    string                  // 会话 ID
    ChatType  string                  // 会话类型
    Sender    *IMSender               // 发送者信息
    MsgType   string                  // 消息类型
    EventType string                  // 事件类型
    Content   string                  // 消息内容
    Extensions map[string]interface{} // 平台扩展字段
}
```

## 处理器类型

| 处理器 | 名称 | 说明 |
|--------|------|------|
| 消息转换 | `im/transform` | 统一消息格式转换 |
| ACK 响应 | `im/ack` | 简单确认响应 |
| 飞书解密 | `im/feishu/decrypt` | 飞书消息解密 |
| 飞书 URL 验证 | `im/feishu/urlVerify` | 飞书回调 URL 验证 |
| 钉钉签名验证 | `im/dingtalk/verifySignature` | 钉钉请求签名验证 |
| 企业微信解密 | `im/wecom/decrypt` | 企业微信消息解密 |
| 企业微信加密响应 | `im/wecom/encryptResponse` | 企业微信响应加密 |

## 依赖

- [rulego](https://github.com/rulego/rulego) - 规则引擎
- [larksuite/oapi-sdk-go](https://github.com/larksuite/oapi-sdk-go) - 飞书 SDK

## 许可证

Apache License 2.0