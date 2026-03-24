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

package feishu

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/teambuf/tpclaw-components-im/api"
)

func TestFeishuAdapter_Platform(t *testing.T) {
	adapter := NewAdapter(&Config{})
	if adapter.Platform() != api.PlatformFeishu {
		t.Errorf("Expected platform %s, got %s", api.PlatformFeishu, adapter.Platform())
	}
}

func TestFeishuAdapter_HandleChallenge(t *testing.T) {
	adapter := NewAdapter(&Config{
		VerificationToken: "YOUR_VERIFICATION_TOKEN_HERE",
	})

	// Test valid challenge request
	challengeReq := ChallengeRequest{
		Challenge: "test_challenge_code",
		Token:     "test_token",
		Type:      "url_verification",
	}
	body, _ := json.Marshal(challengeReq)

	response, handled, err := adapter.HandleChallenge(body)
	if err != nil {
		t.Errorf("HandleChallenge failed: %v", err)
	}
	if !handled {
		t.Error("Expected challenge to be handled")
	}

	var resp map[string]string
	json.Unmarshal(response, &resp)
	if resp["challenge"] != "test_challenge_code" {
		t.Errorf("Expected challenge test_challenge_code, got %s", resp["challenge"])
	}

	// Test invalid token
	challengeReq.Token = "invalid_token"
	body, _ = json.Marshal(challengeReq)
	_, _, err = adapter.HandleChallenge(body)
	if err == nil {
		t.Error("Expected error for invalid token")
	}

	// Test non-challenge request
	nonChallengeBody := []byte(`{"type":"other"}`)
	_, handled, _ = adapter.HandleChallenge(nonChallengeBody)
	if handled {
		t.Error("Expected non-challenge request not to be handled")
	}
}

func TestFeishuAdapter_ParseMessage(t *testing.T) {
	adapter := NewAdapter(&Config{})

	// Create message event
	event := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{
			"event_id":    "event_123",
			"event_type":  "im.message.receive_v1",
			"create_time": "2024-01-01T00:00:00Z",
			"app_id":      "cli_test",
			"tenant_key":  "tenant_123",
		},
		"event": map[string]interface{}{
			"sender": map[string]interface{}{
				"sender_id": map[string]interface{}{
					"user_id":  "user_123",
					"open_id":  "ou_123",
					"union_id": "union_123",
				},
				"sender_type": "user",
			},
			"message": map[string]interface{}{
				"message_id":   "msg_123",
				"chat_id":      "oc_123",
				"chat_type":    "group",
				"message_type": "text",
				"content":      `{"text":"Hello World"}`,
			},
		},
	}
	body, _ := json.Marshal(event)

	msg, err := adapter.ParseMessage(context.Background(), body, nil, nil)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	// Verify message fields
	if msg.Platform != api.PlatformFeishu {
		t.Errorf("Expected platform %s, got %s", api.PlatformFeishu, msg.Platform)
	}
	if msg.ID != "msg_123" {
		t.Errorf("Expected ID msg_123, got %s", msg.ID)
	}
	if msg.ChatID != "oc_123" {
		t.Errorf("Expected ChatID oc_123, got %s", msg.ChatID)
	}
	if msg.Content != "Hello World" {
		t.Errorf("Expected Content 'Hello World', got %s", msg.Content)
	}
	if msg.Sender == nil {
		t.Fatal("Expected Sender to be set")
	}
	if msg.Sender.UserID != "user_123" {
		t.Errorf("Expected Sender.UserID user_123, got %s", msg.Sender.UserID)
	}
	if msg.Sender.OpenID != "ou_123" {
		t.Errorf("Expected Sender.OpenID ou_123, got %s", msg.Sender.OpenID)
	}

	// Verify extensions
	if msg.Extensions == nil {
		t.Fatal("Expected Extensions to be set")
	}
	if v, ok := msg.Extensions[api.MetaFeishuAppID].(string); !ok || v != "cli_test" {
		t.Errorf("Expected Extensions[appId] cli_test, got %v", msg.Extensions[api.MetaFeishuAppID])
	}
}

