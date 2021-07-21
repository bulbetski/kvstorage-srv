package storage

import (
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestStorage_Add(t *testing.T) {
	s := New(DefaultExpiration, 0, 0)
	err := s.Add("test_k", "test_v", DefaultExpiration)
	if err != nil {
		t.Error("Couldn't add key even though it shouldn't exist")
	}
	err = s.Add("test_k", "other_test_v", DefaultExpiration)
	if err == nil {
		t.Error("Successfully added another test_k when it should have returned an error")
	}
}

func TestStorage_Delete(t *testing.T) {
	s := New(DefaultExpiration, 0, 0)
	s.Set("test_k", "test_v", DefaultExpiration)
	s.Delete("test_k")
	v, found := s.Get("test_k")
	if found {
		t.Error("test_k was found, but it should have been deleted")
	}
	if v != nil {
		t.Error("value is not nil:", v)
	}
}

func TestStorage_ItemCount(t *testing.T) {
	s := New(DefaultExpiration, 0, 0)

	s.Set("1", "1", DefaultExpiration)
	s.Set("2", "2", DefaultExpiration)
	s.Set("3", "3", DefaultExpiration)

	if n := s.ItemCount(); n != 3 {
		t.Errorf("Item count is not 3: %d", n)
	}
}

func TestStorage_SaveLoadFile(t *testing.T) {
	f, err := ioutil.TempFile("", "storage.dat")
	if err != nil {
		t.Fatal("Couldn't create storage backup file")
	}
	filename := f.Name()
	f.Close()

	s := New(DefaultExpiration, 0, 0)
	s.Add("a", "a", 0)
	s.SaveFile(filename)

	s = New(DefaultExpiration, 0, 0)
	if err = s.LoadFile(filename); err != nil {
		t.Error(err)
	}
	a, found := s.Get("a")
	if !found {
		t.Error("a was not found")
	}
	if a.(string) != "a" {
		t.Error("a is not a")
	}

	defer os.Remove(filename)
}

//TODO: check if janitor clears all expired items and does it at the right time

func TestStorage_TTL(t *testing.T) {
	s := New(DefaultExpiration, 10*time.Millisecond, 0)
	s.set("k1", "v1", 7*time.Millisecond)
	s.set("k2", "v2", 15*time.Millisecond)
	s.set("k3", "v3", NoExpiration)

	_, found := s.Get("k1")
	if !found {
		t.Error("k1 not found although not expired")
	}
	//not sure if using time.Sleep() in tests is a good idea
	time.Sleep(10 * time.Millisecond)

	_, found = s.Get("k1")
	if found {
		t.Error("k1 was not cleared by janitor")
	}

	_, found = s.Get("k2")
	if !found {
		t.Error("k2 not found after first clear although not expired")
	}

	time.Sleep(10 * time.Millisecond)
	_, found = s.Get("k2")
	if found {
		t.Error("k2 was not cleared")
	}

	_, found = s.Get("k3")
	if !found {
		t.Error("k3 not found when it should never expire")
	}
}

func BenchmarkStorage_GetExpiring(b *testing.B) {
	benchmarkStorageGet(b, 5*time.Minute)
}

func BenchmarkStorage_GetNotExpiring(b *testing.B) {
	benchmarkStorageGet(b, NoExpiration)
}

func benchmarkStorageGet(b *testing.B, exp time.Duration) {
	b.StopTimer()
	s := New(exp, 0, 0)
	s.Set("key", "value", DefaultExpiration)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		s.Get("key")
	}
}

func BenchmarkRWMutexMap_Get(b *testing.B) {
	b.StopTimer()
	m := map[string]string{
		"key": "value",
	}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.RLock()
		_, _ = m["key"]
		mu.RUnlock()
	}
}

func BenchmarkStorage_GetConcurrentExpiring(b *testing.B) {
	benchmarkStorageGetConcurrent(b, 5*time.Minute)
}

func BenchmarkStorage_GetConcurrentNotExpiring(b *testing.B) {
	benchmarkStorageGetConcurrent(b, NoExpiration)
}

func benchmarkStorageGetConcurrent(b *testing.B, exp time.Duration) {
	b.StopTimer()
	s := New(exp, 0, 0)
	s.Set("key", "value", 0)
	wg := sync.WaitGroup{}
	workers := runtime.NumCPU()
	each := b.N / workers
	wg.Add(workers)
	b.StartTimer()
	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < each; j++ {
				s.Get("key")
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkRWMutexMapGetConcurrent(b *testing.B) {
	b.StopTimer()
	m := map[string]string{
		"key": "value",
	}
	mu := sync.RWMutex{}
	wg := new(sync.WaitGroup)
	workers := runtime.NumCPU()
	each := b.N / workers
	wg.Add(workers)
	b.StartTimer()
	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < each; j++ {
				mu.RLock()
				_, _ = m["value"]
				mu.RUnlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkStorage_SetNotExpiring(b *testing.B) {
	benchmarkStorageSet(b, NoExpiration)
}

func BenchmarkStorage_SetExpiring(b *testing.B) {
	benchmarkStorageSet(b, 100*time.Nanosecond)
}

func benchmarkStorageSet(b *testing.B, exp time.Duration) {
	b.StopTimer()
	s := New(exp, 0, 0)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		s.Set("key", "value", DefaultExpiration)
	}
}

func BenchmarkRWMutexMapSet(b *testing.B) {
	b.StopTimer()
	m := map[string]string{}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		m["key"] = "value"
		mu.Unlock()
	}
}

//TODO: понять, почему очень долго происходит удаление при наличии элементов, у которых не истёк срок годности
// (возможно это branch prediction)
func BenchmarkStorage_DeleteExpired(b *testing.B) {
	b.StopTimer()
	s := New(5*time.Minute, 0, 0)
	for i := 0; i < 100000; i++ {
		s.set(strconv.Itoa(i), "val", DefaultExpiration)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		s.DeleteExpired()
	}
}
