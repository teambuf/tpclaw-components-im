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

// Package endpoint provides IM platform endpoints for receiving messages.
// IM 平台端点，用于接收消息。
package endpoint

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"mime"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/maps"
	feishu2 "github.com/teambuf/tpclaw-components-im/adapter/feishu"
	api2 "github.com/teambuf/tpclaw-components-im/api"
	"golang.org/x/image/draw"
)

const (
	// TypeFeishuWebSocket 飞书 WebSocket 端点类型
	TypeFeishuWebSocket = "feishu"
)

// 注册组件
func init() {
	_ = endpoint.Registry.Register(&FeishuWebSocket{})
}

// FeishuWebSocketConfig 飞书 WebSocket 端点配置
type FeishuWebSocketConfig struct {
	// AccountID 账号 ID（用于多账号区分）
	AccountID string `json:"accountId"`
	// AppID 飞书应用 ID
	AppID string `json:"appId"`
	// AppSecret 飞书应用密钥
	AppSecret string `json:"appSecret"`
	// TenantKey 租户 key（可选）
	TenantKey string `json:"tenantKey"`
	// AutoReconnect 是否自动重连（默认 true）
	AutoReconnect bool `json:"autoReconnect"`
	// LogLevel 日志级别：0-debug, 1-info, 2-warn, 3-error
	LogLevel int `json:"logLevel"`
	// ThinkingText 思考中回复的文本（默认 "🤔 思考中..."）
	ThinkingText string `json:"thinkingText"`
	// ImageMaxSize 图片最大尺寸（像素），超过会等比缩放。0 表示不压缩（默认 1024）
	ImageMaxSize int `json:"imageMaxSize"`
	// Media 媒体存储配置
	Media MediaConfig `json:"media"`
}

// MediaConfig 媒体存储配置
type MediaConfig struct {
	// Enable 是否启用媒体存储（默认 false，图片直接 base64 内联）
	Enable bool `json:"enable"`
	// StoragePath 存储目录（相对于工作空间，默认 "media"）
	StoragePath string `json:"storagePath"`
	// MaxFileSize 最大文件大小（字节，默认 50MB）
	MaxFileSize int64 `json:"maxFileSize"`
}

// FeishuWebSocket 飞书 WebSocket 端点（基于官方 SDK 实现）
// 用于通过 WebSocket 长连接接收飞书消息
type FeishuWebSocket struct {
	// BaseEndpoint 提供通用端点功能
	impl.BaseEndpoint
	// RuleConfig 规则引擎配置
	RuleConfig types.Config
	// Config 飞书配置
	Config FeishuWebSocketConfig
	// WorkspaceDir 工作空间目录（用于存储媒体文件）
	WorkspaceDir string
	// ctx 上下文
	ctx context.Context
	// cancel 取消函数
	cancel context.CancelFunc
	// wg 等待组
	wg sync.WaitGroup
	// started 是否已启动
	started bool
	// mu 互斥锁
	mu sync.RWMutex
	// wsClient SDK WebSocket 客户端
	wsClient *larkws.Client
	// eventDispatcher 事件分发器
	eventDispatcher *dispatcher.EventDispatcher
	// larkClient 飞书 API 客户端（用于发送消息）
	larkClient *lark.Client
}

// NewFeishuWebSocket 创建飞书 WebSocket 端点
func NewFeishuWebSocket() *FeishuWebSocket {
	return &FeishuWebSocket{}
}

// Type 返回端点类型
func (e *FeishuWebSocket) Type() string {
	return types.EndpointTypePrefix + TypeFeishuWebSocket
}

// New 创建新实例
func (e *FeishuWebSocket) New() types.Node {
	return NewFeishuWebSocket()
}

// Init 初始化端点
func (e *FeishuWebSocket) Init(config types.Config, configuration types.Configuration) error {
	if err := maps.Map2Struct(configuration, &e.Config); err != nil {
		return fmt.Errorf("failed to parse feishu websocket config: %w", err)
	}

	if e.Config.AppID == "" {
		return fmt.Errorf("appId is required")
	}
	if e.Config.AppSecret == "" {
		return fmt.Errorf("appSecret is required")
	}

	// 设置默认值
	e.Config.AutoReconnect = true // 默认开启自动重连

	// 设置思考中回复默认值
	if e.Config.ThinkingText == "" {
		e.Config.ThinkingText = "🤔 思考中..."
	}

	// 设置图片压缩默认值（1024 像素）
	if e.Config.ImageMaxSize == 0 {
		e.Config.ImageMaxSize = 1024
	}

	// 设置媒体存储默认值
	if e.Config.Media.StoragePath == "" {
		e.Config.Media.StoragePath = "media"
	}
	if e.Config.Media.MaxFileSize == 0 {
		e.Config.Media.MaxFileSize = 50 * 1024 * 1024 // 50MB
	}

	// 设置工作空间目录（使用 StoragePath，应用层应传入绝对路径）
	e.WorkspaceDir = e.Config.Media.StoragePath

	e.RuleConfig = config
	e.ctx, e.cancel = context.WithCancel(context.Background())

	// 创建飞书 API 客户端（用于发送消息）
	e.larkClient = lark.NewClient(e.Config.AppID, e.Config.AppSecret, lark.WithEnableTokenCache(true))

	// 创建事件分发器（WebSocket 模式下不需要 VerificationToken 和 EncryptKey）
	e.eventDispatcher = dispatcher.NewEventDispatcher(
		"",
		"",
	)

	// 注册消息接收处理器
	e.eventDispatcher.OnP2MessageReceiveV1(e.handleMessageReceive)

	// 注册其他事件处理器
	e.eventDispatcher.OnP2ChatMemberUserAddedV1(e.handleChatMemberUserAdded)
	e.eventDispatcher.OnP2ChatMemberUserDeletedV1(e.handleChatMemberUserDeleted)
	e.eventDispatcher.OnP2ChatMemberBotAddedV1(e.handleChatMemberBotAdded)
	e.eventDispatcher.OnP2ChatMemberBotDeletedV1(e.handleChatMemberBotDeleted)
	e.eventDispatcher.OnP2MessageReadV1(e.handleMessageRead)

	return nil
}

// Id 返回端点 ID
func (e *FeishuWebSocket) Id() string {
	return e.Config.AppID
}

// SetOnEvent 设置事件回调
func (e *FeishuWebSocket) SetOnEvent(onEvent endpointApi.OnEvent) {
	e.BaseEndpoint.OnEvent = onEvent
}

// SetWorkspaceDir 设置工作空间目录
func (e *FeishuWebSocket) SetWorkspaceDir(dir string) {
	e.WorkspaceDir = dir
}

// Start 启动端点
func (e *FeishuWebSocket) Start() error {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return nil
	}
	e.started = true

	// 创建新的 context（如果之前的已被取消）
	if e.ctx != nil && e.ctx.Err() != nil {
		// 之前的 context 已取消，创建新的
		e.ctx, e.cancel = context.WithCancel(context.Background())
	}
	e.mu.Unlock()

	// 创建 SDK WebSocket 客户端
	opts := []larkws.ClientOption{
		larkws.WithEventHandler(e.eventDispatcher),
		larkws.WithLogLevel(larkcore.LogLevel(e.Config.LogLevel)),
		larkws.WithAutoReconnect(e.Config.AutoReconnect),
		larkws.WithLogger(&feishuLogger{logger: e.RuleConfig.Logger}),
	}

	e.wsClient = larkws.NewClient(e.Config.AppID, e.Config.AppSecret, opts...)

	// 启动 WebSocket 连接
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		err := e.wsClient.Start(e.ctx)
		if err != nil && e.ctx.Err() == nil {
			e.RuleConfig.Logger.Errorf("Feishu WebSocket error: %v", err)
			e.triggerEvent(endpointApi.EventDisconnect, err)
		}
	}()

	e.triggerEvent(endpointApi.EventConnect, nil)
	e.RuleConfig.Logger.Infof("Feishu WebSocket endpoint started, appId: %s", e.Config.AppID)
	return nil
}

