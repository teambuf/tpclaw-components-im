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

package processor

import (
	"encoding/json"
	"github.com/teambuf/tpclaw-components-im/api"
	"strings"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/api/types/endpoint"
)

// MessageTransformProcessor creates a processor that transforms messages to unified format.
// It extracts IM metadata and stores the unified IMMessage in the exchange.
func MessageTransformProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		msg := exchange.In.GetMsg()
		if msg.Metadata == nil {
			msg.Metadata = types.NewMetadata()
		}

		// Get platform from metadata
		platform := msg.Metadata.GetValue(api.MetaPlatform)
		if platform == "" {
			// Try to detect platform from body
			platform = detectPlatform(exchange.In.Body())
		}

		// Create IMMessage from metadata
		imMsg := &api.IMMessage{}
		imMsg.FromMetadata(extractMetadata(*msg.GetMetadata()))

		// Set content from body
		imMsg.Content = string(exchange.In.Body())
		imMsg.Platform = platform

		// Store IMMessage in exchange for later use
		SetIMMessage(exchange, imMsg)

		return true
	}
}

// AckProcessor creates a processor that returns a simple ACK response.
func AckProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		exchange.Out.SetBody([]byte("success"))
		exchange.Out.Headers().Set("Content-Type", "text/plain")
		return true
	}
}

// JSONResponseProcessor creates a processor that returns a JSON success response.
func JSONResponseProcessor() endpoint.Process {
	return func(ctx endpoint.Router, exchange *endpoint.Exchange) bool {
		resp, _ := json.Marshal(map[string]interface{}{
			"code": 0,
			"msg":  "success",
		})
		exchange.Out.SetBody(resp)
		exchange.Out.Headers().Set("Content-Type", "application/json")
		return true
	}
}

// detectPlatform tries to detect the IM platform from message body.
func detectPlatform(body []byte) string {
	bodyStr := string(body)
	if len(bodyStr) == 0 {
		return ""
	}

	// Feishu: has schema and header fields
	if strings.Contains(bodyStr, `"schema"`) && strings.Contains(bodyStr, `"header"`) {
		return api.PlatformFeishu
	}

	// WeCom: XML format with ToUserName
	if bodyStr[0] == '<' && strings.Contains(bodyStr, "ToUserName") {
		return api.PlatformWeCom
	}

	// DingTalk: has conversationId
	if strings.Contains(bodyStr, `"conversationId"`) {
		return api.PlatformDingTalk
	}

	return ""
}

// extractMetadata converts types.Metadata to map[string]string.
func extractMetadata(metadata types.Metadata) map[string]string {
	result := make(map[string]string)
	metadata.ForEach(func(key, value string) bool {
		if strings.HasPrefix(key, "im.") {
			result[key] = value
		}
		return true
	})
	return result
}

// ContextKey is used for storing values in exchange context.
type ContextKey string

const (
	// KeyIMMessage is the key for IMMessage in exchange.
	KeyIMMessage ContextKey = "im_message"
)

// SetIMMessage stores IMMessage in exchange.
func SetIMMessage(exchange *endpoint.Exchange, msg *api.IMMessage) {
	imMsg := exchange.In.GetMsg()
	if imMsg.Metadata == nil {
		imMsg.Metadata = types.NewMetadata()
	}
	// Store as JSON string in metadata
	data, _ := json.Marshal(msg)
	imMsg.Metadata.PutValue(string(KeyIMMessage), string(data))
}

// GetIMMessage retrieves IMMessage from exchange.
func GetIMMessage(exchange *endpoint.Exchange) *api.IMMessage {
	msg := exchange.In.GetMsg()
	if msg.Metadata == nil {
		return nil
	}
	data := msg.Metadata.GetValue(string(KeyIMMessage))
	if data == "" {
		return nil
	}
	var imMsg api.IMMessage
	if err := json.Unmarshal([]byte(data), &imMsg); err != nil {
		return nil
	}
	return &imMsg
}
