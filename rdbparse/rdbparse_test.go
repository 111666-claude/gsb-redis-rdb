package rdbparse

import "testing"

func TestHeader(t *testing.T) {
	rdb, err := Parse(append(header(), 0xff), &Counter{})
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if rdb.Version != 11 {
		t.Fatalf("版本应是 11：%d", rdb.Version)
	}
}

func TestBadMagic(t *testing.T) {
	if _, err := Parse([]byte("XXXX0011"), &Counter{}); err == nil {
		t.Fatal("魔数不对应报错")
	}
}

func TestSingleEntry(t *testing.T) {
	data := append(header(), 0x00)
	data = append(data, EncodeString("k")...)
	data = append(data, EncodeString("v")...)
	data = append(data, 0xff)
	rdb, _ := Parse(data, &Counter{})
	if len(rdb.Entries) != 1 || rdb.Entries[0].Key != "k" {
		t.Fatalf("条目不对：%+v", rdb.Entries)
	}
}

func TestShortValue(t *testing.T) {
	data := append(header(), 0x00, 0x05, 'a')
	rdb, _ := Parse(data, &Counter{})
	if len(rdb.Entries) != 0 {
		t.Fatalf("长度超出不该产生条目：%+v", rdb.Entries)
	}
}

func TestAuxParsed(t *testing.T) {
	data := append(header(), 0xfa)
	data = append(data, EncodeString("redis-ver")...)
	data = append(data, EncodeString("7.0")...)
	data = append(data, 0xff)
	rdb, _ := Parse(data, &Counter{})
	if rdb.Aux["redis-ver"] != "7.0" {
		t.Fatalf("AUX 不对：%+v", rdb.Aux)
	}
}

func TestCounterCounts(t *testing.T) {
	c := &Counter{}
	Parse(append(header(), 0xff), c)
	if c.Scanned == 0 {
		t.Fatal("应统计比较次数")
	}
}
