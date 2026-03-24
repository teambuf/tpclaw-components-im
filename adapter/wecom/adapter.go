/*
 * Copyright 2024 The RuleGo Project.
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

// Package wecom provides an adapter for WeCom (企业微信/Enterprise WeChat) integration.
package wecom

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/teambuf/tpclaw-components-im/api"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rulego/rulego/api/types/endpoint"
)

// WeComAdapter implements imapi.IMAdapter interface for WeCom.
type WeComAdapter struct {
	config *WeComConfig
	crypto *Crypto
}

// WeComConfig holds the configuration for WeCom adapter.
type WeComConfig struct {
	Token          string // Token for signature verification
	EncodingAESKey string // Base64 encoded AES key (43 characters)
	CorpID         string // Enterprise CorpID
	AgentID        int    // Application AgentID
	Secret         string // Application Secret
}

// NewWeComAdapter creates a new WeCom adapter.
func NewWeComAdapter(config *WeComConfig) (*WeComAdapter, error) {
	crypto, err := NewCrypto(config.Token, config.EncodingAESKey, config.CorpID)
	if err != nil {
		return nil, fmt.Errorf("create crypto failed: %w", err)
	}

	return &WeComAdapter{
		config: config,
		crypto: crypto,
	}, nil
}

// Platform returns the platform identifier.
func (a *WeComAdapter) Platform() string {
	return api.PlatformWeCom
}

// ParseMessage parses the raw request into a unified IMMessage.
func (a *WeComAdapter) ParseMessage(ctx context.Context, body []byte, headers, params map[string]string) (*api.IMMessage, error) {
	// Parse the encrypted message from body
	var receivedMsg ReceivedMessage
	if err := xml.Unmarshal(body, &receivedMsg); err != nil {
		return nil, fmt.Errorf("parse message failed: %w", err)
	}

	// Get signature parameters
	msgSignature := params["msg_signature"]
	timestamp := params["timestamp"]
	nonce := params["nonce"]

	// Verify signature
	if !a.crypto.VerifySignature(msgSignature, timestamp, nonce, receivedMsg.Encrypt) {
		return nil, ErrInvalidSignature
	}

	// Decrypt message
	decrypted, err := a.crypto.Decrypt(receivedMsg.Encrypt)
	if err != nil {
		return nil, fmt.Errorf("decrypt message failed: %w", err)
	}

	// Parse decrypted message
	var callbackMsg CallbackMessage
	if err := xml.Unmarshal(decrypted, &callbackMsg); err != nil {
		return nil, fmt.Errorf("parse decrypted message failed: %w", err)
	}

	// Build unified IMMessage
	msg := &api.IMMessage{
		ID:        callbackMsg.MsgID,
		Platform:  api.PlatformWeCom,
		Timestamp: time.Unix(callbackMsg.CreateTime, 0),
		ChatID:    callbackMsg.FromUserName, // In WeCom, chatID is the sender's user ID for P2P
		ChatType:  api.ChatTypeP2P,          // Default to P2P, can be extended for group
		Sender: &api.IMSender{
			UserID: callbackMsg.FromUserName,
		},
		MsgType:   callbackMsg.MsgType,
		EventType: callbackMsg.Event,
		Content:   callbackMsg.Content,
		RawData:   decrypted,
		Extensions: map[string]interface{}{
			api.MetaWeComCorpID:         a.config.CorpID,
			api.MetaWeComAgentID:        receivedMsg.AgentID,
			api.MetaWeComToUser:         callbackMsg.ToUserName,
			api.MetaWeComFromUser:       callbackMsg.FromUserName,
			api.MetaWeComEventKey:       callbackMsg.EventKey,
			api.MetaWeComChangeType:     callbackMsg.ChangeType,
			api.MetaWeComStreamChatInfo: fmt.Sprintf("1|%s|%s", callbackMsg.FromUserName, callbackMsg.FromUserName),
		},
	}

	// Extract media information based on message type
	a.extractMediaInfo(&callbackMsg, msg)

	return msg, nil
}

// extractMediaInfo extracts media information from the callback message
// similar to Feishu's extractMediaKeys method
func (a *WeComAdapter) extractMediaInfo(callbackMsg *CallbackMessage, msg *api.IMMessage) {
	if msg.Extensions == nil {
		msg.Extensions = make(map[string]interface{})
	}

	switch callbackMsg.MsgType {
	case "image":
		// 图片消息
		if callbackMsg.MediaId != "" {
			msg.Extensions["mediaId"] = callbackMsg.MediaId
			msg.Content = "[图片]"
		}
		if callbackMsg.PicUrl != "" {
			msg.Extensions["picUrl"] = callbackMsg.PicUrl
		}

	case "voice":
		// 语音消息
		if callbackMsg.MediaId != "" {
			msg.Extensions["mediaId"] = callbackMsg.MediaId
			msg.Content = "[语音]"
		}
		if callbackMsg.Format != "" {
			msg.Extensions["format"] = callbackMsg.Format
		}
		if callbackMsg.Recognition != "" {
			msg.Extensions["recognition"] = callbackMsg.Recognition
			msg.Content = callbackMsg.Recognition // 语音识别结果作为内容
		}

	case "video":
		// 视频消息
		if callbackMsg.MediaId != "" {
			msg.Extensions["mediaId"] = callbackMsg.MediaId
			msg.Content = "[视频]"
		}
		if callbackMsg.ThumbMediaId != "" {
			msg.Extensions["thumbMediaId"] = callbackMsg.ThumbMediaId
		}

	case "location":
		// 位置消息
		msg.Extensions["latitude"] = callbackMsg.Location_X
		msg.Extensions["longitude"] = callbackMsg.Location_Y
		msg.Extensions["scale"] = callbackMsg.Scale
		if callbackMsg.Label != "" {
			msg.Extensions["label"] = callbackMsg.Label
			msg.Content = fmt.Sprintf("[位置] %s", callbackMsg.Label)
		} else {
			msg.Content = fmt.Sprintf("[位置] (%.6f, %.6f)", callbackMsg.Location_X, callbackMsg.Location_Y)
		}

	case "link":
		// 链接消息
		if callbackMsg.Title != "" {
			msg.Extensions["title"] = callbackMsg.Title
		}
		if callbackMsg.Description != "" {
			msg.Extensions["description"] = callbackMsg.Description
		}
		if callbackMsg.Url != "" {
			msg.Extensions["url"] = callbackMsg.Url
		}
		if callbackMsg.PicUrl != "" {
			msg.Extensions["picUrl"] = callbackMsg.PicUrl
		}
		if callbackMsg.Title != "" {
			msg.Content = fmt.Sprintf("[链接] %s", callbackMsg.Title)
		} else {
			msg.Content = "[链接]"
		}

	case "file":
		// 文件消息（客服消息）
		if callbackMsg.FileKey != "" {
			msg.Extensions["fileKey"] = callbackMsg.FileKey
			msg.Content = "[文件]"
		}
		if callbackMsg.FileMD5 != "" {
			msg.Extensions["fileMd5"] = callbackMsg.FileMD5
		}
		if callbackMsg.FileSize > 0 {
			msg.Extensions["fileSize"] = callbackMsg.FileSize
		}
		if callbackMsg.Title != "" {
			msg.Extensions["fileName"] = callbackMsg.Title
		}
	}

	// 标记消息是否包含媒体文件
	if _, hasMedia := msg.Extensions["mediaId"]; hasMedia {
		msg.Extensions[api.MetaHasMedia] = "true"
	} else if _, hasPicUrl := msg.Extensions["picUrl"]; hasPicUrl && callbackMsg.MsgType == "image" {
		msg.Extensions[api.MetaHasMedia] = "true"
	} else if _, hasFileKey := msg.Extensions["fileKey"]; hasFileKey {
		msg.Extensions[api.MetaHasMedia] = "true"
	}
}

// VerifySignature verifies the request signature.
func (a *WeComAdapter) VerifySignature(body []byte, headers, params map[string]string) error {
	msgSignature := params["msg_signature"]
	timestamp := params["timestamp"]
	nonce := params["nonce"]

	// Parse the encrypted message from body to get encrypt field
	var receivedMsg ReceivedMessage
	if err := xml.Unmarshal(body, &receivedMsg); err != nil {
		return fmt.Errorf("parse message failed: %w", err)
	}

	if !a.crypto.VerifySignature(msgSignature, timestamp, nonce, receivedMsg.Encrypt) {
		return ErrInvalidSignature
	}

	return nil
}

// HandleChallenge handles URL verification challenges.
// WeCom sends a GET request with echostr parameter for URL verification.
func (a *WeComAdapter) HandleChallenge(body []byte) (response []byte, handled bool, err error) {
	// For WeCom, challenge handling is done via GET request params
	// This method is for POST body challenges (if any)
	// Return handled=false to indicate this is not a challenge
	return nil, false, nil
}

// HandleURLVerification handles GET request URL verification from WeCom.
// This is called when WeCom sends a GET request to verify the callback URL.
func (a *WeComAdapter) HandleURLVerification(params map[string]string) ([]byte, error) {
	msgSignature := params["msg_signature"]
	timestamp := params["timestamp"]
	nonce := params["nonce"]
	echostr := params["echostr"]

	// URL decode the echostr
	// Note: QueryUnescape converts '+' to space, so we need to restore it
	// because base64 encoding may contain '+' characters
	echostr, _ = url.QueryUnescape(echostr)
	echostr = strings.ReplaceAll(echostr, " ", "+")

	// Verify signature
	if !a.crypto.VerifySignature(msgSignature, timestamp, nonce, echostr) {
		return nil, ErrInvalidSignature
	}

	// Decrypt echo string
	echo, err := a.crypto.Decrypt(echostr)
	if err != nil {
		return nil, fmt.Errorf("decrypt echo failed: %w", err)
	}

	return echo, nil
}

// FormatResponse formats the response for the platform.
func (a *WeComAdapter) FormatResponse(msg *api.IMMessage, responseType api.ResponseType) ([]byte, error) {
	switch responseType {
	case api.ResponseTypePlainText:
		return []byte("success"), nil

	case api.ResponseTypeEncrypted:
		return a.formatEncryptedResponse(msg)

	case api.ResponseTypeStream:
		return a.formatStreamResponse(msg, 0) // text

	case api.ResponseTypeJSON:
		return json.Marshal(msg)

	default:
		return []byte("success"), nil
	}
}

// formatEncryptedResponse formats an encrypted XML response.
func (a *WeComAdapter) formatEncryptedResponse(msg *api.IMMessage) ([]byte, error) {
	// Get from and to user info from extensions
	fromUserID := ""
	toUserID := ""
	if msg.Extensions != nil {
		if v, ok := msg.Extensions[api.MetaWeComToUser].(string); ok {
			fromUserID = v // In response, ToUser becomes FromUser
		}
		if v, ok := msg.Extensions[api.MetaWeComFromUser].(string); ok {
			toUserID = v // In response, FromUser becomes ToUser
		}
	}

	// Create text message
	textMsg := NewTextMessage(toUserID, fromUserID, msg.Content)

	// Marshal message to XML
	msgXML, err := textMsg.ToXML()
	if err != nil {
		return nil, fmt.Errorf("marshal message failed: %w", err)
	}

	// Encrypt message
	encrypt, err := a.crypto.Encrypt(msgXML)
	if err != nil {
		return nil, fmt.Errorf("encrypt message failed: %w", err)
	}

	// Generate timestamp and nonce
	timestamp := GenerateTimestamp()
	nonce := GenerateNonce()

	// Compute signature
	signature := a.crypto.ComputeSignature(timestamp, nonce, encrypt)

	// Create encrypted response
	response := NewEncryptedResponse(encrypt, signature, timestamp, nonce)

	// Marshal response as XML
	return response.ToXML()
}

// formatStreamResponse formats a stream response for AI chat.
func (a *WeComAdapter) formatStreamResponse(msg *api.IMMessage, msgType int) ([]byte, error) {
	// Get stream chat info from extensions
	chatType := 1
	userID := msg.ChatID
	chatID := msg.ChatID

	if msg.Extensions != nil {
		if v, ok := msg.Extensions[api.MetaWeComStreamChatInfo].(string); ok && v != "" {
			// Parse chat info format: "chatType|userID|chatID"
			// Implementation can parse this if needed
			_ = v
		}
	}

	// Create stream response
	response := &StreamResponse{
		SpNum: "1",
		SpChatInfo: &SpChatInfo{
			SpChatType: chatType,
			SpUserID:   userID,
			SpChatID:   chatID,
		},
		SpMsg: &SpMsg{
			SpMsgType: msgType,
		},
	}

	// Set content based on message type
	if msgType == 1 {
		response.SpMsg.SpContent = &StreamMarkdownContent{Content: msg.Content}
	} else {
		response.SpMsg.SpContent = &StreamTextContent{Content: msg.Content}
	}

	return json.Marshal(response)
}

// CreateProcessor creates a platform-specific processor.
func (a *WeComAdapter) CreateProcessor(processorType api.ProcessorType, config interface{}) (endpoint.Process, error) {
	switch processorType {
	case api.ProcessorTypeURLVerify:
		return a.urlVerifyProcessor(), nil

	case api.ProcessorTypeDecrypt:
		return a.decryptProcessor(), nil

	case api.ProcessorTypeEncryptResponse:
		return a.encryptedResponseProcessor(), nil

	case api.ProcessorTypeStreamResponse:
		msgType := 0 // default to text
		if cfg, ok := config.(map[string]interface{}); ok {
			if v, ok := cfg["msgType"].(int); ok {
				msgType = v
			} else if v, ok := cfg["msgType"].(float64); ok {
				msgType = int(v)
			}
		}
		return a.streamResponseProcessor(msgType), nil

	case api.ProcessorTypeAck:
		return a.ackProcessor(), nil

	default:
		return nil, fmt.Errorf("unknown processor type: %s", processorType)
	}
}

// urlVerifyProcessor handles URL verification callback from WeCom.
func (a *WeComAdapter) urlVerifyProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		// Get verification parameters from query string
		params := map[string]string{
			"msg_signature": exchange.In.GetParam("msg_signature"),
			"timestamp":     exchange.In.GetParam("timestamp"),
			"nonce":         exchange.In.GetParam("nonce"),
			"echostr":       exchange.In.GetParam("echostr"),
		}
		result, err := a.HandleURLVerification(params)
		if err != nil {
			exchange.Out.SetError(err)
			return false
		}

		exchange.Out.SetBody(result)
		exchange.Out.Headers().Set("Content-Type", "text/plain")
		return true
	}
}

// decryptProcessor handles message decryption.
func (a *WeComAdapter) decryptProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		body := exchange.In.Body()
		params := map[string]string{
			"msg_signature": exchange.In.GetParam("msg_signature"),
			"timestamp":     exchange.In.GetParam("timestamp"),
			"nonce":         exchange.In.GetParam("nonce"),
		}

		msg, err := a.ParseMessage(context.Background(), body, nil, params)
		if err != nil {
			exchange.Out.SetError(err)
			return false
		}

		// Store the parsed message in exchange
		exchange.In.SetBody(msg.RawData)

		// Set metadata
		metadata := msg.ToMetadata()
		for k, v := range metadata {
			exchange.In.GetMsg().Metadata.PutValue(k, v)
		}

		// Store WeCom-specific info
		if msg.Extensions != nil {
			for k, v := range msg.Extensions {
				if vs, ok := v.(string); ok {
					exchange.In.GetMsg().Metadata.PutValue(k, vs)
				}
			}
		}

		return true
	}
}

// encryptedResponseProcessor handles encrypted response formatting.
func (a *WeComAdapter) encryptedResponseProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := &api.IMMessage{
			Content: string(exchange.Out.Body()),
		}

		// Get metadata from In message
		inMsg := exchange.In.GetMsg()
		if inMsg.Metadata != nil {
			msg.Extensions = make(map[string]interface{})
			msg.Extensions[api.MetaWeComToUser] = inMsg.Metadata.GetValue(api.MetaWeComToUser)
			msg.Extensions[api.MetaWeComFromUser] = inMsg.Metadata.GetValue(api.MetaWeComFromUser)
		}

		result, err := a.FormatResponse(msg, api.ResponseTypeEncrypted)
		if err != nil {
			exchange.Out.SetError(err)
			return false
		}

		exchange.Out.SetBody(result)
		exchange.Out.Headers().Set("Content-Type", "application/xml")
		return true
	}
}

// streamResponseProcessor handles stream response formatting.
func (a *WeComAdapter) streamResponseProcessor(msgType int) endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := &api.IMMessage{
			Content: string(exchange.Out.Body()),
		}

		// Get metadata from In message
		inMsg := exchange.In.GetMsg()
		if inMsg.Metadata != nil {
			chatInfo := inMsg.Metadata.GetValue(api.MetaWeComStreamChatInfo)
			msg.Extensions = map[string]interface{}{
				api.MetaWeComStreamChatInfo: chatInfo,
			}
			msg.ChatID = inMsg.Metadata.GetValue(api.MetaChatID)
		}

		result, err := a.formatStreamResponse(msg, msgType)
		if err != nil {
			exchange.Out.SetError(err)
			return false
		}

		exchange.Out.SetBody(result)
		exchange.Out.Headers().Set("Content-Type", "application/json")
		return true
	}
}

// ackProcessor handles simple ACK response to platform.
func (a *WeComAdapter) ackProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("success"))
		exchange.Out.Headers().Set("Content-Type", "text/plain")
		return true
	}
}

// GenerateNonce generates a random nonce string.
func GenerateNonce() string {
	// Use random bytes to ensure uniqueness
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// GenerateTimestamp generates a timestamp string.
func GenerateTimestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// GetCrypto returns the crypto instance for advanced usage.
func (a *WeComAdapter) GetCrypto() *Crypto {
	return a.crypto
}

// GetConfig returns the config.
func (a *WeComAdapter) GetConfig() *WeComConfig {
	return a.config
}
