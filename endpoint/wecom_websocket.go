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

package endpoint

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
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

	"github.com/google/uuid"
	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/endpoint/impl"
	"github.com/rulego/rulego/utils/maps"
	"golang.org/x/image/draw"

	api2 "github.com/teambuf/tpclaw-components-im/api"
)

const (
	// TypeWeComWebSocket 企业微信 WebSocket 端点类型
	TypeWeComWebSocket = "wecom"
)

func init() {
	_ = endpoint.Registry.Register(&WeComWebSocket{})
}

// WeComWebSocketConfig 企业微信 WebSocket 端点配置
type WeComWebSocketConfig struct {
	// AccountID 账号 ID（用于多账号区分）
	AccountID string `json:"accountId"`
	// BotID 智能机器人 BotID（必填）
	BotID string `json:"botId"`
	// Secret 长连接专用密钥（必填）
	Secret string `json:"secret"`
	// AutoReconnect 是否自动重连（默认 true）
	AutoReconnect bool `json:"autoReconnect"`
	// ThinkingText 思考中回复的文本（默认 "思考中..."）
	ThinkingText string `json:"thinkingText"`
	// ImageMaxSize 图片最大尺寸（像素），超过会等比缩放。0 表示不压缩（默认 1024）
	ImageMaxSize int `json:"imageMaxSize"`
	// Media 媒体存储配置
	Media MediaConfig `json:"media"`
}

// WeComWebSocket 企业微信 WebSocket 端点
type WeComWebSocket struct {
	impl.BaseEndpoint
	RuleConfig   types.Config
	Config       WeComWebSocketConfig
	WorkspaceDir string

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
	mu      sync.RWMutex

	wsClient      *wecomWSClient
	streamIDCache sync.Map // reqID -> streamID（用于流式消息更新）
}

// NewWeComWebSocket 创建企业微信 WebSocket 端点
func NewWeComWebSocket() *WeComWebSocket {
	return &WeComWebSocket{}
}

// Type 返回端点类型
func (e *WeComWebSocket) Type() string {
	return types.EndpointTypePrefix + TypeWeComWebSocket
}

// New 创建新实例
func (e *WeComWebSocket) New() types.Node {
	return NewWeComWebSocket()
}

// Init 初始化端点
func (e *WeComWebSocket) Init(config types.Config, configuration types.Configuration) error {
	if err := maps.Map2Struct(configuration, &e.Config); err != nil {
		return fmt.Errorf("failed to parse wecom websocket config: %w", err)
	}

	if e.Config.BotID == "" {
		return fmt.Errorf("botId is required")
	}
	if e.Config.Secret == "" {
		return fmt.Errorf("secret is required")
	}

	// 设置默认值
	e.Config.AutoReconnect = true
	if e.Config.ThinkingText == "" {
		e.Config.ThinkingText = "思考中..."
	}
	if e.Config.ImageMaxSize == 0 {
		e.Config.ImageMaxSize = 1024
	}
	if e.Config.Media.StoragePath == "" {
		e.Config.Media.StoragePath = "media"
	}
	if e.Config.Media.MaxFileSize == 0 {
		e.Config.Media.MaxFileSize = 50 * 1024 * 1024
	}
	e.WorkspaceDir = e.Config.Media.StoragePath

	e.RuleConfig = config
	e.ctx, e.cancel = context.WithCancel(context.Background())

	// 创建 WebSocket 客户端
	wsConfig := wecomWSClientConfig{
		BotID:             e.Config.BotID,
		Secret:            e.Config.Secret,
		WSURL:             wecomWSURL,
		HeartbeatInterval: 30 * time.Second,
		AutoReconnect:     e.Config.AutoReconnect,
		ReconnectDelay:    3 * time.Second,
	}
	e.wsClient = newWecomWSClient(wsConfig, e.RuleConfig.Logger)

	// 注册回调
	e.wsClient.OnMessage = e.handleMsgCallback
	e.wsClient.OnEvent = e.handleEventCallback

	// 注册到全局注册表供应用层复用
	RegisterWecomWSClient(e.Config.BotID, e.wsClient)

	return nil
}

// Id 返回端点 ID
func (e *WeComWebSocket) Id() string {
	return e.Config.BotID
}

// SetOnEvent 设置事件回调
func (e *WeComWebSocket) SetOnEvent(onEvent endpointApi.OnEvent) {
	e.BaseEndpoint.OnEvent = onEvent
}

// SetWorkspaceDir 设置工作空间目录
func (e *WeComWebSocket) SetWorkspaceDir(dir string) {
	e.WorkspaceDir = dir
}

// AddRouter 添加路由器
func (e *WeComWebSocket) AddRouter(router endpointApi.Router, params ...interface{}) (string, error) {
	if router == nil {
		return "", fmt.Errorf("router can not be nil")
	}
	e.CheckAndSetRouterId(router)
	e.saveRouter(router)
	return router.GetId(), nil
}

