package metrics

import (
	"sync"
)

// Record 定义时序数据记录的接口
type Record interface {
	Timestamp() int64 // Unix 秒级时间戳
}

// TimeSeriesDB 基于内存的时序数据库（泛型）
type TimeSeriesDB[T Record] struct {
	mu       sync.RWMutex
	records  []T
	capacity int
	size     int
	head     int
}

// NewTimeSeriesDB 创建一个新的时序数据库
func NewTimeSeriesDB[T Record](capacity int) *TimeSeriesDB[T] {
	return &TimeSeriesDB[T]{
		records:  make([]T, capacity),
		capacity: capacity,
		size:     0,
		head:     0,
	}
}

// Append 添加一条记录到时序数据库中，达到上限后淘汰最老的数据
func (db *TimeSeriesDB[T]) Append(record T) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.records[db.head] = record
	db.head = (db.head + 1) % db.capacity
	if db.size < db.capacity {
		db.size++
	}
}

// GetLatest 获取最新的n条记录
func (db *TimeSeriesDB[T]) GetLatest(n int) []T {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if n <= 0 || db.size == 0 {
		return []T{}
	}

	if n > db.size {
		n = db.size
	}

	result := make([]T, n)
	for i := 0; i < n; i++ {
		idx := (db.head - 1 - i + db.capacity) % db.capacity
		result[i] = db.records[idx]
	}
	return result
}

// GetRange 获取指定时间范围内的记录
func (db *TimeSeriesDB[T]) GetRange(start, end int64) []T {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.size == 0 {
		return []T{}
	}

	var result []T
	for i := 0; i < db.size; i++ {
		idx := (db.head - 1 - i + db.capacity) % db.capacity
		record := db.records[idx]
		ts := record.Timestamp()
		if ts >= start && ts <= end {
			result = append(result, record)
		}
	}
	return result
}

// GetAll 获取所有记录（按时间从新到旧）
func (db *TimeSeriesDB[T]) GetAll() []T {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.size == 0 {
		return []T{}
	}

	result := make([]T, db.size)
	for i := 0; i < db.size; i++ {
		idx := (db.head - 1 - i + db.capacity) % db.capacity
		result[i] = db.records[idx]
	}
	return result
}

// Size 获取当前存储的记录数量
func (db *TimeSeriesDB[T]) Size() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.size
}

// Capacity 获取最大存储容量
func (db *TimeSeriesDB[T]) Capacity() int {
	return db.capacity
}

// Clear 清空所有记录
func (db *TimeSeriesDB[T]) Clear() {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.size = 0
	db.head = 0
}
