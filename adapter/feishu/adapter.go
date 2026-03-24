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

// Package feishu provides Feishu (Lark) IM adapter implementation.
package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/teambuf/tpclaw-components-im/api"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/rulego/rulego/api/types/endpoint"
)

// Adapter implements IMAdapter for Feishu platform.
type Adapter struct {
	config *Config
}

// Config holds Feishu adapter configuration.
type Config struct {
	AppID             string `json:"appId"`
	AppSecret         string `json:"appSecret"`
	EncryptKey        string `json:"encryptKey"`
	VerificationToken string `json:"verificationToken"`
}

// NewAdapter creates a new Feishu adapter.
func NewAdapter(config *Config) *Adapter {
	return &Adapter{config: config}
}

// Platform returns the platform identifier.
func (a *Adapter) Platform() string {
	return api.PlatformFeishu
}

// ParseMessage parses the raw request into a unified IMMessage.
func (a *Adapter) ParseMessage(ctx context.Context, body []byte, headers, params map[string]string) (*api.IMMessage, error) {
	// 使用 SDK 的解密功能处理加密消息
	var plainBody []byte
	if a.config.EncryptKey != "" {
		var encryptMsg larkevent.EventEncryptMsg
		if err := json.Unmarshal(body, &encryptMsg); err == nil && encryptMsg.Encrypt != "" {
			decrypted, err := larkevent.EventDecrypt(encryptMsg.Encrypt, a.config.EncryptKey)
			if err != nil {
				return nil, fmt.Errorf("decrypt failed: %w", err)
			}
			plainBody = decrypted
		} else {
			plainBody = body
		}
	} else {
		plainBody = body
	}

	// 解析事件头
	var webhookEvent WebhookEvent
	if err := json.Unmarshal(plainBody, &webhookEvent); err != nil {
		return nil, fmt.Errorf("parse event header failed: %w", err)
	}

	// 创建统一消息
	msg := &api.IMMessage{
		Platform:  api.PlatformFeishu,
		MsgType:   webhookEvent.Header.EventType,
		EventType: webhookEvent.Header.EventType,
		RawData:   plainBody,
		Extensions: map[string]interface{}{
			api.MetaFeishuAppID:     webhookEvent.Header.AppID,
			api.MetaFeishuTenantKey: webhookEvent.Header.TenantKey,
			api.MetaFeishuEventID:   webhookEvent.Header.EventID,
		},
	}

	// 解析时间戳
	if webhookEvent.Header.CreateTime != "" {
		if ts, err := time.Parse(time.RFC3339, webhookEvent.Header.CreateTime); err == nil {
			msg.Timestamp = ts
		}
	}

	// 处理消息接收事件
	if webhookEvent.Header.EventType == "im.message.receive_v1" {
		return a.parseMessageReceiveEvent(plainBody, msg)
	}

	return msg, nil
}

// parseMessageReceiveEvent 解析消息接收事件
func (a *Adapter) parseMessageReceiveEvent(body []byte, msg *api.IMMessage) (*api.IMMessage, error) {
	// 使用 SDK 的事件类型解析
	var event larkim.P2MessageReceiveV1
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("parse message event failed: %w", err)
	}

	if event.Event == nil || event.Event.Message == nil {
		return msg, nil
	}

	// 设置消息信息
	msg.ID = larkcore.StringValue(event.Event.Message.MessageId)
	msg.MsgType = larkcore.StringValue(event.Event.Message.MessageType)
	msg.ChatID = larkcore.StringValue(event.Event.Message.ChatId)
	msg.ChatType = larkcore.StringValue(event.Event.Message.ChatType)

	// 设置发送者信息
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		msg.Sender = &api.IMSender{
			UserID:  larkcore.StringValue(event.Event.Sender.SenderId.UserId),
			OpenID:  larkcore.StringValue(event.Event.Sender.SenderId.OpenId),
			UnionID: larkcore.StringValue(event.Event.Sender.SenderId.UnionId),
		}
	}

	// 设置扩展信息
	msg.Extensions[api.MetaFeishuRootID] = larkcore.StringValue(event.Event.Message.RootId)
	msg.Extensions[api.MetaFeishuParentID] = larkcore.StringValue(event.Event.Message.ParentId)

	// 解析消息内容
	if event.Event.Message.Content != nil {
		msg.Content = a.parseContent(msg.MsgType, *event.Event.Message.Content)
		// 提取媒体 key（用于后续下载）
		a.extractMediaKeys(msg.MsgType, *event.Event.Message.Content, msg.Extensions)
	}

	return msg, nil
}

