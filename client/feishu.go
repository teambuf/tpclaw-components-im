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
	"encoding/base64"
	"encoding/json"
	"fmt"
	imapi "github.com/teambuf/tpclaw-components-im/api"
	"io"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// FeishuClient 飞书客户端（基于官方 SDK 实现）
type FeishuClient struct {
	client    *lark.Client
	appID     string
	appSecret string
	tenantKey string
}

// FeishuConfig 飞书客户端配置
type FeishuConfig struct {
	AppID     string
	AppSecret string
	TenantKey string // 可选：多租户场景
}

// NewFeishuClient 创建飞书客户端
func NewFeishuClient(config *FeishuConfig) *FeishuClient {
	// 使用官方 SDK 创建客户端，自动管理 token
	client := lark.NewClient(config.AppID, config.AppSecret,
		lark.WithEnableTokenCache(true),
	)

	return &FeishuClient{
		client:    client,
		appID:     config.AppID,
		appSecret: config.AppSecret,
		tenantKey: config.TenantKey,
	}
}

// Platform 返回平台标识
func (c *FeishuClient) Platform() string {
	return imapi.PlatformFeishu
}

// detectReceiveIdType 根据 ID 前缀自动判断 receive_id_type
func detectReceiveIdType(id string) string {
	switch {
	case strings.HasPrefix(id, "ou_"):
		return larkim.ReceiveIdTypeOpenId
	case strings.HasPrefix(id, "on_"):
		return larkim.ReceiveIdTypeUnionId
	case strings.HasPrefix(id, "oc_"):
		return larkim.ReceiveIdTypeChatId
	default:
		// 默认使用 chat_id
		return larkim.ReceiveIdTypeChatId
	}
}

// SetTenantKey 设置租户 key（多租户场景）
func (c *FeishuClient) SetTenantKey(tenantKey string) {
	c.tenantKey = tenantKey
}

// GetLarkClient 获取底层 lark 客户端（用于高级用法）
func (c *FeishuClient) GetLarkClient() *lark.Client {
	return c.client
}

// SendMessage 发送消息
func (c *FeishuClient) SendMessage(ctx context.Context, target, message string) error {
	// 使用 SDK 的消息构建器
	content := larkim.NewTextMsgBuilder().
		Text(message).
		Build()

	// 构建请求
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(detectReceiveIdType(target)).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			ReceiveId(target).
			Content(content).
			Build()).
		Build()

	// 发送请求
	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error (code %d): %s, request_id: %s",
			resp.Code, resp.Msg, resp.RequestId())
	}

	return nil
}

// ReplyMessage 回复消息
func (c *FeishuClient) ReplyMessage(ctx context.Context, msgID, message string) error {
	// 使用 SDK 的消息构建器
	content := larkim.NewTextMsgBuilder().
		Text(message).
		Build()

	// 构建请求
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(msgID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			Content(content).
			Build()).
		Build()

	// 发送请求
	resp, err := c.client.Im.Message.Reply(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to reply message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error (code %d): %s, request_id: %s",
			resp.Code, resp.Msg, resp.RequestId())
	}

	return nil
}

// UpdateCard 更新卡片
func (c *FeishuClient) UpdateCard(ctx context.Context, cardID string, data map[string]interface{}) error {
	// 将 data 转换为 JSON
	contentBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal card data: %w", err)
	}

	// 构建请求
	req := larkim.NewPatchMessageReqBuilder().
		MessageId(cardID).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(string(contentBytes)).
			Build()).
		Build()

	// 发送请求
	resp, err := c.client.Im.Message.Patch(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to update card: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error (code %d): %s, request_id: %s",
			resp.Code, resp.Msg, resp.RequestId())
	}

	return nil
}

// SendMessageWithReceiveIdType 发送消息（指定接收者 ID 类型）
// receiveIdType: chat_id, open_id, user_id, union_id
func (c *FeishuClient) SendMessageWithReceiveIdType(ctx context.Context, receiveIdType, target, message string) error {
	content := larkim.NewTextMsgBuilder().
		Text(message).
		Build()

	var idType string
	switch receiveIdType {
	case "open_id":
		idType = larkim.ReceiveIdTypeOpenId
	case "user_id":
		idType = larkim.ReceiveIdTypeUserId
	case "union_id":
		idType = larkim.ReceiveIdTypeUnionId
	default:
		idType = larkim.ReceiveIdTypeChatId
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(idType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			ReceiveId(target).
			Content(content).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error (code %d): %s, request_id: %s",
			resp.Code, resp.Msg, resp.RequestId())
	}

	return nil
}

// SendInteractiveMessage 发送交互式卡片消息
func (c *FeishuClient) SendInteractiveMessage(ctx context.Context, target string, card *larkcard.MessageCard) error {
	cardContent, err := card.String()
	if err != nil {
		return fmt.Errorf("failed to build card: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(detectReceiveIdType(target)).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			ReceiveId(target).
			Content(cardContent).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send interactive message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error (code %d): %s, request_id: %s",
			resp.Code, resp.Msg, resp.RequestId())
	}

	return nil
}

