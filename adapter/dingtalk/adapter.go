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

// Package dingtalk provides DingTalk IM platform adapter implementation.
// 钉钉适配器实现包
package dingtalk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/teambuf/tpclaw-components-im/api"
	"time"

	"github.com/rulego/rulego/api/types/endpoint"
)

// DingTalkAdapter implements imapi.IMAdapter interface for DingTalk platform.
// DingTalkAdapter 实现钉钉平台的 IMAdapter 接口
type DingTalkAdapter struct {
	config *DingTalkConfig
	crypto *Crypto
}

// DingTalkConfig holds the configuration for DingTalk adapter.
// DingTalkConfig 钉钉适配器配置
type DingTalkConfig struct {
	// AppKey 钉钉应用 Key
	AppKey string `json:"appKey"`
	// AppSecret 钉钉应用密钥
	AppSecret string `json:"appSecret"`
	// Token 验证 Token（用于回调签名验证）
	Token string `json:"token"`
	// AESKey 加密密钥（可选）
	AESKey string `json:"aesKey"`
}

// Ensure DingTalkAdapter implements imapi.IMAdapter interface
var _ api.IMAdapter = (*DingTalkAdapter)(nil)

// NewDingTalkAdapter creates a new DingTalk adapter with the given configuration.
// NewDingTalkAdapter 创建新的钉钉适配器
func NewDingTalkAdapter(config *DingTalkConfig) *DingTalkAdapter {
	var crypto *Crypto
	if config.Token != "" {
		crypto, _ = NewCrypto(config.Token)
	}
	return &DingTalkAdapter{
		config: config,
		crypto: crypto,
	}
}

// Platform returns the platform identifier.
// Platform 返回平台标识
func (a *DingTalkAdapter) Platform() string {
	return api.PlatformDingTalk
}

// ParseMessage parses the raw request into a unified IMMessage.
// ParseMessage 解析原始请求为统一的 IMMessage
func (a *DingTalkAdapter) ParseMessage(ctx context.Context, body []byte, headers, params map[string]string) (*api.IMMessage, error) {
	var event DingTalkEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("failed to parse dingtalk event: %w", err)
	}

	// Extract message content
	content := event.Content.Content
	if content == "" && event.Text != "" {
		content = event.Text
	}

	// Determine event type
	eventType := event.MsgType
	if event.MsgType == "card" {
		eventType = api.EventTypeCardClick
	} else {
		eventType = api.EventTypeMessageReceived
	}

	// Build unified message
	msg := &api.IMMessage{
		ID:        event.MsgId,
		Platform:  api.PlatformDingTalk,
		Timestamp: time.Unix(event.CreateTime/1000, 0),
		ChatID:    event.ConversationID,
		ChatType:  a.convertChatType(event.ChatType),
		Sender: &api.IMSender{
			UserID:  event.SenderId,
			Name:    event.SenderNick,
			StaffID: event.SenderStaffId,
			CorpID:  event.SenderCorpId,
			IsAdmin: event.IsAdmin,
		},
		MsgType:    event.MsgType,
		EventType:  eventType,
		Content:    content,
		RawData:    body,
		Extensions: make(map[string]interface{}),
	}

	// Add platform-specific extensions
	msg.Extensions["conversationType"] = event.ConversationType
	msg.Extensions["sessionWebhook"] = event.SessionWebhook
	msg.Extensions["chatbotUserId"] = event.ChatbotUserId

	// Add at users
	if len(event.AtUsers) > 0 {
		atUsers := make([]map[string]string, len(event.AtUsers))
		for i, u := range event.AtUsers {
			atUsers[i] = map[string]string{
				"dingtalkId": u.DingTalkId,
				"staffId":    u.StaffId,
			}
		}
		msg.Extensions["atUsers"] = atUsers
	}

	// Extract media information based on content type
	a.extractMediaInfo(&event, msg)

	return msg, nil
}

