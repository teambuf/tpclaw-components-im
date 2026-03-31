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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rulego/rulego/api/types"
)

// ==================== WebSocket 协议常量 ====================

const (
	// 企业微信智能机器人 WebSocket 地址
	wecomWSURL = "wss://openws.work.weixin.qq.com"

	// 协议命令
	wecomCmdSubscribe      = "aibot_subscribe"
	wecomCmdMsgCallback    = "aibot_msg_callback"
	wecomCmdEventCallback  = "aibot_event_callback"
	wecomCmdRespondMsg     = "aibot_respond_msg"
	wecomCmdRespondWelcome = "aibot_respond_welcome_msg"
	wecomCmdRespondUpdate  = "aibot_respond_update_msg"
	wecomCmdSendMsg        = "aibot_send_msg"
	wecomCmdPing           = "ping"

	// 媒体上传相关命令
	wecomCmdUploadMediaInit   = "aibot_upload_media_init"
	wecomCmdUploadMediaChunk  = "aibot_upload_media_chunk"
	wecomCmdUploadMediaFinish = "aibot_upload_media_finish"

	// 事件类型
	wecomEventEnterChat    = "enter_chat"
	wecomEventDisconnected = "disconnected_event"
	wecomEventTemplateCard = "template_card_event"
	wecomEventFeedback     = "feedback_event"
)

// ==================== WebSocket 消息协议结构 ====================

// wecomWSMessage WebSocket 消息通用结构
type wecomWSMessage struct {
	Cmd     string          `json:"cmd"`
	Headers wecomWSHeaders  `json:"headers"`
	Body    json.RawMessage `json:"body,omitempty"`
}

type wecomWSHeaders struct {
	ReqID string `json:"req_id"`
}

// wecomWSResponse WebSocket 响应通用结构
type wecomWSResponse struct {
	Headers wecomWSHeaders  `json:"headers"`
	Body    json.RawMessage `json:"body,omitempty"`
	ErrCode int             `json:"errcode"`
	ErrMsg  string          `json:"errmsg"`
}

// wecomMsgCallback 消息回调 body
type wecomMsgCallback struct {
	MsgID    string          `json:"msgid"`
	AIBotID  string          `json:"aibotid"`
	ChatID   string          `json:"chatid"`
	ChatType string          `json:"chattype"` // single / group
	From     wecomFrom       `json:"from"`
	MsgType  string          `json:"msgtype"` // text, image, mixed, voice, file, video
	Text     *wecomTextBody  `json:"text,omitempty"`
	Image    *wecomMediaBody `json:"image,omitempty"`
	File     *wecomMediaBody `json:"file,omitempty"`
	Video    *wecomMediaBody `json:"video,omitempty"`
	Voice    *wecomMediaBody `json:"voice,omitempty"`
	Mixed    *wecomMixedBody `json:"mixed,omitempty"`
}

type wecomFrom struct {
	UserID string `json:"userid"`
}

type wecomTextBody struct {
	Content string `json:"content"`
}

type wecomMediaBody struct {
	URL    string `json:"url"`
	AESKey string `json:"aeskey"`
}

type wecomMixedBody struct {
	Items []wecomMixedItem `json:"msg_item"`
}

type wecomMixedItem struct {
	MsgType string          `json:"msgtype"`
	Text    *wecomTextBody  `json:"text,omitempty"`
	Image   *wecomMediaBody `json:"image,omitempty"`
}

// wecomEventCallback 事件回调 body
type wecomEventCallback struct {
	MsgID      string     `json:"msgid"`
	CreateTime int64      `json:"create_time"`
	AIBotID    string     `json:"aibotid"`
	ChatID     string     `json:"chatid,omitempty"`
	ChatType   string     `json:"chattype,omitempty"`
	From       wecomFrom  `json:"from,omitempty"`
	MsgType    string     `json:"msgtype"` // 固定 "event"
	Event      wecomEvent `json:"event"`
}

type wecomEvent struct {
	EventType string `json:"eventtype"` // enter_chat, template_card_event, feedback_event, disconnected_event
}

// WecomWSSender 企业微信 WebSocket 发送器接口
// 提取此接口以便应用层通过全局注册表复用长连接发送主动消息
type WecomWSSender interface {
	SendCmdRaw(cmd map[string]interface{}) error
	UploadMedia(ctx context.Context, mediaType string, filename string, md5Str string, data []byte) (string, error)
}

