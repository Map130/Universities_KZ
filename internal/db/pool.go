// Package db реализует подключение к SurrealDB и миграцию схемы.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/surrealdb/surrealdb.go"
)

// Pool — пул подключений к SurrealDB.
//
// SurrealDB Go SDK v1.x использует одно WebSocket-соединение на экземпляр
// *surrealdb.DB. Под конкурентной нагрузкой (десятки горутин Fiber) одно
// соединение захлёбывается: запросы интерферируют, ответы перемешиваются,
// SDK возвращает ошибки или nil-результаты.
//
// Pool создаёт N независимых подключений и раздаёт их по round-robin.
// Это простейший и достаточный подход для read-heavy нагрузки, где
// не требуется привязка транзакции к конкретному соединению.
type Pool struct {
	conns []*surrealdb.DB
	size  int
	next  atomic.Uint64
}

// PoolConfig — параметры создания пула.
type PoolConfig struct {
	// Config — параметры подключения к SurrealDB (URL, User, Pass, NS, DB).
	Config Config

	// Size — количество соединений в пуле.
	// Рекомендуемое значение: 2× количество ядер CPU для read-heavy нагрузки.
	// Минимум 1. Если указано 0, будет использовано значение по умолчанию (8).
	Size int
}

// DefaultPoolSize — размер пула по умолчанию.
const DefaultPoolSize = 8

// NewPool создаёт пул из N подключений к SurrealDB.
// Каждое подключение проходит полный цикл: connect → sign in → use ns/db.
// Если хотя бы одно подключение не удалось установить, все ранее
// созданные соединения закрываются, и возвращается ошибка.
func NewPool(ctx context.Context, cfg PoolConfig) (*Pool, error) {
	size := cfg.Size
	if size <= 0 {
		size = DefaultPoolSize
	}

	conns := make([]*surrealdb.DB, 0, size)

	// В случае ошибки закрываем все уже открытые соединения.
	cleanup := func() {
		for _, c := range conns {
			_ = c.Close(ctx)
		}
	}

	for i := 0; i < size; i++ {
		conn, err := Connect(ctx, cfg.Config)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("db.Pool: connection %d/%d: %w", i+1, size, err)
		}
		conns = append(conns, conn)
	}

	slog.Info("created database pool", "size", size, "url", cfg.Config.URL)

	return &Pool{
		conns: conns,
		size:  size,
	}, nil
}

// Get возвращает следующее соединение из пула по round-robin.
// Метод безопасен для конкурентного вызова из множества горутин.
// Соединение НЕ нужно возвращать обратно — оно остаётся в пуле.
func (p *Pool) Get() *surrealdb.DB {
	idx := p.next.Add(1) - 1
	return p.conns[idx%uint64(p.size)]
}

// Size возвращает количество соединений в пуле.
func (p *Pool) Size() int {
	return p.size
}

// Close закрывает все соединения в пуле.
// После вызова Close пул нельзя использовать.
func (p *Pool) Close(ctx context.Context) error {
	var firstErr error
	for i, conn := range p.conns {
		if err := conn.Close(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("db.Pool: close connection %d: %w", i, err)
		}
	}
	slog.Info("closed database pool", "size", p.size)
	return firstErr
}

// RunMigrationsOnPool выполняет миграции через первое соединение пула.
// Достаточно выполнить один раз — схема применяется ко всей базе.
func RunMigrationsOnPool(ctx context.Context, pool *Pool) error {
	return RunMigrations(ctx, pool.conns[0])
}
