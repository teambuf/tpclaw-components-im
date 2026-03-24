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
	"encoding/json"
	"time"
)

// IMMessage represents a unified IM message format across different platforms.
// It contains all common fields and platform-specific extensions.
type IMMessage struct {
	// ============== Basic Info ==============
	ID        string    `json:"id,omitempty"`
	Platform  string    `json:"platform,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`

	// ============== Chat Info ==============
	ChatID   string `json:"chatId,omitempty"`
	ChatType string `json:"chatType,omitempty"`

	// ============== Sender Info ==============
	Sender *IMSender `json:"sender,omitempty"`

	// ============== Message Content ==============
	MsgType   string          `json:"msgType,omitempty"`
	EventType string          `json:"eventType,omitempty"`
	Content   string          `json:"content,omitempty"`
	RawData   json.RawMessage `json:"rawData,omitempty"`

	// ============== Reply Info ==============
	ReplyTo  string `json:"replyTo,omitempty"`
	RootID   string `json:"rootId,omitempty"`
	ParentID string `json:"parentId,omitempty"`

	// ============== Extensions ==============
	// Platform-specific data stored here
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// IMSender represents the sender information.
type IMSender struct {
	UserID  string `json:"userId,omitempty"`
	OpenID  string `json:"openId,omitempty"`
	UnionID string `json:"unionId,omitempty"`
	Name    string `json:"name,omitempty"`
	Avatar  string `json:"avatar,omitempty"`
	IsAdmin bool   `json:"isAdmin,omitempty"`

	// Platform-specific fields
	StaffID string `json:"staffId,omitempty"`
	CorpID  string `json:"corpId,omitempty"`
}

// ToMetadata converts IMMessage to metadata map.
func (m *IMMessage) ToMetadata() map[string]string {
	metadata := make(map[string]string)

	// ============== Basic Info ==============
	if m.Platform != "" {
		metadata[MetaPlatform] = m.Platform
	}
	if m.ID != "" {
		metadata[MetaMsgID] = m.ID
	}
	if m.MsgType != "" {
		metadata[MetaMsgType] = m.MsgType
	}
	if m.EventType != "" {
		metadata[MetaEventType] = m.EventType
	}
	if !m.Timestamp.IsZero() {
		metadata[MetaTimestamp] = m.Timestamp.Format(time.RFC3339)
	}
	if m.Content != "" {
		metadata[MetaContent] = m.Content
	}

	// ============== Chat Info ==============
	if m.ChatID != "" {
		metadata[MetaChatID] = m.ChatID
	}
	if m.ChatType != "" {
		metadata[MetaChatType] = m.ChatType
		metadata[MetaIsGroup] = boolToString(m.ChatType == ChatTypeGroup)
	}

	// ============== Sender Info (平铺方式) ==============
	if m.Sender != nil {
		if m.Sender.UserID != "" {
			metadata[MetaUserID] = m.Sender.UserID
			metadata[MetaSenderID] = m.Sender.UserID
		}
		if m.Sender.OpenID != "" {
			metadata[MetaSenderOpenID] = m.Sender.OpenID
		}
		if m.Sender.UnionID != "" {
			metadata[MetaSenderUnionID] = m.Sender.UnionID
		}
		if m.Sender.Name != "" {
			metadata[MetaSenderName] = m.Sender.Name
		}
		if m.Sender.Avatar != "" {
			metadata[MetaSenderAvatar] = m.Sender.Avatar
		}
		if m.Sender.IsAdmin {
			metadata[MetaSenderIsAdmin] = "true"
		}
		if m.Sender.StaffID != "" {
			metadata[MetaDingTalkSenderStaffID] = m.Sender.StaffID
		}
		if m.Sender.CorpID != "" {
			metadata[MetaDingTalkSenderCorpID] = m.Sender.CorpID
		}
	}

	// ============== Reply Info ==============
	if m.ReplyTo != "" {
		metadata[MetaReplyMsgID] = m.ReplyTo
	}
	if m.RootID != "" {
		metadata[MetaRootID] = m.RootID
		metadata[MetaIsReply] = "true"
	}
	if m.ParentID != "" {
		metadata[MetaParentID] = m.ParentID
	}

	// ============== Extensions ==============
	for k, v := range m.Extensions {
		if str, ok := v.(string); ok {
			metadata[k] = str
		}
	}

	return metadata
}

// boolToString converts bool to string.
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// FromMetadata fills IMMessage from metadata map.
func (m *IMMessage) FromMetadata(metadata map[string]string) {
	// ============== Basic Info ==============
	m.Platform = metadata[MetaPlatform]
	m.ID = metadata[MetaMsgID]
	m.MsgType = metadata[MetaMsgType]
	m.EventType = metadata[MetaEventType]
	m.Content = metadata[MetaContent]

	if ts := metadata[MetaTimestamp]; ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			m.Timestamp = t
		}
	}

	// ============== Chat Info ==============
	m.ChatID = metadata[MetaChatID]
	m.ChatType = metadata[MetaChatType]

	// ============== Sender Info ==============
	if metadata[MetaSenderID] != "" || metadata[MetaSenderOpenID] != "" ||
		metadata[MetaSenderName] != "" || metadata[MetaSenderUnionID] != "" {
		if m.Sender == nil {
			m.Sender = &IMSender{}
		}
		m.Sender.UserID = metadata[MetaSenderID]
		m.Sender.OpenID = metadata[MetaSenderOpenID]
		m.Sender.UnionID = metadata[MetaSenderUnionID]
		m.Sender.Name = metadata[MetaSenderName]
		m.Sender.Avatar = metadata[MetaSenderAvatar]
		m.Sender.IsAdmin = metadata[MetaSenderIsAdmin] == "true"
		m.Sender.StaffID = metadata[MetaDingTalkSenderStaffID]
		m.Sender.CorpID = metadata[MetaDingTalkSenderCorpID]
	}

	// ============== Reply Info ==============
	m.ReplyTo = metadata[MetaReplyMsgID]
	m.RootID = metadata[MetaRootID]
	m.ParentID = metadata[MetaParentID]
}

