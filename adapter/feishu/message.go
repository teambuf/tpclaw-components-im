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

package feishu

// WebhookEvent represents a Feishu webhook event.
type WebhookEvent struct {
	Schema string `json:"schema"`
	Header struct {
		EventID    string `json:"event_id"`
		EventType  string `json:"event_type"`
		CreateTime string `json:"create_time"`
		Token      string `json:"token"`
		AppID      string `json:"app_id"`
		TenantKey  string `json:"tenant_key"`
	} `json:"header"`
	Event interface{} `json:"event"`
}

// MessageEvent represents a Feishu message event.
type MessageEvent struct {
	Sender struct {
		SenderID struct {
			UnionID string `json:"union_id"`
			UserID  string `json:"user_id"`
			OpenID  string `json:"open_id"`
		} `json:"sender_id"`
		SenderType string `json:"sender_type"`
		TenantKey  string `json:"tenant_key"`
	} `json:"sender"`
	Message struct {
		MessageID   string `json:"message_id"`
		RootID      string `json:"root_id"`
		ParentID    string `json:"parent_id"`
		CreateTime  string `json:"create_time"`
		ChatID      string `json:"chat_id"`
		ChatType    string `json:"chat_type"`
		MessageType string `json:"message_type"`
		Content     string `json:"content"`
		Mentions    []struct {
			Key string `json:"key"`
			ID  struct {
				UnionID string `json:"union_id"`
				UserID  string `json:"user_id"`
				OpenID  string `json:"open_id"`
			} `json:"id"`
			Name      string `json:"name"`
			TenantKey string `json:"tenant_key"`
		} `json:"mentions"`
	} `json:"message"`
}

// ChallengeRequest represents a URL verification request.
type ChallengeRequest struct {
	Challenge string `json:"challenge"`
	Token     string `json:"token"`
	Type      string `json:"type"`
}

// TextContent represents text message content.
type TextContent struct {
	Text string `json:"text"`
}

// PostContent represents post message content.
type PostContent struct {
	Title   string      `json:"title"`
	Content interface{} `json:"content"`
}

// ImageContent represents image message content.
type ImageContent struct {
	ImageKey string `json:"image_key"`
}

// FileContent represents file message content.
type FileContent struct {
	FileKey  string `json:"file_key"`
	FileName string `json:"file_name"`
}

// AudioContent represents audio message content.
type AudioContent struct {
	FileKey    string `json:"file_key"`
	Duration   int    `json:"duration"`
	FormatType string `json:"format_type"`
}

// VideoContent represents video message content.
type VideoContent struct {
	FileKey  string `json:"file_key"`
	Duration int    `json:"duration"`
}

// SendTextMessage represents a text message to send.
type SendTextMessage struct {
	ReceiveID string      `json:"receive_id"`
	MsgType   string      `json:"msg_type"`
	Content   TextContent `json:"content"`
}

// SendMarkdownMessage represents a markdown message to send.
type SendMarkdownMessage struct {
	ReceiveID string `json:"receive_id"`
	MsgType   string `json:"msg_type"`
	Content   struct {
		Content string `json:"content"`
	} `json:"content"`
}

// APIResponse represents Feishu API response.
type APIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// TokenResponse represents token API response.
type TokenResponse struct {
	Code           int    `json:"code"`
	Msg            string `json:"msg"`
	AppAccessToken string `json:"app_access_token"`
	Expire         int    `json:"expire"`
}
