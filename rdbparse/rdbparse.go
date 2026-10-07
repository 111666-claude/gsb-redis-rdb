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

func (r *reader) length(c *Counter) (int, bool) {
	b, ok := r.byteAt(c)
	if !ok {
		return 0, false
	}
	return int(b & 0x3f), true
}

func (r *reader) stringValue(c *Counter) (string, bool) {
	length, ok := r.length(c)
	if !ok || r.pos+length > len(r.data) {
		return "", false
	}
	value := string(r.data[r.pos : r.pos+length])
	r.pos += length
	return value, true
}

// Parse 解析整份文件。
func Parse(data []byte, c *Counter) (*RDB, error) {
	if len(data) < 9 || string(data[:5]) != "REDIS" {
		return nil, errors.New("bad-magic")
	}
	rdb := &RDB{Aux: map[string]string{}}
	rdb.Version, _ = strconv.Atoi(string(data[5:9]))
	r := &reader{data: data, pos: 9}
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
		default:
			key, ok1 := r.stringValue(c)
			value, ok2 := r.stringValue(c)
			if !ok1 || !ok2 {
				return rdb, nil
			}
			rdb.Entries = append(rdb.Entries, Entry{Key: key, Value: value})
		}
	}
	return rdb, nil
}

// Get 按键取条目。
func (rdb *RDB) Get(key string, c *Counter) *Entry {
	for i := range rdb.Entries {
		c.Scanned++
		if rdb.Entries[i].Key == key {
			return &rdb.Entries[i]
		}
	}
	return nil
}
