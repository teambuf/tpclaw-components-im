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
	"fmt"
	imapi "github.com/teambuf/tpclaw-components-im/api"
	"github.com/teambuf/tpclaw-components-im/processor"

	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/utils/maps"
)

func init() {
	// Register WeCom processors with bridge support (for both code and DSL usage)
	// 注册企业微信处理器（同时支持代码和 DSL 方式使用）
	processor.RegisterProcessorWithBridge(imapi.ProcessorWeComURLVerify, createProcessor(imapi.ProcessorTypeURLVerify))
	processor.RegisterProcessorWithBridge(imapi.ProcessorWeComDecrypt, createProcessor(imapi.ProcessorTypeDecrypt))
	processor.RegisterProcessorWithBridge(imapi.ProcessorWeComEncryptResp, createProcessor(imapi.ProcessorTypeEncryptResponse))
	processor.RegisterProcessorWithBridge(imapi.ProcessorWeComStreamResp, createProcessor(imapi.ProcessorTypeStreamResponse))
	processor.RegisterProcessorWithBridge(imapi.ProcessorWeComAck, createProcessor(imapi.ProcessorTypeAck))
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

// createAdapterFromConfig creates a WeComAdapter from the configuration interface
func createAdapterFromConfig(config interface{}) (*WeComAdapter, error) {
	var cfg WeComConfig
	if err := maps.Map2Struct(config, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse wecom config: %w", err)
	}

	// Validate required config
	if cfg.Token == "" || cfg.EncodingAESKey == "" || cfg.CorpID == "" {
		return nil, fmt.Errorf("missing required wecom config: token, encodingAESKey, corpId")
	}

	return NewWeComAdapter(&cfg)
}
