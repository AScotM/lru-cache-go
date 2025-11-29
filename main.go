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

func NewSecureLRUCache(capacity int) *SecureLRUCache {
	if capacity < 1 {
		panic("Capacity must be at least 1")
	}
	return &SecureLRUCache{
		capacity: capacity,
		cache:    make(map[int]*Node),
	}
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

func (c *SecureLRUCache) Get(key int) int {
	c.mu.RLock()
	node, exists := c.cache[key]
	c.mu.RUnlock()

	if !exists {
		return -1
	}

	c.mu.Lock()
	c.removeNode(node)
	c.addToHead(node)
	c.mu.Unlock()

	return node.value
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

	node := &Node{key: key, value: value}
	c.cache[key] = node
	c.addToHead(node)

	if c.size > c.capacity {
		lru := c.tail
		if lru != nil {
			c.removeNode(lru)
			delete(c.cache, lru.key)
		}
	}
}

func (c *SecureLRUCache) Dump() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := make(map[int]int)
	order := make([]int, 0)

	for node := c.head; node != nil; node = node.next {
		items[node.key] = node.value
		order = append(order, node.key)
	}

	return map[string]interface{}{
		"capacity": c.capacity,
		"size":     c.size,
		"items":    items,
		"order":    order,
	}
}

func main() {
	fmt.Println("=== Secure LRU Cache Demo (Capacity: 2) ===")
	fmt.Println()

	cache := NewSecureLRUCache(2)

	cache.Put(1, 1)
	fmt.Printf("Put(1, 1) - Cache: %s\n", marshal(cache.Dump()))

	cache.Put(2, 2)
	fmt.Printf("Put(2, 2) - Cache: %s\n", marshal(cache.Dump()))

	fmt.Printf("Get(1): %d\n", cache.Get(1))

	cache.Put(3, 3)
	fmt.Printf("Put(3, 3) - Cache: %s\n", marshal(cache.Dump()))

	fmt.Printf("Get(2): %d\n", cache.Get(2))

	cache.Put(4, 4)
	fmt.Printf("Put(4, 4) - Cache: %s\n", marshal(cache.Dump()))

	fmt.Printf("Get(1): %d\n", cache.Get(1))
	fmt.Printf("Get(3): %d\n", cache.Get(3))
	fmt.Printf("Get(4): %d\n", cache.Get(4))

	fmt.Println()
	fmt.Println("=== Demo Complete ===")
}

func marshal(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
