package layer

import (
	"encoding/json"
	"fmt"
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

func CacheSet(key []byte, value []byte, ttl int64) error {
	return store.TTLSet(bData, key, value, ttl)
}
func CacheGet(key []byte) []byte {
	return store.Get(bData, key)
}

func LoadMeta(key []byte) (*ObjectMeta, error) {
	b := store.Get(bData, key)
	if len(b) < 2 {
		return nil, fmt.Errorf("meta %s not found", key)
	}
	var om *ObjectMeta
	return om, json.Unmarshal(b, &om)
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