// parseContent 解析消息内容
func (a *Adapter) parseContent(msgType, content string) string {
	switch msgType {
	case "text":
		var tc struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &tc); err == nil {
			return tc.Text
		}
	case "post":
		var pc struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal([]byte(content), &pc); err == nil {
			return pc.Title
		}
	}
	return content
}

// extractMediaKeys 从消息内容中提取媒体 key
// 这些 key 用于后续下载媒体文件
func (a *Adapter) extractMediaKeys(msgType, content string, extensions map[string]interface{}) {
	if extensions == nil {
		return
	}

	switch msgType {
	case "image":
		// 图片消息: {"image_key": "img_xxx"}
		var ic struct {
			ImageKey string `json:"image_key"`
		}
		if err := json.Unmarshal([]byte(content), &ic); err == nil && ic.ImageKey != "" {
			extensions["image_key"] = ic.ImageKey
		}

	case "audio":
		// 语音消息: {"file_key": "xxx", "duration": 1000}
		var ac struct {
			FileKey  string `json:"file_key"`
			Duration int    `json:"duration"`
		}
		if err := json.Unmarshal([]byte(content), &ac); err == nil && ac.FileKey != "" {
			extensions["file_key"] = ac.FileKey
			if ac.Duration > 0 {
				extensions["duration"] = ac.Duration
			}
		}

	case "video":
		// 视频消息: {"file_key": "xxx", "duration": 10000}
		var vc struct {
			FileKey  string `json:"file_key"`
			Duration int    `json:"duration"`
		}
		if err := json.Unmarshal([]byte(content), &vc); err == nil && vc.FileKey != "" {
			extensions["file_key"] = vc.FileKey
			if vc.Duration > 0 {
				extensions["duration"] = vc.Duration
			}
		}

	case "file":
		// 文件消息: {"file_key": "xxx", "file_name": "test.pdf", "file_size": 1024}
		var fc struct {
			FileKey  string `json:"file_key"`
			FileName string `json:"file_name"`
			FileSize int64  `json:"file_size"`
		}
		if err := json.Unmarshal([]byte(content), &fc); err == nil && fc.FileKey != "" {
			extensions["file_key"] = fc.FileKey
			if fc.FileName != "" {
				extensions["file_name"] = fc.FileName
			}
			if fc.FileSize > 0 {
				extensions["file_size"] = fc.FileSize
			}
		}

	case "sticker":
		// 表情包消息: {"file_key": "xxx"}
		var sc struct {
			FileKey string `json:"file_key"`
		}
		if err := json.Unmarshal([]byte(content), &sc); err == nil && sc.FileKey != "" {
			extensions["file_key"] = sc.FileKey
		}

	case "post":
		// 富文本消息：提取内嵌图片的 image_key
		a.extractPostImageKeys(content, extensions)

	case "media":
		// 媒体消息（通用）: {"file_key": "xxx", "file_name": "xxx"}
		var mc struct {
			FileKey  string `json:"file_key"`
			FileName string `json:"file_name"`
		}
		if err := json.Unmarshal([]byte(content), &mc); err == nil && mc.FileKey != "" {
			extensions["file_key"] = mc.FileKey
			if mc.FileName != "" {
				extensions["file_name"] = mc.FileName
			}
		}
	}
}

// extractPostImageKeys 从富文本消息中提取图片 key
func (a *Adapter) extractPostImageKeys(content string, extensions map[string]interface{}) {
	// 富文本消息结构较为复杂，需要递归遍历
	// 简化处理：使用正则或 JSON 遍历提取所有 image_key
	var postContent struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(content), &postContent); err != nil {
		return
	}

	// 尝试解析为完整的富文本结构
	var fullPost map[string]interface{}
	if err := json.Unmarshal([]byte(content), &fullPost); err != nil {
		return
	}

	var imageKeys []string
	a.extractImageKeysFromValue(fullPost, &imageKeys)

	if len(imageKeys) > 0 {
		extensions["image_keys"] = imageKeys
	}
}

// extractImageKeysFromValue 递归提取 image_key
func (a *Adapter) extractImageKeysFromValue(v interface{}, imageKeys *[]string) {
	switch val := v.(type) {
	case map[string]interface{}:
		// 检查是否有 image_key
		if ik, ok := val["image_key"].(string); ok && ik != "" {
			*imageKeys = append(*imageKeys, ik)
		}
		// 递归处理所有值
		for _, child := range val {
			a.extractImageKeysFromValue(child, imageKeys)
		}
	case []interface{}:
		for _, item := range val {
			a.extractImageKeysFromValue(item, imageKeys)
		}
	}
}