// AddRouter 添加路由器
func (e *FeishuWebSocket) AddRouter(router endpointApi.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", fmt.Errorf("router can not be nil")
	}
	e.CheckAndSetRouterId(router)
	e.saveRouter(router)
	return router.GetId(), nil
}

// RemoveRouter 移除路由器
func (e *FeishuWebSocket) RemoveRouter(routerId string, params ...interface{}) error {
	router := e.deleteRouter(routerId)
	if router == nil {
		return fmt.Errorf("router not found: %s", routerId)
	}
	return nil
}

// Destroy 销毁端点
func (e *FeishuWebSocket) Destroy() {
	e.mu.Lock()
	e.started = false
	e.mu.Unlock()

	// 先关闭 WebSocket 客户端（这会关闭连接并停止自动重连）
	if e.wsClient != nil {
		e.wsClient.Close()
	}

	// 再取消 context，让 SDK 的 Start 方法退出
	if e.cancel != nil {
		e.cancel()
	}

	// 等待 WebSocket 连接关闭，最多等待 5 秒
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 正常关闭
		e.RuleConfig.Logger.Infof("Feishu WebSocket endpoint destroyed, appId: %s", e.Config.AppID)
	case <-time.After(5 * time.Second):
		// 超时，记录警告
		e.RuleConfig.Logger.Warnf("Feishu WebSocket destroy timeout, appId: %s", e.Config.AppID)
		// 即使超时也继续，context 已取消，SDK 应该最终会退出
	}

	// 清空客户端引用
	e.wsClient = nil
}

// handleMessageReceive 处理接收到的消息事件
func (e *FeishuWebSocket) handleMessageReceive(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event.Event == nil || event.Event.Message == nil {
		return nil
	}

	// 如果 endpoint 已销毁（ctx 被取消），则不处理消息
	if e.ctx != nil && e.ctx.Err() != nil {
		if e.RuleConfig.Logger != nil {
			e.RuleConfig.Logger.Warnf("[Feishu] endpoint is destroyed, ignore message: %s", larkcore.StringValue(event.Event.Message.MessageId))
		}
		return nil
	}

	// DEBUG: 打印原始接收到的飞书消息
	if e.RuleConfig.Logger != nil {
		contentPreview := ""
		if event.Event.Message.Content != nil {
			contentPreview = *event.Event.Message.Content
			if len(contentPreview) > 500 {
				contentPreview = contentPreview[:500] + "..."
			}
		}
		e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 收到消息: msgType=%s, chatId=%s, content=%s",
			larkcore.StringValue(event.Event.Message.MessageType),
			larkcore.StringValue(event.Event.Message.ChatId),
			contentPreview)
	}

	// 构建 IMMessage
	msg := &api2.IMMessage{
		Platform:  api2.PlatformFeishu,
		EventType: "im.message.receive_v1",
		RawData:   []byte{},
	}

	// 设置消息基本信息
	msg.ID = larkcore.StringValue(event.Event.Message.MessageId)
	msg.MsgType = larkcore.StringValue(event.Event.Message.MessageType)
	msg.ChatID = larkcore.StringValue(event.Event.Message.ChatId)
	msg.ChatType = larkcore.StringValue(event.Event.Message.ChatType)

	// 设置发送者信息
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		msg.Sender = &api2.IMSender{
			UserID:  larkcore.StringValue(event.Event.Sender.SenderId.UserId),
			OpenID:  larkcore.StringValue(event.Event.Sender.SenderId.OpenId),
			UnionID: larkcore.StringValue(event.Event.Sender.SenderId.UnionId),
		}
	}

	// 解析消息内容（传递消息 ID 用于下载富文本图片）
	if event.Event.Message.Content != nil {
		msg.Content, msg.Extensions = e.parseMessageContentWithImages(ctx, msg.ID, msg.MsgType, *event.Event.Message.Content)
	}

	// 设置扩展信息
	if msg.Extensions == nil {
		msg.Extensions = make(map[string]interface{})
	}
	msg.Extensions[api2.MetaFeishuAppID] = event.EventV2Base.Header.AppID
	msg.Extensions[api2.MetaFeishuTenantKey] = event.EventV2Base.Header.TenantKey
	msg.Extensions[api2.MetaFeishuEventID] = event.EventV2Base.Header.EventID
	msg.Extensions[api2.MetaFeishuRootID] = larkcore.StringValue(event.Event.Message.RootId)
	msg.Extensions[api2.MetaFeishuParentID] = larkcore.StringValue(event.Event.Message.ParentId)

	// 获取引用/回复的消息内容
	parentID := larkcore.StringValue(event.Event.Message.ParentId)
	if parentID != "" {
		tenantKey := msg.Extensions[api2.MetaFeishuTenantKey].(string)
		quoteContent := e.getQuoteMessageContent(ctx, parentID, tenantKey)
		if quoteContent != "" {
			msg.Extensions[api2.MetaQuoteContent] = quoteContent
		}
	}

	// 发送"思考中"回复（使用消息 ID 进行回复），保存消息 ID 用于后续 Patch 更新
	thinkingMsgId := e.sendThinkingReply(msg.ID, msg.Extensions[api2.MetaFeishuTenantKey])

	// 处理消息
	e.processMessage(msg, thinkingMsgId)

	return nil
}

// handleChatMemberUserAdded 处理用户进群事件
func (e *FeishuWebSocket) handleChatMemberUserAdded(ctx context.Context, event *larkim.P2ChatMemberUserAddedV1) error {
	msg := &api2.IMMessage{
		Platform:  api2.PlatformFeishu,
		EventType: "im.chat.member.user.added_v1",
		MsgType:   "event",
	}

	if event.Event != nil {
		msg.ChatID = larkcore.StringValue(event.Event.ChatId)
	}

	e.processMessage(msg, "")
	return nil
}

// handleChatMemberUserDeleted 处理用户出群事件
func (e *FeishuWebSocket) handleChatMemberUserDeleted(ctx context.Context, event *larkim.P2ChatMemberUserDeletedV1) error {
	msg := &api2.IMMessage{
		Platform:  api2.PlatformFeishu,
		EventType: "im.chat.member.user.deleted_v1",
		MsgType:   "event",
	}

	if event.Event != nil {
		msg.ChatID = larkcore.StringValue(event.Event.ChatId)
	}

	e.processMessage(msg, "")
	return nil
}

// handleChatMemberBotAdded 处理机器人进群事件
func (e *FeishuWebSocket) handleChatMemberBotAdded(ctx context.Context, event *larkim.P2ChatMemberBotAddedV1) error {
	msg := &api2.IMMessage{
		Platform:  api2.PlatformFeishu,
		EventType: "im.chat.member.bot.added_v1",
		MsgType:   "event",
	}

	if event.Event != nil {
		msg.ChatID = larkcore.StringValue(event.Event.ChatId)
	}

	e.processMessage(msg, "")
	return nil
}

// handleChatMemberBotDeleted 处理机器人被移出群事件
func (e *FeishuWebSocket) handleChatMemberBotDeleted(ctx context.Context, event *larkim.P2ChatMemberBotDeletedV1) error {
	msg := &api2.IMMessage{
		Platform:  api2.PlatformFeishu,
		EventType: "im.chat.member.bot.deleted_v1",
		MsgType:   "event",
	}

	if event.Event != nil {
		msg.ChatID = larkcore.StringValue(event.Event.ChatId)
	}

	e.processMessage(msg, "")
	return nil
}

