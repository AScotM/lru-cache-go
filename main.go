package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

type Node struct {
	key   int
	value int
	prev  *Node
	next  *Node
}

type SecureLRUCache struct {
	capacity      int
	cache         map[int]*Node
	head          *Node
	tail          *Node
	mu            sync.RWMutex
	hits          int64
	misses        int64
	evictions     int64
	enableMetrics bool
}

func NewSecureLRUCache(capacity int) (*SecureLRUCache, error) {
	if capacity < 1 {
		return nil, fmt.Errorf("capacity must be at least 1")
	}
	
	head := &Node{key: -1, value: -1}
	tail := &Node{key: -1, value: -1}
	head.next = tail
	tail.prev = head
	
	return &SecureLRUCache{
		capacity: capacity,
		cache:    make(map[int]*Node),
		head:     head,
		tail:     tail,
	}, nil
}

func (c *SecureLRUCache) removeNode(node *Node) {
	if node == nil || node.prev == nil || node.next == nil {
		return
	}
	
	if node == c.head || node == c.tail {
		return
	}
	
	node.prev.next = node.next
	node.next.prev = node.prev
	
	node.prev = nil
	node.next = nil
}

func (c *SecureLRUCache) addToHead(node *Node) {
	if node == nil {
		return
	}
	
	if node == c.head || node == c.tail {
		return
	}
	
	node.next = c.head.next
	node.prev = c.head
	
	c.head.next.prev = node
	c.head.next = node
}

func (c *SecureLRUCache) moveToHead(node *Node) {
	if node == nil || node.prev == nil || node.next == nil {
		return
	}
	
	if node == c.head || node == c.tail {
		return
	}
	
	if node == c.head.next {
		return
	}
	
	c.removeNode(node)
	c.addToHead(node)
}

func (c *SecureLRUCache) Get(key int) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.cache[key]
	if !exists {
		if c.enableMetrics {
			atomic.AddInt64(&c.misses, 1)
		}
		return 0, false
	}

	c.moveToHead(node)
	if c.enableMetrics {
		atomic.AddInt64(&c.hits, 1)
	}
	return node.value, true
}

func (c *SecureLRUCache) GetOrDefault(key int, defaultValue int) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.cache[key]
	if !exists {
		if c.enableMetrics {
			atomic.AddInt64(&c.misses, 1)
		}
		return defaultValue
	}

	c.moveToHead(node)
	if c.enableMetrics {
		atomic.AddInt64(&c.hits, 1)
	}
	return node.value
}

func (c *SecureLRUCache) Put(key, value int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, exists := c.cache[key]; exists {
		node.value = value
		c.moveToHead(node)
		return nil
	}

	if len(c.cache) >= c.capacity {
		lru := c.tail.prev
		if lru != c.head {
			c.removeNode(lru)
			delete(c.cache, lru.key)
			if c.enableMetrics {
				atomic.AddInt64(&c.evictions, 1)
			}
		} else {
			return fmt.Errorf("cache is full and cannot evict")
		}
	}

	node := &Node{key: key, value: value}
	c.cache[key] = node
	c.addToHead(node)
	return nil
}

func (c *SecureLRUCache) Contains(key int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.cache[key]
	return exists
}

func (c *SecureLRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

func (c *SecureLRUCache) Capacity() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capacity
}

func (c *SecureLRUCache) Resize(newCapacity int) error {
	if newCapacity < 1 {
		return fmt.Errorf("capacity must be at least 1")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if newCapacity < c.capacity && len(c.cache) > newCapacity {
		// Remove enough nodes to fit new capacity
		toRemove := len(c.cache) - newCapacity
		for i := 0; i < toRemove; i++ {
			lru := c.tail.prev
			if lru == c.head {
				break
			}
			
			c.removeNode(lru)
			delete(c.cache, lru.key)
			if c.enableMetrics {
				atomic.AddInt64(&c.evictions, 1)
			}
		}
	}

	c.capacity = newCapacity
	return nil
}

func (c *SecureLRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[int]*Node)
	c.head.next = c.tail
	c.tail.prev = c.head
}

func (c *SecureLRUCache) Remove(key int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.cache[key]
	if !exists {
		return false
	}

	c.removeNode(node)
	delete(c.cache, key)
	return true
}

type CacheDump struct {
	Capacity int         `json:"capacity"`
	Size     int         `json:"size"`
	Items    map[int]int `json:"items"`
	Order    []int       `json:"order"`
}

func (c *SecureLRUCache) Dump() CacheDump {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := make(map[int]int, len(c.cache))
	order := make([]int, 0, len(c.cache))

	for node := c.head.next; node != c.tail; node = node.next {
		items[node.key] = node.value
		order = append(order, node.key)
	}

	return CacheDump{
		Capacity: c.capacity,
		Size:     len(c.cache),
		Items:    items,
		Order:    order,
	}
}

