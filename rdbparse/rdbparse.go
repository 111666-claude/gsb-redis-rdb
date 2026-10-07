// Package rdbparse 解析 Redis RDB。
package rdbparse

import (
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

// length 解析长度编码：返回长度、是否为整数特例、是否成功。
func (r *reader) length(c *Counter) (int, bool, bool) {
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
		var n uint32
		for i := 0; i < 4; i++ {
			next, ok := r.byteAt(c)
			if !ok {
				return 0, false, false
			}
			n = n<<8 | uint32(next)
		}
		return int(n), false, true
	default:
		return int(b & 0x3f), true, true
	}
}

// encodedInt 按整数特例读出有符号小端整数，以十进制串呈现。
func (r *reader) encodedInt(c *Counter, enc int) (string, bool) {
	var size int
	switch enc {
	case 0:
		size = 1
	case 1:
		size = 2
	case 2:
		size = 4
	default:
		return "", false
	}
	var n int64
	for i := 0; i < size; i++ {
		b, ok := r.byteAt(c)
		if !ok {
			return "", false
		}
		n |= int64(b) << (8 * i)
	}
	switch size {
	case 1:
		n = int64(int8(n))
	case 2:
		n = int64(int16(n))
	case 4:
		n = int64(int32(n))
	}
	return strconv.FormatInt(n, 10), true
}

func (r *reader) stringValue(c *Counter) (string, bool) {
	length, encoded, ok := r.length(c)
	if !ok {
		return "", false
	}
	if encoded {
		return r.encodedInt(c, length)
	}
	if r.pos+length > len(r.data) {
		return "", false
	}
	value := string(r.data[r.pos : r.pos+length])
	r.pos += length
	return value, true
}

func (r *reader) uintLE(c *Counter, size int) (int64, bool) {
	var n int64
	for i := 0; i < size; i++ {
		b, ok := r.byteAt(c)
		if !ok {
			return 0, false
		}
		n |= int64(b) << (8 * i)
	}
	return n, true
}

// Parse 解析整份文件。
func Parse(data []byte, c *Counter) (*RDB, error) {
	if len(data) < 9 || string(data[:5]) != "REDIS" {
		return nil, errors.New("bad-magic")
	}
	rdb := &RDB{Aux: map[string]string{}}
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
			ms, ok := r.uintLE(c, 8)
			if !ok {
				return rdb, nil
			}
			expireAt = ms
		case 0xfd:
			sec, ok := r.uintLE(c, 4)
			if !ok {
				return rdb, nil
			}
			expireAt = sec * 1000
		case 0xfe:
			n, encoded, ok := r.length(c)
			if !ok || encoded {
				return rdb, nil
			}
			db = n
		default:
			key, ok1 := r.stringValue(c)
			value, ok2 := r.stringValue(c)
			if !ok1 || !ok2 {
				return rdb, nil
			}
			rdb.Entries = append(rdb.Entries, Entry{DB: db, Key: key, Value: value, ExpireAt: expireAt})
			expireAt = 0
		}
	}
	return rdb, nil
}

// Get 按键取条目。
func (rdb *RDB) Get(key string, c *Counter) *Entry {
	c.Scanned++
	if rdb.index == nil {
		rdb.index = make(map[string]int, len(rdb.Entries))
		for i := range rdb.Entries {
			if _, ok := rdb.index[rdb.Entries[i].Key]; !ok {
				rdb.index[rdb.Entries[i].Key] = i
			}
		}
	}
	i, ok := rdb.index[key]
	if !ok {
		return nil
	}
	return &rdb.Entries[i]
}