// handleMessageRead 处理消息已读事件
// 注意：已读回执事件不需要往下传给 Agent，只需返回 nil 表示 ACK 即可
func (e *FeishuWebSocket) handleMessageRead(ctx context.Context, event *larkim.P2MessageReadV1) error {
	// 已读回执事件不需要处理，直接 ACK
	return nil
}

// processMessage 处理消息并路由
// thinkingMsgId: "思考中"卡片的消息 ID，用于最终 Patch 更新内容（为空则发新消息）
func (e *FeishuWebSocket) processMessage(msg *api2.IMMessage, thinkingMsgId string) {
	// 如果 endpoint 已销毁（ctx 被取消），则不处理消息
	if e.ctx != nil && e.ctx.Err() != nil {
		if e.RuleConfig.Logger != nil {
			e.RuleConfig.Logger.Warnf("[Feishu] endpoint is destroyed, ignore message: %s", msg.ID)
		}
		return
	}

	// 构建消息数据
	// 如果包含图片，构建多模态消息格式
	messageData := e.buildMessageData(msg)
	messageJSON, _ := json.Marshal(messageData)

	// 创建 RuleMsg
	ruleMsg := types.NewMsg(0, msg.EventType, types.JSON, types.NewMetadata(), string(messageJSON))
	ruleMsgPtr := &ruleMsg

	// 设置元数据
	for k, v := range msg.ToMetadata() {
		ruleMsg.Metadata.PutValue(k, v)
	}

	// 设置通道类型（用于切面识别通道）
	ruleMsg.Metadata.PutValue(api2.MetaChannel, api2.ChannelFeishu)
	ruleMsg.Metadata.PutValue("im.platform", api2.PlatformFeishu)

	// IM 通道需要加载历史消息以保持上下文
	ruleMsg.Metadata.PutValue(api2.MetaLoadHistory, "true")

	// 设置机器人 ID 和 Account ID
	ruleMsg.Metadata.PutValue(api2.MetaBotID, e.Config.AppID)
	if e.Config.AccountID != "" {
		ruleMsg.Metadata.PutValue(api2.MetaAccountID, e.Config.AccountID)
	} else {
		ruleMsg.Metadata.PutValue(api2.MetaAccountID, e.Config.AppID)
	}

	// 获取 tenantKey（优先从消息元数据获取，其次从配置获取）
	tenantKey := ""
	if tk, ok := msg.Extensions[api2.MetaFeishuTenantKey].(string); ok {
		tenantKey = tk
	}
	if tenantKey == "" {
		tenantKey = e.Config.TenantKey
	}

	// 获取消息 ID 用于错误回复
	messageId := msg.ID
	chatId := msg.ChatID

	// 将"思考中"卡片的消息 ID 传入 metadata，用于最终 Patch 更新
	if thinkingMsgId != "" {
		ruleMsg.Metadata.PutValue("im.thinkingMsgId", thinkingMsgId)
	}

	// 创建 Exchange
	exchange := &endpoint.Exchange{
		In:  NewFeishuRequestMessage(ruleMsgPtr),
		Out: NewFeishuResponseMessage(e.larkClient, ruleMsgPtr, tenantKey, e.RuleConfig.Logger),
	}

	// 遍历所有路由器并处理
	for _, router := range e.RouterStorage {
		if router == nil {
			continue
		}
		e.DoProcess(e.ctx, router, exchange)
	}

	// 检查是否有错误，如果有则发送友好的错误提示给用户
	if exchange.Out.GetError() != nil {
		e.sendErrorMessage(messageId, chatId, tenantKey, exchange.Out.GetError())
	}
}

// sendErrorMessage 发送错误消息给用户（友好提示）
func (e *FeishuWebSocket) sendErrorMessage(messageId, chatId, tenantKey string, err error) {
	if err == nil {
		return
	}

	// 构建友好的错误提示
	errorMessage := e.buildFriendlyErrorMessage(err)

	e.RuleConfig.Logger.Warnf("[Feishu] Agent error: %v", err)

	// 优先使用消息 ID 回复
	if messageId != "" {
		e.sendReplyMessage(messageId, tenantKey, errorMessage)
	} else if chatId != "" {
		e.sendNewMessage(chatId, tenantKey, errorMessage)
	}
}

// buildFriendlyErrorMessage 构建友好的错误提示消息
func (e *FeishuWebSocket) buildFriendlyErrorMessage(err error) string {
	errMsg := err.Error()

	// 根据错误类型生成友好的提示
	switch {
	case strings.Contains(errMsg, "tool not found"):
		return "抱歉，工具调用失败。请稍后重试或联系管理员检查工具配置。"
	case strings.Contains(errMsg, "context deadline exceeded") || strings.Contains(errMsg, "timeout"):
		return "抱歉，处理超时了。请稍后重试或简化您的问题。"
	case strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "429"):
		return "抱歉，API 调用频率超限。请稍后重试。"
	case strings.Contains(errMsg, "authentication") || strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "401"):
		return "抱歉，认证失败。请联系管理员检查 API 配置。"
	case strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "network"):
		return "抱歉，网络连接出现问题。请稍后重试。"
	default:
		return fmt.Sprintf("抱歉，处理您的请求时出现错误。请稍后重试。\n\n错误详情：%s", truncateError(errMsg, 200))
	}
}

