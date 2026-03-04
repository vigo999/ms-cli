package session

// Store 会话存储接口
type Store interface {
	// Save 保存会话
	Save(session *Session) error

	// Load 加载会话
	Load(id ID) (*Session, error)

	// Delete 删除会话
	Delete(id ID) error

	// List 列出所有会话信息
	List() ([]Info, error)

	// ListFiltered 根据过滤器列出会话
	ListFiltered(filter Filter) ([]Info, error)

	// Exists 检查会话是否存在
	Exists(id ID) bool

	// Close 关闭存储
	Close() error
}
