/*
 * Copyright 2024 The RuleGo Project.
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
	"encoding/xml"
	"time"
)

// ReceivedMessage represents the encrypted message received from WeCom.
// This is the XML structure posted by WeCom to the callback URL.
type ReceivedMessage struct {
	ToUserName string `xml:"ToUserName"` // Developer's WeCom ID
	Encrypt    string `xml:"Encrypt"`    // Encrypted message body
	AgentID    string `xml:"AgentID"`    // Application AgentID
}

// CallbackMessage represents the decrypted callback message from WeCom.
type CallbackMessage struct {
	ToUserName   string `xml:"ToUserName"`   // Developer's WeCom ID
	FromUserName string `xml:"FromUserName"` // Sender's user ID
	CreateTime   int64  `xml:"CreateTime"`   // Message creation time (unix timestamp)
	MsgType      string `xml:"MsgType"`      // Message type: text, image, voice, video, location, link, event
	Content      string `xml:"Content"`      // Text content (for text messages)
	MsgID        string `xml:"MsgId"`        // Message ID (unique)
	AgentID      string `xml:"AgentID"`      // Application AgentID

	// Event related fields
	Event      string `xml:"Event"`      // Event type: subscribe, unsubscribe, enter_agent, etc.
	EventKey   string `xml:"EventKey"`   // Event key value
	ChangeType string `xml:"ChangeType"` // Change type for contact change events

	// Image message fields
	PicUrl  string `xml:"PicUrl"`  // Image URL
	MediaId string `xml:"MediaId"` // Media ID (for image, voice, video)

	// Voice message fields
	Format      string `xml:"Format"`      // Voice format (amr, speex, etc.)
	Recognition string `xml:"Recognition"` // Voice recognition result (if enabled)

	// Video message fields
	ThumbMediaId string `xml:"ThumbMediaId"` // Video thumbnail media ID

	// Location message fields
	Location_X float64 `xml:"Location_X"` // Latitude
	Location_Y float64 `xml:"Location_Y"` // Longitude
	Scale      int     `xml:"Scale"`      // Map scale
	Label      string  `xml:"Label"`      // Location label

	// Link message fields
	Title       string `xml:"Title"`       // Link title
	Description string `xml:"Description"` // Link description
	Url         string `xml:"Url"`         // Link URL

	// Attachment message fields (for wechat customer service)
	FileKey  string `xml:"FileKey"`  // File key
	FileMD5  string `xml:"FileMd5"`  // File MD5
	FileSize int64  `xml:"FileSize"` // File size
}

// TextMessage represents a text message to be sent to WeCom.
type TextMessage struct {
	ToUserName   string `xml:"ToUserName"`   // Receiver's user ID
	FromUserName string `xml:"FromUserName"` // Developer's WeCom ID
	CreateTime   int64  `xml:"CreateTime"`   // Message creation time
	MsgType      string `xml:"MsgType"`      // Message type: text
	Content      string `xml:"Content"`      // Text content
}

// NewTextMessage creates a new text message.
func NewTextMessage(toUserName, fromUserName, content string) *TextMessage {
	return &TextMessage{
		ToUserName:   toUserName,
		FromUserName: fromUserName,
		CreateTime:   time.Now().Unix(),
		MsgType:      "text",
		Content:      content,
	}
}

// ToXML converts the message to XML format.
func (m *TextMessage) ToXML() ([]byte, error) {
	return xml.Marshal(m)
}

// StreamResponse represents the response for streaming messages.
// This is used for AI chat scenarios where the response is streamed.
type StreamResponse struct {
	SpNum      string      `json:"sp_num"`       // Serial number
	SpChatInfo *SpChatInfo `json:"sp_chat_info"` // Chat info
	SpMsg      *SpMsg      `json:"sp_msg"`       // Message content
}

// SpChatInfo contains the chat information for stream response.
type SpChatInfo struct {
	SpChatType int    `json:"sp_chat_type"` // Chat type: 1=one-on-one, 2=group
	SpUserID   string `json:"sp_userid"`    // User ID
	SpChatID   string `json:"sp_chatid"`    // Chat ID
}

// SpMsg contains the message content for stream response.
type SpMsg struct {
	SpMsgType int         `json:"sp_msg_type"` // Message type: 0=text, 1=markdown
	SpContent interface{} `json:"sp_content"`  // Message content
}

// StreamTextContent represents text content in stream response.
type StreamTextContent struct {
	Content string `json:"content"` // Text content
}

// StreamMarkdownContent represents markdown content in stream response.
type StreamMarkdownContent struct {
	Content string `json:"content"` // Markdown content
}

// NewStreamTextResponse creates a new stream response with text content.
func NewStreamTextResponse(chatType int, userID, chatID, content string) *StreamResponse {
	return &StreamResponse{
		SpNum: "1",
		SpChatInfo: &SpChatInfo{
			SpChatType: chatType,
			SpUserID:   userID,
			SpChatID:   chatID,
		},
		SpMsg: &SpMsg{
			SpMsgType: 0,
			SpContent: &StreamTextContent{Content: content},
		},
	}
}

// NewStreamMarkdownResponse creates a new stream response with markdown content.
func NewStreamMarkdownResponse(chatType int, userID, chatID, content string) *StreamResponse {
	return &StreamResponse{
		SpNum: "1",
		SpChatInfo: &SpChatInfo{
			SpChatType: chatType,
			SpUserID:   userID,
			SpChatID:   chatID,
		},
		SpMsg: &SpMsg{
			SpMsgType: 1,
			SpContent: &StreamMarkdownContent{Content: content},
		},
	}
}

// EncryptedResponse represents the encrypted response to WeCom.
type EncryptedResponse struct {
	Encrypt      string `xml:"Encrypt"`      // Encrypted message content
	MsgSignature string `xml:"MsgSignature"` // Message signature
	TimeStamp    string `xml:"TimeStamp"`    // Timestamp
	Nonce        string `xml:"Nonce"`        // Random string
}

// NewEncryptedResponse creates a new encrypted response.
func NewEncryptedResponse(encrypt, signature, timestamp, nonce string) *EncryptedResponse {
	return &EncryptedResponse{
		Encrypt:      encrypt,
		MsgSignature: signature,
		TimeStamp:    timestamp,
		Nonce:        nonce,
	}
}

// ToXML converts the encrypted response to XML format.
func (r *EncryptedResponse) ToXML() ([]byte, error) {
	type xmlEnvelope struct {
		XMLName xml.Name `xml:"xml"`
		*EncryptedResponse
	}
	return xml.Marshal(&xmlEnvelope{EncryptedResponse: r})
}