// sendReplyMessage 回复消息（使用卡片格式）
func (e *FeishuWebSocket) sendReplyMessage(messageId, tenantKey, message string) {
	cardContent := feishu2.NewCardV2().
		AddMarkdown(message).
		MustString()

	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageId).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			Content(cardContent).
			ReplyInThread(false).
			Build()).
		Build()

	var opts []larkcore.RequestOptionFunc
	if tenantKey != "" {
		opts = append(opts, larkcore.WithTenantKey(tenantKey))
	}

	resp, err := e.larkClient.Im.V1.Message.Reply(context.Background(), req, opts...)
	if err != nil {
		e.RuleConfig.Logger.Warnf("[Feishu] send error reply failed: %v", err)
		return
	}
	if !resp.Success() {
		e.RuleConfig.Logger.Warnf("[Feishu] send error reply failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}
}

// sendNewMessage 发送新消息（使用卡片格式）
func (e *FeishuWebSocket) sendNewMessage(chatId, tenantKey, message string) {
	cardContent := feishu2.NewCardV2().
		AddMarkdown(message).
		MustString()

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			ReceiveId(chatId).
			Content(cardContent).
			Build()).
		Build()

	var opts []larkcore.RequestOptionFunc
	if tenantKey != "" {
		opts = append(opts, larkcore.WithTenantKey(tenantKey))
	}

	resp, err := e.larkClient.Im.Message.Create(context.Background(), req, opts...)
	if err != nil {
		e.RuleConfig.Logger.Warnf("[Feishu] send error message failed: %v", err)
		return
	}
	if !resp.Success() {
		e.RuleConfig.Logger.Warnf("[Feishu] send error message failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// truncateError 截断错误信息
func truncateError(errMsg string, maxLen int) string {
	// 移除多余的换行和空格
	errMsg = strings.ReplaceAll(errMsg, "\n", " ")
	errMsg = strings.TrimSpace(errMsg)
	return truncateString(errMsg, maxLen)
}

// ChatRequest 用于构建发送给 Agent 的请求格式（OpenAI 标准）
type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
}

// ChatMessage 单条消息（OpenAI 标准格式）
type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string 或 []ContentPart（多模态）
}

// ContentPart OpenAI 标准的消息内容部分
type ContentPart struct {
	Type     string    `json:"type"`                // "text" 或 "image_url"
	Text     string    `json:"text,omitempty"`      // 文本内容（type="text" 时）
	ImageURL *ImageURL `json:"image_url,omitempty"` // 图片 URL（type="image_url" 时）
}

// ImageURL 图片 URL 结构
type ImageURL struct {
	URL string `json:"url"` // 图片 URL 或 base64 数据
}

// buildMessageData 构建发送给 Agent 的消息数据（OpenAI 标准格式）
func (e *FeishuWebSocket) buildMessageData(msg *api2.IMMessage) *ChatRequest {
	// 检查是否有图片
	images, hasImages := msg.Extensions[api2.MetaImages].([]string)

	// 获取引用消息内容
	quoteContent := ""
	if qc, ok := msg.Extensions[api2.MetaQuoteContent].(string); ok && qc != "" {
		quoteContent = qc
	}

	// 构建最终的用户消息内容（如果有引用内容则拼接）
	userContent := msg.Content
	if quoteContent != "" {
		userContent = fmt.Sprintf("[引用消息]\n%s\n\n%s", quoteContent, msg.Content)
	}

	// DEBUG: 打印 extensions 内容
	e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] buildMessageData: msgType=%s, content=%s, hasImages=%v, imagesCount=%d, hasQuote=%v",
		msg.MsgType, msg.Content, hasImages, len(images), quoteContent != "")

	// DEBUG: 打印所有 extensions
	for k, v := range msg.Extensions {
		// 截断过长的值（如 base64 图片数据）
		valStr := fmt.Sprintf("%v", v)
		if len(valStr) > 200 {
			valStr = valStr[:200] + "..."
		}
		e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] Extension: key=%s, type=%T, value=%s", k, v, valStr)
	}

	if hasImages && len(images) > 0 {
		// 多模态格式：content 为 ContentPart 数组
		var contentParts []ContentPart

		// 添加图片
		for i, img := range images {
			// 截断日志显示
			imgPreview := img
			if len(img) > 100 {
				imgPreview = img[:100] + "..."
			}
			e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 添加图片[%d]: %s", i, imgPreview)
			contentParts = append(contentParts, ContentPart{
				Type: "image_url",
				ImageURL: &ImageURL{
					URL: img, // base64 数据或本地路径
				},
			})
		}

		// 添加文本
		if userContent != "" {
			contentParts = append(contentParts, ContentPart{
				Type: "text",
				Text: userContent,
			})
		}

		result := &ChatRequest{
			Messages: []ChatMessage{
				{
					Role:    "user",
					Content: contentParts,
				},
			},
		}

		// DEBUG: 打印构建的消息 JSON
		jsonData, _ := json.Marshal(result)
		e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 构建的多模态消息(JSON): %s", string(jsonData))

		return result
	}

	// 纯文本格式：content 为字符串
	result := &ChatRequest{
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: userContent,
			},
		},
	}

	// DEBUG: 打印构建的消息 JSON
	jsonData, _ := json.Marshal(result)
	e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 构建的纯文本消息(JSON): %s", string(jsonData))

	return result
}

// parseMessageContentWithImages 解析消息内容，支持图片下载转 base64
// 返回文本内容和扩展信息（包含图片 base64 数据）
// messageId 用于下载富文本中的图片（需要使用消息资源 API）
func (e *FeishuWebSocket) parseMessageContentWithImages(ctx context.Context, messageId, msgType, content string) (string, map[string]interface{}) {
	extensions := make(map[string]interface{})

	switch msgType {
	case "image":
		// 解析图片消息
		var imgContent feishu2.ImageContent
		if err := json.Unmarshal([]byte(content), &imgContent); err != nil {
			e.RuleConfig.Logger.Warnf("[Feishu] parse image content error: %v", err)
			return content, extensions
		}

		e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 图片消息: imageKey=%s", imgContent.ImageKey)

		// 默认保存图片到本地（大模型节点会自动读取并转 base64）
		// 如果 WorkspaceDir 未设置，则回退到 base64 方式
		if e.WorkspaceDir != "" {
			mediaFile, err := e.saveMediaFile(ctx, "image", imgContent.ImageKey, "")
			if err != nil {
				e.RuleConfig.Logger.Warnf("[Feishu][DEBUG] save image error: %v, fallback to base64", err)
				// 保存失败，回退到 base64
				base64Img, err := e.getImageAsBase64(ctx, imgContent.ImageKey)
				if err != nil {
					e.RuleConfig.Logger.Warnf("[Feishu][DEBUG] get image base64 error: %v", err)
					return "[图片加载失败]", extensions
				}
				e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 图片转 base64 成功: 长度=%d", len(base64Img))
				extensions[api2.MetaImages] = []string{base64Img}
				return "[图片]", extensions
			}
			// 保存成功，存储本地文件路径（绝对路径）
			// 大模型节点会自动读取本地文件并转 base64
			absolutePath := filepath.Join(e.WorkspaceDir, mediaFile.RelativePath)
			e.RuleConfig.Logger.Infof("[Feishu] 图片保存成功: %s", absolutePath)
			extensions[api2.MetaImages] = []string{absolutePath}
			extensions[api2.MetaMediaFiles] = []api2.MediaFile{mediaFile}
			extensions[api2.MetaHasMedia] = true
			return "[图片]", extensions
		}

		// WorkspaceDir 未设置，回退到 base64 方式
		base64Img, err := e.getImageAsBase64(ctx, imgContent.ImageKey)
		if err != nil {
			e.RuleConfig.Logger.Warnf("[Feishu][DEBUG] get image base64 error: %v", err)
			return "[图片加载失败]", extensions
		}
		e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 图片转 base64 成功: 长度=%d", len(base64Img))
		extensions[api2.MetaImages] = []string{base64Img}
		return "[图片]", extensions

	case "audio":
		// 解析语音消息
		var audioContent struct {
			FileKey  string `json:"file_key"`
			Duration int    `json:"duration"`
		}
		if err := json.Unmarshal([]byte(content), &audioContent); err != nil {
			e.RuleConfig.Logger.Warnf("[Feishu] parse audio content error: %v", err)
			return content, extensions
		}

		// 保存语音文件
		if e.Config.Media.Enable && e.WorkspaceDir != "" {
			mediaFile, err := e.saveMediaFile(ctx, "audio", audioContent.FileKey, "")
			if err != nil {
				e.RuleConfig.Logger.Warnf("[Feishu] save audio error: %v", err)
				return "[语音保存失败]", extensions
			}
			mediaFile.Duration = audioContent.Duration
			extensions[api2.MetaMediaFiles] = []api2.MediaFile{mediaFile}
			extensions[api2.MetaHasMedia] = true
			return fmt.Sprintf("[语音消息] (时长: %dms)", audioContent.Duration), extensions
		}
		return fmt.Sprintf("[语音消息] (时长: %dms)", audioContent.Duration), extensions

	case "video":
		// 解析视频消息
		var videoContent struct {
			FileKey  string `json:"file_key"`
			Duration int    `json:"duration"`
		}
		if err := json.Unmarshal([]byte(content), &videoContent); err != nil {
			e.RuleConfig.Logger.Warnf("[Feishu] parse video content error: %v", err)
			return content, extensions
		}

		// 保存视频文件
		if e.Config.Media.Enable && e.WorkspaceDir != "" {
			mediaFile, err := e.saveMediaFile(ctx, "video", videoContent.FileKey, "")
			if err != nil {
				e.RuleConfig.Logger.Warnf("[Feishu] save video error: %v", err)
				return "[视频保存失败]", extensions
			}
			mediaFile.Duration = videoContent.Duration
			extensions[api2.MetaMediaFiles] = []api2.MediaFile{mediaFile}
			extensions[api2.MetaHasMedia] = true
			return fmt.Sprintf("[视频消息] (时长: %dms)", videoContent.Duration), extensions
		}
		return fmt.Sprintf("[视频消息] (时长: %dms)", videoContent.Duration), extensions

	case "file":
		// 解析文件消息
		var fileContent struct {
			FileKey  string `json:"file_key"`
			FileName string `json:"file_name"`
			FileSize int64  `json:"file_size"`
		}
		if err := json.Unmarshal([]byte(content), &fileContent); err != nil {
			e.RuleConfig.Logger.Warnf("[Feishu] parse file content error: %v", err)
			return content, extensions
		}

		// 保存文件
		if e.Config.Media.Enable && e.WorkspaceDir != "" {
			mediaFile, err := e.saveMediaFile(ctx, "file", fileContent.FileKey, fileContent.FileName)
			if err != nil {
				e.RuleConfig.Logger.Warnf("[Feishu] save file error: %v", err)
				return "[文件保存失败]", extensions
			}
			extensions[api2.MetaMediaFiles] = []api2.MediaFile{mediaFile}
			extensions[api2.MetaHasMedia] = true
			return fmt.Sprintf("[文件: %s] (大小: %d bytes)", fileContent.FileName, fileContent.FileSize), extensions
		}
		return fmt.Sprintf("[文件: %s] (大小: %d bytes)", fileContent.FileName, fileContent.FileSize), extensions

	case "post":
		// 解析富文本消息
		return e.parsePostContent(ctx, messageId, content, extensions)

	case "text":
		var textContent feishu2.TextContent
		if err := json.Unmarshal([]byte(content), &textContent); err != nil {
			return content, extensions
		}
		return textContent.Text, extensions

	default:
		return content, extensions
	}
}

