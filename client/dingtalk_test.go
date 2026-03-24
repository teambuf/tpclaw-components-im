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
	"errors"
	"os"
	"testing"

	imapi "github.com/teambuf/tpclaw-components-im/api"
)

// Test configuration loaded from environment variables
// Set these environment variables for integration testing:
// DINGTALK_APP_KEY, DINGTALK_APP_SECRET
func getTestDingTalkConfig() *DingTalkConfig {
	return &DingTalkConfig{
		AppKey:    getEnvOrDefault("DINGTALK_APP_KEY", "YOUR_APP_KEY_HERE"),
		AppSecret: getEnvOrDefault("DINGTALK_APP_SECRET", "YOUR_APP_SECRET_HERE"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestDingTalkClient_New 测试创建钉钉客户端
func TestDingTalkClient_New(t *testing.T) {
	config := getTestDingTalkConfig()

	client := NewDingTalkClient(config)
	if client == nil {
		t.Fatal("expected client to be created")
	}
	if client.appKey != "test_app_key" {
		t.Errorf("expected appKey to be test_app_key, got %s", client.appKey)
	}
	if client.appSecret != "test_app_secret" {
		t.Errorf("expected appSecret to be test_app_secret, got %s", client.appSecret)
	}
}

// TestDingTalkClient_Platform 测试平台标识
func TestDingTalkClient_Platform(t *testing.T) {
	config := getTestDingTalkConfig()
	client := NewDingTalkClient(config)

	if client.Platform() != imapi.PlatformDingTalk {
		t.Errorf("expected platform to be %s, got %s", imapi.PlatformDingTalk, client.Platform())
	}
}

// TestDingTalkClient_ReplyMessage 测试回复消息（应返回错误）
func TestDingTalkClient_ReplyMessage(t *testing.T) {
	config := getTestDingTalkConfig()
	client := NewDingTalkClient(config)

	err := client.ReplyMessage(context.Background(), "msg_id", "test message")
	if err == nil {
		t.Error("expected error for ReplyMessage, got nil")
	}
	if !errors.Is(err, errors.New("dingtalk does not support direct reply, use SendMessage instead")) {
		// 检查错误消息包含预期内容
		if err.Error() != "dingtalk does not support direct reply, use SendMessage instead" {
			t.Errorf("unexpected error message: %v", err)
		}
	}
}

// TestDingTalkClient_UploadImage_InvalidPath 测试上传图片无效路径
func TestDingTalkClient_UploadImage_InvalidPath(t *testing.T) {
	config := getTestDingTalkConfig()
	client := NewDingTalkClient(config)

	_, err := client.UploadImage(context.Background(), "/nonexistent/path/image.jpg")
	if err == nil {
		t.Error("expected error for invalid file path, got nil")
	}
}

// TestDingTalkClient_UploadFile_InvalidPath 测试上传文件无效路径
func TestDingTalkClient_UploadFile_InvalidPath(t *testing.T) {
	config := getTestDingTalkConfig()
	client := NewDingTalkClient(config)

	_, err := client.UploadFile(context.Background(), "/nonexistent/path/file.txt", "test.txt")
	if err == nil {
		t.Error("expected error for invalid file path, got nil")
	}
}