// RemoveRouter 移除路由器
func (e *WeComWebSocket) RemoveRouter(routerId string, params ...interface{}) error {
	router := e.deleteRouter(routerId)
	if router == nil {
		return fmt.Errorf("router not found: %s", routerId)
	}
	return nil
}

// saveRouter 保存路由器
func (e *WeComWebSocket) saveRouter(router endpointApi.Router) {
	e.Lock()
	defer e.Unlock()
	if e.RouterStorage == nil {
		e.RouterStorage = make(map[string]endpointApi.Router)
	}
	e.RouterStorage[router.GetId()] = router
}

// deleteRouter 删除路由器
func (e *WeComWebSocket) deleteRouter(routerId string) endpointApi.Router {
	e.Lock()
	defer e.Unlock()
	if e.RouterStorage == nil {
		return nil
	}
	router := e.RouterStorage[routerId]
	delete(e.RouterStorage, routerId)
	return router
}

// Start 启动端点
func (e *WeComWebSocket) Start() error {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return nil
	}
	e.started = true

	if e.ctx != nil && e.ctx.Err() != nil {
		e.ctx, e.cancel = context.WithCancel(context.Background())
	}
	e.mu.Unlock()

	// 启动 WebSocket 客户端
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		err := e.wsClient.start(e.ctx)
		if err != nil && e.ctx.Err() == nil {
			e.RuleConfig.Logger.Errorf("WeCom WebSocket error: %v", err)
			e.triggerEvent(endpointApi.EventDisconnect, err)
		}
	}()

	e.triggerEvent(endpointApi.EventConnect, nil)
	e.RuleConfig.Logger.Infof("WeCom WebSocket endpoint started, botId: %s", e.Config.BotID)
	return nil
}

// Destroy 销毁端点
func (e *WeComWebSocket) Destroy() {
	if e.Config.BotID != "" {
		UnregisterWecomWSClient(e.Config.BotID)
	}

	e.mu.Lock()
	e.started = false
	e.mu.Unlock()

	if e.wsClient != nil {
		e.wsClient.close()
	}

	if e.cancel != nil {
		e.cancel()
	}

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		e.RuleConfig.Logger.Infof("WeCom WebSocket endpoint destroyed, botId: %s", e.Config.BotID)
	case <-time.After(5 * time.Second):
		e.RuleConfig.Logger.Warnf("WeCom WebSocket destroy timeout, botId: %s", e.Config.BotID)
	}

	e.wsClient = nil
}

// ==================== 消息处理 ====================

// handleMsgCallback 处理消息回调
func (e *WeComWebSocket) handleMsgCallback(cmd string, reqID string, body json.RawMessage) {
	if e.ctx != nil && e.ctx.Err() != nil {
		return
	}

	var msgCallback wecomMsgCallback
	if err := json.Unmarshal(body, &msgCallback); err != nil {
		e.RuleConfig.Logger.Warnf("[WeCom] parse msg callback failed: %v", err)
		return
	}

	e.RuleConfig.Logger.Infof("[WeCom][MSG] 收到消息回调: msgType=%s, chatType=%s, chatId=%s, userId=%s, msgId=%s",
		msgCallback.MsgType, msgCallback.ChatType, msgCallback.ChatID, msgCallback.From.UserID, msgCallback.MsgID)

	// 打印完整的回调 body（用于调试 mixed 等复杂消息）
	bodyPreview := string(body)
	if len(bodyPreview) > 2000 {
		bodyPreview = bodyPreview[:2000] + "...(truncated)"
	}
	e.RuleConfig.Logger.Debugf("[WeCom][MSG] 回调 body: %s", bodyPreview)

	// 构建 IMMessage（使用统一元数据键）
	msg := &api2.IMMessage{
		Platform:  api2.PlatformWeCom,
		EventType: api2.EventTypeMessageReceived,
		RawData:   body,
	}
	msg.ID = msgCallback.MsgID
	msg.MsgType = msgCallback.MsgType
	msg.ChatID = resolveWecomChatID(msgCallback)
	msg.ChatType = convertWecomChatType(msgCallback.ChatType)
	msg.Sender = &api2.IMSender{
		UserID: msgCallback.From.UserID,
	}

	// 解析消息内容
	msg.Content, msg.Extensions = e.parseMessageContent(msgCallback)
	if msg.Extensions == nil {
		msg.Extensions = make(map[string]interface{})
	}

	e.RuleConfig.Logger.Infof("[WeCom][MSG] 解析结果: msgType=%s, content=%s, extensionskeys=%v",
		msgCallback.MsgType, msg.Content, func() []string {
			keys := make([]string, 0, len(msg.Extensions))
			for k := range msg.Extensions {
				keys = append(keys, k)
			}
			return keys
		}())

	// 设置平台特定扩展
	msg.Extensions[api2.MetaWeComBotID] = msgCallback.AIBotID
	msg.Extensions[api2.MetaWeComReqID] = reqID
	msg.Extensions[api2.MetaWeComChatID] = msgCallback.ChatID
	msg.Extensions[api2.MetaWeComChatType] = msgCallback.ChatType

	// 发送"思考中"流式消息
	streamID := e.sendThinkingReply(reqID)

	// 处理消息
	if streamID != "" {
		msg.Extensions[api2.MetaWeComStreamID] = streamID
	}
	e.processMessage(msg, reqID)
}

