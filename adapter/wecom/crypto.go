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
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

var (
	ErrInvalidSignature      = errors.New("wecom: invalid signature")
	ErrInvalidEncodingAESKey = errors.New("wecom: invalid encoding AES key")
	ErrInvalidCorpID         = errors.New("wecom: invalid corpId")
	ErrInvalidEncryptedMsg   = errors.New("wecom: invalid encrypted message format")
	ErrDecryptFailed         = errors.New("wecom: decrypt failed")
)

// Crypto provides encryption and decryption utilities for WeCom smart robot.
type Crypto struct {
	token     string // Token for signature verification
	aesKey    []byte // Decoded AES key (32 bytes)
	receiveID string // The receiveID (corpId) of the enterprise
}

// NewCrypto creates a new Crypto instance.
func NewCrypto(token, encodingAESKey, receiveID string) (*Crypto, error) {
	if len(encodingAESKey) != 43 {
		return nil, ErrInvalidEncodingAESKey
	}

	// Decode the base64 encoded AES key
	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil || len(aesKey) != 32 {
		return nil, ErrInvalidEncodingAESKey
	}

	return &Crypto{
		token:     token,
		aesKey:    aesKey,
		receiveID: receiveID,
	}, nil
}

// Token returns the token.
func (c *Crypto) Token() string {
	return c.token
}

// ReceiveID returns the receive ID (corpId).
func (c *Crypto) ReceiveID() string {
	return c.receiveID
}

// VerifySignature verifies the signature from WeCom callback URL.
// The signature is computed as: sha1(sort(token, timestamp, nonce, encrypt))
func (c *Crypto) VerifySignature(signature, timestamp, nonce, encrypt string) bool {
	expected := c.ComputeSignature(timestamp, nonce, encrypt)
	return signature == expected
}

// ComputeSignature computes the signature for the given parameters.
func (c *Crypto) ComputeSignature(timestamp, nonce, encrypt string) string {
	// Sort the parameters
	params := []string{c.token, timestamp, nonce, encrypt}
	sort.Strings(params)

	// Concatenate and compute SHA1
	combined := strings.Join(params, "")
	hash := sha1.Sum([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// Decrypt decrypts the encrypted message from WeCom.
// The encrypted message format:
// Base64Decode(encrypt) -> AESDecrypt -> random(16 bytes) + msg_len(4 bytes) + msg + receiveid
func (c *Crypto) Decrypt(encrypt string) ([]byte, error) {
	// Base64 decode
	ciphertext, err := base64.StdEncoding.DecodeString(encrypt)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode failed: %v", ErrDecryptFailed, err)
	}

	// AES decrypt
	plaintext, err := c.aesDecrypt(ciphertext)
	if err != nil {
		return nil, err
	}

	// Parse the decrypted content
	// Format: random(16 bytes) + msg_len(4 bytes) + msg + receiveid
	if len(plaintext) < 20 {
		return nil, ErrInvalidEncryptedMsg
	}

	// Extract message length (bytes 16-20, big-endian)
	msgLen := binary.BigEndian.Uint32(plaintext[16:20])
	if len(plaintext) < int(20+msgLen) {
		return nil, ErrInvalidEncryptedMsg
	}

	// Extract message and receiveid
	msg := plaintext[20 : 20+msgLen]
	receiveID := plaintext[20+msgLen:]

	// Verify receiveid
	if string(receiveID) != c.receiveID {
		return nil, ErrInvalidCorpID
	}

	return msg, nil
}

// Encrypt encrypts the message to be sent to WeCom.
// The encrypted message format:
// AESEncrypt(random(16 bytes) + msg_len(4 bytes) + msg + receiveid) -> Base64Encode
func (c *Crypto) Encrypt(msg []byte) (string, error) {
	// Create random bytes (16 bytes)
	random := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", fmt.Errorf("generate random bytes failed: %v", err)
	}

	// Create message length (4 bytes, big-endian)
	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(msg)))

	// Concatenate: random + msg_len + msg + receiveid
	plaintext := bytes.Join([][]byte{
		random,
		msgLen,
		msg,
		[]byte(c.receiveID),
	}, nil)

	// AES encrypt
	ciphertext, err := c.aesEncrypt(plaintext)
	if err != nil {
		return "", err
	}

	// Base64 encode
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// aesDecrypt performs AES-256-CBC decryption with PKCS7 unpadding.
func (c *Crypto) aesDecrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return nil, fmt.Errorf("%w: create cipher failed: %v", ErrDecryptFailed, err)
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, ErrInvalidEncryptedMsg
	}

	// The IV is the first 16 bytes of the ciphertext
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, ErrInvalidEncryptedMsg
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return nil, fmt.Errorf("%w: remove padding failed: %v", ErrDecryptFailed, err)
	}

	return plaintext, nil
}

// aesEncrypt performs AES-256-CBC encryption with PKCS7 padding.
func (c *Crypto) aesEncrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher failed: %v", err)
	}

	// Apply PKCS7 padding
	plaintext = pkcs7Pad(plaintext, aes.BlockSize)

	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("generate IV failed: %v", err)
	}

	ciphertext := make([]byte, len(plaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)

	// Prepend IV to ciphertext
	return append(iv, ciphertext...), nil
}

// pkcs7Pad applies PKCS7 padding.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// pkcs7Unpad removes PKCS7 padding.
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	padding := int(data[len(data)-1])
	if padding > len(data) || padding > aes.BlockSize {
		return nil, errors.New("invalid padding")
	}

	// Verify padding
	for i := len(data) - padding; i < len(data); i++ {
		if int(data[i]) != padding {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padding], nil
}
