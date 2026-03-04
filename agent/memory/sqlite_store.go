package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unsafe"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore 基于 SQLite 的记忆存储实现
type SQLiteStore struct {
	db     *sql.DB
	config Config
}

// NewSQLiteStore 创建新的 SQLite 存储
func NewSQLiteStore(dbPath string, cfg Config) (*SQLiteStore, error) {
	if dbPath == "" {
		dbPath = ":memory:"
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	store := &SQLiteStore{
		db:     db,
		config: cfg,
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

// initSchema 初始化数据库表结构
func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		content TEXT NOT NULL,
		metadata TEXT,
		tags TEXT,
		embedding BLOB,
		importance INTEGER DEFAULT 5,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME,
		access_count INTEGER DEFAULT 0,
		last_access DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
	CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at);
	CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance);
	CREATE INDEX IF NOT EXISTS idx_memories_expires_at ON memories(expires_at);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Save 保存记忆项
func (s *SQLiteStore) Save(item *MemoryItem) error {
	// 检查是否已存在
	exists, err := s.exists(item.ID)
	if err != nil {
		return err
	}

	if exists {
		return s.update(item)
	}
	return s.insert(item)
}

// insert 插入新记忆项
func (s *SQLiteStore) insert(item *MemoryItem) error {
	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	tagsJSON, err := json.Marshal(item.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	query := `
		INSERT INTO memories (id, type, content, metadata, tags, embedding, importance, 
			created_at, updated_at, expires_at, access_count, last_access)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query,
		item.ID,
		string(item.Type),
		item.Content,
		string(metadataJSON),
		string(tagsJSON),
		serializeEmbedding(item.Embedding),
		item.Importance,
		item.CreatedAt,
		item.UpdatedAt,
		item.ExpiresAt,
		item.AccessCount,
		item.LastAccess,
	)

	if err != nil {
		return fmt.Errorf("insert memory: %w", err)
	}

	return nil
}

// update 更新现有记忆项
func (s *SQLiteStore) update(item *MemoryItem) error {
	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	tagsJSON, err := json.Marshal(item.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	query := `
		UPDATE memories SET
			type = ?,
			content = ?,
			metadata = ?,
			tags = ?,
			embedding = ?,
			importance = ?,
			updated_at = ?,
			expires_at = ?,
			access_count = ?,
			last_access = ?
		WHERE id = ?
	`

	item.UpdatedAt = time.Now()

	_, err = s.db.Exec(query,
		string(item.Type),
		item.Content,
		string(metadataJSON),
		string(tagsJSON),
		serializeEmbedding(item.Embedding),
		item.Importance,
		item.UpdatedAt,
		item.ExpiresAt,
		item.AccessCount,
		item.LastAccess,
		item.ID,
	)

	if err != nil {
		return fmt.Errorf("update memory: %w", err)
	}

	return nil
}

// Get 根据 ID 获取记忆项
func (s *SQLiteStore) Get(id string) (*MemoryItem, error) {
	query := `
		SELECT id, type, content, metadata, tags, embedding, importance,
			created_at, updated_at, expires_at, access_count, last_access
		FROM memories
		WHERE id = ?
	`

	row := s.db.QueryRow(query, id)
	return s.scanRow(row)
}

// Query 根据查询条件检索记忆
func (s *SQLiteStore) Query(q Query) ([]*MemoryItem, error) {
	builder := &queryBuilder{}
	builder.add("1 = 1") // 基础条件

	// 类型过滤
	if len(q.Types) > 0 {
		placeholders := make([]string, len(q.Types))
		args := make([]any, len(q.Types))
		for i, t := range q.Types {
			placeholders[i] = "?"
			args[i] = string(t)
		}
		builder.add(fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ", ")), args...)
	}

	// 关键词过滤（简单实现：在 content 中搜索）
	if len(q.Keywords) > 0 {
		for _, kw := range q.Keywords {
			builder.add("content LIKE ?", "%"+kw+"%")
		}
	}

	// 标签过滤
	if len(q.Tags) > 0 {
		for _, tag := range q.Tags {
			builder.add("tags LIKE ?", "%\""+tag+"\"%")
		}
	}

	// 重要性过滤
	if q.MinImportance > 0 {
		builder.add("importance >= ?", q.MinImportance)
	}

	// 时间范围过滤
	if q.TimeRange != nil {
		if q.TimeRange.Start != nil {
			builder.add("created_at >= ?", *q.TimeRange.Start)
		}
		if q.TimeRange.End != nil {
			builder.add("created_at <= ?", *q.TimeRange.End)
		}
	}

	// 排序
	orderBy := sanitizeOrderBy(string(q.OrderBy))
	orderDir := "ASC"
	if q.OrderDesc {
		orderDir = "DESC"
	}

	// 构建查询
	query := fmt.Sprintf(`
		SELECT id, type, content, metadata, tags, embedding, importance,
			created_at, updated_at, expires_at, access_count, last_access
		FROM memories
		WHERE %s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, builder.whereClause(), orderBy, orderDir)

	args := append(builder.args, q.Limit, q.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	var results []*MemoryItem
	for rows.Next() {
		item, err := s.scanRows(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, rows.Err()
}

// Delete 删除记忆项
func (s *SQLiteStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM memories WHERE id = ?", id)
	return err
}

// DeleteBefore 删除指定时间之前的记忆
func (s *SQLiteStore) DeleteBefore(t time.Time) error {
	_, err := s.db.Exec("DELETE FROM memories WHERE created_at < ?", t)
	return err
}

// DeleteExpired 删除过期记忆
func (s *SQLiteStore) DeleteExpired() error {
	_, err := s.db.Exec("DELETE FROM memories WHERE expires_at IS NOT NULL AND expires_at < ?", time.Now())
	return err
}

// DeleteByType 删除指定类型的记忆
func (s *SQLiteStore) DeleteByType(memType MemoryType) error {
	_, err := s.db.Exec("DELETE FROM memories WHERE type = ?", string(memType))
	return err
}

// List 列出所有记忆项（分页）
func (s *SQLiteStore) List(limit, offset int) ([]*MemoryItem, error) {
	return s.Query(Query{Limit: limit, Offset: offset})
}

// Count 获取记忆项总数
func (s *SQLiteStore) Count() (int64, error) {
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&count)
	return count, err
}

// CountByType 按类型统计记忆数量
func (s *SQLiteStore) CountByType() (map[MemoryType]int64, error) {
	rows, err := s.db.Query("SELECT type, COUNT(*) FROM memories GROUP BY type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[MemoryType]int64)
	for rows.Next() {
		var typeStr string
		var count int64
		if err := rows.Scan(&typeStr, &count); err != nil {
			return nil, err
		}
		result[MemoryType(typeStr)] = count
	}

	return result, rows.Err()
}

// GetStats 获取统计信息
func (s *SQLiteStore) GetStats() (MemoryStats, error) {
	stats := MemoryStats{
		ByType: make(map[MemoryType]int64),
	}

	// 总数
	total, err := s.Count()
	if err != nil {
		return stats, err
	}
	stats.TotalCount = total

	// 按类型统计
	byType, err := s.CountByType()
	if err != nil {
		return stats, err
	}
	stats.ByType = byType

	// 过期数量
	var expiredCount int64
	err = s.db.QueryRow(
		"SELECT COUNT(*) FROM memories WHERE expires_at IS NOT NULL AND expires_at < ?",
		time.Now(),
	).Scan(&expiredCount)
	if err != nil {
		return stats, err
	}
	stats.ExpiredCount = expiredCount

	// 平均重要性
	var avgImportance sql.NullFloat64
	err = s.db.QueryRow("SELECT AVG(importance) FROM memories").Scan(&avgImportance)
	if err != nil {
		return stats, err
	}
	if avgImportance.Valid {
		stats.AvgImportance = avgImportance.Float64
	}

	return stats, nil
}

// Close 关闭数据库连接
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// exists 检查记忆项是否存在
func (s *SQLiteStore) exists(id string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM memories WHERE id = ?", id).Scan(&count)
	return count > 0, err
}

// scanRow 扫描单行
func (s *SQLiteStore) scanRow(row *sql.Row) (*MemoryItem, error) {
	var item MemoryItem
	var metadataStr, tagsStr string
	var embeddingBlob []byte
	var expiresAt sql.NullTime
	var lastAccess sql.NullTime

	err := row.Scan(
		&item.ID,
		&item.Type,
		&item.Content,
		&metadataStr,
		&tagsStr,
		&embeddingBlob,
		&item.Importance,
		&item.CreatedAt,
		&item.UpdatedAt,
		&expiresAt,
		&item.AccessCount,
		&lastAccess,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// 解析 metadata
	if metadataStr != "" {
		json.Unmarshal([]byte(metadataStr), &item.Metadata)
	}

	// 解析 tags
	if tagsStr != "" {
		json.Unmarshal([]byte(tagsStr), &item.Tags)
	}

	// 解析 embedding
	if embeddingBlob != nil {
		item.Embedding = deserializeEmbedding(embeddingBlob)
	}

	// 解析时间
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}
	if lastAccess.Valid {
		item.LastAccess = &lastAccess.Time
	}

	return &item, nil
}

// scanRows 扫描多行
func (s *SQLiteStore) scanRows(rows *sql.Rows) (*MemoryItem, error) {
	var item MemoryItem
	var metadataStr, tagsStr string
	var embeddingBlob []byte
	var expiresAt sql.NullTime
	var lastAccess sql.NullTime

	err := rows.Scan(
		&item.ID,
		&item.Type,
		&item.Content,
		&metadataStr,
		&tagsStr,
		&embeddingBlob,
		&item.Importance,
		&item.CreatedAt,
		&item.UpdatedAt,
		&expiresAt,
		&item.AccessCount,
		&lastAccess,
	)
	if err != nil {
		return nil, err
	}

	// 解析 metadata
	if metadataStr != "" {
		json.Unmarshal([]byte(metadataStr), &item.Metadata)
	}

	// 解析 tags
	if tagsStr != "" {
		json.Unmarshal([]byte(tagsStr), &item.Tags)
	}

	// 解析 embedding
	if embeddingBlob != nil {
		item.Embedding = deserializeEmbedding(embeddingBlob)
	}

	// 解析时间
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}
	if lastAccess.Valid {
		item.LastAccess = &lastAccess.Time
	}

	return &item, nil
}

// queryBuilder 查询构建器
type queryBuilder struct {
	conditions []string
	args       []any
}

func (qb *queryBuilder) add(condition string, args ...any) {
	qb.conditions = append(qb.conditions, condition)
	qb.args = append(qb.args, args...)
}

func (qb *queryBuilder) whereClause() string {
	return strings.Join(qb.conditions, " AND ")
}

func sanitizeOrderBy(orderBy string) string {
	switch orderBy {
	case string(OrderByCreatedAt),
		string(OrderByUpdatedAt),
		string(OrderByImportance),
		string(OrderByAccessCount),
		string(OrderByLastAccess):
		return orderBy
	default:
		return string(OrderByCreatedAt)
	}
}

// serializeEmbedding 将 embedding 序列化为字节
func serializeEmbedding(embedding []float32) []byte {
	if embedding == nil {
		return nil
	}
	// 简单实现：每个 float32 占 4 字节
	data := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		// 将 float32 转换为字节
		bits := *(*uint32)(unsafe.Pointer(&v))
		data[i*4] = byte(bits)
		data[i*4+1] = byte(bits >> 8)
		data[i*4+2] = byte(bits >> 16)
		data[i*4+3] = byte(bits >> 24)
	}
	return data
}

// deserializeEmbedding 将字节反序列化为 embedding
func deserializeEmbedding(data []byte) []float32 {
	if data == nil || len(data)%4 != 0 {
		return nil
	}
	embedding := make([]float32, len(data)/4)
	for i := range embedding {
		bits := uint32(data[i*4]) |
			uint32(data[i*4+1])<<8 |
			uint32(data[i*4+2])<<16 |
			uint32(data[i*4+3])<<24
		embedding[i] = *(*float32)(unsafe.Pointer(&bits))
	}
	return embedding
}
