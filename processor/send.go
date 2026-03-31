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

package processor

import (
	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
	imapi "github.com/teambuf/tpclaw-components-im/api"
)

func init() {
	// 注册公共的发送处理器到 OutBuiltins
	// 具体的平台响应消息结构体（如 FeishuResponseMessage, WeComResponseMessage）的 SetBody 方法会自动发送消息
	processor.OutBuiltins.Register(imapi.ProcessorSend, createSendProcessor)
}

// createSendProcessor 创建通用的发送处理器
// 只负责将响应内容设置到 Out.Body
// 实际的消息发送由各自平台对应的 ResponseMessage.SetBody 自动完成
func createSendProcessor(router endpoint.Router, exchange *endpoint.Exchange) bool {
	var responseBody string

	// 检查是否有错误
	if err := exchange.Out.GetError(); err != nil {
		// 有错误时，将错误信息作为响应内容
		responseBody = "处理请求时发生错误: " + err.Error()

		// 清除 Out 上的 Error 状态，避免底层 SetBody 被拦截（如 WeComResponseMessage.SetBody）
		exchange.Out.SetError(nil)
	} else {
		// 获取正常的响应内容
		if msg := exchange.Out.GetMsg(); msg != nil {
			responseBody = msg.GetData()
		} else if body := exchange.Out.Body(); len(body) > 0 {
			responseBody = string(body)
		}
	}

	// 如果没有响应内容，不发送
	if responseBody == "" {
		return true
	}

	// 设置响应体，各平台的 ResponseMessage.SetBody 会自动发送消息
	exchange.Out.SetBody([]byte(responseBody))

	return true
}
