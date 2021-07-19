package storage

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

//TODO:
//    * добавить поддержку репликации

type Item struct {
	Object     interface{}
	Expiration int64
}

func (item *Item) Expired() bool {
	//Item never expires when its Expiration == 0
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration
}

const (
	NoExpiration      time.Duration = -1
	DefaultExpiration time.Duration = 0
)

type Storage struct {
	filePath          string
	defaultExpiration time.Duration
	items             map[string]Item
	mu                sync.RWMutex
	janitor           *janitor
}

//If the duration is 0, default expiration time is used.
//If it is -1, item never expires.
func (s *Storage) Set(key string, value interface{}, duration time.Duration) {
	if duration == DefaultExpiration {
		duration = s.defaultExpiration
	}
	var exp int64
	if duration > 0 {
		exp = time.Now().Add(duration).UnixNano()
	}

	s.mu.Lock()

	s.items[key] = Item{
		Object:     value,
		Expiration: exp,
	}

	s.mu.Unlock()
}

func (s *Storage) set(key string, value interface{}, duration time.Duration) {
	if duration == DefaultExpiration {
		duration = s.defaultExpiration
	}
	var exp int64
	if duration > 0 {
		exp = time.Now().Add(duration).UnixNano()
	}

	s.items[key] = Item{
		Object:     value,
		Expiration: exp,
	}
}

func (s *Storage) Add(key string, value interface{}, duration time.Duration) error {
	s.mu.Lock()
	_, found := s.items[key]
	if found {
		s.mu.Unlock()
		return fmt.Errorf("item %s already exists", key)
	}

	s.set(key, value, duration)
	s.mu.Unlock()
	return nil
}

func (s *Storage) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.items[key]
	if ok {
		delete(s.items, key)
	}
	return ok
}

func (s *Storage) Get(key string) (interface{}, bool) {
	s.mu.RLock()

	item, found := s.items[key]
	if !found {
		s.mu.RUnlock()
		return nil, false
	}

	if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		s.mu.RUnlock()
		return nil, false
	}

	s.mu.RUnlock()
	return item.Object, true
}

func (s *Storage) Items() map[string]Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := make(map[string]Item)
	now := time.Now().UnixNano()
	for k, v := range s.items {
		if v.Expiration > 0 && now > v.Expiration {
			continue
		}
		m[k] = v
	}
	return m
}

func (s *Storage) ItemCount() int {
	s.mu.RLock()
	n := len(s.items)
	s.mu.RUnlock()
	return n
}

func (s *Storage) DeleteExpired() {
	now := time.Now().UnixNano()
	s.mu.Lock()
	for k, v := range s.items {
		if v.Expiration > 0 && now > v.Expiration {
			delete(s.items, k)
		}
	}
	s.mu.Unlock()
}

type janitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *janitor) Run(s *Storage) {
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			s.DeleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopJanitor(s *Storage) {
	s.janitor.stop <- true
}

func runJanitor(s *Storage, interval time.Duration) {
	j := &janitor{
		Interval: interval,
		stop:     make(chan bool),
	}
	s.janitor = j
	go j.Run(s)
}

func newStorage(de time.Duration, m map[string]Item) *Storage {
	//if defaultExpiration is not provided, set it to NoExpiration
	if de == 0 {
		de = NoExpiration
	}

	s := &Storage{
		defaultExpiration: de,
		items:             m,
	}

	return s
}

func newsWithJanitor(de, ci time.Duration, m map[string]Item) *Storage {
	s := newStorage(de, m)
	if ci > 0 {
		runJanitor(s, ci)
		runtime.SetFinalizer(s, stopJanitor)
	}
	return s
}

func New(defaultExpiration, cleanupInterval time.Duration, DBSize int) *Storage {
	items := make(map[string]Item, DBSize)
	return newsWithJanitor(defaultExpiration, cleanupInterval, items)
}

func (s *Storage) Save(w io.Writer) error {
	enc := gob.NewEncoder(w)
	m := s.Items()
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range m {
		gob.Register(v.Object)
	}
	err := enc.Encode(&m)
	return err
}

func (s *Storage) SaveFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	err = s.Save(f)
	if err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (s *Storage) Load(r io.Reader) error {
	dec := gob.NewDecoder(r)
	items := map[string]Item{}
	err := dec.Decode(&items)
	if err == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		for k, v := range items {
			s.items[k] = v
		}
	}
	return err
}

func (s *Storage) LoadFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	err = s.Load(f)
	if err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
