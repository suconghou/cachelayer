package layer

import "github.com/suconghou/cachelayer/store"

var (
	bData = []byte("data")
)

func CacheSet(key []byte, value []byte, ttl int64) error {
	return store.TTLSet(bData, key, value, ttl)
}
func CacheGet(key []byte) []byte {
	return store.Get(bData, key)
}
