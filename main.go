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
	return &SecureLRUCache{
		capacity: capacity,
		cache:    make(map[int]*Node),
	}, nil
}

func (c *SecureLRUCache) removeNode(node *Node) {
	if node == nil {
		return
	}

	if node.prev != nil {
		node.prev.next = node.next
	} else {
		c.head = node.next
	}

	if node.next != nil {
		node.next.prev = node.prev
	} else {
		c.tail = node.prev
	}

	node.prev = nil
	node.next = nil
	c.size--
}

func (c *SecureLRUCache) addToHead(node *Node) {
	if node == nil {
		return
	}

	node.next = c.head
	node.prev = nil

	if c.head != nil {
		c.head.prev = node
	}
	c.head = node

	if c.tail == nil {
		c.tail = node
	}
	c.size++
}

func (c *SecureLRUCache) Get(key int) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.cache[key]
	if !exists {
		return 0, false
	}

	c.removeNode(node)
	c.addToHead(node)
	return node.value, true
}

func (c *SecureLRUCache) GetOrDefault(key int, defaultValue int) int {
	if value, found := c.Get(key); found {
		return value
	}
	return defaultValue
}

func (c *SecureLRUCache) Put(key, value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, exists := c.cache[key]; exists {
		node.value = value
		c.removeNode(node)
		c.addToHead(node)
		return
	}

	if c.size >= c.capacity && c.tail != nil {
		lru := c.tail
		c.removeNode(lru)
		delete(c.cache, lru.key)
	}

	node := &Node{key: key, value: value}
	c.cache[key] = node
	c.addToHead(node)
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
	return c.capacity
}

func (c *SecureLRUCache) Resize(newCapacity int) error {
	if newCapacity < 1 {
		return fmt.Errorf("capacity must be at least 1")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if newCapacity < c.capacity && c.size > newCapacity {
		for c.size > newCapacity && c.tail != nil {
			lru := c.tail
			c.removeNode(lru)
			delete(c.cache, lru.key)
		}
	}

	c.capacity = newCapacity
	return nil
}

func (c *SecureLRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[int]*Node)
	c.head = nil
	c.tail = nil
	c.size = 0
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

	for node := c.head; node != nil; node = node.next {
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

	cache.Clear()
	fmt.Printf("After Clear - Size: %d\n", cache.Size())

	fmt.Println()
	fmt.Println("=== Demo Complete ===")
}
