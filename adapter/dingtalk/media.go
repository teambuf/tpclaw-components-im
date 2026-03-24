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

package dingtalk

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
	// 钉钉 API 地址
	dingTalkAPIBase = "https://oapi.dingtalk.com"
	// accessToken 缓存提前刷新时间（5分钟）
	accessTokenRefreshAhead = 5 * time.Minute
)

// MediaClient 钉钉媒体文件客户端
type MediaClient struct {
	appKey     string
	appSecret  string
	httpClient *http.Client

	// accessToken 缓存
	accessToken    string
	tokenExpiresAt time.Time
	tokenMu        sync.RWMutex
}

// NewMediaClient 创建钉钉媒体客户端
func NewMediaClient(appKey, appSecret string) *MediaClient {
	return &MediaClient{
		appKey:    appKey,
		appSecret: appSecret,
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
	// 钉钉机器人下载媒体文件: https://oapi.dingtalk.com/chat/download?access_token=ACCESS_TOKEN&media_id=MEDIA_ID
	url := fmt.Sprintf("%s/chat/download?access_token=%s&media_id=%s", dingTalkAPIBase, token, mediaID)

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

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response body failed: %w", err)
	}

	return data, mimeType, nil
}

// DownloadImage 实现 MediaDownloader 接口，下载图片
// 钉钉的图片也是通过 media_id 下载
func (c *MediaClient) DownloadImage(ctx context.Context, imageKey string) ([]byte, string, error) {
	return c.DownloadMedia(ctx, imageKey)
}

// DownloadMediaFromWebhook 通过 sessionWebhook 下载媒体文件
// 钉钉机器人消息中会包含 sessionWebhook，可用于下载媒体
func (c *MediaClient) DownloadMediaFromWebhook(ctx context.Context, sessionWebhook, mediaID string) ([]byte, string, error) {
	if sessionWebhook == "" {
		return c.DownloadMedia(ctx, mediaID)
	}

	// 使用 sessionWebhook 下载
	url := fmt.Sprintf("%s?media_id=%s", sessionWebhook, mediaID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download media from webhook failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("download media failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
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
	if c.accessToken != "" && time.Now().Before(c.tokenExpiresAt.Add(-accessTokenRefreshAhead)) {
		token := c.accessToken
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	// 需要刷新 token
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// 双重检查
	if c.accessToken != "" && time.Now().Before(c.tokenExpiresAt.Add(-accessTokenRefreshAhead)) {
		return c.accessToken, nil
	}

	// 获取新的 access_token
	url := fmt.Sprintf("%s/gettoken?appkey=%s&appsecret=%s", dingTalkAPIBase, c.appKey, c.appSecret)

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

	var tokenResp DingTalkTokenResponse
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