func (c *SecureLRUCache) ToJSON() (string, error) {
	dump := c.Dump()
	bytes, err := json.Marshal(dump)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (c *SecureLRUCache) ToJSONPretty() (string, error) {
	dump := c.Dump()
	bytes, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (c *SecureLRUCache) Peek(key int) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	node, exists := c.cache[key]
	if !exists {
		return 0, false
	}
	return node.value, true
}

func (c *SecureLRUCache) Keys() []int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]int, 0, len(c.cache))
	for node := c.head.next; node != c.tail; node = node.next {
		keys = append(keys, node.key)
	}
	return keys
}

func (c *SecureLRUCache) Values() []int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	values := make([]int, 0, len(c.cache))
	for node := c.head.next; node != c.tail; node = node.next {
		values = append(values, node.value)
	}
	return values
}

func (c *SecureLRUCache) Range(f func(key, value int) bool) {
	c.mu.RLock()
	
	items := make([]struct {
		key   int
		value int
	}, 0, len(c.cache))
	
	for node := c.head.next; node != c.tail; node = node.next {
		items = append(items, struct {
			key   int
			value int
		}{node.key, node.value})
	}
	
	c.mu.RUnlock()
	
	for _, item := range items {
		if !f(item.key, item.value) {
			break
		}
	}
}

type CacheStats struct {
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Evictions int64 `json:"evictions"`
	Size      int   `json:"size"`
	Capacity  int   `json:"capacity"`
}

func (c *SecureLRUCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return CacheStats{
		Hits:      atomic.LoadInt64(&c.hits),
		Misses:    atomic.LoadInt64(&c.misses),
		Evictions: atomic.LoadInt64(&c.evictions),
		Size:      len(c.cache),
		Capacity:  c.capacity,
	}
}

func main() {
	fmt.Println("=== Secure LRU Cache Demo (Capacity: 2) ===")
	fmt.Println()

	cache, err := NewSecureLRUCache(2)
	if err != nil {
		fmt.Printf("Error creating cache: %v\n", err)
		return
	}

	cache.Put(1, 1)
	if jsonStr, err := cache.ToJSON(); err == nil {
		fmt.Printf("Put(1, 1) - Cache: %s\n", jsonStr)
	}

	cache.Put(2, 2)
	if jsonStr, err := cache.ToJSON(); err == nil {
		fmt.Printf("Put(2, 2) - Cache: %s\n", jsonStr)
	}

	val, found := cache.Get(1)
	fmt.Printf("Get(1): %d, Found: %v\n", val, found)

	cache.Put(3, 3)
	if jsonStr, err := cache.ToJSON(); err == nil {
		fmt.Printf("Put(3, 3) - Cache: %s\n", jsonStr)
	}

	val, found = cache.Get(2)
	fmt.Printf("Get(2): %d, Found: %v\n", val, found)

	val = cache.GetOrDefault(2, 999)
	fmt.Printf("GetOrDefault(2, 999): %d\n", val)

	cache.Put(4, 4)
	if jsonStr, err := cache.ToJSON(); err == nil {
		fmt.Printf("Put(4, 4) - Cache: %s\n", jsonStr)
	}

	val, found = cache.Get(1)
	fmt.Printf("Get(1): %d, Found: %v\n", val, found)

	val, found = cache.Get(3)
	fmt.Printf("Get(3): %d, Found: %v\n", val, found)

	val, found = cache.Get(4)
	fmt.Printf("Get(4): %d, Found: %v\n", val, found)

	fmt.Printf("Cache size: %d\n", cache.Size())
	fmt.Printf("Cache capacity: %d\n", cache.Capacity())
	fmt.Printf("Contains key 3: %v\n", cache.Contains(3))
	fmt.Printf("Contains key 99: %v\n", cache.Contains(99))

	val, found = cache.Peek(3)
	fmt.Printf("Peek(3): %d, Found: %v\n", val, found)

	keys := cache.Keys()
	fmt.Printf("Keys in cache: %v\n", keys)

	err = cache.Resize(3)
	if err != nil {
		fmt.Printf("Resize error: %v\n", err)
	} else {
		fmt.Printf("Resized to capacity: %d\n", cache.Capacity())
	}

	cache.Put(5, 5)
	if jsonStr, err := cache.ToJSON(); err == nil {
		fmt.Printf("After Put(5, 5) - Cache: %s\n", jsonStr)
	}

	removed := cache.Remove(4)
	fmt.Printf("Remove(4): %v\n", removed)

	cache.Clear()
	fmt.Printf("After Clear - Size: %d\n", cache.Size())

	fmt.Println()
	fmt.Println("=== Demo Complete ===")
}
