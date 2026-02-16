package vectordb

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	chromem "github.com/philippgille/chromem-go"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

// Store 向量存储封装
type Store struct {
	db       *chromem.DB
	dataPath string
}

// Document 待索引文档
type Document struct {
	ID       string            // 文档唯一标识
	Content  string            // 文本内容
	Metadata map[string]string // 元数据（tags, source 等）
}

// Result 检索结果
type Result struct {
	ID         string            `json:"id"`
	Content    string            `json:"content"`
	Metadata   map[string]string `json:"metadata"`
	Similarity float32           `json:"similarity"`
}

// NewStore 创建向量存储
// dataPath 为持久化目录，空字符串表示纯内存模式
func NewStore(dataPath string) (*Store, error) {
	var db *chromem.DB
	var err error

	if dataPath != "" {
		dbPath := filepath.Join(dataPath, "vectordb")
		db, err = chromem.NewPersistentDB(dbPath, false)
	} else {
		db = chromem.NewDB()
	}
	if err != nil {
		return nil, fmt.Errorf("创建向量数据库失败: %w", err)
	}

	logger.Info("向量数据库初始化完成", zap.String("path", dataPath))
	return &Store{db: db, dataPath: dataPath}, nil
}

// EnsureCollection 确保 Collection 存在
func (s *Store) EnsureCollection(name string, embeddingFunc chromem.EmbeddingFunc) (*chromem.Collection, error) {
	col, err := s.db.GetOrCreateCollection(name, nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("创建 collection %s 失败: %w", name, err)
	}
	return col, nil
}

// Index 索引文档到指定 Collection
func (s *Store) Index(ctx context.Context, collection *chromem.Collection, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	chromDocs := make([]chromem.Document, len(docs))
	for i, d := range docs {
		chromDocs[i] = chromem.Document{
			ID:       d.ID,
			Content:  d.Content,
			Metadata: d.Metadata,
		}
	}

	if err := collection.AddDocuments(ctx, chromDocs, runtime.NumCPU()); err != nil {
		return fmt.Errorf("索引文档失败: %w", err)
	}

	logger.Info("文档索引完成", zap.Int("count", len(docs)))
	return nil
}

// Query 语义检索
func (s *Store) Query(ctx context.Context, collection *chromem.Collection, query string, topK int) ([]Result, error) {
	results, err := collection.Query(ctx, query, topK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("语义检索失败: %w", err)
	}

	out := make([]Result, len(results))
	for i, r := range results {
		out[i] = Result{
			ID:         r.ID,
			Content:    r.Content,
			Metadata:   r.Metadata,
			Similarity: r.Similarity,
		}
	}

	return out, nil
}
