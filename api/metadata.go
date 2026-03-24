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

// Package api provides core interfaces and types for IM integration.
package api

// Metadata key constants for IM messages.
const (
	// ============== Common Metadata Keys ==============
	MetaPlatform  = "im.platform"
	MetaChatID    = "im.chatId"
	MetaUserID    = "im.userId"
	MetaMsgID     = "im.msgId"
	MetaMsgType   = "im.msgType"
	MetaEventType = "im.eventType"
	MetaChatType  = "im.chatType"
	MetaTimestamp = "im.timestamp"

	// ============== Extended Common Metadata Keys ==============
	MetaIsGroup       = "im.isGroup"       // 是否群聊 (true/false)
	MetaSenderID      = "im.senderId"      // 发送者 ID
	MetaSenderName    = "im.senderName"    // 发送者昵称
	MetaSenderOpenID  = "im.senderOpenId"  // 发送者 OpenID
	MetaSenderUnionID = "im.senderUnionId" // 发送者 UnionID
	MetaSenderAvatar  = "im.senderAvatar"  // 发送者头像
	MetaSenderIsAdmin = "im.senderIsAdmin" // 发送者是否管理员 (true/false)
	MetaReceiverID    = "im.receiverId"    // 接收者/机器人 ID
	MetaBotID         = "im.botId"         // 机器人 ID (同 receiverId，语义更明确)
	MetaContent       = "im.content"       // 消息内容
	MetaIsReply       = "im.isReply"       // 是否回复消息 (true/false)
	MetaRootID        = "im.rootId"        // 根消息 ID (话题/线程)
	MetaParentID      = "im.parentId"      // 父消息 ID
	MetaQuoteContent  = "im.quoteContent"  // 引用/回复的消息内容
	MetaImages        = "im.images"        // 图片列表 (base64 格式)
	MetaChannel       = "im.channel"       // 通道标识（如 feishu, feishu_webhook)
	MetaLoadHistory   = "loadHistory"      // 是否加载历史消息 (true/false)

	// ============== Media Metadata Keys ==============
	MetaMediaFiles = "im.mediaFiles" // 媒体文件列表 ([]MediaFile)
	MetaHasMedia   = "im.hasMedia"   // 是否包含媒体文件 (bool)

	// ============== Response Metadata Keys ==============
	MetaResponseChatID = "im.responseChatId"
	MetaReplyMsgID     = "im.replyMsgId"

	// ============== Feishu Specific Keys ==============
	MetaFeishuAppID      = "im.appId"
	MetaFeishuTenantKey  = "im.tenantKey"
	MetaFeishuOpenID     = "im.openId"
	MetaFeishuUnionID    = "im.unionId"
	MetaFeishuEventID    = "im.eventId"
	MetaFeishuRootID     = MetaRootID   // 别名，兼容旧代码
	MetaFeishuParentID   = MetaParentID // 别名，兼容旧代码
	MetaFeishuSenderType = "im.senderType"
	MetaFeishuReadTime   = "im.readTime" // 消息已读时间

	// ============== DingTalk Specific Keys ==============
	MetaDingTalkAppKey           = "im.appKey"
	MetaDingTalkSenderID         = "im.senderId"
	MetaDingTalkSenderNick       = "im.senderNick"
	MetaDingTalkSenderStaffID    = "im.senderStaffId"
	MetaDingTalkSenderCorpID     = "im.senderCorpId"
	MetaDingTalkSessionWebhook   = "im.sessionWebhook"
	MetaDingTalkIsAdmin          = "im.isAdmin"
	MetaDingTalkConversationType = "im.conversationType"

	// ============== WeCom Specific Keys ==============
	MetaWeComCorpID         = "im.corpId"
	MetaWeComAgentID        = "im.agentId"
	MetaWeComToUser         = "im.toUser"
	MetaWeComFromUser       = "im.fromUser"
	MetaWeComEventKey       = "im.eventKey"
	MetaWeComChangeType     = "im.changeType"
	MetaWeComStreamChatInfo = "im.streamChatInfo"
)

// Platform constants.
const (
	PlatformFeishu   = "feishu"
	PlatformDingTalk = "dingtalk"
	PlatformWeCom    = "wecom"
	PlatformSlack    = "slack"
	PlatformDiscord  = "discord"
	PlatformTelegram = "telegram"
)

// Channel constants (区分同一平台的不同连接方式)
const (
	// ChannelFeishu 飞书 WebSocket 长连接模式（默认）
	ChannelFeishu = "feishu"
	// ChannelFeishuWebhook 飞书 Webhook 推送模式
	ChannelFeishuWebhook = "feishu_webhook"
)

// Message type constants.
const (
	MsgTypeText     = "text"
	MsgTypeMarkdown = "markdown"
	MsgTypeImage    = "image"
	MsgTypeVoice    = "voice"
	MsgTypeVideo    = "video"
	MsgTypeFile     = "file"
	MsgTypeAudio    = "audio"
	MsgTypeCard     = "card"
	MsgTypeEvent    = "event"
	MsgTypePost     = "post"
)

// Chat type constants.
const (
	ChatTypeGroup = "group"
	ChatTypeP2P   = "p2p"
)

// EventType constants.
const (
	EventTypeMessageReceived = "message_received"
	EventTypeCardClick       = "card_click"
	EventTypeUserEnter       = "user_enter"
	EventTypeUserLeave       = "user_leave"
	EventTypeURLVerify       = "url_verify"
)

// MediaFile 媒体文件信息
type MediaFile struct {
	Type         string `json:"type"`         // image/audio/video/file
	FileName     string `json:"fileName"`     // 文件名
	RelativePath string `json:"relativePath"` // 相对于工作空间的路径
	FileSize     int64  `json:"fileSize"`     // 文件大小（字节）
	MimeType     string `json:"mimeType"`     // MIME 类型
	Duration     int    `json:"duration"`     // 时长（毫秒，音视频）
}
