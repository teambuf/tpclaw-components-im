/*
 * Copyright 2026 The TPClaw Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"context"

	"github.com/rulego/rulego/api/types/endpoint"
)

// ResponseType defines the type of response format.
type ResponseType string

const (
	// ResponseTypePlainText is a plain text response
	ResponseTypePlainText ResponseType = "plain"

	// ResponseTypeJSON is a JSON response
	ResponseTypeJSON ResponseType = "json"

	// ResponseTypeEncrypted is an encrypted response (WeCom)
	ResponseTypeEncrypted ResponseType = "encrypted"

	// ResponseTypeStream is a streaming response for AI
	ResponseTypeStream ResponseType = "stream"
)

// ProcessorType defines the type of processor.
// 统一的处理器类型，所有平台适配器应使用这些标准类型
type ProcessorType string

const (
	// ProcessorTypeDecrypt decrypts incoming messages
	// 解密传入消息
	ProcessorTypeDecrypt ProcessorType = "decrypt"

	// ProcessorTypeTransform transforms messages to unified format
	// 转换消息为统一格式
	ProcessorTypeTransform ProcessorType = "transform"

	// ProcessorTypeEncryptResponse encrypts outgoing responses
	// 加密传出响应（企业微信等需要）
	ProcessorTypeEncryptResponse ProcessorType = "encryptResponse"

	// ProcessorTypeStreamResponse handles streaming responses
	// 处理流式响应（AI 场景）
	ProcessorTypeStreamResponse ProcessorType = "streamResponse"

	// ProcessorTypeAck handles simple ACK responses to platform
	// 处理简单的 ACK 响应（确认收到消息）
	ProcessorTypeAck ProcessorType = "ack"

	// ProcessorTypeSuccessResponse handles success response formatting
	// 处理成功响应格式化
	ProcessorTypeSuccessResponse ProcessorType = "successResponse"

	// ProcessorTypeURLVerify handles URL verification callbacks
	// 处理 URL 验证回调（平台配置验证）
	ProcessorTypeURLVerify ProcessorType = "urlVerify"

	// ProcessorTypeVerifySignature verifies request signature
	// 验证请求签名（可选，某些平台在 URL 验证之外需要）
	ProcessorTypeVerifySignature ProcessorType = "verifySignature"
)

// IMAdapter defines the interface for IM platform adapters.
// Each platform (Feishu, DingTalk, WeCom) should implement this interface.
type IMAdapter interface {
	// Platform returns the platform identifier.
	Platform() string

	// ParseMessage parses the raw request into a unified IMMessage.
	ParseMessage(ctx context.Context, body []byte, headers, params map[string]string) (*IMMessage, error)

	// VerifySignature verifies the request signature.
	VerifySignature(body []byte, headers, params map[string]string) error

	// HandleChallenge handles URL verification challenges.
	// Returns (response, handled, error). If handled is false, continue normal processing.
	HandleChallenge(body []byte) (response []byte, handled bool, err error)

	// FormatResponse formats the response for the platform.
	FormatResponse(msg *IMMessage, responseType ResponseType) ([]byte, error)

	// CreateProcessor creates a platform-specific processor.
	CreateProcessor(processorType ProcessorType, config interface{}) (endpoint.Process, error)
}

// MediaDownloader defines the interface for media downloading capability.
// 平台适配器可选实现此接口以支持媒体文件下载
type MediaDownloader interface {
	// DownloadMedia downloads a media file by media ID.
	// DownloadMedia 通过媒体 ID 下载媒体文件
	// Returns (fileData, mimeType, error)
	DownloadMedia(ctx context.Context, mediaID string) ([]byte, string, error)

	// DownloadImage downloads an image by URL or media ID.
	// DownloadImage 通过 URL 或媒体 ID 下载图片
	// Returns (fileData, mimeType, error)
	DownloadImage(ctx context.Context, imageKey string) ([]byte, string, error)
}

// ProcessorFactory is a function that creates a processor.
type ProcessorFactory func(config interface{}) (endpoint.Process, error)

// Predefined processor names for registration.
// 预定义的处理器注册名称，格式为 im/{platform}/{type}
const (
	// ProcessorMessageTransform is the unified message transform processor
	ProcessorMessageTransform = "im/transform"

	// ProcessorAck is the simple ACK response processor
	ProcessorAck = "im/ack"

	// Feishu processors
	ProcessorFeishuDecrypt   = "im/feishu/decrypt"
	ProcessorFeishuURLVerify = "im/feishu/urlVerify"
	ProcessorFeishuAck       = "im/feishu/ack"

	// DingTalk processors
	ProcessorDingTalkURLVerify = "im/dingtalk/urlVerify"
	ProcessorDingTalkVerifySig = "im/dingtalk/verifySignature"
	ProcessorDingTalkAck       = "im/dingtalk/ack"

	// WeCom processors
	ProcessorWeComURLVerify   = "im/wecom/urlVerify"
	ProcessorWeComDecrypt     = "im/wecom/decrypt"
	ProcessorWeComEncryptResp = "im/wecom/encryptResponse"
	ProcessorWeComStreamResp  = "im/wecom/streamResponse"
	ProcessorWeComAck         = "im/wecom/ack"

	// Send processors
	ProcessorFeishuSend = "im/feishu/send"
)

// Additional metadata keys for internal use (platform-specific extensions)
// Note: Common metadata keys are defined in metadata.go
const (
	// MetaFeishuStreamChat is used for Feishu stream chat info
	MetaFeishuStreamChat = "im.feishuStreamChat"
)
