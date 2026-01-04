package orm

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type Time struct {
	time.Time
}

var _ json.Unmarshaler = &Time{}

// UnmarshalJSON implements json.Unmarshaler.
func (t *Time) UnmarshalJSON(b []byte) error {
	l := len(b)
	s := unsafe.String(unsafe.SliceData(b), l)
	if v, err := strconv.Atoi(s); err == nil {
		switch l {
		case 1:
			*t = Time{}
		case 10:
			*t = Time{time.Unix(int64(v), 0)}
		case 13:
			*t = Time{time.UnixMilli(int64(v))}
		default:
			return json.Unmarshal(b, &t.Time)
		}
		return nil
	}

	str := strings.Trim(s, `"`)
	if str == "" {
		*t = Time{}
		return nil
	}

	date, err := time.ParseInLocation(time.DateTime, str, time.Local)
	if err == nil {
		t.Time = date
		return nil
	}
	return json.Unmarshal(b, &t.Time)
}

func Now() Time {
	return Time{time.Now()}
}

// ParseTimeToLayout 解析字符串对应的 layout
// 仅支持 年-月-日 或 年/月/日 等这种格式
func ParseTimeToLayout(value string) string {
	var layout string
	// 拼凑日期
	if len(value) >= 7 {
		layout += fmt.Sprintf("2006%c01%c02", value[4], value[7])
	}
	// 拼凑时间
	if len(value) >= 19 {
		layout += " 15:04:05"
	}
	if len(value) > 19 {
		suffix := value[19:]
		var rear strings.Builder
		for _, c := range suffix {
			if c == '.' {
				rear.WriteString(".")
			} else if c == '+' || c == '-' {
				rear.WriteString("-07:00")
				break
			} else {
				rear.WriteString("9")
			}
		}
		return layout + rear.String()
	}
	return layout
}

// Scan implements scaner
func (t *Time) Scan(input any) error {
	var date time.Time
	switch value := input.(type) {
	case time.Time:
		date = value
	// 兼容 sqlite，其存储是字符串
	case string:
		layout := ParseTimeToLayout(value)
		d, err := time.Parse(layout, value)
		if err != nil {
			return fmt.Errorf("pkg: can not convert %v to timestamptz layout[%s]", input, layout)
		}
		date = d
	default:
		return fmt.Errorf("pkg: can not convert %v to timestamptz", input)
	}
	*t = Time{Time: date}
	return nil
}

func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.Time.Format(time.DateTime) + `"`), nil
}

func (t Time) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil // nolint
	}
	return t.Time, nil
}
