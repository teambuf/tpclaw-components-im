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

package client

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"

	imapi "github.com/teambuf/tpclaw-components-im/api"
	"github.com/teambuf/tpclaw-components-im/endpoint"
)

// WeComBotClient 企业微信智能机器人客户端（长连接模式）
// 通过复用 Endpoint 建立的 WebSocket 连接来发送主动消息
type WeComBotClient struct {
	botID string
}

// NewWeComBotClient 创建企业微信智能机器人客户端
func NewWeComBotClient(botID string) *WeComBotClient {
	return &WeComBotClient{
		botID: botID,
	}
}

// Platform 返回平台标识
func (c *WeComBotClient) Platform() string {
	return imapi.PlatformWeCom
}

// getSender 获取 WebSocket 发送器
func (c *WeComBotClient) getSender() (endpoint.WecomWSSender, error) {
	sender, ok := endpoint.GetWecomWSClient(c.botID)
	if !ok {
		return nil, fmt.Errorf("wecom ws client not found for bot %s, maybe endpoint is not started", c.botID)
	}
	return sender, nil
}

// SendMessage 发送文本消息
func (c *WeComBotClient) SendMessage(ctx context.Context, target, message string) error {
	sender, err := c.getSender()
	if err != nil {
		return err
	}

	cmd := map[string]interface{}{
		"cmd": "aibot_send_msg",
		"body": map[string]interface{}{
			"chatid":  target,
			"msgtype": "markdown",
			"markdown": map[string]interface{}{
				"content": message,
			},
		},
	}

	return sender.SendCmdRaw(cmd)
}

// ReplyMessage 回复指定消息（不支持，长连接被动回复在 Endpoint 内部通过 stream 方式实现）
func (c *WeComBotClient) ReplyMessage(ctx context.Context, msgID, message string) error {
	return fmt.Errorf("reply message not supported for wecom bot client")
}

// UpdateCard 更新交互式卡片（不支持）
func (c *WeComBotClient) UpdateCard(ctx context.Context, cardID string, data map[string]interface{}) error {
	return fmt.Errorf("update card not supported for wecom bot client")
}

// UploadImage 上传图片
func (c *WeComBotClient) UploadImage(ctx context.Context, imagePath string) (string, error) {
	sender, err := c.getSender()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}

	hash := md5.Sum(data)
	md5Str := hex.EncodeToString(hash[:])

	mediaID, err := sender.UploadMedia(ctx, "image", "image.png", md5Str, data)
	if err != nil {
		return "", fmt.Errorf("failed to upload image via ws: %w", err)
	}

	return mediaID, nil
}

// UploadFile 上传文件
func (c *WeComBotClient) UploadFile(ctx context.Context, filePath, fileName string) (string, error) {
	sender, err := c.getSender()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	hash := md5.Sum(data)
	md5Str := hex.EncodeToString(hash[:])

	mediaID, err := sender.UploadMedia(ctx, "file", fileName, md5Str, data)
	if err != nil {
		return "", fmt.Errorf("failed to upload file via ws: %w", err)
	}

	return mediaID, nil
}

// SendImageMessage 发送图片消息
func (c *WeComBotClient) SendImageMessage(ctx context.Context, target, imageKey string) error {
	sender, err := c.getSender()
	if err != nil {
		return err
	}

	cmd := map[string]interface{}{
		"cmd": "aibot_send_msg",
		"body": map[string]interface{}{
			"chatid":  target,
			"msgtype": "image",
			"image": map[string]interface{}{
				"media_id": imageKey,
			},
		},
	}

	return sender.SendCmdRaw(cmd)
}

// SendFileMessage 发送文件消息
func (c *WeComBotClient) SendFileMessage(ctx context.Context, target, fileKey, fileName string) error {
	sender, err := c.getSender()
	if err != nil {
		return err
	}

	cmd := map[string]interface{}{
		"cmd": "aibot_send_msg",
		"body": map[string]interface{}{
			"chatid":  target,
			"msgtype": "file",
			"file": map[string]interface{}{
				"media_id": fileKey,
			},
		},
	}

	return sender.SendCmdRaw(cmd)
}
