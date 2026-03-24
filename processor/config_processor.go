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

// Package processor provides configurable processors that read config from From.GetConfiguration().
package processor

import (
	"fmt"
	imapi "github.com/teambuf/tpclaw-components-im/api"
	"sync"

	"github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/builtin/processor"
)

// globalProcessorCache 全局处理器缓存，使用 sync.Map 优化并发读取
var globalProcessorCache sync.Map // key: string, value: endpoint.Process

// getConfig 从 From.GetConfiguration() 获取配置
func getConfig(router endpoint.Router, processorName string) interface{} {
	from := router.GetFrom()
	if from == nil {
		return nil
	}

	config := from.GetConfiguration()
	if config == nil {
		return nil
	}

	return config[processorName]
}

// configToKey 生成缓存 key，使用配置指针地址优化性能
func configToKey(processorName string, cfg interface{}) string {
	if cfg == nil {
		return processorName
	}
	// 使用指针地址作为 key 的一部分，避免 fmt.Sprintf("%v") 的开销和不稳定性
	return fmt.Sprintf("%s:%p", processorName, cfg)
}

// ConfigurableProcessor creates a cached processor that reads configuration from From.GetConfiguration().
// The configuration key is the processor name.
//
// ConfigurableProcessor 创建一个带缓存的处理器，从 From.GetConfiguration() 读取配置。
// 配置的 key 是处理器名称。对相同配置复用处理器实例。
func ConfigurableProcessor(processorName string, factory imapi.ProcessorFactory) endpoint.Process {
	return func(router endpoint.Router, exchange *endpoint.Exchange) bool {
		processorConfig := getConfig(router, processorName)
		cacheKey := configToKey(processorName, processorConfig)

		// 快速路径：从缓存获取
		if proc, ok := globalProcessorCache.Load(cacheKey); ok {
			return proc.(endpoint.Process)(router, exchange)
		}

		// 慢速路径：创建新处理器
		proc, err := factory(processorConfig)
		if err != nil {
			exchange.Out.SetError(fmt.Errorf("failed to create processor %s: %w", processorName, err))
			return false
		}

		// 缓存处理器（使用 LoadOrStore 避免竞态条件下的重复创建）
		if actual, loaded := globalProcessorCache.LoadOrStore(cacheKey, proc); loaded {
			proc = actual.(endpoint.Process)
		}

		return proc(router, exchange)
	}
}

// RegisterIMProcessor registers an IM processor with configuration support.
func RegisterIMProcessor(processorName string, factory imapi.ProcessorFactory) {
	processor.InBuiltins.Register(processorName, ConfigurableProcessor(processorName, factory))
}

// BridgeGlobalRegistry creates bridge processors for all processors in GlobalRegistry.
func BridgeGlobalRegistry() {
	names := GlobalRegistry.GetNames()
	for _, name := range names {
		processorName := name
		processor.InBuiltins.Register(processorName, ConfigurableProcessor(
			processorName,
			func(config interface{}) (endpoint.Process, error) {
				return GlobalRegistry.Create(processorName, config)
			},
		))
	}
}

// BridgeProcessor creates a bridge for a single processor from GlobalRegistry to InBuiltins.
func BridgeProcessor(processorName string) {
	if factory, ok := GlobalRegistry.Get(processorName); ok {
		processor.InBuiltins.Register(processorName, ConfigurableProcessor(
			processorName,
			func(config interface{}) (endpoint.Process, error) {
				return factory(config)
			},
		))
	}
}

// RegisterProcessorWithBridge registers a processor to both GlobalRegistry and InBuiltins.
// This is the recommended way to register IM processors that need configuration.
func RegisterProcessorWithBridge(name string, factory imapi.ProcessorFactory) {
	GlobalRegistry.Register(name, factory)
	processor.InBuiltins.Register(name, ConfigurableProcessor(name, factory))
}

// ClearCache clears the processor cache.
func ClearCache() {
	globalProcessorCache.Range(func(key, value interface{}) bool {
		globalProcessorCache.Delete(key)
		return true
	})
}
