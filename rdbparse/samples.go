package rdbparse

import "fmt"

// Scenario 是一个固定场景。
type Scenario struct {
	Name string
	Run  func() map[string]any
}

func header() []byte {
	return []byte("REDIS0011")
}

// EncodeString 用 6 位或 14 位长度编码一个字符串。
func EncodeString(s string) []byte {
	n := len(s)
	if n < 64 {
		return append([]byte{byte(n)}, []byte(s)...)
	}
	out := []byte{0x40 | byte(n>>8), byte(n)}
	return append(out, []byte(s)...)
}

// Samples 返回全部场景。
func Samples() []Scenario {
	return []Scenario{
		{Name: "len14", Run: func() map[string]any {
			data := append(header(), 0x00)
			data = append(data, EncodeString("key")...)
			data = append(data, EncodeString(string(make([]byte, 100)))...)
			data = append(data, 0xff)
			rdb, _ := Parse(data, &Counter{})
			if len(rdb.Entries) == 0 {
				return map[string]any{"len": 0}
			}
			return map[string]any{"len": len(rdb.Entries[0].Value)}
		}},
		{Name: "intenc", Run: func() map[string]any {
			data := append(header(), 0x00)
			data = append(data, EncodeString("n")...)
			data = append(data, 0xc0, 0x05)
			data = append(data, 0xff)
			rdb, _ := Parse(data, &Counter{})
			if len(rdb.Entries) == 0 {
				return map[string]any{"value": ""}
			}
			return map[string]any{"value": rdb.Entries[0].Value}
		}},
		{Name: "expire", Run: func() map[string]any {
			data := append(header(), 0xfc, 0x10, 0x27, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
			data = append(data, 0x00)
			data = append(data, EncodeString("k")...)
			data = append(data, EncodeString("v")...)
			data = append(data, 0xff)
			rdb, _ := Parse(data, &Counter{})
			if len(rdb.Entries) == 0 {
				return map[string]any{"expire": 0}
			}
			return map[string]any{"expire": rdb.Entries[0].ExpireAt}
		}},
		{Name: "db", Run: func() map[string]any {
			data := append(header(), 0xfe, 0x02)
			data = append(data, 0x00)
			data = append(data, EncodeString("k")...)
			data = append(data, EncodeString("v")...)
			data = append(data, 0xff)
			rdb, _ := Parse(data, &Counter{})
			if len(rdb.Entries) == 0 {
				return map[string]any{"db": -1}
			}
			return map[string]any{"db": rdb.Entries[0].DB}
		}},
	}
}

// Work 跑规模线场景。
func Work(n int) map[string]any {
	data := append([]byte{}, header()...)
	for i := 0; i < n; i++ {
		data = append(data, 0x00)
		data = append(data, EncodeString(fmt.Sprintf("k%04d", i))...)
		data = append(data, EncodeString("v")...)
	}
	data = append(data, 0xff)
	rdb, _ := Parse(data, &Counter{})
	c := &Counter{}
	for i := 0; i < n; i++ {
		rdb.Get(fmt.Sprintf("k%04d", i), c)
	}
	return map[string]any{"keys": n, "scanned": c.Scanned}
}
