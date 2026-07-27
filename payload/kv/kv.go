package kv

import (
	"fmt"
	"sync"

	"github.com/negasus/haproxy-spoe-go/typeddata"
	"github.com/negasus/haproxy-spoe-go/varint"
)

var kvPool = sync.Pool{
	New: func() interface{} {
		return NewKV()
	},
}

func AcquireKV() *KV {
	return kvPool.Get().(*KV)
}

func ReleaseKV(kv *KV) {
	kv.Reset()
	kvPool.Put(kv)
}

type Item struct {
	Name  string
	Value interface{}
}

type KV struct {
	m   []Item
	tmp []byte
}

func NewKV() *KV {
	kv := &KV{
		m:   make([]Item, 0),
		tmp: make([]byte, 10),
	}

	return kv
}

func (kv *KV) Data() []Item {
	return kv.m
}

func (kv *KV) Reset() {
	kv.m = kv.m[:0]
}

func (kv *KV) Add(key string, value interface{}) {
	kv.m = append(kv.m, Item{key, value})
}

func (kv *KV) Get(key string) (interface{}, bool) {
	for i := range kv.m {
		if kv.m[i].Name == key {
			return kv.m[i].Value, true
		}
	}

	return nil, false
}

func (kv *KV) Bytes() ([]byte, error) {
	buf := make([]byte, 0)

	for _, item := range kv.m {
		n := varint.PutUvarint(kv.tmp, uint64(len(item.Name)))
		if n <= 0 {
			return nil, fmt.Errorf("error encode KV key length")
		}
		buf = append(buf, kv.tmp[:n]...)
		buf = append(buf, item.Name...)

		data, n, err := typeddata.Encode(item.Value, make([]byte, 0))
		if err != nil {
			return nil, err
		}

		buf = append(buf, data[:n]...)
	}

	return buf, nil
}

func (kv *KV) Unmarshal(buf []byte) error {
	var key string
	var value interface{}
	var n int
	var err error
	var keyLen uint64

	for {
		if len(buf) == 0 {
			break
		}

		keyLen, n = varint.Uvarint(buf)
		if n <= 0 {
			return fmt.Errorf("error unmarshal KV key length: truncated varint")
		}
		buf = buf[n:]
		if keyLen > uint64(len(buf)) {
			return fmt.Errorf("error unmarshal KV, wrong buf len. Expect %d, got %d", keyLen, len(buf))
		}

		key = string(buf[:keyLen])
		buf = buf[keyLen:]

		value, n, err = typeddata.Decode(buf)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(buf) {
			return fmt.Errorf("error unmarshal KV value: invalid byte count %d", n)
		}
		buf = buf[n:]

		kv.m = append(kv.m, Item{key, value})
	}

	return nil
}

func (kv *KV) UnmarshalNB(buf []byte, count int) (int, error) {
	if count < 0 {
		return 0, fmt.Errorf("negative kv count")
	}

	var key string
	var value interface{}
	var n int
	var err error
	var keyLen uint64

	var readBytes int

	for i := 0; i < count; i++ {
		if len(buf) == 0 {
			return readBytes, fmt.Errorf("buffer unexpectly end")
		}

		keyLen, n = varint.Uvarint(buf)
		if n <= 0 {
			return readBytes, fmt.Errorf("error unmarshal KV key length: truncated varint")
		}
		buf = buf[n:]
		readBytes += n
		if keyLen > uint64(len(buf)) {
			return readBytes, fmt.Errorf("error unmarshal KV, wrong buf len. Expect %d, got %d", keyLen, len(buf))
		}

		key = string(buf[:keyLen])
		buf = buf[keyLen:]
		readBytes += int(keyLen)

		value, n, err = typeddata.Decode(buf)
		if err != nil {
			return readBytes, err
		}
		if n <= 0 || n > len(buf) {
			return readBytes, fmt.Errorf("error unmarshal KV value: invalid byte count %d", n)
		}
		buf = buf[n:]
		readBytes += n

		kv.m = append(kv.m, Item{key, value})
	}

	return readBytes, nil
}