// handleEventCallback 处理事件回调
func (e *WeComWebSocket) handleEventCallback(cmd string, reqID string, body json.RawMessage) {
	if e.ctx != nil && e.ctx.Err() != nil {
		return
	}

	var eventCallback wecomEventCallback
	if err := json.Unmarshal(body, &eventCallback); err != nil {
		e.RuleConfig.Logger.Warnf("[WeCom] parse event callback failed: %v", err)
		return
	}

	e.RuleConfig.Logger.Debugf("[WeCom] event callback: eventType=%s", eventCallback.Event.EventType)

	switch eventCallback.Event.EventType {
	case wecomEventEnterChat:
		// 进入会话事件，发送欢迎语
		e.sendWelcomeMessage(reqID)
	case wecomEventDisconnected:
		e.RuleConfig.Logger.Infof("[WeCom] disconnected event received")
	default:
		// 其他事件，构建消息传递
		msg := &api2.IMMessage{
			Platform:  api2.PlatformWeCom,
			EventType: eventCallback.Event.EventType,
			MsgType:   api2.MsgTypeEvent,
		}
		msg.ID = eventCallback.MsgID
		msg.ChatID = eventCallback.ChatID
		msg.ChatType = convertWecomChatType(eventCallback.ChatType)
		if eventCallback.From.UserID != "" {
			msg.Sender = &api2.IMSender{UserID: eventCallback.From.UserID}
		}
		if msg.Extensions == nil {
			msg.Extensions = make(map[string]interface{})
		}
		msg.Extensions[api2.MetaWeComReqID] = reqID
		msg.Extensions[api2.MetaWeComBotID] = eventCallback.AIBotID
		e.processMessage(msg, reqID)
	}
}

// ==================== 消息解析 ====================

// parseMessageContent 解析消息内容
func (e *WeComWebSocket) parseMessageContent(msgCallback wecomMsgCallback) (string, map[string]interface{}) {
	extensions := make(map[string]interface{})

	switch msgCallback.MsgType {
	case "text":
		if msgCallback.Text != nil {
			return msgCallback.Text.Content, extensions
		}
		return "", extensions

	case "image":
		if msgCallback.Image != nil {
			return e.parseMediaImage(msgCallback.Image, extensions)
		}
		return "[图片]", extensions

	case "voice":
		if msgCallback.Voice != nil {
			return e.parseMediaVoice(msgCallback.Voice, extensions)
		}
		return "[语音]", extensions

	case "video":
		if msgCallback.Video != nil {
			return e.parseMediaVideo(msgCallback.Video, extensions)
		}
		return "[视频]", extensions

	case "file":
		if msgCallback.File != nil {
			return e.parseMediaFile(msgCallback.File, extensions)
		}
		return "[文件]", extensions

	case "mixed":
		if msgCallback.Mixed != nil {
			return e.parseMixedContent(msgCallback.Mixed, extensions)
		}
		return "[图文混排]", extensions

	default:
		return fmt.Sprintf("[不支持的消息类型: %s]", msgCallback.MsgType), extensions
	}
}

// parseMediaImage 解析图片媒体
func (e *WeComWebSocket) parseMediaImage(image *wecomMediaBody, extensions map[string]interface{}) (string, map[string]interface{}) {
	if image.URL == "" {
		return "[图片]", extensions
	}

	data, mimeType, err := wecomDownloadMedia(image.URL, image.AESKey)
	if err != nil {
		e.RuleConfig.Logger.Warnf("[WeCom] download image failed: %v", err)
		return "[图片加载失败]", extensions
	}

	// 压缩图片
	if e.Config.ImageMaxSize > 0 {
		compressed, compressedMime, err := wecomCompressImage(data, mimeType, e.Config.ImageMaxSize)
		if err == nil {
			data = compressed
			mimeType = compressedMime
		}
	}

	if e.WorkspaceDir != "" {
		mediaFile, err := e.saveMediaFile(data, "image", "", mimeType)
		if err != nil {
			e.RuleConfig.Logger.Warnf("[WeCom] save image failed: %v, fallback to base64", err)
			base64Img := base64.StdEncoding.EncodeToString(data)
			extensions[api2.MetaImages] = []string{fmt.Sprintf("data:%s;base64,%s", mimeType, base64Img)}
			return "[图片]", extensions
		}
		// 存储本地文件路径（绝对路径），与飞书逻辑对齐，大模型节点会自动读取本地文件并转 base64
		absolutePath := filepath.Join(e.WorkspaceDir, mediaFile.RelativePath)
		e.RuleConfig.Logger.Infof("[WeCom] 图片保存成功: %s", absolutePath)
		extensions[api2.MetaImages] = []string{absolutePath}
		extensions[api2.MetaMediaFiles] = []api2.MediaFile{mediaFile}
		extensions[api2.MetaHasMedia] = true
		return "[图片]", extensions
	}

	base64Img := base64.StdEncoding.EncodeToString(data)
	extensions[api2.MetaImages] = []string{fmt.Sprintf("data:%s;base64,%s", mimeType, base64Img)}
	return "[图片]", extensions
}

