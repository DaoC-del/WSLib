package main

import (
	"encoding/json"
	"log"
	"math/rand/v2"
	"net/http"
	"time"

	"nhooyr.io/websocket"
)

func main() {
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			log.Println("accept:", err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "bye")

		log.Println("client connected")

		// 读协程（可忽略）
		go func() {
			for {
				_, data, err := c.Read(r.Context())
				if err != nil {
					log.Println("read:", err)
					return
				}
				log.Println("recv:", string(data))
			}
		}()

		// 写协程：随机发“上班/下班”
		enc := json.NewEncoder(websocket.NetConn(r.Context(), c, websocket.MessageText))
		for {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Second):
				payload := map[string]any{
					"time":         time.Now().Unix(),
					"post_type":    "message",
					"message_type": "group",
					"user_id":      10000 + rand.IntN(3),
					"raw_message":  []string{"上班", "下班"}[rand.IntN(2)], // 或 "状态"
					"message":      []string{"上班", "下班"}[rand.IntN(2)],
				}
				if err := enc.Encode(payload); err != nil {
					log.Println("write:", err)
					return
				}
				log.Println("sent onebot-like message")
			}
		}
	})

	log.Println("mock ws server at http://127.0.0.1:8081/ws")
	if err := http.ListenAndServe("127.0.0.1:8081", nil); err != nil {
		log.Fatal(err)
	}
}
