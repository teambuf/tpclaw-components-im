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
	"encoding/xml"
	"os"
	"testing"

	"github.com/teambuf/tpclaw-components-im/api"
)

// Test configuration loaded from environment variables
// Set these environment variables for integration testing:
// WECOM_TOKEN, WECOM_ENCODING_AES_KEY, WECOM_CORP_ID, WECOM_AGENT_ID, WECOM_SECRET
// For unit tests, placeholder values are used
func getTestConfig() *WeComConfig {
	return &WeComConfig{
		Token:          getEnvOrDefault("WECOM_TOKEN", "YOUR_TOKEN_HERE"),
		EncodingAESKey: getEnvOrDefault("WECOM_ENCODING_AES_KEY", "YOUR_ENCODING_AES_KEY_HERE_43CHARS"),
		CorpID:         getEnvOrDefault("WECOM_CORP_ID", "YOUR_CORP_ID"),
		AgentID:        123456,
		Secret:         getEnvOrDefault("WECOM_SECRET", "YOUR_SECRET_HERE"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// testConfig is kept for backward compatibility but uses environment variables
var testConfig = getTestConfig()

func TestWeComAdapter_Platform(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	if adapter.Platform() != api.PlatformWeCom {
		t.Errorf("Expected platform %s, got %s", api.PlatformWeCom, adapter.Platform())
	}
}

func TestWeComAdapter_HandleChallenge(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	// WeCom challenge is handled via GET request, not POST body
	body := []byte("test body")
	response, handled, err := adapter.HandleChallenge(body)

	if err != nil {
		t.Errorf("HandleChallenge failed: %v", err)
	}
	if handled {
		t.Error("Expected challenge not to be handled via POST body")
	}
	if response != nil {
		t.Error("Expected nil response")
	}
}

func TestWeComAdapter_HandleURLVerification(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	// Create test echo string
	echoStr := "test_echo_string"
	encryptedEcho, err := adapter.crypto.Encrypt([]byte(echoStr))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Generate signature
	timestamp := GenerateTimestamp()
	nonce := GenerateNonce()
	signature := adapter.crypto.ComputeSignature(timestamp, nonce, encryptedEcho)

	// Test URL verification
	params := map[string]string{
		"msg_signature": signature,
		"timestamp":     timestamp,
		"nonce":         nonce,
		"echostr":       encryptedEcho,
	}

	response, err := adapter.HandleURLVerification(params)
	if err != nil {
		t.Fatalf("HandleURLVerification failed: %v", err)
	}

	if string(response) != echoStr {
		t.Errorf("Expected echo '%s', got '%s'", echoStr, string(response))
	}
}

func TestWeComAdapter_HandleURLVerification_InvalidSignature(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	params := map[string]string{
		"msg_signature": "invalid_signature",
		"timestamp":     "1234567890",
		"nonce":         "test_nonce",
		"echostr":       "test_echo",
	}

	_, err = adapter.HandleURLVerification(params)
	if err == nil {
		t.Error("Expected error for invalid signature")
	}
	if err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature, got %v", err)
	}
}

func TestWeComAdapter_ParseMessage(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	// Create test message
	callbackMsg := CallbackMessage{
		ToUserName:   "test_corp",
		FromUserName: "user_123",
		CreateTime:   1704067200,
		MsgType:      "text",
		Content:      "Hello World",
		MsgID:        "msg_123",
	}

	msgXML, err := xml.Marshal(&callbackMsg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Encrypt the message
	encrypted, err := adapter.crypto.Encrypt(msgXML)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Generate signature
	timestamp := GenerateTimestamp()
	nonce := GenerateNonce()
	signature := adapter.crypto.ComputeSignature(timestamp, nonce, encrypted)

	// Create received message
	receivedMsg := ReceivedMessage{
		ToUserName: "test_corp",
		AgentID:    "123456",
		Encrypt:    encrypted,
	}
	receivedXML, _ := xml.Marshal(&receivedMsg)

	// Parse message
	params := map[string]string{
		"msg_signature": signature,
		"timestamp":     timestamp,
		"nonce":         nonce,
	}

	msg, err := adapter.ParseMessage(context.Background(), receivedXML, nil, params)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	// Verify message fields
	if msg.Platform != api.PlatformWeCom {
		t.Errorf("Expected platform %s, got %s", api.PlatformWeCom, msg.Platform)
	}
	if msg.ID != "msg_123" {
		t.Errorf("Expected ID msg_123, got %s", msg.ID)
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

	// Verify extensions
	if msg.Extensions == nil {
		t.Fatal("Expected Extensions to be set")
	}
	if v, ok := msg.Extensions[api.MetaWeComFromUser].(string); !ok || v != "user_123" {
		t.Errorf("Expected Extensions[fromUser] user_123, got %v", msg.Extensions[api.MetaWeComFromUser])
	}
}

func TestWeComAdapter_ParseMessage_Event(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	// Create event message
	callbackMsg := CallbackMessage{
		ToUserName:   "test_corp",
		FromUserName: "user_456",
		CreateTime:   1704067200,
		MsgType:      "event",
		Event:        "click",
		EventKey:     "button_123",
	}

	msgXML, _ := xml.Marshal(&callbackMsg)
	encrypted, _ := adapter.crypto.Encrypt(msgXML)

	timestamp := GenerateTimestamp()
	nonce := GenerateNonce()
	signature := adapter.crypto.ComputeSignature(timestamp, nonce, encrypted)

	receivedMsg := ReceivedMessage{
		ToUserName: "test_corp",
		AgentID:    "123456",
		Encrypt:    encrypted,
	}
	receivedXML, _ := xml.Marshal(&receivedMsg)

	params := map[string]string{
		"msg_signature": signature,
		"timestamp":     timestamp,
		"nonce":         nonce,
	}

	msg, err := adapter.ParseMessage(context.Background(), receivedXML, nil, params)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	if msg.MsgType != "event" {
		t.Errorf("Expected MsgType event, got %s", msg.MsgType)
	}
	if msg.EventType != "click" {
		t.Errorf("Expected EventType click, got %s", msg.EventType)
	}
}

func TestWeComAdapter_ParseMessage_InvalidSignature(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	receivedMsg := ReceivedMessage{
		ToUserName: "test_corp",
		AgentID:    "123456",
		Encrypt:    "test_encrypt",
	}
	receivedXML, _ := xml.Marshal(&receivedMsg)

	params := map[string]string{
		"msg_signature": "invalid_signature",
		"timestamp":     "1234567890",
		"nonce":         "test_nonce",
	}

	_, err = adapter.ParseMessage(context.Background(), receivedXML, nil, params)
	if err == nil {
		t.Error("Expected error for invalid signature")
	}
}

func TestWeComAdapter_FormatResponse(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	msg := &api.IMMessage{
		Content: "test response",
	}

	// Test plain text response
	response, err := adapter.FormatResponse(msg, api.ResponseTypePlainText)
	if err != nil {
		t.Fatalf("FormatResponse failed: %v", err)
	}
	if string(response) != "success" {
		t.Errorf("Expected 'success', got '%s'", string(response))
	}

	// Test JSON response
	response, err = adapter.FormatResponse(msg, api.ResponseTypeJSON)
	if err != nil {
		t.Fatalf("FormatResponse failed: %v", err)
	}
	if response == nil {
		t.Error("Expected JSON response")
	}
}

func TestWeComAdapter_FormatResponse_Encrypted(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	msg := &api.IMMessage{
		Content: "test encrypted response",
		Extensions: map[string]interface{}{
			api.MetaWeComToUser:   "user_123",
			api.MetaWeComFromUser: "corp_123",
		},
	}

	response, err := adapter.FormatResponse(msg, api.ResponseTypeEncrypted)
	if err != nil {
		t.Fatalf("FormatResponse failed: %v", err)
	}

	// Verify response is valid XML
	var resp EncryptedResponse
	err = xml.Unmarshal(response, &resp)
	if err != nil {
		t.Errorf("Response is not valid XML: %v", err)
	}
	if resp.Encrypt == "" {
		t.Error("Expected Encrypt to be set")
	}
	if resp.MsgSignature == "" {
		t.Error("Expected MsgSignature to be set")
	}
}

func TestWeComAdapter_FormatResponse_Stream(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	msg := &api.IMMessage{
		Content: "test stream response",
		ChatID:  "user_123",
		Extensions: map[string]interface{}{
			api.MetaWeComStreamChatInfo: "1|user_123|user_123",
		},
	}

	response, err := adapter.FormatResponse(msg, api.ResponseTypeStream)
	if err != nil {
		t.Fatalf("FormatResponse failed: %v", err)
	}
	if response == nil {
		t.Error("Expected stream response")
	}
}

func TestWeComAdapter_CreateProcessor(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	// Test URL verify processor
	processor, err := adapter.CreateProcessor(api.ProcessorTypeURLVerify, nil)
	if err != nil {
		t.Fatalf("CreateProcessor failed: %v", err)
	}
	if processor == nil {
		t.Error("Expected processor to be created")
	}

	// Test decrypt processor
	processor, err = adapter.CreateProcessor(api.ProcessorTypeDecrypt, nil)
	if err != nil {
		t.Fatalf("CreateProcessor failed: %v", err)
	}
	if processor == nil {
		t.Error("Expected processor to be created")
	}

	// Test encrypt response processor
	processor, err = adapter.CreateProcessor(api.ProcessorTypeEncryptResponse, nil)
	if err != nil {
		t.Fatalf("CreateProcessor failed: %v", err)
	}
	if processor == nil {
		t.Error("Expected processor to be created")
	}

	// Test stream response processor
	processor, err = adapter.CreateProcessor(api.ProcessorTypeStreamResponse, map[string]interface{}{"msgType": 1})
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

func TestWeComAdapter_GenerateNonce(t *testing.T) {
	nonce1 := GenerateNonce()
	nonce2 := GenerateNonce()

	if nonce1 == "" {
		t.Error("Expected nonce to be generated")
	}
	if nonce1 == nonce2 {
		t.Error("Expected different nonces")
	}
}

func TestWeComAdapter_GenerateTimestamp(t *testing.T) {
	ts := GenerateTimestamp()
	if ts == "" {
		t.Error("Expected timestamp to be generated")
	}
}

func TestWeComAdapter_GetCrypto(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	crypto := adapter.GetCrypto()
	if crypto == nil {
		t.Error("Expected crypto to be returned")
	}
}

func TestWeComAdapter_GetConfig(t *testing.T) {
	adapter, err := NewWeComAdapter(testConfig)
	if err != nil {
		t.Fatalf("NewWeComAdapter failed: %v", err)
	}

	config := adapter.GetConfig()
	if config == nil {
		t.Error("Expected config to be returned")
	}
	if config.CorpID != testConfig.CorpID {
		t.Errorf("Expected CorpID %s, got %s", testConfig.CorpID, config.CorpID)
	}
}
