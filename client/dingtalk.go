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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	imapi "github.com/teambuf/tpclaw-components-im/api"
	"github.com/teambuf/tpclaw-components-im/internal/cache"
	"github.com/teambuf/tpclaw-components-im/internal/constants"
	"github.com/teambuf/tpclaw-components-im/internal/httpclient"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

// DingTalkClient 钉钉客户端
type DingTalkClient struct {
	appKey      string
	appSecret   string
	tokenCache  *cache.TokenCache
	httpClient  *http.Client
	uploadMutex sync.Mutex
}

// DingTalkConfig 钉钉客户端配置
type DingTalkConfig struct {
	AppKey    string
	AppSecret string
}

// NewDingTalkClient 创建钉钉客户端
func NewDingTalkClient(config *DingTalkConfig) *DingTalkClient {
	return &DingTalkClient{
		appKey:     config.AppKey,
		appSecret:  config.AppSecret,
		tokenCache: cache.NewTokenCache(),
		httpClient: httpclient.DefaultClient(),
	}
}

// Platform 返回平台标识
func (c *DingTalkClient) Platform() string {
	return imapi.PlatformDingTalk
}

// SendMessage 发送消息
func (c *DingTalkClient) SendMessage(ctx context.Context, target, message string) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	return c.sendMessage(ctx, token, target, message)
}

// ReplyMessage 回复消息（钉钉不支持直接回复）
func (c *DingTalkClient) ReplyMessage(ctx context.Context, msgID, message string) error {
	return fmt.Errorf("dingtalk does not support direct reply, use SendMessage instead")
}

// UpdateCard 更新卡片
func (c *DingTalkClient) UpdateCard(ctx context.Context, cardID string, data map[string]interface{}) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	return c.updateCard(ctx, token, cardID, data)
}

// getAccessToken 获取访问令牌
func (c *DingTalkClient) getAccessToken(ctx context.Context) (string, error) {
	return c.tokenCache.GetOrFetch(ctx, func(ctx context.Context) (string, int, error) {
		reqURL := fmt.Sprintf("%s/gettoken?appkey=%s&appsecret=%s",
			constants.DingTalkOAPIBase, url.QueryEscape(c.appKey), url.QueryEscape(c.appSecret))

		httpReq, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return "", 0, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return "", 0, fmt.Errorf("failed to request access token: %w", err)
		}
		defer resp.Body.Close()

		var result struct {
			Errcode     int    `json:"errcode"`
			Errmsg      string `json:"errmsg"`
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", 0, fmt.Errorf("failed to parse response: %w", err)
		}

		if result.Errcode != 0 {
			return "", 0, fmt.Errorf("failed to get access token: %s", result.Errmsg)
		}

		return result.AccessToken, result.ExpiresIn, nil
	})
}

// sendMessage 发送文本消息
func (c *DingTalkClient) sendMessage(ctx context.Context, token, target, message string) error {
	reqBody := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": message,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/v1.0/robot/groupMessages/send?access_token=%s",
		constants.DingTalkAPIBase, token)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	return c.checkResponse(resp)
}

// updateCard 更新卡片
func (c *DingTalkClient) updateCard(ctx context.Context, token, cardID string, data map[string]interface{}) error {
	reqBody := map[string]interface{}{
		"cardId": cardID,
		"data":   data,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/v1.0/robot/cards/modify?access_token=%s",
		constants.DingTalkAPIBase, token)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to update card: %w", err)
	}
	defer resp.Body.Close()

	return c.checkResponse(resp)
}

// UploadImage 上传图片
func (c *DingTalkClient) UploadImage(ctx context.Context, imagePath string) (string, error) {
	return c.uploadMultipartFile(ctx, "image", imagePath, filepath.Base(imagePath))
}

// UploadFile 上传文件
func (c *DingTalkClient) UploadFile(ctx context.Context, filePath, fileName string) (string, error) {
	return c.uploadMultipartFile(ctx, "file", filePath, fileName)
}

// uploadMultipartFile 上传文件（通用方法）
func (c *DingTalkClient) uploadMultipartFile(ctx context.Context, fileType, filePath, fileName string) (string, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// 创建 multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// 添加文件字段
	part, err := writer.CreateFormFile("media", fileName)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	// 构建请求 URL
	reqURL := fmt.Sprintf("%s/media/upload?access_token=%s&type=%s",
		constants.DingTalkOAPIBase, token, fileType)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, &body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
		MediaId string `json:"media_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Errcode != 0 {
		return "", fmt.Errorf("failed to upload file: %s", result.Errmsg)
	}

	return result.MediaId, nil
}

// SendImageMessage 发送图片消息
func (c *DingTalkClient) SendImageMessage(ctx context.Context, target, imageKey string) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	return c.sendMediaMessage(ctx, token, target, "image", map[string]interface{}{"media_id": imageKey})
}

// SendFileMessage 发送文件消息
func (c *DingTalkClient) SendFileMessage(ctx context.Context, target, fileKey, fileName string) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	return c.sendMediaMessage(ctx, token, target, "file", map[string]interface{}{
		"media_id":  fileKey,
		"file_name": fileName,
	})
}

// sendMediaMessage 发送媒体消息（通用方法）
func (c *DingTalkClient) sendMediaMessage(ctx context.Context, token, target, msgType string, content map[string]interface{}) error {
	reqBody := map[string]interface{}{
		"msgkey":  target,
		"msgtype": msgType,
		msgType:   content,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	reqURL := fmt.Sprintf("%s/v1.0/robot/groupMessages/send?access_token=%s",
		constants.DingTalkAPIBase, token)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send media message: %w", err)
	}
	defer resp.Body.Close()

	return c.checkResponse(resp)
}

// checkResponse 检查响应
func (c *DingTalkClient) checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil // 无法解析，假设成功
	}

	if result.Errcode != 0 {
		return fmt.Errorf("API error: %s", result.Errmsg)
	}

	return nil
}
