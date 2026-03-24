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
 * WITHOUT WARRANTIES OR conditions of any kind, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package constants 提供IM组件的共享常量
package constants

import "time"

// 超时配置
const (
	// DefaultTimeout 默认 HTTP 请求超时时间
	DefaultTimeout = 30 * time.Second
	// UploadTimeout 文件上传超时时间
	UploadTimeout = 60 * time.Second
	// TokenRefreshAhead Token 提前刷新时间
	TokenRefreshAhead = 5 * time.Minute
)

// 钉钉 API URL
const (
	// DingTalkOAPIBase 钉钉旧 API 基础地址
	DingTalkOAPIBase = "https://oapi.dingtalk.com"
	// DingTalkAPIBase 钉钉新 API 基础地址
	DingTalkAPIBase = "https://api.dingtalk.com"
	// DingTalkRobotGroupMessagesSend 机器人群消息发送接口
	DingTalkRobotGroupMessagesSend = DingTalkAPIBase + "/v1.0/robot/groupMessages/send"
	// DingTalkRobotCardsModify 机器人卡片修改接口
	DingTalkRobotCardsModify = DingTalkAPIBase + "/v1.0/robot/cards/modify"
	// DingTalkGetToken 获取 Token 接口
	DingTalkGetToken = DingTalkOAPIBase + "/gettoken"
	// DingTalkMediaUpload 媒体文件上传接口
	DingTalkMediaUpload = DingTalkOAPIBase + "/media/upload"
	// DingTalkChatDownload 聊天文件下载接口
	DingTalkChatDownload = DingTalkOAPIBase + "/chat/download"
)

// 企业微信 API URL
const (
	// WeComAPIBase 企业微信 API 基础地址
	WeComAPIBase = "https://qyapi.weixin.qq.com/cgi-bin"
	// WeComMessageSend 消息发送接口
	WeComMessageSend = WeComAPIBase + "/message/send"
	// WeComMessageUpdateTaskcard 更新任务卡片接口
	WeComMessageUpdateTaskcard = WeComAPIBase + "/message/update_taskcard"
	// WeComGetToken 获取 Token 接口
	WeComGetToken = WeComAPIBase + "/gettoken"
	// WeComMediaUpload 媒体文件上传接口
	WeComMediaUpload = WeComAPIBase + "/media/upload"
	// WeComMediaGet 获取媒体文件接口
	WeComMediaGet = WeComAPIBase + "/media/get"
)

// MIME 类型
const (
	// MIMEOctetStream 通用二进制流
	MIMEOctetStream = "application/octet-stream"
	// MIMEImageJPEG JPEG 图片
	MIMEImageJPEG = "image/jpeg"
	// MIMEImagePNG PNG 图片
	MIMEImagePNG = "image/png"
)

// 媒体占位符
const (
	// PlaceholderImage 图片占位符
	PlaceholderImage = "[图片]"
	// PlaceholderVoice 语音占位符
	PlaceholderVoice = "[语音]"
	// PlaceholderVideo 视频占位符
	PlaceholderVideo = "[视频]"
	// PlaceholderFile 文件占位符
	PlaceholderFile = "[文件]"
	// PlaceholderLink 链接占位符
	PlaceholderLink = "[链接]"
	// PlaceholderLocation 位置占位符
	PlaceholderLocation = "[位置]"
)

// 响应文本
const (
	// ResponseSuccess 成功响应
	ResponseSuccess = "success"
)

// 消息类型
const (
	// MsgTypeText 文本消息
	MsgTypeText = "text"
	// MsgTypeMarkdown Markdown 消息
	MsgTypeMarkdown = "markdown"
	// MsgTypeImage 图片消息
	MsgTypeImage = "image"
	// MsgTypeFile 文件消息
	MsgTypeFile = "file"
	// MsgTypeVoice 语音消息
	MsgTypeVoice = "voice"
	// MsgTypeVideo 视频消息
	MsgTypeVideo = "video"
	// MsgTypeLocation 位置消息
	MsgTypeLocation = "location"
	// MsgTypeLink 链接消息
	MsgTypeLink = "link"
)
