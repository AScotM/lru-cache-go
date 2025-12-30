package main

import (
	"encoding/json"
	"fmt"
	"sync"
)

type Node struct {
	key   int
	value int
	prev  *Node
	next  *Node
}

type SecureLRUCache struct {
	capacity int
	cache    map[int]*Node
	head     *Node
	tail     *Node
	size     int
	mu       sync.RWMutex
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
		size:     0,
	}, nil
}

func (c *SecureLRUCache) removeNode(node *Node) {
	if node == nil || node.prev == nil || node.next == nil {
		return
	}
	
	if node == c.head || node == c.tail {
		return
	}
	
	prev := node.prev
	next := node.next
	
	prev.next = next
	next.prev = prev
	
	node.prev = nil
	node.next = nil
	c.size--
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
	c.size++
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
	
	prev := node.prev
	next := node.next
	
	prev.next = next
	next.prev = prev
	
	node.next = c.head.next
	node.prev = c.head
	
	c.head.next.prev = node
	c.head.next = node
}

func (c *SecureLRUCache) Get(key int) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.cache[key]
	if !exists {
		return 0, false
	}

	c.moveToHead(node)
	return node.value, true
}

func (c *SecureLRUCache) GetOrDefault(key int, defaultValue int) int {
	c.mu.RLock()
	node, exists := c.cache[key]
	c.mu.RUnlock()
	
	if !exists {
		return defaultValue
	}
	
	c.mu.Lock()
	c.moveToHead(node)
	value := node.value
	c.mu.Unlock()
	
	return value
}

func (c *SecureLRUCache) Put(key, value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, exists := c.cache[key]; exists {
		node.value = value
		c.moveToHead(node)
		return
	}

	if c.size >= c.capacity {
		lru := c.tail.prev
		if lru != c.head {
			delete(c.cache, lru.key)
			
			lruPrev := lru.prev
			lruNext := lru.next
			
			lruPrev.next = lruNext
			lruNext.prev = lruPrev
			
			lru.prev = nil
			lru.next = nil
			c.size--
		}
	}

	node := &Node{key: key, value: value}
	c.cache[key] = node
	
	node.next = c.head.next
	node.prev = c.head
	
	c.head.next.prev = node
	c.head.next = node
	c.size++
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
	return c.size
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

	if newCapacity < c.capacity && c.size > newCapacity {
		for c.size > newCapacity {
			lru := c.tail.prev
			if lru == c.head {
				break
			}
			
			delete(c.cache, lru.key)
			
			lruPrev := lru.prev
			lruNext := lru.next
			
			lruPrev.next = lruNext
			lruNext.prev = lruPrev
			
			lru.prev = nil
			lru.next = nil
			c.size--
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
	c.size = 0
}

func (c *SecureLRUCache) Remove(key int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.cache[key]
	if !exists {
		return false
	}

	delete(c.cache, key)
	
	prev := node.prev
	next := node.next
	
	if prev != nil && next != nil {
		prev.next = next
		next.prev = prev
		
		node.prev = nil
		node.next = nil
		c.size--
	}
	
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

	items := make(map[int]int)
	order := make([]int, 0, c.size)

	for node := c.head.next; node != c.tail; node = node.next {
		items[node.key] = node.value
		order = append(order, node.key)
	}

	return CacheDump{
		Capacity: c.capacity,
		Size:     c.size,
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

	keys := make([]int, 0, c.size)
	for node := c.head.next; node != c.tail; node = node.next {
		keys = append(keys, node.key)
	}
	return keys
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