// parseMediaVoice 解析语音
func (e *WeComWebSocket) parseMediaVoice(voice *wecomMediaBody, extensions map[string]interface{}) (string, map[string]interface{}) {
	if voice.URL == "" {
		return "[语音]", extensions
	}
	if e.Config.Media.Enable && e.WorkspaceDir != "" {
		data, mimeType, err := wecomDownloadMedia(voice.URL, voice.AESKey)
		if err != nil {
			return "[语音保存失败]", extensions
		}
		mediaFile, err := e.saveMediaFile(data, "audio", "", mimeType)
		if err != nil {
			return "[语音保存失败]", extensions
		}
		extensions[api2.MetaMediaFiles] = []api2.MediaFile{mediaFile}
		extensions[api2.MetaHasMedia] = true
		return "[语音消息]", extensions
	}
	return "[语音消息]", extensions
}

// parseMediaVideo 解析视频
func (e *WeComWebSocket) parseMediaVideo(video *wecomMediaBody, extensions map[string]interface{}) (string, map[string]interface{}) {
	if video.URL == "" {
		return "[视频]", extensions
	}
	if e.Config.Media.Enable && e.WorkspaceDir != "" {
		data, mimeType, err := wecomDownloadMedia(video.URL, video.AESKey)
		if err != nil {
			return "[视频保存失败]", extensions
		}
		mediaFile, err := e.saveMediaFile(data, "video", "", mimeType)
		if err != nil {
			return "[视频保存失败]", extensions
		}
		extensions[api2.MetaMediaFiles] = []api2.MediaFile{mediaFile}
		extensions[api2.MetaHasMedia] = true
		return "[视频消息]", extensions
	}
	return "[视频消息]", extensions
}

// parseMediaFile 解析文件
func (e *WeComWebSocket) parseMediaFile(file *wecomMediaBody, extensions map[string]interface{}) (string, map[string]interface{}) {
	if file.URL == "" {
		return "[文件]", extensions
	}
	if e.Config.Media.Enable && e.WorkspaceDir != "" {
		data, mimeType, err := wecomDownloadMedia(file.URL, file.AESKey)
		if err != nil {
			return "[文件保存失败]", extensions
		}
		mediaFile, err := e.saveMediaFile(data, "file", "", mimeType)
		if err != nil {
			return "[文件保存失败]", extensions
		}
		extensions[api2.MetaMediaFiles] = []api2.MediaFile{mediaFile}
		extensions[api2.MetaHasMedia] = true
		return "[文件消息]", extensions
	}
	return "[文件消息]", extensions
}

// parseMixedContent 解析图文混排
func (e *WeComWebSocket) parseMixedContent(mixed *wecomMixedBody, extensions map[string]interface{}) (string, map[string]interface{}) {
	var textBuilder strings.Builder
	var images []string

	e.RuleConfig.Logger.Infof("[WeCom][MIXED] 开始解析图文混排, items数量=%d", len(mixed.Items))

	for i, item := range mixed.Items {
		switch item.MsgType {
		case "text":
			if item.Text != nil {
				e.RuleConfig.Logger.Infof("[WeCom][MIXED] item[%d]: type=text, content=%s", i, item.Text.Content)
				textBuilder.WriteString(item.Text.Content)
			}
		case "image":
			if item.Image != nil {
				e.RuleConfig.Logger.Infof("[WeCom][MIXED] item[%d]: type=image, url=%s, aeskey=%s", i, item.Image.URL, item.Image.AESKey)
				// 图片通过 image_url 传给模型，不需要在文本中添加占位符
				_, imgExts := e.parseMediaImage(item.Image, make(map[string]interface{}))
				if imgs, ok := imgExts[api2.MetaImages].([]string); ok {
					images = append(images, imgs...)
					e.RuleConfig.Logger.Infof("[WeCom][MIXED] item[%d]: 图片解析成功, 数量=%d", i, len(imgs))
				} else {
					e.RuleConfig.Logger.Warnf("[WeCom][MIXED] item[%d]: 图片解析失败，未获取到图片数据", i)
				}
			}
		default:
			e.RuleConfig.Logger.Warnf("[WeCom][MIXED] item[%d]: 不支持的类型=%s", i, item.MsgType)
		}
	}

	if len(images) > 0 {
		extensions[api2.MetaImages] = images
	}

	result := textBuilder.String()
	e.RuleConfig.Logger.Infof("[WeCom][MIXED] 解析完成: text=%s, imagesCount=%d", result, len(images))

	return result, extensions
}