// IMResponse represents a unified IM response format.
type IMResponse struct {
	Platform string `json:"platform,omitempty"`
	ChatID   string `json:"chatId,omitempty"`
	ReplyTo  string `json:"replyTo,omitempty"`
	MsgType  string `json:"msgType,omitempty"`
	Content  string `json:"content,omitempty"`

	// For stream responses
	IsChunk     bool `json:"isChunk,omitempty"`
	IsCompleted bool `json:"isCompleted,omitempty"`
	ToolCall    bool `json:"toolCall,omitempty"`

	// Extensions for platform-specific data
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// ToIMMessage converts IMResponse to IMMessage.
func (r *IMResponse) ToIMMessage() *IMMessage {
	return &IMMessage{
		Platform:   r.Platform,
		ChatID:     r.ChatID,
		ReplyTo:    r.ReplyTo,
		MsgType:    r.MsgType,
		Content:    r.Content,
		Extensions: r.Extensions,
	}
}

// MetadataWriter defines an interface for writing metadata.
// MetadataWriter 定义元数据写入接口
type MetadataWriter interface {
	PutValue(key, value string)
}

// SetIMMetadata writes IMMessage metadata to a MetadataWriter.
// This is useful for setting metadata on RuleMsg or other types that implement MetadataWriter.
// SetIMMetadata 将 IMMessage 的元数据写入 MetadataWriter
func SetIMMetadata(m *IMMessage, writer MetadataWriter, extraFields map[string]string) {
	if m == nil || writer == nil {
		return
	}

	// Set basic metadata from ToMetadata()
	for k, v := range m.ToMetadata() {
		writer.PutValue(k, v)
	}

	// Set extra fields
	for k, v := range extraFields {
		if v != "" {
			writer.PutValue(k, v)
		}
	}
}

// SetSenderMetadata writes sender info to metadata writer directly.
// SetSenderMetadata 直接将发送者信息写入元数据
func SetSenderMetadata(sender *IMSender, writer MetadataWriter) {
	if sender == nil || writer == nil {
		return
	}

	if sender.UserID != "" {
		writer.PutValue(MetaUserID, sender.UserID)
		writer.PutValue(MetaSenderID, sender.UserID)
	}
	if sender.OpenID != "" {
		writer.PutValue(MetaSenderOpenID, sender.OpenID)
	}
	if sender.UnionID != "" {
		writer.PutValue(MetaSenderUnionID, sender.UnionID)
	}
	if sender.Name != "" {
		writer.PutValue(MetaSenderName, sender.Name)
	}
	if sender.Avatar != "" {
		writer.PutValue(MetaSenderAvatar, sender.Avatar)
	}
	if sender.IsAdmin {
		writer.PutValue(MetaSenderIsAdmin, "true")
	}
	if sender.StaffID != "" {
		writer.PutValue(MetaDingTalkSenderStaffID, sender.StaffID)
	}
	if sender.CorpID != "" {
		writer.PutValue(MetaDingTalkSenderCorpID, sender.CorpID)
	}
}

// SetChatMetadata writes chat context info to metadata writer directly.
// SetChatMetadata 直接将聊天上下文信息写入元数据
func SetChatMetadata(chatID, chatType, msgID string, writer MetadataWriter) {
	if writer == nil {
		return
	}

	if chatID != "" {
		writer.PutValue(MetaChatID, chatID)
	}
	if chatType != "" {
		writer.PutValue(MetaChatType, chatType)
		if chatType == ChatTypeGroup {
			writer.PutValue(MetaIsGroup, "true")
		} else {
			writer.PutValue(MetaIsGroup, "false")
		}
	}
	if msgID != "" {
		writer.PutValue(MetaMsgID, msgID)
	}
}

// SetReplyMetadata writes reply info to metadata writer directly.
// SetReplyMetadata 直接将回复信息写入元数据
func SetReplyMetadata(rootID, parentID string, writer MetadataWriter) {
	if writer == nil {
		return
	}

	if rootID != "" {
		writer.PutValue(MetaRootID, rootID)
		writer.PutValue(MetaIsReply, "true")
	}
	if parentID != "" {
		writer.PutValue(MetaParentID, parentID)
	}
}
