package store

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/suconghou/cachelayer/util"

	"github.com/tidwall/gjson"
	bolt "go.etcd.io/bbolt"
)

var (
	db   *bolt.DB
	bTTL = []byte("ttl")
)

// Init create db file or init , with ttl
func Init(dbfile string) error {
	var err error
	db, err = bolt.Open(dbfile, 0666, &bolt.Options{Timeout: 1 * time.Second})
	return err
}

func Set(b1, key, value []byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(b1)
		if err != nil {
			return err
		}
		return b.Put(key, value)
	})
}

func Set2(b1, b2, key, value []byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(b1)
		if err != nil {
			return err
		}
		bb, err := b.CreateBucketIfNotExists(b2)
		if err != nil {
			return err
		}
		return bb.Put(key, value)
	})
}

func TTLSet(b1, key, value []byte, ttl int64) error {
	if ttl <= 0 {
		return db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists(b1)
			if err != nil {
				return err
			}
			if err = b.Put(key, value); err != nil {
				return err
			}
			bb := tx.Bucket(bTTL)
			if bb == nil {
				return nil
			}
			return bb.Delete(bytes.Join([][]byte{b1, key}, []byte(":")))
		})
	}
	tt, err := json.Marshal([]any{util.NOW + ttl, string(b1), string(key)})
	if err != nil {
		return err
	}
	return db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bTTL)
		if err != nil {
			return err
		}
		if err = b.Put(bytes.Join([][]byte{b1, key}, []byte(":")), tt); err != nil {
			return err
		}
		bb, err := tx.CreateBucketIfNotExists(b1)
		if err != nil {
			return err
		}
		return bb.Put(key, value)
	})
}

func TTLSet2(b1, b2, key, value []byte, ttl int64) error {
	if ttl <= 0 {
		return db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists(b1)
			if err != nil {
				return err
			}
			bb, err := b.CreateBucketIfNotExists(b2)
			if err != nil {
				return err
			}
			if err = bb.Put(key, value); err != nil {
				return err
			}
			bbb := tx.Bucket(bTTL)
			if bbb == nil {
				return nil
			}
			return bbb.Delete(bytes.Join([][]byte{b1, b2, key}, []byte(":")))
		})
	}
	tt, err := json.Marshal([]any{util.NOW + ttl, string(b1), string(b2), string(key)})
	if err != nil {
		return err
	}
	return db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bTTL)
		if err != nil {
			return err
		}
		if err = b.Put(bytes.Join([][]byte{b1, b2, key}, []byte(":")), tt); err != nil {
			return err
		}
		bb, err := tx.CreateBucketIfNotExists(b1)
		if err != nil {
			return err
		}
		bbb, err := bb.CreateBucketIfNotExists(b2)
		if err != nil {
			return err
		}
		return bbb.Put(key, value)
	})
}

func Get(b1, key []byte) []byte {
	var value []byte
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(b1)
		if b == nil {
			return nil
		}
		v := b.Get(key)
		if v == nil {
			return nil
		}
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})
	return value
}

func Get2(b1, b2, key []byte) []byte {
	var value []byte
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(b1)
		if b == nil {
			return nil
		}
		bb := b.Bucket(b2)
		if bb == nil {
			return nil
		}
		v := bb.Get(key)
		if v == nil {
			return nil
		}
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})
	return value
}

func Del(b1 []byte, keys [][]byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		if keys == nil {
			return tx.DeleteBucket(b1)
		}
		b := tx.Bucket(b1)
		if b == nil {
			return nil
		}
		for _, key := range keys {
			if err := b.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

func Del2(b1, b2 []byte, keys [][]byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(b1)
		if b == nil {
			return nil
		}
		if keys == nil {
			return b.DeleteBucket(b2)
		}
		bb := b.Bucket(b2)
		if bb == nil {
			return nil
		}
		for _, key := range keys {
			if err := bb.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

func ForEach(b1 []byte, fn func(key, value []byte) error) error {
	return db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(b1)
		if b == nil {
			return nil
		}
		return b.ForEach(fn)
	})
}

// 遍历2级bucket
func ForEach2(b1 []byte, fn1 func(k1, v1 []byte) error, fn2 func(b2, k2, v2 []byte) error) error {
	return db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(b1)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			if v == nil {
				bb := b.Bucket(k)
				if bb == nil {
					return nil
				}
				return bb.ForEach(func(kk, vv []byte) error {
					return fn2(k, kk, vv)
				})
			} else {
				return fn1(k, v)
			}
		})
	})
}

func Expire() error {
	var (
		t        = util.NOW
		keys     = [][]byte{}
		toDelete = make([][]gjson.Result, 0)
		addKeys  = func(k []byte) {
			key := make([]byte, len(k))
			copy(key, k)
			keys = append(keys, key)
		}
		iterate = func(k, v []byte) error {
			j := gjson.ParseBytes(v).Array()
			l := len(j)
			if l < 3 || l > 4 {
				// 不合法的数据，删除这个键值
				addKeys(k)
				return nil
			}
			if j[0].Int() > t {
				return nil
			}
			addKeys(k)
			toDelete = append(toDelete, j)
			return nil
		}
	)
	err := ForEach(bTTL, iterate)
	if err != nil {
		return err
	}
	for _, j := range toDelete {
		if len(j) == 3 {
			if err = Del([]byte(j[1].Str), [][]byte{[]byte(j[2].Str)}); err != nil {
				return err
			}
		} else {
			if err = Del2([]byte(j[1].Str), []byte(j[2].Str), [][]byte{[]byte(j[3].Str)}); err != nil {
				return err
			}
		}
	}
	return Del(bTTL, keys)
}
