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
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

var (
	ErrInvalidSignature = errors.New("dingtalk: invalid signature")
)

// Crypto provides signature verification for DingTalk callback.
type Crypto struct {
	token string // Token for signature verification
}

// NewCrypto creates a new Crypto instance.
func NewCrypto(token string) (*Crypto, error) {
	return &Crypto{
		token: token,
	}, nil
}

// VerifySignature verifies the signature from DingTalk callback URL.
// The signature is computed as: sha1(sort(token, timestamp, nonce))
func (c *Crypto) VerifySignature(signature, timestamp, nonce string) bool {
	expected := c.ComputeSignature(timestamp, nonce)
	return signature == expected
}

// ComputeSignature computes the signature for the given parameters.
func (c *Crypto) ComputeSignature(timestamp, nonce string) string {
	// Sort the parameters
	params := []string{c.token, timestamp, nonce}
	sort.Strings(params)

	// Concatenate and compute SHA1
	combined := strings.Join(params, "")
	hash := sha1.Sum([]byte(combined))
	return hex.EncodeToString(hash[:])
}
