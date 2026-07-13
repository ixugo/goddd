package orm

import (
	"encoding/json"
	"testing"
	"time"
)

type Date struct {
	Date Time `json:"date"`
}

func TestUnmarshalJSON(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		dateVal any
		check   func(t *testing.T, d Date)
	}{
		{
			name:    "数字0反序列化为零值时间",
			dateVal: 0,
			check: func(t *testing.T, d Date) {
				if !d.Date.Time.IsZero() {
					t.Errorf("期望零值时间, got %v", d.Date.Time)
				}
			},
		},
		{
			name:    "null反序列化为零值时间",
			dateVal: nil,
			check: func(t *testing.T, d Date) {
				if !d.Date.Time.IsZero() {
					t.Errorf("期望零值时间, got %v", d.Date.Time)
				}
			},
		},
		{
			name:    "Unix秒时间戳正确解析",
			dateVal: now.Unix(),
			check: func(t *testing.T, d Date) {
				expect := time.Unix(now.Unix(), 0)
				if !d.Date.Time.Equal(expect) {
					t.Errorf("期望 %v, got %v", expect, d.Date.Time)
				}
			},
		},
		{
			name:    "Unix毫秒时间戳正确解析",
			dateVal: now.UnixMilli(),
			check: func(t *testing.T, d Date) {
				expect := time.UnixMilli(now.UnixMilli())
				if !d.Date.Time.Equal(expect) {
					t.Errorf("期望 %v, got %v", expect, d.Date.Time)
				}
			},
		},
		{
			name:    "DateTime格式字符串正确解析",
			dateVal: now.Format(time.DateTime),
			check: func(t *testing.T, d Date) {
				expect, _ := time.ParseInLocation(time.DateTime, now.Format(time.DateTime), time.Local)
				if !d.Date.Time.Equal(expect) {
					t.Errorf("期望 %v, got %v", expect, d.Date.Time)
				}
			},
		},

		{
			name:    "字符串毫秒时间戳正确解析",
			dateVal: "1767196800000",
			check: func(t *testing.T, d Date) {
				millis := int64(1767196800000)
				expect := time.Unix(0, millis*int64(time.Millisecond))
				if !d.Date.Time.Equal(expect) {
					t.Errorf("期望 %v, got %v", expect, d.Date.Time)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(map[string]any{"date": tt.dateVal})
			var date Date
			if err := json.Unmarshal(b, &date); err != nil {
				t.Fatalf("unmarshal err: %v", err)
			}
			tt.check(t, date)
		})
	}
}
