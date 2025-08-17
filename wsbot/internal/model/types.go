package model

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type Message struct {
	SelfID      string
	UserID      string
	Text        string
	Timestamp   time.Time
	MessageType string // "group" / "private"
	GroupID     string // 群消息才有
	FromSelf    bool   // 是否机器人自己发的消息（需要忽略）
}

// OneBot v11 事件（NapCat）
type onebotV11 struct {
	SelfID      *int64          `json:"self_id"`
	Time        int64           `json:"time"`
	PostType    string          `json:"post_type"`
	MessageType string          `json:"message_type"`
	SubType     string          `json:"sub_type"`
	UserID      *int64          `json:"user_id"`
	GroupID     *int64          `json:"group_id"`
	Message     json.RawMessage `json:"message"`     // string 或 []segment
	RawMessage  string          `json:"raw_message"` // 兜底
	Sender      *struct {
		UserID *int64 `json:"user_id"`
	} `json:"sender"`
}

// 内部通用
type internalMsg struct {
	UserID    string `json:"user_id"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

type segment struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

func DecodeOneBotOrInternal(b []byte) (Message, bool) {
	// —— OneBot v11 (NapCat) —— //
	var ob onebotV11
	if err := json.Unmarshal(b, &ob); err == nil && ob.PostType == "message" && ob.UserID != nil {
		txt := extractText(ob.Message, ob.RawMessage)

		self := ""
		if ob.SelfID != nil {
			self = strconv.FormatInt(*ob.SelfID, 10)
		}
		fromSelf := false
		if ob.SelfID != nil {
			// 常见判断：sender.user_id == self_id 视为机器人自己发的
			if ob.Sender != nil && ob.Sender.UserID != nil && *ob.Sender.UserID == *ob.SelfID {
				fromSelf = true
			}
			// 兼容：有些实现把 user_id 也写成 self_id（更保守）
			if ob.UserID != nil && *ob.UserID == *ob.SelfID {
				fromSelf = true
			}
		}

		m := Message{
			SelfID:      self,
			UserID:      strconv.FormatInt(*ob.UserID, 10),
			Text:        txt,
			Timestamp:   time.Unix(ob.Time, 0),
			MessageType: ob.MessageType,
			FromSelf:    fromSelf,
		}
		if ob.GroupID != nil {
			m.GroupID = strconv.FormatInt(*ob.GroupID, 10)
		}
		return m, true
	}

	// —— 内部通用 —— //
	var im internalMsg
	if err := json.Unmarshal(b, &im); err == nil && im.UserID != "" && im.Text != "" {
		ts, _ := time.Parse(time.RFC3339, im.Timestamp)
		return Message{UserID: im.UserID, Text: im.Text, Timestamp: ts}, true
	}
	return Message{}, false
}

func extractText(raw json.RawMessage, fallback string) string {
	if len(raw) == 0 {
		return fallback
	}
	// 1) 尝试字符串
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return s
	}
	// 2) 尝试段数组
	var segs []segment
	if err := json.Unmarshal(raw, &segs); err == nil && len(segs) > 0 {
		var b strings.Builder
		for _, seg := range segs {
			if seg.Type == "text" {
				if t, ok := seg.Data["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		if b.Len() > 0 {
			return b.String()
		}
	}
	// 3) 兜底
	return fallback
}