// 全局企业微信 WebSocket 客户端注册表，用于应用层复用长连接主动发送消息
var (
	wecomWSClients   = make(map[string]WecomWSSender)
	wecomWSClientsMu sync.RWMutex
)

// RegisterWecomWSClient 注册企业微信 WebSocket 客户端
func RegisterWecomWSClient(botID string, client WecomWSSender) {
	wecomWSClientsMu.Lock()
	defer wecomWSClientsMu.Unlock()
	wecomWSClients[botID] = client
}

// UnregisterWecomWSClient 注销企业微信 WebSocket 客户端
func UnregisterWecomWSClient(botID string) {
	wecomWSClientsMu.Lock()
	defer wecomWSClientsMu.Unlock()
	delete(wecomWSClients, botID)
}

// GetWecomWSClient 获取已注册的企业微信 WebSocket 客户端
func GetWecomWSClient(botID string) (WecomWSSender, bool) {
	wecomWSClientsMu.RLock()
	defer wecomWSClientsMu.RUnlock()
	client, ok := wecomWSClients[botID]
	return client, ok
}

// ==================== WebSocket 客户端 ====================

// wecomWSClientConfig WebSocket 客户端配置
type wecomWSClientConfig struct {
	BotID             string
	Secret            string
	WSURL             string
	HeartbeatInterval time.Duration
	AutoReconnect     bool
	ReconnectDelay    time.Duration
}

// wecomWSClient 企业微信智能机器人 WebSocket 客户端
// 封装连接、认证、心跳、断线重连等底层逻辑
type wecomWSClient struct {
	config wecomWSClientConfig
	conn   *websocket.Conn
	connMu sync.Mutex // 保护 conn 的写操作

	// 全局生命周期
	ctx    context.Context
	cancel context.CancelFunc

	// 连接生命周期，用于在重连时取消旧的 goroutines
	connCtx    context.Context
	connCancel context.CancelFunc

	reconnectMu sync.Mutex // 防止并发重连
	wg          sync.WaitGroup
	logger      types.Logger

	pendingReqs sync.Map // reqID -> chan []byte 用于同步等待响应

	// 回调：收到消息/事件时调用
	OnMessage func(cmd string, reqID string, body json.RawMessage)
	OnEvent   func(cmd string, reqID string, body json.RawMessage)
}

// newWecomWSClient 创建 WebSocket 客户端
func newWecomWSClient(config wecomWSClientConfig, logger types.Logger) *wecomWSClient {
	if config.WSURL == "" {
		config.WSURL = wecomWSURL
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 30 * time.Second
	}
	if config.ReconnectDelay == 0 {
		config.ReconnectDelay = 3 * time.Second
	}
	return &wecomWSClient{
		config: config,
		logger: logger,
	}
}

// start 启动 WebSocket 客户端（连接 + 认证 + 读循环 + 心跳）
func (c *wecomWSClient) start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	return c.connectAndRun()
}

// connectAndRun 建立连接并启动相关的读和心跳协程
func (c *wecomWSClient) connectAndRun() error {
	if err := c.connect(); err != nil {
		return fmt.Errorf("wecom ws connect failed: %w", err)
	}

	c.connMu.Lock()
	c.connCtx, c.connCancel = context.WithCancel(c.ctx)
	c.connMu.Unlock()

	c.wg.Add(1)
	go c.readLoop()

	c.wg.Add(1)
	go c.heartbeatLoop()

	return nil
}