// extractMediaInfo extracts media information from the DingTalk event
// similar to Feishu's extractMediaKeys method
func (a *DingTalkAdapter) extractMediaInfo(event *DingTalkEvent, msg *api.IMMessage) {
	if msg.Extensions == nil {
		msg.Extensions = make(map[string]interface{})
	}

	contentType := event.Content.ContentType

	switch contentType {
	case "picture", "image":
		// 图片消息
		if event.Content.MediaId != "" {
			msg.Extensions["mediaId"] = event.Content.MediaId
			msg.Extensions["imageKey"] = event.Content.MediaId
			msg.Content = "[图片]"
			msg.Extensions[api.MetaHasMedia] = "true"
		}

	case "audio", "voice":
		// 语音消息
		if event.Content.MediaId != "" {
			msg.Extensions["mediaId"] = event.Content.MediaId
			msg.Extensions["fileKey"] = event.Content.MediaId
			msg.Content = "[语音]"
			msg.Extensions[api.MetaHasMedia] = "true"
		}

	case "video":
		// 视频消息
		if event.Content.MediaId != "" {
			msg.Extensions["mediaId"] = event.Content.MediaId
			msg.Extensions["fileKey"] = event.Content.MediaId
			msg.Content = "[视频]"
			msg.Extensions[api.MetaHasMedia] = "true"
		}

	case "file":
		// 文件消息
		if event.Content.MediaId != "" {
			msg.Extensions["mediaId"] = event.Content.MediaId
			msg.Extensions["fileKey"] = event.Content.MediaId
			msg.Content = "[文件]"
			msg.Extensions[api.MetaHasMedia] = "true"
		}

	case "richText":
		// 富文本消息 - 可能包含图片
		if event.Content.RichContent != "" {
			msg.Extensions["richContent"] = event.Content.RichContent
			// 尝试从富文本中提取图片 key（简单实现）
			// 实际可能需要解析富文本 JSON 结构
		}

	default:
		// 对于其他类型，检查是否有 MediaId
		if event.Content.MediaId != "" {
			msg.Extensions["mediaId"] = event.Content.MediaId
			msg.Extensions["fileKey"] = event.Content.MediaId
			msg.Extensions[api.MetaHasMedia] = "true"
		}
	}
}

// VerifySignature verifies the request signature using HMAC-SHA256.
// VerifySignature 使用 HMAC-SHA256 验证请求签名
func (a *DingTalkAdapter) VerifySignature(body []byte, headers, params map[string]string) error {
	// If no secret configured, skip verification
	if a.config.AppSecret == "" {
		return nil
	}

	timestamp := headers["timestamp"]
	signature := headers["sign"]

	// If no signature headers, skip verification (optional)
	if timestamp == "" || signature == "" {
		return nil
	}

	// DingTalk signature algorithm: stringToSign = timestamp + "\n" + appSecret
	stringToSign := timestamp + "\n" + a.config.AppSecret
	h := hmac.New(sha256.New, []byte(a.config.AppSecret))
	h.Write([]byte(stringToSign))
	expected := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if signature != expected {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// HandleChallenge handles URL verification challenges.
// HandleChallenge 处理 URL 验证挑战
// DingTalk does not use challenge mechanism like Feishu, returns nil, false, nil
func (a *DingTalkAdapter) HandleChallenge(body []byte) (response []byte, handled bool, err error) {
	// DingTalk does not have URL verification challenge
	// Return nil, false, nil to indicate not handled
	return nil, false, nil
}

// HandleURLVerification handles GET request URL verification from DingTalk.
// This is called when DingTalk sends a GET request to verify the callback URL.
func (a *DingTalkAdapter) HandleURLVerification(params map[string]string) ([]byte, error) {
	if a.crypto == nil {
		return nil, errors.New("dingtalk: token not configured")
	}

	signature := params["signature"]
	timestamp := params["timestamp"]
	nonce := params["nonce"]

	// Verify signature
	if !a.crypto.VerifySignature(signature, timestamp, nonce) {
		return nil, ErrInvalidSignature
	}

	// DingTalk expects "success" as the response for URL verification
	return []byte("success"), nil
}

// FormatResponse formats the response for the DingTalk platform.
// FormatResponse 格式化钉钉平台的响应
func (a *DingTalkAdapter) FormatResponse(msg *api.IMMessage, responseType api.ResponseType) ([]byte, error) {
	switch responseType {
	case api.ResponseTypePlainText:
		return a.formatTextResponse(msg)
	case api.ResponseTypeJSON:
		return a.formatJSONResponse(msg)
	default:
		return a.formatTextResponse(msg)
	}
}

// formatTextResponse formats a plain text response
func (a *DingTalkAdapter) formatTextResponse(msg *api.IMMessage) ([]byte, error) {
	resp := DingTalkTextMessage{
		MsgType: "text",
	}
	resp.Content.Content = msg.Content

	return json.Marshal(resp)
}

// formatJSONResponse formats a JSON response
func (a *DingTalkAdapter) formatJSONResponse(msg *api.IMMessage) ([]byte, error) {
	// Return standard success response
	return json.Marshal(DingTalkAPIResponse{
		Errcode: 0,
		Errmsg:  "success",
	})
}

// CreateProcessor creates a platform-specific processor.
// CreateProcessor 创建平台特定的处理器
func (a *DingTalkAdapter) CreateProcessor(processorType api.ProcessorType, config interface{}) (endpoint.Process, error) {
	switch processorType {
	case api.ProcessorTypeTransform:
		return a.createTransformProcessor(config)
	case api.ProcessorTypeAck:
		return a.createAckProcessor(config)
	case api.ProcessorTypeURLVerify:
		return a.createURLVerifyProcessor(config)
	case api.ProcessorTypeVerifySignature:
		return a.createVerifySignatureProcessor(config)
	default:
		return nil, fmt.Errorf("unsupported processor type: %s", processorType)
	}
}

// createTransformProcessor creates a message transform processor
func (a *DingTalkAdapter) createTransformProcessor(config interface{}) (endpoint.Process, error) {
	return func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		// Get request body
		in := exchange.In
		if in == nil {
			return false
		}

		body := in.Body()
		if len(body) == 0 {
			return true
		}

		// Parse DingTalk event
		var event DingTalkEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return true
		}

		// Store event in exchange for later use
		if exchange.In.Headers() != nil {
			exchange.In.Headers().Set("dingtalk-event-id", event.MsgId)
			exchange.In.Headers().Set("dingtalk-conversation-id", event.ConversationID)
		}

		return true
	}, nil
}

