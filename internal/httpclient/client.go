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

// Package httpclient 提供共享的 HTTP 客户端
package httpclient

import (
	"github.com/teambuf/tpclaw-components-im/internal/constants"
	"net/http"
	"sync"
)

var (
	defaultClient *http.Client
	uploadClient  *http.Client
	clientOnce    sync.Once
)

// GetDefaultClient 返回默认 HTTP 客户端（30s 超时)
func GetDefaultClient() *http.Client {
	clientOnce.Do(func() {
		if defaultClient == nil {
			defaultClient = &http.Client{
				Timeout: constants.DefaultTimeout,
			}
		}
	})
	return defaultClient
}

// GetUploadClient 返回上传文件专用 HTTP 客户端（60s 超时)
func GetUploadClient() *http.Client {
	if uploadClient == nil {
		uploadClient = &http.Client{
			Timeout: constants.UploadTimeout,
		}
	}
	return uploadClient
}

// DefaultClient 返回默认 HTTP 客户端（30s 超时)
// Deprecated: Use GetDefaultClient instead
func DefaultClient() *http.Client {
	return GetDefaultClient()
}

// UploadClient 返回上传文件专用 HTTP 客户端
// Deprecated: Use GetUploadClient instead
func UploadClient() *http.Client {
	return GetUploadClient()
}
