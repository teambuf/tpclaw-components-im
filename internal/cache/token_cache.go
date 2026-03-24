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
 * WITHOUT WARRANTIES OR conditions of any kind, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package cache 提供通用的 Token 缓存工具
package cache

import (
	"context"
	"sync"
	"time"
)

// TokenCache 通用 Token 缓存结构
type TokenCache struct {
	token    string
	expireAt time.Time
	mu       sync.RWMutex
}

// NewTokenCache 创建新的 Token 缓存
func NewTokenCache() *TokenCache {
	return &TokenCache{token: ""}
}

// GetOrFetch 获取或刷新 Token
// 如果缓存有效且未过期，返回缓存的 Token
// 否则需要调用 fetchFunc 获取新 Token
func (c *TokenCache) GetOrFetch(ctx context.Context, fetchFunc func(ctx context.Context) (token string, expiresIn int, err error)) (string, error) {
	// 先读后检查
	c.mu.RLock()
	if c.token != "" && time.Now().Before(c.expireAt) {
		token := c.token
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	// 飘要获取新 Token
	c.mu.Lock()
	defer c.mu.Unlock()

	// 再次检查（双重检查锁定）
	if c.token != "" && time.Now().Before(c.expireAt) {
		return c.token, nil
	}

	// 调用 fetchFunc 获取新 Token
	token, expiresIn, err := fetchFunc(ctx)
	if err != nil {
		c.token = ""
		return "", err
	}

	// 缓存 token，设置过期时间
	c.token = token
	c.expireAt = time.Now().Add(time.Duration(expiresIn) * time.Second)

	return token, nil
}
