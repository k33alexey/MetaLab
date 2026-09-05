package vm

import (
	"sync"
	"testing"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
	"github.com/k33alexey/MetaLab/internal/bsl/compiler"
)

func TestCacheReusesValidatedMachine(t *testing.T) {
	t.Parallel()

	cache := newTestCache(t, 2)
	encoded := compileEncoded(t, "Function Value() Return 42; EndFunction")
	first, err := cache.Load(encoded)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Load(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("identical bytecode produced different machines")
	}
	stats := cache.Stats()
	if stats.Entries != 1 || stats.Misses != 1 || stats.Hits != 1 || stats.Bytes == 0 {
		t.Fatalf("cache stats = %+v", stats)
	}
}

func TestCacheSeparatesServerAndClientArtifacts(t *testing.T) {
	t.Parallel()

	cache := newTestCache(t, 2)
	encoded := compileEncoded(t, "&AtServer\nFunction Value() Return 42; EndFunction")
	server, err := cache.Load(encoded)
	if err != nil {
		t.Fatal(err)
	}
	client, err := cache.LoadClient(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if server == client || cache.Stats().Entries != 2 {
		t.Fatalf("server/client cache collision: %+v", cache.Stats())
	}
	value, err := server.Call("Value")
	if err != nil || value.String() != "42" {
		t.Fatalf("server Value() = %v, %v", value, err)
	}
	if _, err := client.NewClientContext(nil).Call("Value"); err == nil {
		t.Fatal("client executed a server-only routine")
	}
}

func TestCacheUsesLRUEviction(t *testing.T) {
	t.Parallel()

	cache := newTestCache(t, 2)
	first := compileEncoded(t, "Function Value() Return 1; EndFunction")
	second := compileEncoded(t, "Function Value() Return 2; EndFunction")
	third := compileEncoded(t, "Function Value() Return 3; EndFunction")
	firstMachine, _ := cache.Load(first)
	secondMachine, _ := cache.Load(second)
	if _, err := cache.Load(first); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Load(third); err != nil {
		t.Fatal(err)
	}
	reloaded, err := cache.Load(second)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded == secondMachine || firstMachine == nil {
		t.Fatal("least recently used machine was not evicted")
	}
	stats := cache.Stats()
	if stats.Entries != 2 || stats.Evictions != 2 || stats.Misses != 4 {
		t.Fatalf("cache stats = %+v", stats)
	}
}

func TestCacheDeduplicatesConcurrentLoads(t *testing.T) {
	t.Parallel()

	cache := newTestCache(t, 4)
	encoded := compileEncoded(t, "Function Value() Return 42; EndFunction")
	const workers = 64
	machines := make([]*Machine, workers)
	errors := make([]error, workers)
	var start sync.WaitGroup
	start.Add(1)
	var workersDone sync.WaitGroup
	for index := range workers {
		workersDone.Add(1)
		go func(index int) {
			defer workersDone.Done()
			start.Wait()
			machines[index], errors[index] = cache.Load(encoded)
		}(index)
	}
	start.Done()
	workersDone.Wait()
	for index := range workers {
		if errors[index] != nil || machines[index] != machines[0] {
			t.Fatalf("load %d = %p, %v", index, machines[index], errors[index])
		}
	}
	stats := cache.Stats()
	if stats.Misses != 1 || stats.Hits != workers-1 || stats.Entries != 1 {
		t.Fatalf("cache stats = %+v", stats)
	}
}

func TestCacheRejectsInvalidConfigurationAndBytecode(t *testing.T) {
	t.Parallel()

	if _, err := NewCache(CacheOptions{}); err == nil {
		t.Fatal("NewCache accepted empty options")
	}
	if _, err := NewCache(CacheOptions{MaxEntries: 1, MaxBytes: 1}); err == nil {
		t.Fatal("NewCache accepted empty VM limits")
	}
	cache := newTestCache(t, 1)
	if _, err := cache.Load([]byte("invalid")); err == nil {
		t.Fatal("cache accepted invalid bytecode")
	}
	if stats := cache.Stats(); stats.Entries != 0 || stats.Misses != 1 {
		t.Fatalf("cache stats = %+v", stats)
	}
}

func TestCacheDoesNotRetainEntryAboveByteLimit(t *testing.T) {
	t.Parallel()

	options := DefaultCacheOptions()
	options.MaxEntries = 2
	options.MaxBytes = 1
	cache, err := NewCache(options)
	if err != nil {
		t.Fatal(err)
	}
	encoded := compileEncoded(t, "Function Value() Return 42; EndFunction")
	first, err := cache.Load(encoded)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.Load(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("oversized machine was retained")
	}
	if stats := cache.Stats(); stats.Entries != 0 || stats.Bytes != 0 || stats.Misses != 2 {
		t.Fatalf("cache stats = %+v", stats)
	}
}

func newTestCache(t testing.TB, entries int) *Cache {
	t.Helper()
	options := DefaultCacheOptions()
	options.MaxEntries = entries
	options.MaxBytes = 1 << 20
	cache, err := NewCache(options)
	if err != nil {
		t.Fatal(err)
	}
	return cache
}

func compileEncoded(t testing.TB, source string) []byte {
	t.Helper()
	program, diagnostics := compiler.CompileSource("cache.bsl", source)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	encoded, err := bytecode.MarshalBinary(program)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func BenchmarkCacheHit(b *testing.B) {
	cache := newTestCache(b, 4)
	encoded := compileEncoded(b, "Function Value() Return 42; EndFunction")
	expected, err := cache.Load(encoded)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		machine, loadErr := cache.Load(encoded)
		if loadErr != nil || machine != expected {
			b.Fatal(loadErr)
		}
	}
}
