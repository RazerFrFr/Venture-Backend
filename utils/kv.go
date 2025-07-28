package utils

import (
	"sync"
	"time"
)

type entry struct {
	value      interface{}
	expiration int64
}

type KV struct {
	data map[string]entry
	mu   sync.RWMutex
}

func NewKV() *KV {
	kv := &KV{data: make(map[string]entry)}
	go kv.cleanup()
	return kv
}

func (k *KV) Get(key string) (interface{}, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	e, ok := k.data[key]
	if !ok || (e.expiration > 0 && time.Now().UnixNano() > e.expiration) {
		return nil, false
	}
	return e.value, true
}

func (k *KV) Set(key string, value interface{}) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.data[key] = entry{value: value, expiration: 0}
}

func (k *KV) SetTTL(key string, value interface{}, ttl time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.data[key] = entry{value: value, expiration: time.Now().Add(ttl).UnixNano()}
}

func (k *KV) cleanup() {
	for {
		time.Sleep(time.Minute)
		k.mu.Lock()
		now := time.Now().UnixNano()
		for kKey, kVal := range k.data {
			if kVal.expiration > 0 && now > kVal.expiration {
				delete(k.data, kKey)
			}
		}
		k.mu.Unlock()
	}
}

var Store = NewKV()