// parsePostContent 解析富文本消息，提取文本和图片
// messageId 用于下载富文本中的图片（需要使用消息资源 API）
func (e *FeishuWebSocket) parsePostContent(ctx context.Context, messageId, content string, extensions map[string]interface{}) (string, map[string]interface{}) {
	e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 解析富文本消息: content=%s", content)

	var postContent struct {
		Title   string                     `json:"title"`
		Content [][]map[string]interface{} `json:"content"`
	}
	if err := json.Unmarshal([]byte(content), &postContent); err != nil {
		e.RuleConfig.Logger.Warnf("[Feishu] parse post content error: %v", err)
		return content, extensions
	}

	var textBuilder strings.Builder
	var mediaFiles []api2.MediaFile
	var images []string

	// 添加标题
	if postContent.Title != "" {
		textBuilder.WriteString(postContent.Title)
		textBuilder.WriteString("\n")
	}

	// 遍历内容
	for _, paragraph := range postContent.Content {
		for _, element := range paragraph {
			tag, _ := element["tag"].(string)
			switch tag {
			case "text":
				if text, ok := element["text"].(string); ok {
					textBuilder.WriteString(text)
				}
			case "a":
				if text, ok := element["text"].(string); ok {
					textBuilder.WriteString(text)
				}
			case "at":
				if userName, ok := element["user_name"].(string); ok {
					textBuilder.WriteString("@")
					textBuilder.WriteString(userName)
				}
			case "img":
				textBuilder.WriteString("[图片]")
				// 处理图片（富文本图片需要使用消息资源 API GetMessageResource）
				if imageKey, ok := element["image_key"].(string); ok && imageKey != "" {
					e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 富文本中发现图片: imageKey=%s, messageId=%s", imageKey, messageId)
					// 默认保存图片到本地（大模型节点会自动读取并转 base64）
					if e.WorkspaceDir != "" {
						mediaFile, err := e.saveMessageResourceFile(ctx, messageId, imageKey)
						if err == nil {
							mediaFiles = append(mediaFiles, mediaFile)
							// 存储本地文件路径（绝对路径）
							absolutePath := filepath.Join(e.WorkspaceDir, mediaFile.RelativePath)
							images = append(images, absolutePath)
							e.RuleConfig.Logger.Infof("[Feishu] 富文本图片保存成功: %s", absolutePath)
						} else {
							e.RuleConfig.Logger.Warnf("[Feishu][DEBUG] 保存图片失败: %v, fallback to base64", err)
							// 保存失败，回退到 base64
							base64Img, err := e.getMessageResourceAsBase64(ctx, messageId, imageKey)
							if err == nil {
								e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 富文本图片转 base64 成功: 长度=%d", len(base64Img))
								images = append(images, base64Img)
							} else {
								e.RuleConfig.Logger.Warnf("[Feishu][DEBUG] 富文本图片下载失败: %v", err)
							}
						}
					} else {
						// WorkspaceDir 未设置，回退到 base64 方式
						base64Img, err := e.getMessageResourceAsBase64(ctx, messageId, imageKey)
						if err == nil {
							e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 富文本图片转 base64 成功: 长度=%d", len(base64Img))
							images = append(images, base64Img)
						} else {
							e.RuleConfig.Logger.Warnf("[Feishu][DEBUG] 富文本图片下载失败: %v", err)
						}
					}
				} else {
					e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 富文本中图片元素缺少 image_key: element=%v", element)
				}
			}
		}
		textBuilder.WriteString("\n")
	}

	// 设置扩展信息
	if len(mediaFiles) > 0 {
		extensions[api2.MetaMediaFiles] = mediaFiles
		extensions[api2.MetaHasMedia] = true
	}
	if len(images) > 0 {
		extensions[api2.MetaImages] = images
	}

	e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 富文本解析完成: 文本=%s, 图片数量=%d", textBuilder.String(), len(images))

	return textBuilder.String(), extensions
}

// saveMediaFile 保存媒体文件到工作空间
func (e *FeishuWebSocket) saveMediaFile(ctx context.Context, mediaType, fileKey, fileName string) (api2.MediaFile, error) {
	// 1. 下载文件
	fileData, mimeType, err := e.downloadFile(ctx, mediaType, fileKey)
	if err != nil {
		return api2.MediaFile{}, fmt.Errorf("download file failed: %w", err)
	}

	// 2. 检查文件大小
	if int64(len(fileData)) > e.Config.Media.MaxFileSize {
		return api2.MediaFile{}, fmt.Errorf("file size %d exceeds limit %d", len(fileData), e.Config.Media.MaxFileSize)
	}

	// 3. 构建存储路径
	// WorkspaceDir 已经是完整的存储目录（如 /data/media/main）
	// 简化路径: feishu/image/
	dirPath := filepath.Join(e.WorkspaceDir, "feishu", mediaType)

	// 4. 生成文件名
	now := time.Now()
	if fileName == "" {
		ext := mimeToExt(mimeType)
		fileName = fmt.Sprintf("%s_%d%s", mediaType, now.UnixMilli(), ext)
	}

	filePath := filepath.Join(dirPath, fileName)

	// 5. 创建目录并保存文件
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return api2.MediaFile{}, fmt.Errorf("create directory failed: %w", err)
	}
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		return api2.MediaFile{}, fmt.Errorf("write file failed: %w", err)
	}

	// 6. 计算相对路径（相对于 WorkspaceDir）
	relativePath := filepath.Join("feishu", mediaType, fileName)

	e.RuleConfig.Logger.Debugf("[Feishu] media saved: %s (%d bytes)", filePath, len(fileData))

	return api2.MediaFile{
		Type:         mediaType,
		FileName:     fileName,
		RelativePath: relativePath,
		FileSize:     int64(len(fileData)),
		MimeType:     mimeType,
	}, nil
}

// downloadFile 下载飞书文件
func (e *FeishuWebSocket) downloadFile(ctx context.Context, mediaType, fileKey string) ([]byte, string, error) {
	switch mediaType {
	case "image":
		return e.downloadImage(ctx, fileKey)
	case "audio", "video", "file":
		return e.downloadFileResource(ctx, fileKey)
	default:
		return nil, "", fmt.Errorf("unsupported media type: %s", mediaType)
	}
}