// ==================== 消息路由 ====================

// processMessage 处理消息并路由
func (e *WeComWebSocket) processMessage(msg *api2.IMMessage, reqID string) {
	if e.ctx != nil && e.ctx.Err() != nil {
		return
	}

	// 构建 ChatRequest（OpenAI 标准格式，支持多模态）
	messageData := e.buildMessageData(msg)
	messageJSON, _ := json.Marshal(messageData)

	// 创建 RuleMsg
	ruleMsg := types.NewMsg(0, msg.EventType, types.JSON, types.NewMetadata(), string(messageJSON))
	ruleMsgPtr := &ruleMsg

	// 设置元数据
	for k, v := range msg.ToMetadata() {
		ruleMsg.Metadata.PutValue(k, v)
	}

	// 设置通道类型
	ruleMsg.Metadata.PutValue(api2.MetaChannel, api2.ChannelWeCom)
	ruleMsg.Metadata.PutValue(api2.MetaPlatform, api2.PlatformWeCom)
	ruleMsg.Metadata.PutValue(api2.MetaLoadHistory, "true")
	ruleMsg.Metadata.PutValue(api2.MetaBotID, e.Config.BotID)
	if e.Config.AccountID != "" {
		ruleMsg.Metadata.PutValue(api2.MetaAccountID, e.Config.AccountID)
	} else {
		ruleMsg.Metadata.PutValue(api2.MetaAccountID, e.Config.BotID)
	}
	ruleMsg.Metadata.PutValue(api2.MetaWeComReqID, reqID)

	// 如果有 streamID，传递给 Metadata 以便后续通过 SetBody 响应
	if streamID, ok := e.streamIDCache.Load(reqID); ok {
		ruleMsg.Metadata.PutValue(api2.MetaWeComStreamID, streamID.(string))
	}

	_ = msg.ID
	_ = msg.ChatID

	// 创建 Exchange
	exchange := &endpoint.Exchange{
		In:  NewWeComRequestMessage(ruleMsgPtr),
		Out: NewWeComResponseMessage(e.wsClient, ruleMsgPtr, e.Config.BotID, e.RuleConfig.Logger),
	}

	// 遍历路由处理
	for _, router := range e.RouterStorage {
		if router == nil {
			continue
		}
		e.DoProcess(e.ctx, router, exchange)
	}

	// 错误处理
	if exchange.Out.GetError() != nil {
		e.sendErrorMessage(reqID, exchange.Out.GetError())
	}
}

// buildMessageData 构建消息数据（OpenAI 标准格式，复用飞书模式）
func (e *WeComWebSocket) buildMessageData(msg *api2.IMMessage) *ChatRequest {
	images, hasImages := msg.Extensions[api2.MetaImages].([]string)

	quoteContent := ""
	if qc, ok := msg.Extensions[api2.MetaQuoteContent].(string); ok && qc != "" {
		quoteContent = qc
	}

	userContent := msg.Content
	if quoteContent != "" {
		userContent = fmt.Sprintf("[引用消息]\n%s\n\n%s", quoteContent, msg.Content)
	}

	// DEBUG: 打印 extensions 内容（对齐飞书）
	e.RuleConfig.Logger.Debugf("[WeCom][DEBUG] buildMessageData: msgType=%s, content=%s, hasImages=%v, imagesCount=%d, hasQuote=%v",
		msg.MsgType, msg.Content, hasImages, len(images), quoteContent != "")

	// DEBUG: 打印所有 extensions（对齐飞书）
	for k, v := range msg.Extensions {
		// 截断过长的值（如 base64 图片数据）
		valStr := fmt.Sprintf("%v", v)
		if len(valStr) > 200 {
			valStr = valStr[:200] + "..."
		}
		e.RuleConfig.Logger.Debugf("[WeCom][DEBUG] Extension: key=%s, type=%T, value=%s", k, v, valStr)
	}

	if hasImages && len(images) > 0 {
		var contentParts []ContentPart
		for i, img := range images {
			// 截断日志显示（对齐飞书）
			imgPreview := img
			if len(img) > 100 {
				imgPreview = img[:100] + "..."
			}
			e.RuleConfig.Logger.Debugf("[WeCom][DEBUG] 添加图片[%d]: %s", i, imgPreview)
			contentParts = append(contentParts, ContentPart{
				Type:     "image_url",
				ImageURL: &ImageURL{URL: img},
			})
		}
		if userContent != "" {
			contentParts = append(contentParts, ContentPart{
				Type: "text",
				Text: userContent,
			})
		}
		result := &ChatRequest{
			Messages: []ChatMessage{
				{Role: "user", Content: contentParts},
			},
		}

		// DEBUG: 打印构建的消息 JSON（对齐飞书）
		jsonData, _ := json.Marshal(result)
		e.RuleConfig.Logger.Debugf("[WeCom][DEBUG] 构建的多模态消息(JSON): %s", string(jsonData))

		return result
	}

	result := &ChatRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: userContent},
		},
	}

	// DEBUG: 打印构建的消息 JSON（对齐飞书）
	jsonData, _ := json.Marshal(result)
	e.RuleConfig.Logger.Debugf("[WeCom][DEBUG] 构建的纯文本消息(JSON): %s", string(jsonData))

	return result
}

