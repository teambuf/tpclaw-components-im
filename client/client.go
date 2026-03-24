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

// Package client provides IM platform clients for sending messages.
// IM 平台客户端，用于发送消息。
package client

import (
	"context"
	"fmt"
	imapi "github.com/teambuf/tpclaw-components-im/api"
)

// IMClient IM 平台客户端接口（用于发送消息）
type IMClient interface {
	// Platform 返回平台标识
	Platform() string

	// SendMessage 发送消息到指定目标
	// target: 目标地址（用户ID/群ID/频道ID）
	// message: 消息内容
	SendMessage(ctx context.Context, target, message string) error

	// ReplyMessage 回复指定消息
	// msgID: 要回复的消息ID
	// message: 回复内容
	// 注意：不是所有平台都支持直接回复
	ReplyMessage(ctx context.Context, msgID, message string) error

	// UpdateCard 更新交互式卡片
	// cardID: 卡片ID
	// data: 卡片数据
	UpdateCard(ctx context.Context, cardID string, data map[string]interface{}) error

	// UploadImage 上传图片
	// imagePath: 图片文件路径
	// 返回: imageKey 用于后续发送图片消息
	UploadImage(ctx context.Context, imagePath string) (string, error)

	// UploadFile 上传文件
	// filePath: 文件路径
	// fileName: 文件名（展示给用户）
	// 返回: fileKey 用于后续发送文件消息
	UploadFile(ctx context.Context, filePath, fileName string) (string, error)

	// SendImageMessage 发送图片消息
	// target: 目标地址
	// imageKey: 上传后获得的图片 key
	SendImageMessage(ctx context.Context, target, imageKey string) error

	// SendFileMessage 发送文件消息
	// target: 目标地址
	// fileKey: 上传后获得的文件 key
	// fileName: 文件名（展示给用户）
	SendFileMessage(ctx context.Context, target, fileKey, fileName string) error
}

// Config 客户端通用配置
type Config struct {
	// Platform 平台类型
	Platform string
	// AppID 应用 ID
	AppID string
	// AppSecret 应用密钥
	AppSecret string
	// Extra 额外配置
	Extra map[string]interface{}
}

// NewClient 创建客户端（简化版，使用通用配置）
func NewClient(config *Config) (IMClient, error) {
	switch config.Platform {
	case imapi.PlatformFeishu:
		return NewFeishuClient(&FeishuConfig{
			AppID:     config.AppID,
			AppSecret: config.AppSecret,
		}), nil
	case imapi.PlatformDingTalk:
		return NewDingTalkClient(&DingTalkConfig{
			AppKey:    config.AppID,
			AppSecret: config.AppSecret,
		}), nil
	case imapi.PlatformWeCom:
		return NewWeComClient(&WeComConfig{
			CorpID: config.AppID,
			Secret: config.AppSecret,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", config.Platform)
	}
}