// downloadImage 下载图片
// 注意：富文本中的图片 key 格式为 img_v3_xxx，需要使用 File API 而不是 Image API
func (e *FeishuWebSocket) downloadImage(ctx context.Context, imageKey string) ([]byte, string, error) {
	// 富文本图片 key 以 "img_v3_" 开头，需要使用 File API
	if strings.HasPrefix(imageKey, "img_v3_") {
		e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] 检测到富文本图片 key，使用 File API: %s", imageKey)
		return e.downloadFileResource(ctx, imageKey)
	}

	req := larkim.NewGetImageReqBuilder().
		ImageKey(imageKey).
		Build()

	resp, err := e.larkClient.Im.Image.Get(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get image: %w", err)
	}

	if !resp.Success() {
		return nil, "", fmt.Errorf("feishu API error (code %d): %s", resp.Code, resp.Msg)
	}

	if resp.File == nil {
		return nil, "", fmt.Errorf("image data is empty")
	}

	imageData, err := io.ReadAll(resp.File)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	// 压缩图片
	compressedData, mimeType, err := e.compressImage(imageData, "png")
	if err != nil {
		// 压缩失败，使用原始数据
		e.RuleConfig.Logger.Warnf("[Feishu] image compression failed: %v", err)
		mimeType = detectMimeType(imageData)
		compressedData = imageData
	}

	return compressedData, mimeType, nil
}

// downloadFileResource 下载文件资源（音频、视频、文件）
func (e *FeishuWebSocket) downloadFileResource(ctx context.Context, fileKey string) ([]byte, string, error) {
	req := larkim.NewGetFileReqBuilder().
		FileKey(fileKey).
		Build()

	resp, err := e.larkClient.Im.File.Get(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file: %w", err)
	}

	if !resp.Success() {
		return nil, "", fmt.Errorf("feishu API error (code %d): %s", resp.Code, resp.Msg)
	}

	if resp.File == nil {
		return nil, "", fmt.Errorf("file data is empty")
	}

	fileData, err := io.ReadAll(resp.File)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file data: %w", err)
	}

	mimeType := detectMimeType(fileData)
	if resp.FileName != "" {
		mimeType = mime.TypeByExtension(filepath.Ext(resp.FileName))
	}

	return fileData, mimeType, nil
}

// downloadMessageResource 下载消息资源（用于富文本图片等用户发送的资源）
// 使用 GetMessageResource API，需要提供消息 ID 和资源 key
func (e *FeishuWebSocket) downloadMessageResource(ctx context.Context, messageId, fileKey string) ([]byte, string, error) {
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageId).
		FileKey(fileKey).
		Type("image").
		Build()

	resp, err := e.larkClient.Im.V1.MessageResource.Get(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get message resource: %w", err)
	}

	if !resp.Success() {
		return nil, "", fmt.Errorf("feishu API error (code %d): %s", resp.Code, resp.Msg)
	}

	if resp.File == nil {
		return nil, "", fmt.Errorf("message resource data is empty")
	}

	resourceData, err := io.ReadAll(resp.File)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read message resource data: %w", err)
	}

	mimeType := detectMimeType(resourceData)
	return resourceData, mimeType, nil
}

// saveMessageResourceFile 保存消息资源文件到工作空间（用于富文本图片）
func (e *FeishuWebSocket) saveMessageResourceFile(ctx context.Context, messageId, fileKey string) (api2.MediaFile, error) {
	// 1. 下载消息资源
	fileData, mimeType, err := e.downloadMessageResource(ctx, messageId, fileKey)
	if err != nil {
		return api2.MediaFile{}, fmt.Errorf("download message resource failed: %w", err)
	}

	// 2. 检查文件大小
	if int64(len(fileData)) > e.Config.Media.MaxFileSize {
		return api2.MediaFile{}, fmt.Errorf("file size %d exceeds limit %d", len(fileData), e.Config.Media.MaxFileSize)
	}

	// 3. 构建存储路径
	// WorkspaceDir 已经是完整的存储目录（如 /data/media/main）
	// 简化路径: feishu/image/
	dirPath := filepath.Join(e.WorkspaceDir, "feishu", "image")

	// 4. 生成文件名
	now := time.Now()
	ext := mimeToExt(mimeType)
	fileName := fmt.Sprintf("rich_text_image_%d%s", now.UnixMilli(), ext)
	filePath := filepath.Join(dirPath, fileName)

	// 5. 创建目录并保存文件
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return api2.MediaFile{}, fmt.Errorf("create directory failed: %w", err)
	}
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		return api2.MediaFile{}, fmt.Errorf("write file failed: %w", err)
	}

	// 6. 计算相对路径（相对于 WorkspaceDir）
	relativePath := filepath.Join("feishu", "image", fileName)

	e.RuleConfig.Logger.Debugf("[Feishu] message resource saved: %s (%d bytes)", filePath, len(fileData))

	return api2.MediaFile{
		Type:         "image",
		FileName:     fileName,
		RelativePath: relativePath,
		FileSize:     int64(len(fileData)),
		MimeType:     mimeType,
	}, nil
}

// getMessageResourceAsBase64 下载消息资源并返回 base64 格式（用于富文本图片）
func (e *FeishuWebSocket) getMessageResourceAsBase64(ctx context.Context, messageId, fileKey string) (string, error) {
	imageData, mimeType, err := e.downloadMessageResource(ctx, messageId, fileKey)
	if err != nil {
		return "", fmt.Errorf("failed to download message resource: %w", err)
	}

	// 压缩图片
	compressedData, mimeType, err := e.compressImage(imageData, "png")
	if err != nil {
		// 压缩失败，使用原始数据
		e.RuleConfig.Logger.Warnf("[Feishu] image compression failed, using original: %v", err)
		mimeType = detectMimeType(imageData)
		compressedData = imageData
	}

	// 转换为 base64
	base64Data := base64.StdEncoding.EncodeToString(compressedData)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data), nil
}

// detectMimeType 检测 MIME 类型
func detectMimeType(data []byte) string {
	return http.DetectContentType(data)
}

// mimeToExt MIME 类型转文件扩展名
func mimeToExt(mimeType string) string {
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/ogg", "audio/oga":
		return ".ogg"
	case "audio/wav", "audio/wave":
		return ".wav"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "application/pdf":
		return ".pdf"
	default:
		// 尝试从 MIME 类型推断
		if ext, err := mime.ExtensionsByType(mimeType); err == nil && len(ext) > 0 {
			return ext[0]
		}
		return ""
	}
}