func TestFeishuAdapter_ParseMessage_OtherEvent(t *testing.T) {
	adapter := NewAdapter(&Config{})

	// Create non-message event
	event := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{
			"event_id":   "event_456",
			"event_type": "contact.user.created_v3",
			"app_id":     "cli_test",
		},
		"event": map[string]interface{}{
			"user_id": "user_456",
		},
	}
	body, _ := json.Marshal(event)

	msg, err := adapter.ParseMessage(context.Background(), body, nil, nil)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	if msg.MsgType != "contact.user.created_v3" {
		t.Errorf("Expected MsgType contact.user.created_v3, got %s", msg.MsgType)
	}
}

func TestFeishuAdapter_FormatResponse(t *testing.T) {
	adapter := NewAdapter(&Config{})

	msg := &api.IMMessage{
		Content: "test response",
	}

	// Test JSON response
	response, err := adapter.FormatResponse(msg, api.ResponseTypeJSON)
	if err != nil {
		t.Fatalf("FormatResponse failed: %v", err)
	}

	var resp map[string]interface{}
	json.Unmarshal(response, &resp)
	if resp["code"].(float64) != 0 {
		t.Errorf("Expected code 0, got %v", resp["code"])
	}

	// Test plain text response
	response, err = adapter.FormatResponse(msg, api.ResponseTypePlainText)
	if err != nil {
		t.Fatalf("FormatResponse failed: %v", err)
	}
	if string(response) != "success" {
		t.Errorf("Expected 'success', got %s", string(response))
	}
}

func TestFeishuAdapter_VerifySignature(t *testing.T) {
	adapter := NewAdapter(&Config{
		EncryptKey: "YOUR_ENCRYPT_KEY_32_CHARACTERS_LONG",
	})

	// Test without signature headers - should pass
	err := adapter.VerifySignature([]byte("test body"), nil, nil)
	if err != nil {
		t.Errorf("Expected no error without signature, got %v", err)
	}

	// Test with empty headers
	headers := map[string]string{}
	err = adapter.VerifySignature([]byte("test body"), headers, nil)
	if err != nil {
		t.Errorf("Expected no error with empty headers, got %v", err)
	}
}

func TestFeishuAdapter_CreateProcessor(t *testing.T) {
	adapter := NewAdapter(&Config{})

	// Test transform processor
	processor, err := adapter.CreateProcessor(api.ProcessorTypeTransform, nil)
	if err != nil {
		t.Fatalf("CreateProcessor failed: %v", err)
	}
	if processor == nil {
		t.Error("Expected processor to be created")
	}

	// Test success response processor
	processor, err = adapter.CreateProcessor(api.ProcessorTypeSuccessResponse, nil)
	if err != nil {
		t.Fatalf("CreateProcessor failed: %v", err)
	}
	if processor == nil {
		t.Error("Expected processor to be created")
	}
}

func TestParseContent(t *testing.T) {
	adapter := &Adapter{}

	// Test text content
	textContent := `{"text":"Hello World"}`
	result := adapter.parseContent("text", textContent)
	if result != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", result)
	}

	// Test post content
	postContent := `{"title":"Test Title"}`
	result = adapter.parseContent("post", postContent)
	if result != "Test Title" {
		t.Errorf("Expected 'Test Title', got '%s'", result)
	}

	// Test unknown type - return as is
	result = adapter.parseContent("unknown", "raw content")
	if result != "raw content" {
		t.Errorf("Expected 'raw content', got '%s'", result)
	}
}

func TestGetHeader(t *testing.T) {
	headers := map[string]string{
		"X-Test-Header": "test_value",
	}

	result := getHeader(headers, "X-Test-Header")
	if result != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", result)
	}

	result = getHeader(headers, "X-Non-Existing")
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}
