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
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	imapi "github.com/teambuf/tpclaw-components-im/api"
)

func init() {
	// 注册飞书发送处理器到 OutBuiltins
	// FeishuResponseMessage.SetBody 会自动发送消息（如果有 msgId 或 chatId）
	processor.OutBuiltins.Register(imapi.ProcessorFeishuSend, createFeishuSendProcessor)
}

// createFeishuSendProcessor 创建飞书发送处理器
// 简化版本：只负责将响应内容设置到 Out.Body
// 实际的消息发送由 FeishuResponseMessage.SetBody 自动完成
func createFeishuSendProcessor(router endpoint.Router, exchange *endpoint.Exchange) bool {
	// 检查是否有错误
	if err := exchange.Out.GetError(); err != nil {
		// 有错误时不发送
		return true
	}

	// 获取响应内容
	var responseBody string
	if msg := exchange.Out.GetMsg(); msg != nil {
		responseBody = msg.GetData()
	} else if body := exchange.Out.Body(); len(body) > 0 {
		responseBody = string(body)
	}

	// 如果没有响应内容，不发送
	if responseBody == "" {
		return true
	}

	// 设置响应体，FeishuResponseMessage.SetBody 会自动发送消息
	exchange.Out.SetBody([]byte(responseBody))

	return true
}