// ==================== 消息回复 ====================

// sendThinkingReply 发送"思考中"流式消息，返回生成的 streamID
func (e *WeComWebSocket) sendThinkingReply(reqID string) string {
	streamID := uuid.New().String()
	e.streamIDCache.Store(reqID, streamID)

	cmd := map[string]interface{}{
		"cmd":     wecomCmdRespondMsg,
		"headers": map[string]string{"req_id": reqID},
		"body": map[string]interface{}{
			"msgtype": "stream",
			"stream": map[string]interface{}{
				"id":      streamID,
				"finish":  false,
				"content": e.Config.ThinkingText,
			},
		},
	}

	if err := e.wsClient.sendCmdRaw(cmd); err != nil {
		e.RuleConfig.Logger.Warnf("[WeCom] send thinking reply failed: %v", err)
	}

	return streamID
}

// sendStreamReply 发送流式回复（最终结果）
func (e *WeComWebSocket) sendStreamReply(reqID, content string) {
	// 尝试获取缓存的 streamID，如果没有则生成新的
	streamID, ok := e.streamIDCache.Load(reqID)
	if !ok {
		streamID = uuid.New().String()
	}

	// 最终回复：直接发送 finish: true 并且携带完整内容
	finishCmd := map[string]interface{}{
		"cmd":     wecomCmdRespondMsg,
		"headers": map[string]string{"req_id": reqID},
		"body": map[string]interface{}{
			"msgtype": "stream",
			"stream": map[string]interface{}{
				"id":      streamID,
				"finish":  true,
				"content": content,
			},
		},
	}
	if err := e.wsClient.sendCmdRaw(finishCmd); err != nil {
		e.RuleConfig.Logger.Warnf("[WeCom] send stream finish failed: %v", err)
	}

	e.streamIDCache.Delete(reqID)
}

// sendWelcomeMessage 发送欢迎语
func (e *WeComWebSocket) sendWelcomeMessage(reqID string) {
	cmd := map[string]interface{}{
		"cmd":     wecomCmdRespondWelcome,
		"headers": map[string]string{"req_id": reqID},
		"body": map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": e.Config.ThinkingText,
			},
		},
	}
	if err := e.wsClient.sendCmdRaw(cmd); err != nil {
		e.RuleConfig.Logger.Warnf("[WeCom] send welcome message failed: %v", err)
	}
}

// sendErrorMessage 发送错误消息
func (e *WeComWebSocket) sendErrorMessage(reqID string, err error) {
	if err == nil {
		return
	}

	errorMessage := buildFriendlyErrorMessage(err)
	e.RuleConfig.Logger.Warnf("[WeCom] Agent error: %v", err)

	e.sendStreamReply(reqID, errorMessage)
}

// triggerEvent 触发事件
func (e *WeComWebSocket) triggerEvent(eventName string, err error) {
	if e.OnEvent != nil {
		if err != nil {
			e.OnEvent(eventName, err)
		} else {
			e.OnEvent(eventName)
		}
	}
}

// ==================== 媒体处理 ====================

// saveMediaFile 保存媒体文件到工作空间
func (e *WeComWebSocket) saveMediaFile(data []byte, mediaType, fileName, mimeType string) (api2.MediaFile, error) {
	if int64(len(data)) > e.Config.Media.MaxFileSize {
		return api2.MediaFile{}, fmt.Errorf("file size %d exceeds limit %d", len(data), e.Config.Media.MaxFileSize)
	}

	dirPath := filepath.Join(e.WorkspaceDir, "wecom", mediaType)

	if fileName == "" {
		ext := wecomMimeToExt(mimeType)
		fileName = fmt.Sprintf("%s_%d%s", mediaType, time.Now().UnixMilli(), ext)
	}

	filePath := filepath.Join(dirPath, fileName)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return api2.MediaFile{}, fmt.Errorf("create directory failed: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return api2.MediaFile{}, fmt.Errorf("write file failed: %w", err)
	}

	relativePath := filepath.Join("wecom", mediaType, fileName)

	return api2.MediaFile{
		Type:         mediaType,
		FileName:     fileName,
		RelativePath: relativePath,
		FileSize:     int64(len(data)),
		MimeType:     mimeType,
	}, nil
}

