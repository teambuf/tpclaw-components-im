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

package dingtalk

// DingTalkEvent represents the DingTalk webhook event structure.
// 钉钉机器人 Webhook 事件结构
type DingTalkEvent struct {
	// ChatType 聊天类型 (1:单聊 2:群聊)
	ChatType string `json:"chatType"`
	// ConversationID 会话 ID
	ConversationID string `json:"conversationId"`
	// ConversationType 会话类型
	ConversationType string `json:"conversationType"`
	// AtUsers @用户列表
	AtUsers []DingTalkAtUser `json:"atUsers"`
	// ChatbotUserId 机器人用户 ID
	ChatbotUserId string `json:"chatbotUserId"`
	// MsgId 消息 ID
	MsgId string `json:"msgId"`
	// SenderNick 发送者昵称
	SenderNick string `json:"senderNick"`
	// IsAdmin 是否为管理员
	IsAdmin bool `json:"isAdmin"`
	// SenderStaffId 发送者员工 ID
	SenderStaffId string `json:"senderStaffId"`
	// SenderId 发送者 ID
	SenderId string `json:"senderId"`
	// SessionWebhook 会话 Webhook 地址
	SessionWebhook string `json:"sessionWebhook"`
	// Content 消息内容
	Content DingTalkContent `json:"content"`
	// Text 文本内容 (备用字段)
	Text string `json:"text"`
	// MsgType 消息类型
	MsgType string `json:"msgType"`
	// SenderCorpId 发送者企业 ID
	SenderCorpId string `json:"senderCorpId"`
	// CreateTime 消息创建时间戳
	CreateTime int64 `json:"createTime"`
}

// DingTalkContent represents the content of a DingTalk message.
// 钉钉消息内容结构
type DingTalkContent struct {
	// ContentType 内容类型 (text, picture,richText,etc)
	ContentType string `json:"contentType"`
	// Content 内容
	Content string `json:"content"`
	// RichContent 富文本内容 (可选)
	RichContent string `json:"richContent,omitempty"`
	// MediaId 媒体 ID (图片/文件等)
	MediaId string `json:"mediaId,omitempty"`
}

// DingTalkAtUser represents an @mentioned user in DingTalk.
// 钉钉 @ 用户结构
type DingTalkAtUser struct {
	// DingTalkId 钉钉用户 ID
	DingTalkId string `json:"dingtalkId"`
	// StaffId 员工 ID (可选)
	StaffId string `json:"staffId,omitempty"`
}

// DingTalkTextMessage represents a text message to send.
// 钉钉文本消息发送请求
type DingTalkTextMessage struct {
	MsgType string `json:"msg_type"`
	Content struct {
		Content string `json:"content"`
	} `json:"content"`
}

// DingTalkMarkdownMessage represents a markdown message to send.
// 钉钉 Markdown 消息发送请求
type DingTalkMarkdownMessage struct {
	MsgType string `json:"msg_type"`
	Content struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"content"`
}

// DingTalkCardMessage represents a card message to send.
// 钉钉卡片消息发送请求
type DingTalkCardMessage struct {
	MsgType string `json:"msg_type"`
	Content struct {
		CardTemplateID string                 `json:"cardTemplateId"`
		CardData       map[string]interface{} `json:"cardData"`
	} `json:"content"`
}

// DingTalkAPIResponse represents the standard DingTalk API response.
// 钉钉 API 标准响应
type DingTalkAPIResponse struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
}

// DingTalkTokenResponse represents the DingTalk access token response.
// 钉钉 Access Token 响应
type DingTalkTokenResponse struct {
	Errcode     int    `json:"errcode"`
	Errmsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}