// getImageAsBase64 下载飞书图片并返回 base64 格式
// 注意：富文本中的图片 key 格式为 img_v3_xxx，需要使用 File API 而不是 Image API
func (e *FeishuWebSocket) getImageAsBase64(ctx context.Context, imageKey string) (string, error) {
	var imageData []byte
	var mimeType string
	var err error

	// 富文本图片 key 以 "img_v3_" 开头，需要使用 File API
	if strings.HasPrefix(imageKey, "img_v3_") {
		e.RuleConfig.Logger.Debugf("[Feishu][DEBUG] getImageAsBase64: 检测到富文本图片 key，使用 File API: %s", imageKey)
		imageData, mimeType, err = e.downloadFileResource(ctx, imageKey)
		if err != nil {
			return "", fmt.Errorf("failed to download rich text image: %w", err)
		}
	} else {
		req := larkim.NewGetImageReqBuilder().
			ImageKey(imageKey).
			Build()

		resp, respErr := e.larkClient.Im.Image.Get(ctx, req)
		if respErr != nil {
			return "", fmt.Errorf("failed to get image: %w", respErr)
		}

		if !resp.Success() {
			return "", fmt.Errorf("feishu API error (code %d): %s", resp.Code, resp.Msg)
		}

		// 读取图片数据
		if resp.File == nil {
			return "", fmt.Errorf("image data is empty")
		}

		imageData, err = io.ReadAll(resp.File)
		if err != nil {
			return "", fmt.Errorf("failed to read image data: %w", err)
		}

		// 根据文件名推断原始格式
		originalFormat := "png"
		if resp.FileName != "" {
			switch {
			case strings.HasSuffix(strings.ToLower(resp.FileName), ".jpg"),
				strings.HasSuffix(strings.ToLower(resp.FileName), ".jpeg"):
				originalFormat = "jpeg"
			case strings.HasSuffix(strings.ToLower(resp.FileName), ".gif"):
				originalFormat = "gif"
			case strings.HasSuffix(strings.ToLower(resp.FileName), ".webp"):
				originalFormat = "webp"
			case strings.HasSuffix(strings.ToLower(resp.FileName), ".bmp"):
				originalFormat = "bmp"
			}
		}

		// 压缩图片
		var compressedData []byte
		compressedData, mimeType, err = e.compressImage(imageData, originalFormat)
		if err != nil {
			// 压缩失败，使用原始数据
			e.RuleConfig.Logger.Warnf("[Feishu] image compression failed, using original: %v", err)
			mimeType = "image/" + originalFormat
			if originalFormat == "jpeg" {
				mimeType = "image/jpeg"
			}
			compressedData = imageData
		}
		imageData = compressedData
	}

	// 转换为 base64
	base64Data := base64.StdEncoding.EncodeToString(imageData)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data), nil
}

// compressImage 压缩图片，返回压缩后的数据和 MIME 类型
func (e *FeishuWebSocket) compressImage(data []byte, originalFormat string) ([]byte, string, error) {
	// 解码图片
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	// 检查是否需要压缩
	maxSize := e.Config.ImageMaxSize
	if maxSize <= 0 {
		maxSize = 1024
	}

	// 如果图片尺寸小于限制，不压缩
	if origWidth <= maxSize && origHeight <= maxSize {
		return data, "image/" + format, nil
	}

	// 计算缩放比例
	var newWidth, newHeight int
	if origWidth > origHeight {
		newWidth = maxSize
		newHeight = origHeight * maxSize / origWidth
	} else {
		newHeight = maxSize
		newWidth = origWidth * maxSize / origHeight
	}

	// 创建缩放后的图片
	resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(resized, resized.Bounds(), img, bounds, draw.Over, nil)

	// 编码为 JPEG（压缩效果好）
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", fmt.Errorf("failed to encode image: %w", err)
	}

	e.RuleConfig.Logger.Debugf("[Feishu] image compressed: %dx%d -> %dx%d, size: %d -> %d",
		origWidth, origHeight, newWidth, newHeight, len(data), buf.Len())

	return buf.Bytes(), "image/jpeg", nil
}

// sendThinkingReply 发送"思考中"回复（回复指定消息，使用卡片 JSON 2.0）
// 返回创建的消息 ID，用于后续 Patch 更新内容
func (e *FeishuWebSocket) sendThinkingReply(messageId string, tenantKey interface{}) string {
	// 使用卡片 JSON 2.0 构建消息
	cardContent := feishu2.NewCardV2().
		AddMarkdown(e.Config.ThinkingText).
		MustString()

	// 使用 Reply 方法回复消息
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(messageId).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			Content(cardContent).
			ReplyInThread(false).
			Build()).
		Build()

	// 构建 API 调用选项
	var opts []larkcore.RequestOptionFunc
	if tk, ok := tenantKey.(string); ok && tk != "" {
		opts = append(opts, larkcore.WithTenantKey(tk))
	}

	resp, err := e.larkClient.Im.V1.Message.Reply(context.Background(), req, opts...)
	if err != nil {
		e.RuleConfig.Logger.Warnf("[Feishu] send thinking reply error: %v", err)
		return ""
	}
	if !resp.Success() {
		e.RuleConfig.Logger.Warnf("[Feishu] send thinking reply failed: code=%d, msg=%s", resp.Code, resp.Msg)
		return ""
	}

	// 返回创建的消息 ID
	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId
	}
	return ""
}

// triggerEvent 触发事件
func (e *FeishuWebSocket) triggerEvent(eventName string, err error) {
	if e.OnEvent != nil {
		if err != nil {
			e.OnEvent(eventName, err)
		} else {
			e.OnEvent(eventName)
		}
	}
}

// getQuoteMessageContent 获取引用/回复的消息内容
// 通过飞书 API 根据消息 ID 获取原消息内容
func (e *FeishuWebSocket) getQuoteMessageContent(ctx context.Context, messageId, tenantKey string) string {
	if messageId == "" {
		return ""
	}

	req := larkim.NewGetMessageReqBuilder().
		MessageId(messageId).
		Build()

	var opts []larkcore.RequestOptionFunc
	if tenantKey != "" {
		opts = append(opts, larkcore.WithTenantKey(tenantKey))
	}

	resp, err := e.larkClient.Im.V1.Message.Get(ctx, req, opts...)
	if err != nil {
		e.RuleConfig.Logger.Warnf("[Feishu] get quote message error: %v, messageId=%s", err, messageId)
		return ""
	}

	if !resp.Success() {
		e.RuleConfig.Logger.Warnf("[Feishu] get quote message failed: code=%d, msg=%s, messageId=%s", resp.Code, resp.Msg, messageId)
		return ""
	}

	if resp.Data == nil || resp.Data.Items == nil || len(resp.Data.Items) == 0 {
		e.RuleConfig.Logger.Warnf("[Feishu] quote message not found: messageId=%s", messageId)
		return ""
	}

	// 获取消息内容
	item := resp.Data.Items[0]
	if item == nil || item.Body == nil || item.Body.Content == nil {
		return ""
	}

	content := *item.Body.Content
	msgType := larkcore.StringValue(item.MsgType)

	// 解析消息内容
	parsedContent, _ := e.parseMessageContentWithImages(ctx, messageId, msgType, content)

	e.RuleConfig.Logger.Debugf("[Feishu] get quote message success: messageId=%s, content=%s", messageId, parsedContent)

	return parsedContent
}

// saveRouter 保存路由器
func (e *FeishuWebSocket) saveRouter(router endpointApi.Router) {
	e.Lock()
	defer e.Unlock()
	if e.RouterStorage == nil {
		e.RouterStorage = make(map[string]endpointApi.Router)
	}
	e.RouterStorage[router.GetId()] = router
}

// deleteRouter 删除路由器
func (e *FeishuWebSocket) deleteRouter(routerId string) endpointApi.Router {
	e.Lock()
	defer e.Unlock()
	if e.RouterStorage == nil {
		return nil
	}
	router := e.RouterStorage[routerId]
	delete(e.RouterStorage, routerId)
	return router
}

// feishuLogger 适配 rulego logger 到 SDK logger 接口
// 将飞书 SDK 的日志通过 rulego Logger 输出，支持日志级别过滤
type feishuLogger struct {
	logger types.Logger
}

func (l *feishuLogger) Debug(ctx context.Context, args ...interface{}) {
	if l.logger != nil {
		l.logger.Debugf("%v", args...)
	}
}

func (l *feishuLogger) Info(ctx context.Context, args ...interface{}) {
	if l.logger != nil {
		l.logger.Infof("%v", args...)
	}
}

func (l *feishuLogger) Warn(ctx context.Context, args ...interface{}) {
	if l.logger != nil {
		l.logger.Warnf("%v", args...)
	}
}

func (l *feishuLogger) Error(ctx context.Context, args ...interface{}) {
	if l.logger != nil {
		l.logger.Errorf("%v", args...)
	}
}

