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

package wecom

import (
	"context"
	"encoding/json"
	"fmt"
	imapi "github.com/teambuf/tpclaw-components-im/api"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// 企业微信 API 地址
	weComAPIBase = "https://qyapi.weixin.qq.com/cgi-bin"
	// accessToken 缓存提前刷新时间（5分钟）
	weComTokenRefreshAhead = 5 * time.Minute
)

// MediaClient 企业微信媒体文件客户端
type MediaClient struct {
	corpID     string
	agentID    int
	secret     string
	httpClient *http.Client

	// accessToken 缓存
	accessToken    string
	tokenExpiresAt time.Time
	tokenMu        sync.RWMutex
}

// NewMediaClient 创建企业微信媒体客户端
func NewMediaClient(corpID string, agentID int, secret string) *MediaClient {
	return &MediaClient{
		corpID:  corpID,
		agentID: agentID,
		secret:  secret,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DownloadMedia 实现 MediaDownloader 接口，下载媒体文件
func (c *MediaClient) DownloadMedia(ctx context.Context, mediaID string) ([]byte, string, error) {
	// 获取 access_token
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("get access token failed: %w", err)
	}

	// 构建下载 URL
	// 企业微信下载媒体文件: https://qyapi.weixin.qq.com/cgi-bin/media/get?access_token=ACCESS_TOKEN&media_id=MEDIA_ID
	url := fmt.Sprintf("%s/media/get?access_token=%s&media_id=%s", weComAPIBase, token, mediaID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download media failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("download media failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 获取 Content-Type
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// 检查是否返回了错误 JSON（企业微信有时会返回 JSON 错误而不是 HTTP 错误）
	if mimeType == "application/json" || mimeType == "text/json" {
		body, _ := io.ReadAll(resp.Body)
		var errResp WeComErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Errcode != 0 {
			return nil, "", fmt.Errorf("download media failed: errcode=%d, errmsg=%s", errResp.Errcode, errResp.Errmsg)
		}
		// 不是错误响应，返回原始数据
		return body, mimeType, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response body failed: %w", err)
	}

	return data, mimeType, nil
}

// DownloadImage 实现 MediaDownloader 接口，下载图片
// 企业微信的图片也是通过 media_id 下载
func (c *MediaClient) DownloadImage(ctx context.Context, imageKey string) ([]byte, string, error) {
	return c.DownloadMedia(ctx, imageKey)
}

// DownloadImageFromURL 从图片 URL 下载图片（企业微信图片消息可能包含 PicUrl）
func (c *MediaClient) DownloadImageFromURL(ctx context.Context, picURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, picURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download image failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("download image failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg" // 默认假设为 JPEG
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response body failed: %w", err)
	}

	return data, mimeType, nil
}

// getAccessToken 获取 access_token（带缓存）
func (c *MediaClient) getAccessToken(ctx context.Context) (string, error) {
	// 检查缓存
	c.tokenMu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpiresAt.Add(-weComTokenRefreshAhead)) {
		token := c.accessToken
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	// 需要刷新 token
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// 双重检查
	if c.accessToken != "" && time.Now().Before(c.tokenExpiresAt.Add(-weComTokenRefreshAhead)) {
		return c.accessToken, nil
	}

	// 获取新的 access_token
	// 企业微信获取 access_token: https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=ID&corpsecret=SECRET
	url := fmt.Sprintf("%s/gettoken?corpid=%s&corpsecret=%s", weComAPIBase, c.corpID, c.secret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get token failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body failed: %w", err)
	}

	var tokenResp WeComTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parse token response failed: %w", err)
	}

	if tokenResp.Errcode != 0 {
		return "", fmt.Errorf("get token failed: errcode=%d, errmsg=%s", tokenResp.Errcode, tokenResp.Errmsg)
	}

	// 缓存 token
	c.accessToken = tokenResp.AccessToken
	c.tokenExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return c.accessToken, nil
}

// Ensure MediaClient implements MediaDownloader interface
var _ imapi.MediaDownloader = (*MediaClient)(nil)