// close 关闭 WebSocket 客户端
func (c *wecomWSClient) close() {
	if c.cancel != nil {
		c.cancel()
	}
	c.connMu.Lock()
	if c.connCancel != nil {
		c.connCancel()
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

// sendCmd 发送命令
func (c *wecomWSClient) sendCmd(cmd string, body interface{}) error {
	reqID := uuid.New().String()
	msg := wecomWSMessage{
		Cmd:     cmd,
		Headers: wecomWSHeaders{ReqID: reqID},
	}
	if body != nil {
		data, _ := json.Marshal(body)
		msg.Body = data
	}
	return c.sendRaw(msg)
}

// sendCmdWithReqID 发送命令（指定 reqID）
func (c *wecomWSClient) sendCmdWithReqID(cmd, reqID string, body interface{}) error {
	msg := wecomWSMessage{
		Cmd:     cmd,
		Headers: wecomWSHeaders{ReqID: reqID},
	}
	if body != nil {
		data, _ := json.Marshal(body)
		msg.Body = data
	}
	return c.sendRaw(msg)
}

// SendCmdRaw 发送原始命令（map 格式）
func (c *wecomWSClient) SendCmdRaw(cmd map[string]interface{}) error {
	return c.sendRaw(cmd)
}

// sendCmdRaw 内部方法保持向后兼容（指向公开方法）
func (c *wecomWSClient) sendCmdRaw(cmd map[string]interface{}) error {
	return c.SendCmdRaw(cmd)
}

// syncSendCmd 发送命令并等待响应
func (c *wecomWSClient) syncSendCmd(ctx context.Context, cmd string, body interface{}) (*wecomWSResponse, error) {
	reqID := uuid.New().String()
	msg := wecomWSMessage{
		Cmd:     cmd,
		Headers: wecomWSHeaders{ReqID: reqID},
	}
	if body != nil {
		data, _ := json.Marshal(body)
		msg.Body = data
	}

	ch := make(chan []byte, 1)
	c.pendingReqs.Store(reqID, ch)
	defer c.pendingReqs.Delete(reqID)

	if err := c.sendRaw(msg); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case data := <-ch:
		var resp wecomWSResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal response failed: %w", err)
		}
		if resp.ErrCode != 0 {
			return &resp, fmt.Errorf("api error: %d - %s", resp.ErrCode, resp.ErrMsg)
		}
		return &resp, nil
	}
}

// UploadMedia 上传临时素材（分片上传）
// 返回 media_id 和 error
func (c *wecomWSClient) UploadMedia(ctx context.Context, mediaType string, filename string, md5Str string, data []byte) (string, error) {
	totalSize := len(data)
	const chunkSize = 512 * 1024 // 512KB
	totalChunks := (totalSize + chunkSize - 1) / chunkSize

	// 1. 初始化上传
	initBody := map[string]interface{}{
		"type":         mediaType,
		"filename":     filename,
		"total_size":   totalSize,
		"total_chunks": totalChunks,
	}
	if md5Str != "" {
		initBody["md5"] = md5Str
	}

	initResp, err := c.syncSendCmd(ctx, wecomCmdUploadMediaInit, initBody)
	if err != nil {
		return "", fmt.Errorf("init upload failed: %w", err)
	}

	var initResult struct {
		UploadID string `json:"upload_id"`
	}
	if err := json.Unmarshal(initResp.Body, &initResult); err != nil {
		return "", fmt.Errorf("parse init response failed: %w", err)
	}
	uploadID := initResult.UploadID
	if uploadID == "" {
		return "", fmt.Errorf("empty upload_id received")
	}

	// 2. 分片上传
	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > totalSize {
			end = totalSize
		}
		chunkData := data[start:end]
		base64Data := base64.StdEncoding.EncodeToString(chunkData)

		chunkBody := map[string]interface{}{
			"upload_id":   uploadID,
			"chunk_index": i,
			"base64_data": base64Data,
		}

		_, err := c.syncSendCmd(ctx, wecomCmdUploadMediaChunk, chunkBody)
		if err != nil {
			return "", fmt.Errorf("upload chunk %d failed: %w", i, err)
		}
	}

	// 3. 完成上传
	finishBody := map[string]interface{}{
		"upload_id": uploadID,
	}
	finishResp, err := c.syncSendCmd(ctx, wecomCmdUploadMediaFinish, finishBody)
	if err != nil {
		return "", fmt.Errorf("finish upload failed: %w", err)
	}

	var finishResult struct {
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(finishResp.Body, &finishResult); err != nil {
		return "", fmt.Errorf("parse finish response failed: %w", err)
	}

	if finishResult.MediaID == "" {
		return "", fmt.Errorf("empty media_id received")
	}

	return finishResult.MediaID, nil
}