// FeishuRequestMessage 飞书请求消息实现
type FeishuRequestMessage struct {
	msg        *types.RuleMsg
	body       []byte
	headers    textproto.MIMEHeader
	statusCode int
	err        error
}

// NewFeishuRequestMessage 创建飞书请求消息
func NewFeishuRequestMessage(msg *types.RuleMsg) *FeishuRequestMessage {
	return &FeishuRequestMessage{
		msg:  msg,
		body: []byte(msg.GetData()),
	}
}

// Body 返回消息体
func (m *FeishuRequestMessage) Body() []byte {
	return m.body
}

// SetBody 设置消息体
func (m *FeishuRequestMessage) SetBody(body []byte) {
	m.body = body
}

// Headers 返回请求头
func (m *FeishuRequestMessage) Headers() textproto.MIMEHeader {
	if m.headers == nil {
		m.headers = make(textproto.MIMEHeader)
		if m.msg != nil {
			for k, v := range m.msg.Metadata.Values() {
				m.headers.Set(k, v)
			}
		}
	}
	return m.headers
}

// From 返回消息来源
func (m *FeishuRequestMessage) From() string {
	return "feishu/websocket"
}

// GetParam 获取参数
func (m *FeishuRequestMessage) GetParam(key string) string {
	if m.msg != nil {
		return m.msg.Metadata.GetValue(key)
	}
	return ""
}

// SetMsg 设置规则消息
func (m *FeishuRequestMessage) SetMsg(msg *types.RuleMsg) {
	m.msg = msg
}

// GetMsg 获取规则消息
func (m *FeishuRequestMessage) GetMsg() *types.RuleMsg {
	return m.msg
}

// SetStatusCode 设置状态码
func (m *FeishuRequestMessage) SetStatusCode(statusCode int) {
	m.statusCode = statusCode
}

// SetError 设置错误
func (m *FeishuRequestMessage) SetError(err error) {
	m.err = err
}

// GetError 获取错误
func (m *FeishuRequestMessage) GetError() error {
	return m.err
}

// FeishuResponseMessage 飞书响应消息实现
type FeishuResponseMessage struct {
	body       []byte
	headers    textproto.MIMEHeader
	statusCode int
	err        error
	// larkClient 飞书 API 客户端（用于发送消息）
	larkClient *lark.Client
	// requestMsg 请求消息（用于获取 msgId、chatId 等元数据）
	requestMsg *types.RuleMsg
	// msg 响应消息
	msg *types.RuleMsg
	// tenantKey 租户 key（多租户场景）
	tenantKey string
	// logger 日志器
	logger types.Logger
}

// NewFeishuResponseMessage 创建飞书响应消息
func NewFeishuResponseMessage(client *lark.Client, requestMsg *types.RuleMsg, tenantKey string, logger types.Logger) *FeishuResponseMessage {
	return &FeishuResponseMessage{
		larkClient: client,
		requestMsg: requestMsg,
		tenantKey:  tenantKey,
		logger:     logger,
	}
}

// Body 返回响应体
func (m *FeishuResponseMessage) Body() []byte {
	return m.body
}

// SetBody 设置响应体，如果设置了回复元数据则自动发送消息
func (m *FeishuResponseMessage) SetBody(body []byte) {
	m.body = body

	// 如果没有 client 或 body，不发送
	if m.larkClient == nil || len(body) == 0 {
		return
	}

	// 优先 Patch 更新"思考中"卡片
	if m.requestMsg != nil {
		thinkingMsgId := m.requestMsg.Metadata.GetValue("im.thinkingMsgId")
		if thinkingMsgId != "" {
			patchErr := m.patchCardMessage(thinkingMsgId, string(body))
			if patchErr == nil {
				return // Patch 成功，不需要发新消息
			}
			// Patch 失败则 fallback 到发新消息
			if m.logger != nil {
				m.logger.Warnf("[Feishu] patch thinking card failed, fallback to send new message: %v", patchErr)
			}
		}
	}

	// fallback: 获取 chatId 并发送新消息
	var chatId string
	if m.requestMsg != nil {
		chatId = m.requestMsg.Metadata.GetValue(api2.MetaResponseChatID)
		if chatId == "" {
			chatId = m.requestMsg.Metadata.GetValue(api2.MetaChatID)
		}
	}

	if chatId == "" {
		return
	}

	// 发送消息
	if err := m.sendMessage(chatId, string(body)); err != nil {
		// 记录错误，但不影响流程
		m.err = err
	}
}

// patchCardMessage 通过 Patch API 更新已有卡片消息的内容（原地替换"思考中..."为最终回复）
func (m *FeishuResponseMessage) patchCardMessage(messageId, message string) error {
	cardContent := feishu2.NewCardV2().
		AddMarkdown(message).
		MustString()

	req := larkim.NewPatchMessageReqBuilder().
		MessageId(messageId).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(cardContent).
			Build()).
		Build()

	var opts []larkcore.RequestOptionFunc
	if m.tenantKey != "" {
		opts = append(opts, larkcore.WithTenantKey(m.tenantKey))
	}

	resp, err := m.larkClient.Im.Message.Patch(context.Background(), req, opts...)
	if err != nil {
		return fmt.Errorf("patch message error: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("patch message failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if m.logger != nil {
		m.logger.Debugf("[Feishu] patchCardMessage success: messageId=%s", messageId)
	}
	return nil
}

// sendMessage 发送新消息（支持 markdown 格式，使用卡片 JSON 2.0）
func (m *FeishuResponseMessage) sendMessage(chatId, message string) error {
	// 使用卡片 JSON 2.0 构建消息
	cardContent := feishu2.NewCardV2().
		AddMarkdown(message).
		MustString()

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			ReceiveId(chatId).
			Content(cardContent).
			Build()).
		Build()

	// 构建 API 调用选项
	var opts []larkcore.RequestOptionFunc
	if m.tenantKey != "" {
		opts = append(opts, larkcore.WithTenantKey(m.tenantKey))
	}

	// 调试日志
	if m.logger != nil {
		m.logger.Debugf("[Feishu] sendMessage: chatId=%s, content=%s", chatId, cardContent)
	}

	resp, err := m.larkClient.Im.Message.Create(context.Background(), req, opts...)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("[Feishu] sendMessage error: %v", err)
		}
		return fmt.Errorf("send API error: %w", err)
	}

	// 调试日志：打印完整响应
	if m.logger != nil {
		m.logger.Debugf("[Feishu] sendMessage resp: success=%v, code=%d, msg=%s, request_id=%s",
			resp.Success(), resp.Code, resp.Msg, resp.RequestId())
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error (code %d): %s, request_id: %s", resp.Code, resp.Msg, resp.RequestId())
	}

	return nil
}

// Headers 返回响应头
func (m *FeishuResponseMessage) Headers() textproto.MIMEHeader {
	if m.headers == nil {
		m.headers = make(textproto.MIMEHeader)
	}
	return m.headers
}

// From 返回消息来源
func (m *FeishuResponseMessage) From() string {
	return "feishu/websocket/response"
}

// GetParam 获取参数
func (m *FeishuResponseMessage) GetParam(key string) string {
	return ""
}

// SetMsg 设置规则消息
func (m *FeishuResponseMessage) SetMsg(msg *types.RuleMsg) {
	m.msg = msg
}

// GetMsg 获取规则消息
func (m *FeishuResponseMessage) GetMsg() *types.RuleMsg {
	return m.msg
}

// SetStatusCode 设置状态码
func (m *FeishuResponseMessage) SetStatusCode(statusCode int) {
	m.statusCode = statusCode
}

// SetError 设置错误
func (m *FeishuResponseMessage) SetError(err error) {
	m.err = err
}

// GetError 获取错误
func (m *FeishuResponseMessage) GetError() error {
	return m.err
}
