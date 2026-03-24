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
	"github.com/teambuf/tpclaw-components-im/api"
	"testing"
)

func TestDingTalkAdapter_Platform(t *testing.T) {
	adapter := NewDingTalkAdapter(&DingTalkConfig{})
	if adapter.Platform() != api.PlatformDingTalk {
		t.Errorf("Expected platform %s, got %s", api.PlatformDingTalk, adapter.Platform())
	}
}

func TestDingTalkAdapter_ParseMessage(t *testing.T) {
	adapter := NewDingTalkAdapter(&DingTalkConfig{})

	// Create DingTalk event
	event := DingTalkEvent{
		ChatType:         "2",
		ConversationID:   "conv_123",
		ConversationType: "1",
		MsgId:            "msg_123",
		SenderNick:       "Test User",
		IsAdmin:          false,
		SenderStaffId:    "staff_123",
		SenderId:         "user_123",
		SenderCorpId:     "corp_123",
		SessionWebhook:   "https://webhook.example.com",
		MsgType:          "text",
		CreateTime:       1704067200000,
	}
	event.Content.ContentType = "text"
	event.Content.Content = "Hello World"
	event.AtUsers = []DingTalkAtUser{
		{DingTalkId: "dt_123", StaffId: "staff_456"},
	}

	body, _ := json.Marshal(event)

	msg, err := adapter.ParseMessage(context.Background(), body, nil, nil)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	// Verify message fields
	if msg.Platform != api.PlatformDingTalk {
		t.Errorf("Expected platform %s, got %s", api.PlatformDingTalk, msg.Platform)
	}
	if msg.ID != "msg_123" {
		t.Errorf("Expected ID msg_123, got %s", msg.ID)
	}
	if msg.ChatID != "conv_123" {
		t.Errorf("Expected ChatID conv_123, got %s", msg.ChatID)
	}
	if msg.ChatType != api.ChatTypeGroup {
		t.Errorf("Expected ChatType %s, got %s", api.ChatTypeGroup, msg.ChatType)
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
	if msg.Sender.Name != "Test User" {
		t.Errorf("Expected Sender.Name 'Test User', got %s", msg.Sender.Name)
	}
	if msg.Sender.StaffID != "staff_123" {
		t.Errorf("Expected Sender.StaffID staff_123, got %s", msg.Sender.StaffID)
	}
	if msg.Sender.IsAdmin != false {
		t.Errorf("Expected Sender.IsAdmin false, got %v", msg.Sender.IsAdmin)
	}

	// Verify extensions
	if msg.Extensions == nil {
		t.Fatal("Expected Extensions to be set")
	}
	if v, ok := msg.Extensions["sessionWebhook"].(string); !ok || v != "https://webhook.example.com" {
		t.Errorf("Expected Extensions[sessionWebhook], got %v", msg.Extensions["sessionWebhook"])
	}
}

func TestDingTalkAdapter_ParseMessage_CardEvent(t *testing.T) {
	adapter := NewDingTalkAdapter(&DingTalkConfig{})

	// Create card event
	event := DingTalkEvent{
		MsgType:        "card",
		ConversationID: "conv_456",
		MsgId:          "msg_456",
		SenderId:       "user_456",
	}
	event.Content.ContentType = "card"

	body, _ := json.Marshal(event)

	msg, err := adapter.ParseMessage(context.Background(), body, nil, nil)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	if msg.EventType != api.EventTypeCardClick {
		t.Errorf("Expected EventType %s, got %s", api.EventTypeCardClick, msg.EventType)
	}
}

func TestDingTalkAdapter_ParseMessage_TextField(t *testing.T) {
	adapter := NewDingTalkAdapter(&DingTalkConfig{})

	// Create event with Text field instead of Content.Content
	event := DingTalkEvent{
		MsgType:        "text",
		ConversationID: "conv_789",
		MsgId:          "msg_789",
		SenderId:       "user_789",
		Text:           "Text from Text field",
	}

	body, _ := json.Marshal(event)

	msg, err := adapter.ParseMessage(context.Background(), body, nil, nil)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	if msg.Content != "Text from Text field" {
		t.Errorf("Expected Content 'Text from Text field', got %s", msg.Content)
	}
}

func TestDingTalkAdapter_HandleChallenge(t *testing.T) {
	adapter := NewDingTalkAdapter(&DingTalkConfig{})

	// DingTalk doesn't have challenge mechanism
	body := []byte(`{"type":"test"}`)
	response, handled, err := adapter.HandleChallenge(body)

	if err != nil {
		t.Errorf("HandleChallenge failed: %v", err)
	}
	if handled {
		t.Error("Expected challenge not to be handled")
	}
	if response != nil {
		t.Error("Expected nil response")
	}
}

func TestDingTalkAdapter_VerifySignature(t *testing.T) {
	adapter := NewDingTalkAdapter(&DingTalkConfig{
		AppSecret: "test_secret",
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

	// Test with invalid signature
	headers = map[string]string{
		"timestamp": "1234567890",
		"sign":      "invalid_signature",
	}
	err = adapter.VerifySignature([]byte("test body"), headers, nil)
	if err == nil {
		t.Error("Expected error for invalid signature")
	}
}

func TestDingTalkAdapter_VerifySignature_NoSecret(t *testing.T) {
	adapter := NewDingTalkAdapter(&DingTalkConfig{})

	// Test without secret configured
	headers := map[string]string{
		"timestamp": "1234567890",
		"sign":      "some_signature",
	}
	err := adapter.VerifySignature([]byte("test body"), headers, nil)
	if err != nil {
		t.Errorf("Expected no error without secret configured, got %v", err)
	}
}

func TestDingTalkAdapter_FormatResponse(t *testing.T) {
	adapter := NewDingTalkAdapter(&DingTalkConfig{})

	msg := &api.IMMessage{
		Content: "test response",
	}

	// Test plain text response
	response, err := adapter.FormatResponse(msg, api.ResponseTypePlainText)
	if err != nil {
		t.Fatalf("FormatResponse failed: %v", err)
	}

	var resp map[string]interface{}
	json.Unmarshal(response, &resp)
	if resp["msg_type"].(string) != "text" {
		t.Errorf("Expected msg_type text, got %v", resp["msg_type"])
	}

	// Test JSON response
	response, err = adapter.FormatResponse(msg, api.ResponseTypeJSON)
	if err != nil {
		t.Fatalf("FormatResponse failed: %v", err)
	}

	var apiResp DingTalkAPIResponse
	json.Unmarshal(response, &apiResp)
	if apiResp.Errcode != 0 {
		t.Errorf("Expected Errcode 0, got %d", apiResp.Errcode)
	}
	if apiResp.Errmsg != "success" {
		t.Errorf("Expected Errmsg 'success', got %s", apiResp.Errmsg)
	}
}

func TestDingTalkAdapter_CreateProcessor(t *testing.T) {
	adapter := NewDingTalkAdapter(&DingTalkConfig{})

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

func TestDingTalkAdapter_ConvertChatType(t *testing.T) {
	adapter := &DingTalkAdapter{}

	// Test P2P chat type
	result := adapter.convertChatType("1")
	if result != api.ChatTypeP2P {
		t.Errorf("Expected %s, got %s", api.ChatTypeP2P, result)
	}

	// Test group chat type
	result = adapter.convertChatType("2")
	if result != api.ChatTypeGroup {
		t.Errorf("Expected %s, got %s", api.ChatTypeGroup, result)
	}

	// Test unknown type - return as is
	result = adapter.convertChatType("unknown")
	if result != "unknown" {
		t.Errorf("Expected 'unknown', got %s", result)
	}
}
