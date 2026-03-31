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
	"strconv"
	"testing"
	"time"
)

// TestWeComClient_SendTextMessage 测试企业微信发送文本消息
// 使用环境变量配置敏感信息:
//   - WECOM_CORP_ID: 企业微信 CorpID
//   - WECOM_SECRET: 企业微信 Secret
//   - WECOM_AGENT_ID: 企业微信应用 AgentID
//   - WECOM_USER_ID: 目标用户 ID
//
// 运行测试:
//   go test -v -run TestWeComClient_SendTextMessage -timeout 30s
func TestWeComClient_SendTextMessage(t *testing.T) {
	corpID := os.Getenv("WECOM_CORP_ID")
	secret := os.Getenv("WECOM_SECRET")
	agentIDStr := os.Getenv("WECOM_AGENT_ID")
	userID := os.Getenv("WECOM_USER_ID")

	if corpID == "" || secret == "" || agentIDStr == "" || userID == "" {
		t.Skip("Skipping test: WECOM_CORP_ID, WECOM_SECRET, WECOM_AGENT_ID, and WECOM_USER_ID must be set")
	}

	agentID, err := strconv.Atoi(agentIDStr)
	if err != nil {
		t.Fatalf("Invalid WECOM_AGENT_ID: %v", err)
	}

	client := NewWeComClient(&WeComConfig{
		CorpID:  corpID,
		Secret:  secret,
		AgentID: agentID,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 发送文本消息
	err = client.SendMessage(ctx, userID, "这是一条来自测试的文本消息，包含 Markdown:\n> 引用测试\n**加粗测试**")
	if err != nil {
		t.Fatalf("Failed to send text message: %v", err)
	}
	t.Logf("Text message sent successfully to user: %s", userID)
}

// TestWeComClient_SendImageMessage 测试企业微信上传并发送图片消息
func TestWeComClient_SendImageMessage(t *testing.T) {
	corpID := os.Getenv("WECOM_CORP_ID")
	secret := os.Getenv("WECOM_SECRET")
	agentIDStr := os.Getenv("WECOM_AGENT_ID")
	userID := os.Getenv("WECOM_USER_ID")

	if corpID == "" || secret == "" || agentIDStr == "" || userID == "" {
		t.Skip("Skipping test: WECOM_CORP_ID, WECOM_SECRET, WECOM_AGENT_ID, and WECOM_USER_ID must be set")
	}

	agentID, err := strconv.Atoi(agentIDStr)
	if err != nil {
		t.Fatalf("Invalid WECOM_AGENT_ID: %v", err)
	}

	client := NewWeComClient(&WeComConfig{
		CorpID:  corpID,
		Secret:  secret,
		AgentID: agentID,
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
	err = client.SendImageMessage(ctx, userID, imageKey)
	if err != nil {
		t.Fatalf("Failed to send image message: %v", err)
	}
	t.Logf("Image message sent successfully to user: %s", userID)
}
