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

// Package processor provides unified IM message processors.
package processor

import (
	"fmt"
	imapi "github.com/teambuf/tpclaw-components-im/api"
	"sync"

	"github.com/rulego/rulego/api/types/endpoint"
)

// GlobalRegistry is the global processor registry.
var GlobalRegistry = NewRegistry()

// Registry manages processor factories.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]imapi.ProcessorFactory
}

// NewRegistry creates a new processor registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]imapi.ProcessorFactory),
	}
}

// Register registers a processor factory.
func (r *Registry) Register(name string, factory imapi.ProcessorFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Get retrieves a processor factory by name.
func (r *Registry) Get(name string) (imapi.ProcessorFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.factories[name]
	return factory, ok
}

// Create creates a processor instance.
func (r *Registry) Create(name string, config interface{}) (endpoint.Process, error) {
	factory, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("processor not found: %s", name)
	}
	return factory(config)
}

// GetNames returns all registered processor names.
// GetNames 返回所有已注册的处理器名称。
func (r *Registry) GetNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// RegisterProcessor registers a processor to the global registry.
func RegisterProcessor(name string, factory imapi.ProcessorFactory) {
	GlobalRegistry.Register(name, factory)
}

// CreateProcessor creates a processor from the global registry.
func CreateProcessor(name string, config interface{}) (endpoint.Process, error) {
	return GlobalRegistry.Create(name, config)
}

// init registers built-in processors.
func init() {
	// Register message transform processor
	RegisterProcessor(imapi.ProcessorMessageTransform, func(config interface{}) (endpoint.Process, error) {
		return MessageTransformProcessor(), nil
	})

	// Register ACK response processor
	RegisterProcessor(imapi.ProcessorAck, func(config interface{}) (endpoint.Process, error) {
		return AckProcessor(), nil
	})
}
