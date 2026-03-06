package permission

import (
	"sync"
	"time"
)

// CacheKey 缓存键
type CacheKey struct {
	Permission string
	Pattern    string
	AgentType  string
}

// CacheValue 缓存值
type CacheValue struct {
	Result     EvaluateResult
	ExpireTime time.Time
}

// EvaluationCache 评估缓存
type EvaluationCache struct {
	// 缓存存储
	data map[CacheKey]*CacheValue

	// 互斥锁
	mu sync.RWMutex

	// 配置
	config CacheConfig

	// 停止清理
	stopCleanup chan struct{}
}

// NewEvaluationCache 创建新的评估缓存
func NewEvaluationCache(config CacheConfig) *EvaluationCache {
	cache := &EvaluationCache{
		data:        make(map[CacheKey]*CacheValue),
		config:      config,
		stopCleanup: make(chan struct{}),
	}

	if config.Enabled && config.CleanupInterval > 0 {
		go cache.cleanupLoop()
	}

	return cache
}

// Get 获取缓存
func (c *EvaluationCache) Get(key CacheKey) (EvaluateResult, bool) {
	if !c.config.Enabled {
		return EvaluateResult{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	value, ok := c.data[key]
	if !ok {
		return EvaluateResult{}, false
	}

	// 检查是否过期
	if time.Now().After(value.ExpireTime) {
		return EvaluateResult{}, false
	}

	return value.Result, true
}

// Set 设置缓存
func (c *EvaluationCache) Set(key CacheKey, result EvaluateResult) {
	if !c.config.Enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查缓存大小限制
	if c.config.MaxSize > 0 && len(c.data) >= c.config.MaxSize {
		// 简单的LRU：删除最旧的一条
		c.evictOldest()
	}

	ttl := c.config.TTL
	if ttl <= 0 {
		ttl = 3600 // 默认1小时
	}

	c.data[key] = &CacheValue{
		Result:     result,
		ExpireTime: time.Now().Add(time.Duration(ttl) * time.Second),
	}
}

// Delete 删除缓存
func (c *EvaluationCache) Delete(key CacheKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Clear 清空缓存
func (c *EvaluationCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[CacheKey]*CacheValue)
}

// Size 返回缓存大小
func (c *EvaluationCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// evictOldest 删除最旧的条目
func (c *EvaluationCache) evictOldest() {
	var oldestKey CacheKey
	var oldestTime time.Time
	first := true

	for key, value := range c.data {
		if first || value.ExpireTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = value.ExpireTime
			first = false
		}
	}

	if !first {
		delete(c.data, oldestKey)
	}
}

// cleanupExpired 清理过期条目
func (c *EvaluationCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, value := range c.data {
		if now.After(value.ExpireTime) {
			delete(c.data, key)
		}
	}
}

// cleanupLoop 定期清理循环
func (c *EvaluationCache) cleanupLoop() {
	interval := c.config.CleanupInterval
	if interval <= 0 {
		interval = 300 // 默认5分钟
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

// Stop 停止清理循环
func (c *EvaluationCache) Stop() {
	close(c.stopCleanup)
}

// MatchCache 模式匹配缓存（用于缓存模式编译结果）
type MatchCache struct {
	// 匹配结果缓存
	// key: "pattern:string", value: bool
	data map[string]bool

	// 互斥锁
	mu sync.RWMutex

	// 最大大小
	maxSize int
}

// NewMatchCache 创建新的匹配缓存
func NewMatchCache(maxSize int) *MatchCache {
	return &MatchCache{
		data:    make(map[string]bool),
		maxSize: maxSize,
	}
}

// Get 获取匹配结果
func (c *MatchCache) Get(pattern, s string) (bool, bool) {
	key := pattern + ":" + s

	c.mu.RLock()
	defer c.mu.RUnlock()

	result, ok := c.data[key]
	return result, ok
}

// Set 设置匹配结果
func (c *MatchCache) Set(pattern, s string, result bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 简单的大小限制
	if c.maxSize > 0 && len(c.data) >= c.maxSize {
		// 清空缓存
		c.data = make(map[string]bool)
	}

	key := pattern + ":" + s
	c.data[key] = result
}

// Clear 清空缓存
func (c *MatchCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]bool)
}