// sendRaw 发送原始 JSON 数据
func (c *wecomWSClient) sendRaw(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("websocket not connected")
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// connect 建立 WebSocket 连接并发送订阅请求
func (c *wecomWSClient) connect() error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.DialContext(c.ctx, c.config.WSURL, nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	// 发送订阅请求
	if err := c.subscribe(); err != nil {
		c.connMu.Lock()
		c.conn.Close()
		c.conn = nil
		c.connMu.Unlock()
		return fmt.Errorf("subscribe failed: %w", err)
	}

	if c.logger != nil {
		c.logger.Infof("[WeCom] WebSocket connected and subscribed, botId: %s", c.config.BotID)
	}
	return nil
}

// subscribe 发送订阅请求（aibot_subscribe）
func (c *wecomWSClient) subscribe() error {
	body := map[string]string{
		"bot_id": c.config.BotID,
		"secret": c.config.Secret,
	}
	return c.sendCmd(wecomCmdSubscribe, body)
}

// readLoop 读消息循环
func (c *wecomWSClient) readLoop() {
	defer c.wg.Done()

	c.connMu.Lock()
	connCtx := c.connCtx
	c.connMu.Unlock()

	for {
		select {
		case <-connCtx.Done():
			return
		default:
		}

		c.connMu.Lock()
		conn := c.conn
		c.connMu.Unlock()

		if conn == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if c.ctx.Err() != nil {
				return
			}
			if connCtx.Err() != nil {
				return
			}
			if c.logger != nil {
				c.logger.Warnf("[WeCom] read error: %v", err)
			}
			// 断线重连
			if c.config.AutoReconnect {
				go c.reconnect()
			}
			return
		}

		c.handleMessage(message)
	}
}

// heartbeatLoop 心跳循环（每 30 秒发送 ping）
func (c *wecomWSClient) heartbeatLoop() {
	defer c.wg.Done()

	c.connMu.Lock()
	connCtx := c.connCtx
	c.connMu.Unlock()

	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-connCtx.Done():
			return
		case <-ticker.C:
			err := c.sendCmd(wecomCmdPing, nil)
			if err != nil && c.logger != nil {
				c.logger.Debugf("[WeCom] heartbeat error: %v", err)
			}
		}
	}
}

// handleMessage 处理收到的消息
func (c *wecomWSClient) handleMessage(data []byte) {
	// 打印收到的原始消息
	if c.logger != nil {
		rawStr := string(data)
		if len(rawStr) > 2000 {
			rawStr = rawStr[:2000] + "...(truncated)"
		}
		c.logger.Debugf("[WeCom][RAW] 收到消息: %s", rawStr)
	}

	var resp wecomWSResponse
	if err := json.Unmarshal(data, &resp); err == nil && resp.Headers.ReqID != "" {
		if ch, ok := c.pendingReqs.Load(resp.Headers.ReqID); ok {
			ch.(chan []byte) <- data
			// 如果是响应，则不继续处理为 callback
			if resp.ErrCode != 0 || resp.ErrMsg != "" || len(resp.Body) == 0 {
				return
			}
			// 有些带有 body 的响应可能会被继续处理，但由于 pendingReqs 命中，说明是我们主动发起的请求的响应
			return
		}
	}

	var msg wecomWSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		if c.logger != nil {
			c.logger.Warnf("[WeCom] unmarshal message failed: %v, raw: %s", err, string(data))
		}
		return
	}

	reqID := msg.Headers.ReqID

	switch msg.Cmd {
	case wecomCmdMsgCallback:
		if c.OnMessage != nil {
			c.OnMessage(msg.Cmd, reqID, msg.Body)
		}
	case wecomCmdEventCallback:
		if c.OnEvent != nil {
			c.OnEvent(msg.Cmd, reqID, msg.Body)
		}
	default:
		// pong 或其他响应
		if c.logger != nil {
			c.logger.Debugf("[WeCom] received response: cmd=%s, reqId=%s, raw=%s", msg.Cmd, reqID, string(data))
		}
	}
}

// reconnect 断线重连
func (c *wecomWSClient) reconnect() {
	// 防止并发重连
	if !c.reconnectMu.TryLock() {
		return
	}
	defer c.reconnectMu.Unlock()

	// 确保旧连接资源被清理，并通知旧的协程退出
	c.connMu.Lock()
	if c.connCancel != nil {
		c.connCancel()
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(c.config.ReconnectDelay):
		}

		if c.logger != nil {
			c.logger.Infof("[WeCom] reconnecting...")
		}

		if err := c.connectAndRun(); err != nil {
			if c.logger != nil {
				c.logger.Warnf("[WeCom] reconnect failed: %v", err)
			}
			continue
		}

		// connectAndRun 成功后会自动启动新的 readLoop 和 heartbeatLoop
		return
	}
}
