package memory

import (
	"fmt"
	"sync"
)

// Embedder 向量化接口
type Embedder interface {
	// Embed 将文本转换为向量
	Embed(text string) ([]float64, error)

	// EmbedBatch 批量将文本转换为向量
	EmbedBatch(texts []string) ([][]float64, error)

	// Dimension 返回向量的维度
	Dimension() int
}

// MockEmbedder 模拟向量化器（用于测试）
type MockEmbedder struct {
	DimensionSize int
}

// NewMockEmbedder 创建模拟向量化器
func NewMockEmbedder(dimension int) *MockEmbedder {
	return &MockEmbedder{DimensionSize: dimension}
}

// Embed 模拟向量化
func (m *MockEmbedder) Embed(text string) ([]float64, error) {
	// 生成基于文本内容的伪随机向量
	vector := make([]float64, m.DimensionSize)
	seed := hashString(text)

	for i := 0; i < m.DimensionSize; i++ {
		// 使用简单的伪随机数生成
		seed = (seed*9301 + 49297) % 233280
		vector[i] = float64(seed)/233280.0*2 - 1 // -1 to 1
	}

	return vector, nil
}

// EmbedBatch 批量向量化
func (m *MockEmbedder) EmbedBatch(texts []string) ([][]float64, error) {
	results := make([][]float64, len(texts))
	for i, text := range texts {
		vec, err := m.Embed(text)
		if err != nil {
			return nil, err
		}
		results[i] = vec
	}
	return results, nil
}

// Dimension 返回维度
func (m *MockEmbedder) Dimension() int {
	return m.DimensionSize
}

// hashString 计算字符串的简单哈希
func hashString(s string) int64 {
	var hash int64 = 5381
	for _, c := range s {
		hash = ((hash << 5) + hash) + int64(c)
	}
	return hash
}

// OpenAIEmbedder OpenAI 向量化器（预留）
type OpenAIEmbedder struct {
	apiKey   string
	model    string
	dimension int
}

// NewOpenAIEmbedder 创建 OpenAI 向量化器
func NewOpenAIEmbedder(apiKey string) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		apiKey:    apiKey,
		model:     "text-embedding-ada-002",
		dimension: 1536,
	}
}

// Embed 调用 OpenAI API 进行向量化
func (o *OpenAIEmbedder) Embed(text string) ([]float64, error) {
	// TODO: 实现 OpenAI API 调用
	return nil, fmt.Errorf("not implemented")
}

// EmbedBatch 批量向量化
func (o *OpenAIEmbedder) EmbedBatch(texts []string) ([][]float64, error) {
	// TODO: 实现 OpenAI API 批量调用
	return nil, fmt.Errorf("not implemented")
}

// Dimension 返回维度
func (o *OpenAIEmbedder) Dimension() int {
	return o.dimension
}

// EmbeddingService 向量化服务
type EmbeddingService struct {
	embedder Embedder
}

// NewEmbeddingService 创建向量化服务
func NewEmbeddingService(embedder Embedder) *EmbeddingService {
	return &EmbeddingService{embedder: embedder}
}

// GenerateEmbedding 为记忆项生成向量
func (s *EmbeddingService) GenerateEmbedding(item *MemoryItem) error {
	if s.embedder == nil {
		return fmt.Errorf("embedder not configured")
	}

	vector, err := s.embedder.Embed(item.Content)
	if err != nil {
		return fmt.Errorf("generate embedding: %w", err)
	}

	// 转换为 float32
	item.Embedding = make([]float32, len(vector))
	for i, v := range vector {
		item.Embedding[i] = float32(v)
	}

	return nil
}

// GenerateEmbeddings 为多个记忆项生成向量
func (s *EmbeddingService) GenerateEmbeddings(items []*MemoryItem) error {
	for _, item := range items {
		if err := s.GenerateEmbedding(item); err != nil {
			return err
		}
	}
	return nil
}

// Similarity 计算两个向量的相似度
func Similarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrtFloat64(normA) * sqrtFloat64(normB))
}

// sqrtFloat64 计算平方根
func sqrtFloat64(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// EmbeddingCache 向量缓存
type EmbeddingCache struct {
	mu       sync.RWMutex
	cache    map[string][]float64
	maxSize  int
}

// NewEmbeddingCache 创建向量缓存
func NewEmbeddingCache(maxSize int) *EmbeddingCache {
	return &EmbeddingCache{
		cache:   make(map[string][]float64),
		maxSize: maxSize,
	}
}

// Get 获取缓存的向量
func (c *EmbeddingCache) Get(text string) ([]float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	vec, ok := c.cache[text]
	return vec, ok
}

// Set 设置缓存
func (c *EmbeddingCache) Set(text string, vector []float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果缓存已满，删除一项
	if len(c.cache) >= c.maxSize {
		for k := range c.cache {
			delete(c.cache, k)
			break
		}
	}

	c.cache[text] = vector
}