// VerifySignature verifies the request signature using SDK method.
func (a *Adapter) VerifySignature(body []byte, headers, params map[string]string) error {
	if a.config.EncryptKey == "" {
		return nil
	}

	timestamp := getHeader(headers, "X-Lark-Request-Timestamp")
	nonce := getHeader(headers, "X-Lark-Request-Nonce")
	signature := getHeader(headers, "X-Lark-Signature")

	if timestamp == "" || signature == "" {
		return nil
	}

	// 使用 SDK 的签名验证方法
	targetSign := larkevent.Signature(timestamp, nonce, a.config.EncryptKey, string(body))
	if targetSign == signature {
		return nil
	}

	return fmt.Errorf("signature verification failed")
}

// HandleChallenge handles URL verification challenges.
func (a *Adapter) HandleChallenge(body []byte) (response []byte, handled bool, err error) {
	var challengeReq ChallengeRequest
	if err := json.Unmarshal(body, &challengeReq); err != nil {
		return nil, false, nil
	}

	if challengeReq.Type != "url_verification" {
		return nil, false, nil
	}

	if a.config.VerificationToken != "" && challengeReq.Token != a.config.VerificationToken {
		return nil, true, fmt.Errorf("invalid verification token")
	}

	resp, _ := json.Marshal(map[string]string{"challenge": challengeReq.Challenge})
	return resp, true, nil
}

// FormatResponse formats the response for the platform.
func (a *Adapter) FormatResponse(msg *api.IMMessage, responseType api.ResponseType) ([]byte, error) {
	switch responseType {
	case api.ResponseTypeJSON:
		return json.Marshal(map[string]interface{}{
			"code": 0,
			"msg":  "success",
		})
	default:
		return []byte("success"), nil
	}
}

// CreateProcessor creates a platform-specific processor.
func (a *Adapter) CreateProcessor(processorType api.ProcessorType, config interface{}) (endpoint.Process, error) {
	switch processorType {
	case api.ProcessorTypeURLVerify:
		return a.urlVerifyProcessor(), nil
	case api.ProcessorTypeDecrypt:
		return a.decryptProcessor(), nil
	case api.ProcessorTypeTransform:
		return a.transformProcessor(), nil
	case api.ProcessorTypeAck:
		return a.ackProcessor(), nil
	default:
		return nil, fmt.Errorf("unsupported processor type: %s", processorType)
	}
}

func (a *Adapter) decryptProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		body := exchange.In.Body()

		var encryptMsg larkevent.EventEncryptMsg
		if err := json.Unmarshal(body, &encryptMsg); err != nil || encryptMsg.Encrypt == "" {
			return true // Not encrypted, continue
		}

		// 使用 SDK 的解密方法
		decrypted, err := larkevent.EventDecrypt(encryptMsg.Encrypt, a.config.EncryptKey)
		if err != nil {
			exchange.Out.SetError(fmt.Errorf("decrypt failed: %w", err))
			return false
		}

		exchange.In.SetBody(decrypted)
		return true
	}
}

func (a *Adapter) transformProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		body := exchange.In.Body()

		imMsg, err := a.ParseMessage(context.Background(), body, nil, nil)
		if err != nil {
			exchange.Out.SetError(err)
			return false
		}

		// Update metadata
		for k, v := range imMsg.ToMetadata() {
			msg.Metadata.PutValue(k, v)
		}

		// Set content as body
		exchange.In.SetBody([]byte(imMsg.Content))
		return true
	}
}

func (a *Adapter) ackProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		resp, _ := a.FormatResponse(nil, api.ResponseTypeJSON)
		exchange.Out.SetBody(resp)
		exchange.Out.Headers().Set("Content-Type", "application/json")
		return true
	}
}

// urlVerifyProcessor handles URL verification challenges from Feishu.
func (a *Adapter) urlVerifyProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		body := exchange.In.Body()
		response, handled, err := a.HandleChallenge(body)
		if err != nil {
			exchange.Out.SetError(err)
			return false
		}
		if !handled {
			// Not a challenge request, continue to next processor
			return true
		}
		// Challenge handled, return response
		exchange.Out.SetBody(response)
		exchange.Out.Headers().Set("Content-Type", "application/json")
		return false // Stop processing, response is ready
	}
}

func getHeader(headers map[string]string, key string) string {
	return headers[key]
}
