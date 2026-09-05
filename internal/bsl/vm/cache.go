package vm

import (
	"container/list"
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/k33alexey/MetaLab/internal/bsl/bytecode"
)

const maxCacheInputSize = 64 << 20
const estimatedMachineExpansion = 3

// CacheOptions bounds a shared immutable-machine cache.
type CacheOptions struct {
	MaxEntries int
	MaxBytes   uint64
	Limits     Limits
}

// DefaultCacheOptions returns conservative process-local cache limits.
func DefaultCacheOptions() CacheOptions {
	return CacheOptions{MaxEntries: 128, MaxBytes: 64 << 20, Limits: DefaultLimits()}
}

// CacheStats is a consistent snapshot of cache activity.
type CacheStats struct {
	Entries   int
	Bytes     uint64
	Hits      uint64
	Misses    uint64
	Evictions uint64
}

type cacheKey struct {
	digest [sha256.Size]byte
	client bool
}

type cacheEntry struct {
	key     cacheKey
	machine *Machine
	weight  uint64
}

type cacheLoad struct {
	done    chan struct{}
	machine *Machine
	err     error
}

// Cache reuses validated immutable machines and deduplicates concurrent loads.
// Every caller still creates its own Context, so session state is never shared.
type Cache struct {
	mu      sync.Mutex
	options CacheOptions
	items   map[cacheKey]*list.Element
	lru     list.List
	loading map[cacheKey]*cacheLoad
	stats   CacheStats
}

// NewCache creates a bounded bytecode cache.
func NewCache(options CacheOptions) (*Cache, error) {
	if options.MaxEntries <= 0 {
		return nil, fmt.Errorf("cache entry limit must be positive")
	}
	if options.MaxBytes == 0 {
		return nil, fmt.Errorf("cache byte limit must be positive")
	}
	if err := options.Limits.validate(); err != nil {
		return nil, fmt.Errorf("validate cache VM limits: %w", err)
	}
	return newCache(options), nil
}

// NewDefaultCache creates a cache using DefaultCacheOptions.
func NewDefaultCache() *Cache { return newCache(DefaultCacheOptions()) }

func newCache(options CacheOptions) *Cache {
	return &Cache{
		options: options, items: make(map[cacheKey]*list.Element, options.MaxEntries),
		loading: make(map[cacheKey]*cacheLoad),
	}
}

// Load returns a server machine for encoded bytecode.
func (cache *Cache) Load(encoded []byte) (*Machine, error) {
	return cache.load(encoded, false)
}

// LoadClient returns a client machine with server-only bodies removed.
func (cache *Cache) LoadClient(encoded []byte) (*Machine, error) {
	return cache.load(encoded, true)
}

func (cache *Cache) load(encoded []byte, client bool) (*Machine, error) {
	if len(encoded) > maxCacheInputSize {
		return nil, fmt.Errorf("encoded bytecode is too large: %d bytes", len(encoded))
	}
	key := cacheKey{digest: sha256.Sum256(encoded), client: client}
	cache.mu.Lock()
	if element := cache.items[key]; element != nil {
		cache.stats.Hits++
		cache.lru.MoveToFront(element)
		machine := element.Value.(*cacheEntry).machine
		cache.mu.Unlock()
		return machine, nil
	}
	if pending := cache.loading[key]; pending != nil {
		cache.stats.Hits++
		cache.mu.Unlock()
		<-pending.done
		return pending.machine, pending.err
	}
	pending := &cacheLoad{done: make(chan struct{})}
	cache.loading[key] = pending
	cache.stats.Misses++
	cache.mu.Unlock()

	program, err := bytecode.UnmarshalBinary(encoded)
	if err == nil && client {
		program, err = program.ClientProgram()
	}
	var machine *Machine
	if err == nil {
		machine, err = NewWithLimits(program, cache.options.Limits)
	}
	weight := uint64(len(encoded)) * estimatedMachineExpansion
	if weight < uint64(len(encoded)) {
		weight = ^uint64(0)
	}

	cache.mu.Lock()
	pending.machine, pending.err = machine, err
	delete(cache.loading, key)
	if err == nil && weight <= cache.options.MaxBytes {
		for cache.lru.Len() >= cache.options.MaxEntries || cache.stats.Bytes > cache.options.MaxBytes-weight {
			cache.evictOldest()
		}
		entry := &cacheEntry{key: key, machine: machine, weight: weight}
		cache.items[key] = cache.lru.PushFront(entry)
		cache.stats.Entries++
		cache.stats.Bytes += weight
	}
	close(pending.done)
	cache.mu.Unlock()
	return machine, err
}

func (cache *Cache) evictOldest() {
	element := cache.lru.Back()
	if element == nil {
		return
	}
	entry := element.Value.(*cacheEntry)
	delete(cache.items, entry.key)
	cache.lru.Remove(element)
	cache.stats.Entries--
	cache.stats.Bytes -= entry.weight
	cache.stats.Evictions++
}

// Stats returns a race-free cache snapshot.
func (cache *Cache) Stats() CacheStats {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.stats
}
