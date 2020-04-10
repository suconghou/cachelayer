package store

import (
	"fmt"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	dbInstance *bolt.DB
)

// Init create db file or init , with ttl
func Init(dbfile string) error {
	var err error
	dbInstance, err = bolt.Open(dbfile, 0666, &bolt.Options{Timeout: 1 * time.Second})
	return err
}

// GetCache return nil if key not exist
func GetCache(key string) []byte {
	return Get([]byte("data"), []byte(key))
}

// SetCache set ttl cache
func SetCache(key string, value []byte) error {
	return TTLSet([]byte(key), value, 86400)
}

// TTLSet key and ttl , set ttl first and the set the real key value
func TTLSet(key []byte, value []byte, ttl int64) error {
	err := Set([]byte("ttl"), key, []byte(fmt.Sprintf("%d", time.Now().Unix()+ttl)))
	if err == nil {
		return Set([]byte("data"), key, value)
	}
	return err
}

// Set key and ttl
func Set(bucket []byte, key []byte, value []byte) error {
	return dbInstance.Update(func(tx *bolt.Tx) error {
		bu, err := tx.CreateBucketIfNotExists(bucket)
		if err != nil {
			return err
		}
		err = bu.Put(key, value)
		if err != nil {
			return err
		}
		return nil
	})
}

// Get from bucket
func Get(bucket []byte, key []byte) []byte {
	var value []byte
	dbInstance.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		if b != nil {
			value = b.Get(key)
		}
		return nil
	})
	return value
}

// Del delete the bucket or key in bucket
func Del(bucket []byte, keys [][]byte) error {
	return dbInstance.Update(func(tx *bolt.Tx) error {
		if keys == nil {
			return tx.DeleteBucket(bucket)
		}
		bu := tx.Bucket(bucket)
		if bu == nil {
			return nil
		}
		for _, key := range keys {
			if err := bu.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

// Expire check expire keys in bucket data
func Expire() error {
	var (
		ttl    = []byte("ttl")
		bucket = []byte("data")
		t      = time.Now().Unix()
		keys   = [][]byte{}
	)
	err := dbInstance.View(func(tx *bolt.Tx) error {
		bu := tx.Bucket(ttl)
		if bu == nil {
			return nil
		}
		c := bu.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			tt, err := strconv.ParseInt(string(v), 10, 64)
			if err != nil {
				return err
			}
			if tt > t {
				continue
			}
			keys = append(keys, k)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err = Del(bucket, keys); err != nil {
		return err
	}
	return Del(ttl, keys)
}
