package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"example.com/wsbot/internal/model"
	"example.com/wsbot/internal/store"
	"example.com/wsbot/internal/transport/wsclient"
	"example.com/wsbot/internal/util/config"
)

func main() {
	cfgPath := flag.String("config", "./config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	fs, err := store.NewFileStore(cfg.Store.Path)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}
	defer fs.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 记录最后心跳时间（不刷日志）
	var lastHeartbeat atomic.Value
	lastHeartbeat.Store(time.Now())

	client := wsclient.New(cfg, func(raw []byte) {
		// 先快速识别 meta_event（心跳/生命周期），不刷屏
		if handleMeta(&lastHeartbeat, raw) {
			return
		}
		// 兼容一帧多事件（数组）
		if handlePayload(fs, raw) {
			return
		}
	})

	log.Printf("ws connect to %s\n", cfg.WS.URL)
	if err := client.Start(ctx); err != nil {
		log.Fatalf("ws start: %v", err)
	}

	// 可选：背景健康检查（不刷屏，仅在超时才报警）
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				// 超过 2 分钟没心跳，提示一次
				if time.Since(lastHeartbeat.Load().(time.Time)) > 2*time.Minute {
					log.Println("⚠️ 心跳超时 > 2m，可能已断开（等待自动重连）")
				}
			}
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	if err := client.Close(); err != nil {
		log.Println("close error:", err)
	}
	_ = os.Stderr.Sync()
}

// —— 解析与处理 —— //

// handleMeta: 返回 true 表示这是 meta_event（心跳等）并已处理，不再向下传
func handleMeta(last *atomic.Value, raw []byte) bool {
	var probe struct {
		PostType      string `json:"post_type"`
		MetaEventType string `json:"meta_event_type"`
		Interval      int    `json:"interval"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	if probe.PostType == "meta_event" {
		if probe.MetaEventType == "heartbeat" {
			last.Store(time.Now())
			// 不打印，保持日志干净
		}
		return true
	}
	return false
}

// 既兼容“顶层数组的一帧多事件”，也兼容单对象；返回是否处理过
func handlePayload(fs *store.FileStore, raw []byte) bool {
	b := bytesTrim(raw)
	if len(b) == 0 {
		return false
	}
	if b[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(b, &arr); err != nil {
			return false
		}
		for _, it := range arr {
			handleOne(fs, []byte(it))
		}
		return true
	}
	handleOne(fs, b)
	return true
}

func handleOne(fs *store.FileStore, raw []byte) {
	msg, ok := model.DecodeOneBotOrInternal(raw)
	if !ok {
		return
	}
	// 忽略机器人自己发的消息（例如离开自动回复）
	if msg.FromSelf {
		return
	}

	text := strings.TrimSpace(msg.Text)
	switch text {
	case "上班":
		if err := fs.AppendEvent(msg.UserID, "上班", msg.Timestamp); err != nil {
			log.Printf("append 上班 error: %v", err)
		} else {
			log.Printf("[记录成功] user=%s 上班 at %s", msg.UserID, msg.Timestamp.Format(time.RFC3339))
		}
	case "下班":
		if err := fs.AppendEvent(msg.UserID, "下班", msg.Timestamp); err != nil {
			log.Printf("append 下班 error: %v", err)
		} else {
			log.Printf("[记录成功] user=%s 下班 at %s", msg.UserID, msg.Timestamp.Format(time.RFC3339))
		}
	default:
		// 其它消息忽略
	}
}

func bytesTrim(b []byte) []byte {
	i, j := 0, len(b)-1
	for i <= j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	for j >= i && (b[j] == ' ' || b[j] == '\n' || b[j] == '\r' || b[j] == '\t') {
		j--
	}
	return b[i : j+1]
}
