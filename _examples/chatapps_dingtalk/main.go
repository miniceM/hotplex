package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hrygo/hotplex/chatapps"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	addr := os.Getenv("HOTPLEX_CHATAPPS_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	adapter := chatapps.NewDingTalkAdapter(chatapps.DingTalkConfig{
		ServerAddr: addr,
	}, logger)

	adapter.SetHandler(func(ctx context.Context, msg *chatapps.ChatMessage) error {
		fmt.Printf("\n📥 收到消息 from %s:\n   %s\n", msg.UserID, msg.Content)
		fmt.Println("   正在处理...")

		response := &chatapps.ChatMessage{
			Platform:  "dingtalk",
			SessionID: msg.SessionID,
			Content:   "收到消息: " + msg.Content,
			Metadata:  msg.Metadata,
		}

		if err := adapter.SendMessage(ctx, msg.SessionID, response); err != nil {
			fmt.Printf("   ❌ 发送失败: %v\n", err)
		} else {
			fmt.Println("   ✅ 响应已发送")
		}
		return nil
	})

	if err := adapter.Start(context.Background()); err != nil {
		logger.Error("Failed to start adapter", "error", err)
		os.Exit(1)
	}

	fmt.Println("🎉 DingTalk Chat Adapter 已启动!")
	fmt.Printf("   监听地址: http://localhost%s\n", addr)
	fmt.Println("   回调端点: /webhook")
	fmt.Println("   健康检查: /health")
	fmt.Println("\n按 Ctrl+C 退出")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n👋 正在关闭...")
	adapter.Stop()
}
