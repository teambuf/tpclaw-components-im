package endpoint

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testLogger struct {
	t *testing.T
}

func (l *testLogger) Debugf(format string, args ...interface{}) {
	l.t.Logf("[DEBUG] "+format, args...)
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}
func (l *testLogger) Infof(format string, args ...interface{}) {
	l.t.Logf("[INFO] "+format, args...)
	fmt.Printf("[INFO] "+format+"\n", args...)
}
func (l *testLogger) Warnf(format string, args ...interface{}) {
	l.t.Logf("[WARN] "+format, args...)
	fmt.Printf("[WARN] "+format+"\n", args...)
}
func (l *testLogger) Errorf(format string, args ...interface{}) {
	l.t.Logf("[ERROR] "+format, args...)
	fmt.Printf("[ERROR] "+format+"\n", args...)
}
func (l *testLogger) Printf(format string, args ...interface{}) {
	l.t.Logf("[PRINT] "+format, args...)
	fmt.Printf("[PRINT] "+format+"\n", args...)
}

// TestWeComBot_ProactiveSend 测试企业微信智能机器人主动发送消息
// 运行:
//
//	$env:WECOM_BOT_ID="aib7uXTSe_SZN4wHuaF7uo8O95j0u9PKWeh"
//	$env:WECOM_BOT_SECRET="6v8Y1ZRciWD7jSQnUYIB1W6QhLujPHb4jEU2tQSVtA5"
//	$env:WECOM_USER_ID="parky"
//	go test -v -run TestWeComBot_ProactiveSend
func TestWeComBot_ProactiveSend(t *testing.T) {
	botID := os.Getenv("WECOM_BOT_ID")
	secret := os.Getenv("WECOM_BOT_SECRET")
	userID := os.Getenv("WECOM_USER_ID")

	if botID == "" || secret == "" || userID == "" {
		t.Skip("Skipping test: WECOM_BOT_ID, WECOM_BOT_SECRET, WECOM_USER_ID must be set")
	}

	config := wecomWSClientConfig{
		BotID:             botID,
		Secret:            secret,
		WSURL:             wecomWSURL,
		HeartbeatInterval: 30 * time.Second,
		AutoReconnect:     false,
	}

	client := newWecomWSClient(config, &testLogger{t: t})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 启动并连接
	err := client.start(ctx)
	if err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.close()

	// 等待认证完成
	time.Sleep(2 * time.Second)

	// 构建主动发送的命令
	cmd := map[string]interface{}{
		"cmd": wecomCmdSendMsg,
		"body": map[string]interface{}{
			"chatid":  userID,
			"msgtype": "markdown",
			"markdown": map[string]interface{}{
				"content": "这是一条通过 WebSocket 主动推送的测试消息！\n> Markdown 测试",
			},
		},
	}

	err = client.SendCmdRaw(cmd)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	t.Log("Proactive text message sent successfully. Wait to observe any errors from server...")
	time.Sleep(3 * time.Second)

	// 测试发送图片
	testDataPath := filepath.Join("..", "testdata", "logo.png")
	absPath, err := filepath.Abs(testDataPath)
	if err == nil {
		if data, err := os.ReadFile(absPath); err == nil {
			hash := md5.Sum(data)
			md5Str := hex.EncodeToString(hash[:])

			// 1. 先通过 WebSocket 接口上传媒体文件获取 media_id
			mediaID, err := client.UploadMedia(ctx, "image", "logo.png", md5Str, data)
			if err != nil {
				t.Fatalf("Failed to upload media via WebSocket: %v", err)
			}
			t.Logf("WebSocket UploadMedia success, media_id: %s", mediaID)

			// 2. 使用获取的 media_id 发送图片消息
			cmdImg := map[string]interface{}{
				"cmd": wecomCmdSendMsg,
				"body": map[string]interface{}{
					"chatid":  userID,
					"msgtype": "image",
					"image": map[string]interface{}{
						"media_id": mediaID,
					},
				},
			}
			err = client.SendCmdRaw(cmdImg)
			if err != nil {
				t.Fatalf("Failed to send image message: %v", err)
			}
			t.Log("Proactive image message sent successfully. Wait to observe any errors...")
			time.Sleep(3 * time.Second)
		}
	}
}