// ==================== 工具函数 ====================

// convertWecomChatType 将企业微信 chatType 转换为统一标准
// "single" -> "p2p", "group" -> "group"
func convertWecomChatType(chatType string) string {
	switch chatType {
	case "single":
		return api2.ChatTypeP2P
	case "group":
		return api2.ChatTypeGroup
	default:
		return chatType
	}
}

// resolveWecomChatID 解析企业微信 chatID
// 綈息回调中，单聊时 chatid 可能为空，此时使用 userid 作为 chatID
func resolveWecomChatID(msgCallback wecomMsgCallback) string {
	if msgCallback.ChatID != "" {
		return msgCallback.ChatID
	}
	// 单聊时使用 userid 作为 chatID
	return msgCallback.From.UserID
}

// wecomDownloadMedia 下载并解密企业微信媒体文件
func wecomDownloadMedia(url, aesKey string) ([]byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("download media failed: %w", err)
	}
	defer resp.Body.Close()

	encryptedData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response body failed: %w", err)
	}

	if aesKey == "" {
		mimeType := detectMimeType(encryptedData)
		return encryptedData, mimeType, nil
	}

	decrypted, err := wecomDecryptMedia(encryptedData, aesKey)
	if err != nil {
		// 解密失败，尝试直接使用原始数据（长连接模式下 COS URL 可能已返回未加密数据）
		mimeType := detectMimeType(encryptedData)
		return encryptedData, mimeType, nil
	}

	mimeType := detectMimeType(decrypted)
	return decrypted, mimeType, nil
}

// wecomDecryptMedia AES-256-CBC 解密
func wecomDecryptMedia(encryptedData []byte, aesKeyBase64 string) ([]byte, error) {
	// 企微的 aeskey 可能是 43 字符（无 padding）或 44 字符（带 padding）的标准 base64
	// 长度不是 4 的倍数时，补齐 "=" 使 StdEncoding 能正确解码
	keyStr := aesKeyBase64
	if padding := len(keyStr) % 4; padding != 0 {
		keyStr += strings.Repeat("=", 4-padding)
	}
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("decode aes key failed: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher failed: %w", err)
	}

	iv := key[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(encryptedData))
	mode.CryptBlocks(decrypted, encryptedData)

	return wecomPKCS7Unpad(decrypted), nil
}

// wecomPKCS7Unpad PKCS#7 去填充
func wecomPKCS7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding <= 0 || padding > aes.BlockSize {
		return data
	}
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return data
		}
	}
	return data[:len(data)-padding]
}

// wecomCompressImage 压缩图片（复用飞书模式）
func wecomCompressImage(data []byte, mimeType string, maxSize int) ([]byte, string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, mimeType, nil
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxSize && h <= maxSize {
		return data, mimeType, nil
	}

	scale := float64(maxSize) / float64(max(w, h))
	newW := int(float64(w) * scale)
	newH := int(float64(h) * scale)

	resized := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(resized, resized.Bounds(), img, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 85}); err != nil {
		return data, mimeType, nil
	}

	return buf.Bytes(), "image/jpeg", nil
}

// wecomMimeToExt MIME 类型转扩展名
func wecomMimeToExt(mimeType string) string {
	exts := map[string]string{
		"image/png":  ".png",
		"image/jpeg": ".jpg",
		"image/gif":  ".gif",
		"audio/amr":  ".amr",
		"video/mp4":  ".mp4",
	}
	if ext, ok := exts[mimeType]; ok {
		return ext
	}
	ext, _ := mime.ExtensionsByType(mimeType)
	if len(ext) > 0 {
		return ext[0]
	}
	return ".bin"
}

// buildFriendlyErrorMessage 构建友好的错误提示（复用飞书模式）
func buildFriendlyErrorMessage(err error) string {
	errMsg := err.Error()
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
		return fmt.Sprintf("抱歉，处理您的请求时出现错误。请稍后重试。\n\n错误详情：%s", truncateString(errMsg, 200))
	}
}

// ==================== Request/Response Message ====================

// WeComRequestMessage 请求消息
type WeComRequestMessage struct {
	msg        *types.RuleMsg
	body       []byte
	headers    textproto.MIMEHeader
	statusCode int
	err        error
}

