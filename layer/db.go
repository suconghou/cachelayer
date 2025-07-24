package layer

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/suconghou/cachelayer/store"
)

var (
	bData       = []byte("data")
	storeHeader = []string{"Content-Type", "Accept-Ranges"}
)

type ObjectMeta struct {
	Length int64       `json:"length"`
	Header http.Header `json:"header"`
}

// store 定义了缓存存储的接口
type CacheStore interface {
	// Set 将数据流存储到指定的 key，并设置 TTL（单位：秒）
	Set([]byte, []byte, int64) error

	// Get 从指定的 key 获取一个可读的数据流
	// 返回的 io.ReadCloser 在读取完毕后应该被关闭
	Get([]byte) ([]byte, error)

	// Has 检查指定的 key 是否存在于缓存中,并延长有效期
	Has([]byte, int64) bool
}

type kvstore struct {
	baseKey []byte
}

func (k *kvstore) Set(key []byte, b []byte, ttl int64) error {
	return store.TTLSet(bData, bytes.Join([][]byte{k.baseKey, key}, []byte(":")), b, ttl)
}

func (k *kvstore) Get(key []byte) ([]byte, error) {
	return store.Get(bData, bytes.Join([][]byte{k.baseKey, key}, []byte(":")))
}

func (k *kvstore) Has(key []byte, ttl int64) bool {
	v, err := store.Touch(bData, bytes.Join([][]byte{k.baseKey, key}, []byte(":")), ttl)
	return v && err == nil
}

func NewCacheStore(baseKey []byte) CacheStore {
	return &kvstore{baseKey}
}

func LoadMeta(key []byte) (*ObjectMeta, error) {
	b, err := store.Get(bData, key)
	if err != nil {
		return nil, err
	}
	if len(b) < 2 { // " {} " 是最小的有效 JSON 对象
		return nil, nil
	}
	var om ObjectMeta
	return &om, json.Unmarshal(b, &om)
}

func SetMeta(key []byte, ll int64, h http.Header, ttl int64) (*ObjectMeta, error) {
	var m = http.Header{}
	for _, k := range storeHeader {
		if v := h.Get(k); v != "" {
			m.Set(k, v)
		}
	}
	var om = &ObjectMeta{
		Length: ll,
		Header: m,
	}
	bs, err := json.Marshal(om)
	if err != nil {
		return om, err
	}
	return om, store.TTLSet(bData, key, bs, ttl)
}
