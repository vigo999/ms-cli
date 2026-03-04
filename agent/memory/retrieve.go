package memory

import (
	"sort"
	"strings"
	"time"
)

// Retriever 记忆检索器
type Retriever struct {
	store  Store
	policy Policy
}

// NewRetriever 创建新的检索器
func NewRetriever(store Store, policy Policy) *Retriever {
	return &Retriever{
		store:  store,
		policy: policy,
	}
}

// RetrieveResult 检索结果（扩展原有定义）
type RetrieveResult struct {
	Items      []*MemoryItem
	TotalCount int
	QueryTime  time.Duration
}

// Retrieve 执行检索
func (r *Retriever) Retrieve(q Query) (*RetrieveResult, error) {
	start := time.Now()

	items, err := r.store.Query(q)
	if err != nil {
		return nil, err
	}

	// 过滤过期项（如果存储层没有自动过滤）
	var validItems []*MemoryItem
	for _, item := range items {
		if !item.IsExpired() {
			validItems = append(validItems, item)
		}
	}

	return &RetrieveResult{
		Items:      validItems,
		TotalCount: len(validItems),
		QueryTime:  time.Since(start),
	}, nil
}

// RetrieveByType 按类型检索
func (r *Retriever) RetrieveByType(memType MemoryType, limit int) ([]*MemoryItem, error) {
	q := DefaultQuery()
	q.Types = []MemoryType{memType}
	q.Limit = limit

	result, err := r.Retrieve(q)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// RetrieveByTags 按标签检索
func (r *Retriever) RetrieveByTags(tags []string, limit int) ([]*MemoryItem, error) {
	q := DefaultQuery()
	q.Tags = tags
	q.Limit = limit

	result, err := r.Retrieve(q)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// RetrieveByKeyword 关键词检索
func (r *Retriever) RetrieveByKeyword(keyword string, limit int) ([]*MemoryItem, error) {
	q := DefaultQuery()
	q.Keywords = []string{keyword}
	q.Limit = limit

	result, err := r.Retrieve(q)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// RetrieveRecent 检索最近记忆
func (r *Retriever) RetrieveRecent(duration time.Duration, limit int) ([]*MemoryItem, error) {
	startTime := time.Now().Add(-duration)
	q := DefaultQuery()
	q.TimeRange = &TimeRange{Start: &startTime}
	q.Limit = limit
	q.OrderBy = OrderByCreatedAt
	q.OrderDesc = true

	result, err := r.Retrieve(q)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// RetrieveImportant 检索重要记忆
func (r *Retriever) RetrieveImportant(minImportance int, limit int) ([]*MemoryItem, error) {
	q := DefaultQuery()
	q.MinImportance = minImportance
	q.Limit = limit
	q.OrderBy = OrderByImportance
	q.OrderDesc = true

	result, err := r.Retrieve(q)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// RetrieveForContext 为上下文检索相关记忆
func (r *Retriever) RetrieveForContext(context string, limit int) ([]*MemoryItem, error) {
	// 提取关键词（简单实现：按空格分割）
	keywords := extractKeywords(context)

	q := DefaultQuery()
	q.Keywords = keywords
	q.Limit = limit * 2 // 获取更多，然后评分排序
	q.OrderBy = OrderByImportance
	q.OrderDesc = true

	items, err := r.store.Query(q)
	if err != nil {
		return nil, err
	}

	// 评分排序
	scored := make([]scoredItem, len(items))
	for i, item := range items {
		scored[i] = scoredItem{
			item:  item,
			score: r.calculateRelevance(item, context, keywords),
		}
	}

	// 按相关性排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// 返回最相关的
	if limit > len(scored) {
		limit = len(scored)
	}
	result := make([]*MemoryItem, limit)
	for i := 0; i < limit; i++ {
		result[i] = scored[i].item
	}

	return result, nil
}

// scoredItem 带评分的记忆项
type scoredItem struct {
	item  *MemoryItem
	score float64
}

// calculateRelevance 计算相关性分数
func (r *Retriever) calculateRelevance(item *MemoryItem, context string, keywords []string) float64 {
	var score float64

	// 关键词匹配分数
	contentLower := strings.ToLower(item.Content)
	for _, kw := range keywords {
		if strings.Contains(contentLower, strings.ToLower(kw)) {
			score += 1.0
		}
	}

	// 重要性分数
	score += float64(item.Importance) * 0.5

	// 时效性分数（越新越好）
	age := time.Since(item.CreatedAt)
	if age < 24*time.Hour {
		score += 2.0
	} else if age < 7*24*time.Hour {
		score += 1.0
	}

	// 访问频率分数
	score += float64(item.AccessCount) * 0.1

	return score
}

// extractKeywords 提取关键词
func extractKeywords(text string) []string {
	// 简单实现：按空格分割，过滤短词
	words := strings.Fields(text)
	var keywords []string
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:()[]{}\"'")
		if len(word) >= 3 {
			keywords = append(keywords, word)
		}
	}
	return keywords
}

// SemanticRetriever 语义检索器（预留接口）
type SemanticRetriever struct {
	store     Store
	embedder  Embedder
}

// NewSemanticRetriever 创建语义检索器
func NewSemanticRetriever(store Store, embedder Embedder) *SemanticRetriever {
	return &SemanticRetriever{
		store:     store,
		embedder:  embedder,
	}
}

// Retrieve 基于语义相似度检索
func (sr *SemanticRetriever) Retrieve(query string, limit int) ([]*MemoryItem, error) {
	// 获取查询的 embedding
	queryEmbedding, err := sr.embedder.Embed(query)
	if err != nil {
		return nil, err
	}

	// 获取所有记忆项（这里应该有一个更高效的实现）
	q := DefaultQuery()
	q.Limit = 1000 // 获取大量数据然后排序

	items, err := sr.store.Query(q)
	if err != nil {
		return nil, err
	}

	// 计算相似度并排序
	type scoredItem struct {
		item       *MemoryItem
		similarity float64
	}

	scoredItems := make([]scoredItem, 0, len(items))
	for _, item := range items {
		if len(item.Embedding) > 0 {
			sim := cosineSimilarity(queryEmbedding, embeddingToFloat64(item.Embedding))
			scoredItems = append(scoredItems, scoredItem{item: item, similarity: sim})
		}
	}

	// 按相似度排序
	sort.Slice(scoredItems, func(i, j int) bool {
		return scoredItems[i].similarity > scoredItems[j].similarity
	})

	// 返回最相似的
	if limit > len(scoredItems) {
		limit = len(scoredItems)
	}
	result := make([]*MemoryItem, limit)
	for i := 0; i < limit; i++ {
		result[i] = scoredItems[i].item
	}

	return result, nil
}

// embeddingToFloat64 将 float32 embedding 转换为 float64
func embeddingToFloat64(emb []float32) []float64 {
	result := make([]float64, len(emb))
	for i, v := range emb {
		result[i] = float64(v)
	}
	return result
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt 计算平方根
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// SimpleRetriever 简单检索器
type SimpleRetriever struct {
	store Store
}

// NewSimpleRetriever 创建简单检索器
func NewSimpleRetriever(store Store) *SimpleRetriever {
	return &SimpleRetriever{store: store}
}

// Get 获取单个记忆项
func (sr *SimpleRetriever) Get(id string) (*MemoryItem, error) {
	return sr.store.Get(id)
}

// Search 搜索记忆
func (sr *SimpleRetriever) Search(keyword string) ([]*MemoryItem, error) {
	q := DefaultQuery()
	q.Keywords = []string{keyword}
	return sr.store.Query(q)
}

// ListAll 列出所有记忆
func (sr *SimpleRetriever) ListAll(limit int) ([]*MemoryItem, error) {
	q := DefaultQuery()
	q.Limit = limit
	return sr.store.Query(q)
}
