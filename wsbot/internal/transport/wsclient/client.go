package wsclient

import (
	"context"
	"errors"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"example.com/wsbot/internal/util/config"
	"nhooyr.io/websocket"
)

type OnMessage func(raw []byte)

type Client struct {
	cfg    config.Config
	onMsg  OnMessage
	mu     sync.RWMutex
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

func New(cfg config.Config, onMsg OnMessage) *Client {
	return &Client{cfg: cfg, onMsg: onMsg}
}

func (c *Client) dial(ctx context.Context) (*websocket.Conn, error) {
	opts := &websocket.DialOptions{Subprotocols: []string{"json"}}
	if c.cfg.WS.Token != "" {
		opts.HTTPHeader = http.Header{"Authorization": {"Bearer " + c.cfg.WS.Token}}
	}
	conn, _, err := websocket.Dial(ctx, c.cfg.WS.URL, opts)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (c *Client) Start(parent context.Context) error {
	c.ctx, c.cancel = context.WithCancel(parent)
	go c.run()
	return nil
}

func (c *Client) run() {
	retries := 0
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// 建连（带10s超时）
		dctx, dcancel := context.WithTimeout(c.ctx, 10*time.Second)
		conn, err := c.dial(dctx)
		dcancel()
		if err != nil {
			log.Printf("dial failed: %v", err)
			if !c.cfg.WS.Reconnect.Enabled {
				return
			}
			retries++
			if c.cfg.WS.Reconnect.MaxRetries > 0 && retries > c.cfg.WS.Reconnect.MaxRetries {
				return
			}
			backoff := backoffTime(c.cfg, retries)
			log.Printf("retrying in %s", backoff)
			t := time.NewTimer(backoff)
			select {
			case <-c.ctx.Done():
				return
			case <-t.C:
			}
			continue
		}

		log.Println("ws connected")
		retries = 0

		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()

		// 启动心跳 goroutine；读循环结束时会取消
		hbCtx, hbCancel := context.WithCancel(c.ctx)
		go heartbeat(hbCtx, conn, c.cfg.Heartbeat())

		// 进入读循环
		err = c.readLoop(conn)

		// 读循环退出：停止心跳
		hbCancel()
		log.Printf("read loop ended: %v", err)

		// 清理连接
		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close(websocket.StatusNormalClosure, "reconnect")
			c.conn = nil
		}
		c.mu.Unlock()

		if !c.cfg.WS.Reconnect.Enabled || errors.Is(err, context.Canceled) {
			return
		}
	}
}

// 独立心跳循环：每隔 interval 发送 Ping，并等待 Pong（5s）
func heartbeat(ctx context.Context, conn *websocket.Conn, interval time.Duration) {
	if interval <= 0 {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				// 让读循环去感知到错误并触发重连；这里仅退出
				log.Printf("heartbeat ping failed: %v", err)
				return
			}
		}
	}
}

func (c *Client) readLoop(conn *websocket.Conn) error {
	// 可选：限制消息大小
	conn.SetReadLimit(4 << 20)

	for {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		default:
			// 用带超时的 Context 做读超时；到时继续读，不立即断开
			readCtx := c.ctx
			var cancel context.CancelFunc
			if d := c.cfg.ReadTimeout(); d > 0 {
				readCtx, cancel = context.WithTimeout(c.ctx, d)
			}

			msgType, data, err := conn.Read(readCtx)

			if cancel != nil {
				cancel()
			}
			if err != nil {
				// 读超时：继续下一轮，保持连接由心跳去探活
				if errors.Is(err, context.DeadlineExceeded) {
					continue
				}
				return err
			}
			if msgType == websocket.MessageText || msgType == websocket.MessageBinary {
				if c.onMsg != nil {
					c.onMsg(data)
				}
			}
		}
	}
}

func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close(websocket.StatusNormalClosure, "app closed")
	}
	return nil
}

func backoffTime(cfg config.Config, retries int) time.Duration {
	b := float64(cfg.WS.Reconnect.BaseSeconds)
	m := float64(cfg.WS.Reconnect.MaxSeconds)
	d := time.Duration(math.Min(m, b*math.Pow(2, float64(retries-1)))) * time.Second
	if d <= 0 {
		d = 1 * time.Second
	}
	return d
}