// SendPostMessage 发送富文本消息
func (c *FeishuClient) SendPostMessage(ctx context.Context, target string, post *larkim.MessagePost) error {
	content, err := post.String()
	if err != nil {
		return fmt.Errorf("failed to build post: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypePost).
			ReceiveId(target).
			Content(content).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send post message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error (code %d): %s, request_id: %s",
			resp.Code, resp.Msg, resp.RequestId())
	}

	return nil
}

// SendImageMessage 发送图片消息
func (c *FeishuClient) SendImageMessage(ctx context.Context, target, imageKey string) error {
	msgImage := larkim.MessageImage{ImageKey: imageKey}
	content, err := msgImage.String()
	if err != nil {
		return fmt.Errorf("failed to build image message: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeImage).
			ReceiveId(target).
			Content(content).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send image message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error (code %d): %s, request_id: %s",
			resp.Code, resp.Msg, resp.RequestId())
	}

	return nil
}

// SendFileMessage 发送文件消息
func (c *FeishuClient) SendFileMessage(ctx context.Context, target, fileKey, fileName string) error {
	msgFile := larkim.MessageFile{FileKey: fileKey}
	content, err := msgFile.String()
	if err != nil {
		return fmt.Errorf("failed to build file message: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeFile).
			ReceiveId(target).
			Content(content).
			Build()).
		Build()

	resp, err := c.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send file message: %w", err)
	}

	if !resp.Success() {
		return fmt.Errorf("feishu API error (code %d): %s, request_id: %s",
			resp.Code, resp.Msg, resp.RequestId())
	}

	return nil
}

// UploadImage 上传图片
func (c *FeishuClient) UploadImage(ctx context.Context, imagePath string) (string, error) {
	body, err := larkim.NewCreateImagePathReqBodyBuilder().
		ImagePath(imagePath).
		ImageType(larkim.ImageTypeMessage).
		Build()
	if err != nil {
		return "", fmt.Errorf("failed to build image request: %w", err)
	}

	req := larkim.NewCreateImageReqBuilder().
		Body(body).
		Build()

	resp, err := c.client.Im.Image.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("feishu API error (code %d): %s, request_id: %s",
			resp.Code, resp.Msg, resp.RequestId())
	}

	return *resp.Data.ImageKey, nil
}

// UploadFile 上传文件
func (c *FeishuClient) UploadFile(ctx context.Context, filePath, fileName string) (string, error) {
	// 根据文件扩展名推断文件类型
	fileType := "stream"
	if idx := strings.LastIndex(fileName, "."); idx != -1 {
		ext := strings.ToLower(fileName[idx+1:])
		switch ext {
		case "pdf":
			fileType = "pdf"
		case "doc", "docx":
			fileType = "doc"
		case "xls", "xlsx":
			fileType = "xls"
		case "ppt", "pptx":
			fileType = "ppt"
		}
	}

	body, err := larkim.NewCreateFilePathReqBodyBuilder().
		FilePath(filePath).
		FileName(fileName).
		FileType(fileType).
		Build()
	if err != nil {
		return "", fmt.Errorf("failed to build file request: %w", err)
	}

	req := larkim.NewCreateFileReqBuilder().
		Body(body).
		Build()

	resp, err := c.client.Im.File.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("feishu API error (code %d): %s, request_id: %s",
			resp.Code, resp.Msg, resp.RequestId())
	}

	return *resp.Data.FileKey, nil
}

// GetUserInfo 获取用户信息
func (c *FeishuClient) GetUserInfo(ctx context.Context, userID string) (*larkcore.ApiResp, error) {
	req := larkim.NewGetMessageReqBuilder().
		MessageId(userID).
		Build()

	resp, err := c.client.Im.Message.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	return resp.ApiResp, nil
}

// GetImageAsBase64 下载图片并返回 base64 格式字符串
// 返回格式: data:image/png;base64,iVBORw0KGgo...
func (c *FeishuClient) GetImageAsBase64(ctx context.Context, imageKey string) (string, error) {
	req := larkim.NewGetImageReqBuilder().
		ImageKey(imageKey).
		Build()

	resp, err := c.client.Im.Image.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("feishu API error (code %d): %s", resp.Code, resp.Msg)
	}

	// 读取图片数据
	if resp.File == nil {
		return "", fmt.Errorf("image data is empty")
	}

	imageData, err := io.ReadAll(resp.File)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}

	// 根据文件名推断 MIME 类型
	mimeType := "image/png"
	if resp.FileName != "" {
		switch {
		case strings.HasSuffix(strings.ToLower(resp.FileName), ".jpg"),
			strings.HasSuffix(strings.ToLower(resp.FileName), ".jpeg"):
			mimeType = "image/jpeg"
		case strings.HasSuffix(strings.ToLower(resp.FileName), ".gif"):
			mimeType = "image/gif"
		case strings.HasSuffix(strings.ToLower(resp.FileName), ".webp"):
			mimeType = "image/webp"
		case strings.HasSuffix(strings.ToLower(resp.FileName), ".bmp"):
			mimeType = "image/bmp"
		}
	}

	// 转换为 base64
	base64Data := base64.StdEncoding.EncodeToString(imageData)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data), nil
}
