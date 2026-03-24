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
	"fmt"
	imapi "github.com/teambuf/tpclaw-components-im/api"
	"github.com/teambuf/tpclaw-components-im/processor"

	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/utils/maps"
)

func init() {
	// Register DingTalk processors with bridge support (for both code and DSL usage)
	// 注册钉钉处理器（同时支持代码和 DSL 方式使用）
	processor.RegisterProcessorWithBridge(imapi.ProcessorDingTalkURLVerify, createProcessor(imapi.ProcessorTypeURLVerify))
	processor.RegisterProcessorWithBridge(imapi.ProcessorDingTalkVerifySig, createProcessor(imapi.ProcessorTypeVerifySignature))
	processor.RegisterProcessorWithBridge(imapi.ProcessorDingTalkAck, createProcessor(imapi.ProcessorTypeAck))
}

// createProcessor creates a factory function for the given processor type
func createProcessor(processorType imapi.ProcessorType) imapi.ProcessorFactory {
	return func(config interface{}) (endpoint.Process, error) {
		adapter, err := createAdapterFromConfig(config)
		if err != nil {
			return nil, err
		}
		return adapter.CreateProcessor(processorType, config)
	}
}

// createAdapterFromConfig creates a DingTalkAdapter from the configuration interface
func createAdapterFromConfig(config interface{}) (*DingTalkAdapter, error) {
	var cfg DingTalkConfig
	if err := maps.Map2Struct(config, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse dingtalk config: %w", err)
	}

	// Validate required config
	if cfg.AppSecret == "" {
		return nil, fmt.Errorf("missing required dingtalk config: appSecret")
	}

	return NewDingTalkAdapter(&cfg), nil
}
