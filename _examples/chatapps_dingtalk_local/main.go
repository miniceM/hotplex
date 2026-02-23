package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/hrygo/hotplex/chatapps"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	adapter := chatapps.NewDingTalkAdapter(chatapps.DingTalkConfig{
		ServerAddr: ":8080",
	}, logger)

	adapter.SetHandler(func(ctx context.Context, msg *chatapps.ChatMessage) error {
		fmt.Printf("\n📥 收到消息 from %s:\n   %s\n", msg.UserID, msg.Content)
		fmt.Println("   正在调用 AI...")

		response := &chatapps.ChatMessage{
			Platform:  "dingtalk",
			SessionID: msg.SessionID,
			Content:   fmt.Sprintf("收到: %s (本地测试模式，AI 响应稍后实现)", msg.Content),
			Metadata:  msg.Metadata,
		}

		if err := adapter.SendMessage(ctx, msg.SessionID, response); err != nil {
			fmt.Printf("   ❌ 发送失败: %v\n", err)
		} else {
			fmt.Println("   ✅ 响应已发送")
		}
		return nil
	})

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		logger.Error("Failed to start", "error", err)
		os.Exit(1)
	}

	fmt.Println("🎉 DingTalk Chat 适配器已启动!")
	fmt.Println("   监听地址: http://localhost:8080")
	fmt.Println("   回调端点: /webhook")
	fmt.Println("\n📌 下一步:")
	fmt.Println("   1. 使用 ngrok 暴露到公网: ngrok http 8080")
	fmt.Println("   2. 将 ngrok 地址配置到钉钉回调设置")
	fmt.Println("\n按 Ctrl+C 退出")

	select {}
}