// NewWeComRequestMessage 创建请求消息
func NewWeComRequestMessage(msg *types.RuleMsg) *WeComRequestMessage {
	return &WeComRequestMessage{
		msg:  msg,
		body: []byte(msg.GetData()),
	}
}

func (m *WeComRequestMessage) Body() []byte        { return m.body }
func (m *WeComRequestMessage) SetBody(body []byte) { m.body = body }
func (m *WeComRequestMessage) Headers() textproto.MIMEHeader {
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
func (m *WeComRequestMessage) From() string { return "wecom/websocket" }
func (m *WeComRequestMessage) GetParam(key string) string {
	if m.msg != nil {
		return m.msg.Metadata.GetValue(key)
	}
	return ""
}
func (m *WeComRequestMessage) SetMsg(msg *types.RuleMsg) { m.msg = msg }
func (m *WeComRequestMessage) GetMsg() *types.RuleMsg    { return m.msg }
func (m *WeComRequestMessage) SetStatusCode(code int)    { m.statusCode = code }
func (m *WeComRequestMessage) SetError(err error)        { m.err = err }
func (m *WeComRequestMessage) GetError() error           { return m.err }

// WeComResponseMessage 响应消息
type WeComResponseMessage struct {
	body       []byte
	headers    textproto.MIMEHeader
	statusCode int
	err        error
	wsClient   *wecomWSClient
	requestMsg *types.RuleMsg
	botID      string
	logger     types.Logger
}

// NewWeComResponseMessage 创建响应消息
func NewWeComResponseMessage(wsClient *wecomWSClient, requestMsg *types.RuleMsg, botID string, logger types.Logger) *WeComResponseMessage {
	return &WeComResponseMessage{
		wsClient:   wsClient,
		requestMsg: requestMsg,
		botID:      botID,
		logger:     logger,
	}
}

func (m *WeComResponseMessage) Body() []byte { return m.body }

// SetBody 设置响应体，自动通过 WebSocket 发送流式消息
func (m *WeComResponseMessage) SetBody(body []byte) {
	m.body = body

	m.logger.Debugf("[WeCom] SetBody called, body length: %d", len(body))

	if m.wsClient == nil {
		m.logger.Warnf("[WeCom] Send reply failed: wsClient is nil")
		return
	}
	if len(body) == 0 {
		m.logger.Debugf("[WeCom] Send reply skipped: body is empty")
		return
	}

	// 检查是否有响应错误（如果有的化不发送内容）
	if m.err != nil {
		m.logger.Warnf("[WeCom] Send reply skipped: response has error: %v", m.err)
		return
	}

	reqID := ""
	streamID := ""
	if m.requestMsg != nil {
		// 优先从响应元数据中获取 reqId
		reqID = m.requestMsg.Metadata.GetValue(api2.MetaResponseWeComReqID)
		if reqID == "" {
			reqID = m.requestMsg.Metadata.GetValue(api2.MetaWeComReqID)
		}
		// 获取之前创建的 streamID
		streamID = m.requestMsg.Metadata.GetValue(api2.MetaWeComStreamID)
	}
	if reqID == "" {
		m.logger.Warnf("[WeCom] Send reply failed: missing reqID")
		return
	}

	m.logger.Debugf("[WeCom] Sending reply, reqID: %s, streamID: %s, content length: %d", reqID, streamID, len(body))

	// 如果没有复用的 streamID，生成新的
	if streamID == "" {
		streamID = uuid.New().String()
	}

	// 最终回复：发送 finish: true 和完整内容
	// 根据企微文档，finish: true 时 content 不能是空，它会覆盖流式消息的最终内容
	finishCmd := map[string]interface{}{
		"cmd":     wecomCmdRespondMsg,
		"headers": map[string]string{"req_id": reqID},
		"body": map[string]interface{}{
			"msgtype": "stream",
			"stream": map[string]interface{}{
				"id":      streamID,
				"finish":  true,
				"content": string(body),
			},
		},
	}
	if err := m.wsClient.sendCmdRaw(finishCmd); err != nil {
		m.err = fmt.Errorf("send stream finish failed: %w", err)
	}
}

func (m *WeComResponseMessage) Headers() textproto.MIMEHeader {
	if m.headers == nil {
		m.headers = make(textproto.MIMEHeader)
	}
	return m.headers
}
func (m *WeComResponseMessage) From() string               { return "wecom/websocket/response" }
func (m *WeComResponseMessage) GetParam(key string) string { return "" }
func (m *WeComResponseMessage) SetMsg(msg *types.RuleMsg)  { m.requestMsg = msg }
func (m *WeComResponseMessage) GetMsg() *types.RuleMsg     { return m.requestMsg }
func (m *WeComResponseMessage) SetStatusCode(code int)     { m.statusCode = code }
func (m *WeComResponseMessage) SetError(err error)         { m.err = err }
func (m *WeComResponseMessage) GetError() error            { return m.err }
