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
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WeComClient 企业微信客户端
type WeComClient struct {
	corpID  string
	secret  string
	agentID int

	// Token 缓存
	tokenCache     string
	tokenExpireAt  time.Time
	tokenCacheLock sync.RWMutex
}

// WeComConfig 企业微信客户端配置
type WeComConfig struct {
	CorpID  string
	Secret  string
	AgentID int
}

// NewWeComClient 创建企业微信客户端
func NewWeComClient(config *WeComConfig) *WeComClient {
	return &WeComClient{
		corpID:  config.CorpID,
		secret:  config.Secret,
		agentID: config.AgentID,
	}
}

// Platform 返回平台标识
func (c *WeComClient) Platform() string {
	return imapi.PlatformWeCom
}

// SendMessage 发送消息
func (c *WeComClient) SendMessage(ctx context.Context, target, message string) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	reqURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	req := weComMessageRequest{
		Touser:  target,
		Msgtype: "text",
		Agentid: c.agentID,
	}
	req.Text.Content = message

	jsonReq, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(jsonReq))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	var result weComResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Errcode != 0 {
		return fmt.Errorf("wecom API error: %s", result.Errmsg)
	}

	return nil
}

// ReplyMessage 回复消息（企业微信不支持直接回复）
func (c *WeComClient) ReplyMessage(ctx context.Context, msgID, message string) error {
	return fmt.Errorf("wecom does not support direct reply, use SendMessage instead")
}

// UpdateCard 更新卡片
func (c *WeComClient) UpdateCard(ctx context.Context, cardID string, data map[string]interface{}) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	reqURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/update_taskcard?access_token=%s", token)

	req := map[string]interface{}{
		"task_id": cardID,
		"card":    data,
		"agentid": c.agentID,
	}

	jsonReq, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(jsonReq))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to update card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update card, status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// getAccessToken 获取访问令牌（带缓存）
func (c *WeComClient) getAccessToken(ctx context.Context) (string, error) {
	// 先检查缓存
	c.tokenCacheLock.RLock()
	if c.tokenCache != "" && time.Now().Before(c.tokenExpireAt) {
		token := c.tokenCache
		c.tokenCacheLock.RUnlock()
		return token, nil
	}
	c.tokenCacheLock.RUnlock()

	// 需要重新获取 token
	c.tokenCacheLock.Lock()
	defer c.tokenCacheLock.Unlock()

	// 双重检查
	if c.tokenCache != "" && time.Now().Before(c.tokenExpireAt) {
		return c.tokenCache, nil
	}

	reqURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		c.corpID, c.secret)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to request access token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Errcode     int    `json:"errcode"`
		Errmsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Errcode != 0 {
		return "", fmt.Errorf("failed to get access token: %s", result.Errmsg)
	}

	// 缓存 token，提前 5 分钟过期
	c.tokenCache = result.AccessToken
	c.tokenExpireAt = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	return result.AccessToken, nil
}

// weComMessageRequest 企业微信消息请求
type weComMessageRequest struct {
	Touser  string `json:"touser"`
	Msgtype string `json:"msgtype"`
	Agentid int    `json:"agentid"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

// weComResponse 企业微信 API 响应
type weComResponse struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
}

// UploadImage 上传图片
func (c *WeComClient) UploadImage(ctx context.Context, imagePath string) (string, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// 读取文件
	fileData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}

	// 构建 multipart 请求
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件
	part, err := writer.CreateFormFile("media", filepath.Base(imagePath))
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	_, err = part.Write(fileData)
	if err != nil {
		return "", fmt.Errorf("failed to write file data: %w", err)
	}
	writer.Close()

	// 发送请求
	reqURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=image", token)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
		MediaID string `json:"media_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Errcode != 0 {
		return "", fmt.Errorf("wecom API error: %s", result.Errmsg)
	}

	return result.MediaID, nil
}

// UploadFile 上传文件
func (c *WeComClient) UploadFile(ctx context.Context, filePath, fileName string) (string, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// 读取文件
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// 构建 multipart 请求
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件
	part, err := writer.CreateFormFile("media", fileName)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}
	_, err = part.Write(fileData)
	if err != nil {
		return "", fmt.Errorf("failed to write file data: %w", err)
	}
	writer.Close()

	// 发送请求
	reqURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=file", token)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
		MediaID string `json:"media_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Errcode != 0 {
		return "", fmt.Errorf("wecom API error: %s", result.Errmsg)
	}

	return result.MediaID, nil
}

// SendImageMessage 发送图片消息
func (c *WeComClient) SendImageMessage(ctx context.Context, target, imageKey string) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	reqURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	req := weComImageRequest{
		Touser:  target,
		Msgtype: "image",
		Agentid: c.agentID,
	}
	req.Image.MediaID = imageKey

	jsonReq, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(jsonReq))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send image message: %w", err)
	}
	defer resp.Body.Close()

	var result weComResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Errcode != 0 {
		return fmt.Errorf("wecom API error: %s", result.Errmsg)
	}

	return nil
}

// SendFileMessage 发送文件消息
func (c *WeComClient) SendFileMessage(ctx context.Context, target, fileKey, fileName string) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	reqURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	req := weComFileRequest{
		Touser:  target,
		Msgtype: "file",
		Agentid: c.agentID,
	}
	req.File.MediaID = fileKey

	jsonReq, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(jsonReq))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send file message: %w", err)
	}
	defer resp.Body.Close()

	var result weComResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Errcode != 0 {
		return fmt.Errorf("wecom API error: %s", result.Errmsg)
	}

	return nil
}

// weComImageRequest 企业微信图片消息请求
type weComImageRequest struct {
	Touser  string `json:"touser"`
	Msgtype string `json:"msgtype"`
	Agentid int    `json:"agentid"`
	Image   struct {
		MediaID string `json:"media_id"`
	} `json:"image"`
}

// weComFileRequest 企业微信文件消息请求
type weComFileRequest struct {
	Touser  string `json:"touser"`
	Msgtype string `json:"msgtype"`
	Agentid int    `json:"agentid"`
	File    struct {
		MediaID string `json:"media_id"`
	} `json:"file"`
}
