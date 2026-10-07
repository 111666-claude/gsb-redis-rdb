// Package rdbparse 解析 Redis RDB。
package rdbparse

import (
	"encoding/binary"
	"errors"
	"strconv"
)

// Counter 记录比较次数。
type Counter struct {
	Scanned int
}

// Entry 是一条键值。
type Entry struct {
	DB       int
	Key      string
	Value    string
	ExpireAt int64
}

// RDB 是解析结果。
type RDB struct {
	Version int
	Aux     map[string]string
	Entries []Entry
	index   map[string]int
}

type reader struct {
	data []byte
	pos  int
}

func (r *reader) byteAt(c *Counter) (byte, bool) {
	c.Scanned++
	if r.pos >= len(r.data) {
		return 0, false
	}
	b := r.data[r.pos]
	r.pos++
	return b, true
}

func (r *reader) bytes(c *Counter, n int) ([]byte, bool) {
	c.Scanned++
	if r.pos+n > len(r.data) {
		return nil, false
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, true
}

// length 读长度编码，isSpecial 为真时表示整数特例。
func (r *reader) length(c *Counter) (length int, isSpecial bool, ok bool) {
	b, ok := r.byteAt(c)
	if !ok {
		return 0, false, false
	}
	switch b >> 6 {
	case 0x00:
		return int(b & 0x3f), false, true
	case 0x01:
		next, ok := r.byteAt(c)
		if !ok {
			return 0, false, false
		}
		return int(b&0x3f)<<8 | int(next), false, true
	case 0x02:
		raw, ok := r.bytes(c, 4)
		if !ok {
			return 0, false, false
		}
		return int(binary.BigEndian.Uint32(raw)), false, true
	default:
		return int(b & 0x3f), true, true
	}
}

func (r *reader) stringValue(c *Counter) (string, bool) {
	length, isSpecial, ok := r.length(c)
	if !ok {
		return "", false
	}
	if isSpecial {
		return r.integerValue(c, length)
	}
	raw, ok := r.bytes(c, length)
	if !ok {
		return "", false
	}
	return string(raw), true
}

// integerValue 解整数特例：C0/C1/C2 小端有符号，十进制呈现。
func (r *reader) integerValue(c *Counter, enc int) (string, bool) {
	switch enc {
	case 0:
		raw, ok := r.bytes(c, 1)
		if !ok {
			return "", false
		}
		return strconv.FormatInt(int64(int8(raw[0])), 10), true
	case 1:
		raw, ok := r.bytes(c, 2)
		if !ok {
			return "", false
		}
		return strconv.FormatInt(int64(int16(binary.LittleEndian.Uint16(raw))), 10), true
	case 2:
		raw, ok := r.bytes(c, 4)
		if !ok {
			return "", false
		}
		return strconv.FormatInt(int64(int32(binary.LittleEndian.Uint32(raw))), 10), true
	default:
		return "", false
	}
}

// Parse 解析整份文件。
func Parse(data []byte, c *Counter) (*RDB, error) {
	if len(data) < 9 || string(data[:5]) != "REDIS" {
		return nil, errors.New("bad-magic")
	}
	rdb := &RDB{Aux: map[string]string{}, index: map[string]int{}}
	rdb.Version, _ = strconv.Atoi(string(data[5:9]))
	r := &reader{data: data, pos: 9}
	db := 0
	var expireAt int64
	for r.pos < len(data) {
		op, ok := r.byteAt(c)
		if !ok {
			break
		}
		switch op {
		case 0xff:
			return rdb, nil
		case 0xfa:
			key, ok1 := r.stringValue(c)
			value, ok2 := r.stringValue(c)
			if ok1 && ok2 {
				rdb.Aux[key] = value
			}
		case 0xfc:
			raw, ok := r.bytes(c, 8)
			if !ok {
				return rdb, nil
			}
			expireAt = int64(binary.LittleEndian.Uint64(raw))
		case 0xfd:
			raw, ok := r.bytes(c, 4)
			if !ok {
				return rdb, nil
			}
			expireAt = int64(binary.LittleEndian.Uint32(raw)) * 1000
		case 0xfe:
			n, isSpecial, ok := r.length(c)
			if !ok || isSpecial {
				return rdb, nil
			}
			db = n
		default:
			key, ok1 := r.stringValue(c)
			value, ok2 := r.stringValue(c)
			if !ok1 || !ok2 {
				return rdb, nil
			}
			rdb.index[key] = len(rdb.Entries)
			rdb.Entries = append(rdb.Entries, Entry{DB: db, Key: key, Value: value, ExpireAt: expireAt})
			expireAt = 0
		}
	}
	return rdb, nil
}

// Get 按键取条目。
func (rdb *RDB) Get(key string, c *Counter) *Entry {
	c.Scanned++
	i, ok := rdb.index[key]
	if !ok {
		return nil
	}
	return &rdb.Entries[i]
}
