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
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFeishuClient_SendImageMessage 测试飞书发送图片消息
// 使用环境变量配置敏感信息:
//   - FEISHU_APP_ID: 飞书应用 ID
//   - FEISHU_APP_SECRET: 飞书应用 Secret
//   - FEISHU_CHAT_ID: 目标聊天 ID (格式: oc_xxx)
//
// 运行测试:
//   export FEISHU_APP_ID="cli_xxx"
//   export FEISHU_APP_SECRET="xxx"
//   export FEISHU_CHAT_ID="oc_xxx"
//   go test -v -run TestFeishuClient_SendImageMessage -timeout 60s
func TestFeishuClient_SendImageMessage(t *testing.T) {
	// 从环境变量获取配置
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	chatID := os.Getenv("FEISHU_CHAT_ID")

	if appID == "" || appSecret == "" || chatID == "" {
		t.Skip("Skipping test: FEISHU_APP_ID, FEISHU_APP_SECRET, and FEISHU_CHAT_ID must be set")
	}

	// 创建客户端
	client := NewFeishuClient(&FeishuConfig{
		AppID:     appID,
		AppSecret: appSecret,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 获取测试图片路径
	testDataPath := filepath.Join("..", "testdata", "logo.png")
	absPath, err := filepath.Abs(testDataPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Fatalf("Test image not found: %s", absPath)
	}

	t.Logf("Uploading image from: %s", absPath)

	// 步骤1: 上传图片
	imageKey, err := client.UploadImage(ctx, absPath)
	if err != nil {
		t.Fatalf("Failed to upload image: %v", err)
	}
	t.Logf("Image uploaded successfully, imageKey: %s", imageKey)

	// 步骤2: 发送图片消息
	err = client.SendImageMessage(ctx, chatID, imageKey)
	if err != nil {
		t.Fatalf("Failed to send image message: %v", err)
	}
	t.Logf("Image message sent successfully to chat: %s", chatID)
}

// TestFeishuClient_SendTextMessage 测试飞书发送文本消息
// 用于验证基本连接和权限是否正常
func TestFeishuClient_SendTextMessage(t *testing.T) {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	chatID := os.Getenv("FEISHU_CHAT_ID")

	if appID == "" || appSecret == "" || chatID == "" {
		t.Skip("Skipping test: FEISHU_APP_ID, FEISHU_APP_SECRET, and FEISHU_CHAT_ID must be set")
	}

	client := NewFeishuClient(&FeishuConfig{
		AppID:     appID,
		AppSecret: appSecret,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 发送文本消息
	err := client.SendMessage(ctx, chatID, "这是一条测试消息")
	if err != nil {
		t.Fatalf("Failed to send text message: %v", err)
	}
	t.Logf("Text message sent successfully to chat: %s", chatID)
}

// TestFeishuClient_UploadImage 测试飞书上传图片
// 单独测试上传功能
func TestFeishuClient_UploadImage(t *testing.T) {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")

	if appID == "" || appSecret == "" {
		t.Skip("Skipping test: FEISHU_APP_ID and FEISHU_APP_SECRET must be set")
	}

	client := NewFeishuClient(&FeishuConfig{
		AppID:     appID,
		AppSecret: appSecret,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取测试图片路径
	testDataPath := filepath.Join("..", "testdata", "logo.png")
	absPath, err := filepath.Abs(testDataPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// 上传图片
	imageKey, err := client.UploadImage(ctx, absPath)
	if err != nil {
		t.Fatalf("Failed to upload image: %v", err)
	}

	if imageKey == "" {
		t.Fatal("Expected non-empty imageKey")
	}

	t.Logf("Image uploaded successfully, imageKey: %s", imageKey)
}

// TestFeishuClient_UploadAndSendImage 测试上传并发送图片（完整流程）
func TestFeishuClient_UploadAndSendImage(t *testing.T) {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	chatID := os.Getenv("FEISHU_CHAT_ID")

	if appID == "" || appSecret == "" || chatID == "" {
		t.Skip("Skipping test: FEISHU_APP_ID, FEISHU_APP_SECRET, and FEISHU_CHAT_ID must be set")
	}

	client := NewFeishuClient(&FeishuConfig{
		AppID:     appID,
		AppSecret: appSecret,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 获取测试图片路径
	testDataPath := filepath.Join("..", "testdata", "logo.png")
	absPath, err := filepath.Abs(testDataPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// 使用便捷方法上传并发送
	// 注意: 这里模拟 IMServiceImpl.UploadAndSendImage 的逻辑
	imageKey, err := client.UploadImage(ctx, absPath)
	if err != nil {
		t.Fatalf("Failed to upload image: %v", err)
	}
	t.Logf("Image uploaded, imageKey: %s", imageKey)

	err = client.SendImageMessage(ctx, chatID, imageKey)
	if err != nil {
		t.Fatalf("Failed to send image message: %v", err)
	}

	t.Logf("Image sent successfully to chat: %s", chatID)
}

// TestFeishuClient_SendMessageWithReceiveIdType 测试不同类型的接收者 ID
func TestFeishuClient_SendMessageWithReceiveIdType(t *testing.T) {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	chatID := os.Getenv("FEISHU_CHAT_ID")

	if appID == "" || appSecret == "" || chatID == "" {
		t.Skip("Skipping test: FEISHU_APP_ID, FEISHU_APP_SECRET, and FEISHU_CHAT_ID must be set")
	}

	client := NewFeishuClient(&FeishuConfig{
		AppID:     appID,
		AppSecret: appSecret,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 测试使用 chat_id 类型发送消息
	err := client.SendMessageWithReceiveIdType(ctx, "chat_id", chatID, "测试 chat_id 类型")
	if err != nil {
		t.Fatalf("Failed to send message with chat_id type: %v", err)
	}
	t.Logf("Message sent successfully with chat_id type")
}