// createAckProcessor creates an ACK response processor
func (a *DingTalkAdapter) createAckProcessor(config interface{}) (endpoint.Process, error) {
	return func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		if exchange.Out != nil {
			response, _ := json.Marshal(DingTalkAPIResponse{
				Errcode: 0,
				Errmsg:  "success",
			})
			exchange.Out.SetBody(response)
			exchange.Out.SetStatusCode(200)
		}
		return true
	}, nil
}

// createURLVerifyProcessor creates a URL verification processor for DingTalk GET callback
func (a *DingTalkAdapter) createURLVerifyProcessor(config interface{}) (endpoint.Process, error) {
	return func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		// Build params map from query string
		params := map[string]string{
			"signature": exchange.In.GetParam("signature"),
			"timestamp": exchange.In.GetParam("timestamp"),
			"nonce":     exchange.In.GetParam("nonce"),
		}

		result, err := a.HandleURLVerification(params)
		if err != nil {
			exchange.Out.SetError(err)
			return false
		}

		exchange.Out.SetBody(result)
		exchange.Out.Headers().Set("Content-Type", "text/plain")
		return false // Stop processing, response is ready
	}, nil
}

// createVerifySignatureProcessor creates a signature verify processor
func (a *DingTalkAdapter) createVerifySignatureProcessor(config interface{}) (endpoint.Process, error) {
	return func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		if exchange.In == nil {
			return false
		}

		// Build headers map
		headers := make(map[string]string)
		if h := exchange.In.Headers(); h != nil {
			for key, values := range h {
				if len(values) > 0 {
					headers[key] = values[0]
				}
			}
		}

		// Verify signature
		if err := a.VerifySignature(exchange.In.Body(), headers, nil); err != nil {
			if exchange.Out != nil {
				exchange.Out.SetStatusCode(401)
				exchange.Out.SetError(err)
			}
			return false
		}

		return true
	}, nil
}

// convertChatType converts DingTalk chat type to unified chat type
func (a *DingTalkAdapter) convertChatType(chatType string) string {
	switch chatType {
	case "1":
		return api.ChatTypeP2P
	case "2":
		return api.ChatTypeGroup
	default:
		return chatType
	}
}
