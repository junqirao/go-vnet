package metrics

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testRecord 用于测试的记录类型
type testRecord struct {
	ts    int64
	value int
}

func (r testRecord) Timestamp() int64 {
	return r.ts
}

// TestNewTimeSeriesDB 测试创建时序数据库
func TestNewTimeSeriesDB(t *testing.T) {
	db := NewTimeSeriesDB[testRecord](100)
	if db == nil {
		t.Fatal("NewTimeSeriesDB returned nil")
	}
	if db.Capacity() != 100 {
		t.Errorf("Expected capacity 100, got %d", db.Capacity())
	}
	if db.Size() != 0 {
		t.Errorf("Expected size 0, got %d", db.Size())
	}
}

// TestAppend 测试添加记录
func TestAppend(t *testing.T) {
	db := NewTimeSeriesDB[testRecord](10)

	// 添加第一条记录
	record1 := testRecord{ts: 1, value: 1}
	db.Append(record1)
	if db.Size() != 1 {
		t.Errorf("Expected size 1, got %d", db.Size())
	}

	// 添加多条记录
	for i := 2; i <= 5; i++ {
		db.Append(testRecord{ts: int64(i), value: i})
	}
	if db.Size() != 5 {
		t.Errorf("Expected size 5, got %d", db.Size())
	}
}

// TestCapacityLimit 测试容量限制和旧数据淘汰
func TestCapacityLimit(t *testing.T) {
	capacity := 5
	db := NewTimeSeriesDB[testRecord](capacity)

	// 添加超过容量的记录
	for i := 1; i <= capacity+3; i++ {
		db.Append(testRecord{ts: int64(i), value: i})
	}

	// 验证大小不超过容量
	if db.Size() != capacity {
		t.Errorf("Expected size %d, got %d", capacity, db.Size())
	}

	// 获取最新记录，应该是最新的 capacity 条
	records := db.GetAll()
	if len(records) != capacity {
		t.Errorf("Expected %d records, got %d", capacity, len(records))
	}

	// 验证最老的记录已被淘汰，最新的记录是 capacity+3
	expectedLatestTs := int64(capacity + 3)
	if records[0].Timestamp() != expectedLatestTs {
		t.Errorf("Expected latest timestamp %d, got %d", expectedLatestTs, records[0].Timestamp())
	}
}

// TestGetLatest 测试获取最新记录
func TestGetLatest(t *testing.T) {
	db := NewTimeSeriesDB[testRecord](10)

	// 添加记录
	for i := 1; i <= 5; i++ {
		db.Append(testRecord{ts: int64(i), value: i})
	}

	// 获取最新3条
	records := db.GetLatest(3)
	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}

	// 验证顺序（从新到旧）
	if records[0].Timestamp() != 5 {
		t.Errorf("Expected timestamp 5, got %d", records[0].Timestamp())
	}
	if records[1].Timestamp() != 4 {
		t.Errorf("Expected timestamp 4, got %d", records[1].Timestamp())
	}
	if records[2].Timestamp() != 3 {
		t.Errorf("Expected timestamp 3, got %d", records[2].Timestamp())
	}

	// 测试请求数超过存储数
	records = db.GetLatest(10)
	if len(records) != 5 {
		t.Errorf("Expected 5 records, got %d", len(records))
	}

	// 测试请求0条
	records = db.GetLatest(0)
	if len(records) != 0 {
		t.Errorf("Expected 0 records, got %d", len(records))
	}

	// 测试请求负数
	records = db.GetLatest(-1)
	if len(records) != 0 {
		t.Errorf("Expected 0 records, got %d", len(records))
	}
}

// TestGetRange 测试按时间范围查询
func TestGetRange(t *testing.T) {
	db := NewTimeSeriesDB[testRecord](10)

	// 添加记录
	for i := 1; i <= 10; i++ {
		db.Append(testRecord{ts: int64(i), value: i})
	}

	// 查询范围 [3, 7]
	records := db.GetRange(3, 7)
	if len(records) != 5 {
		t.Errorf("Expected 5 records, got %d", len(records))
	}

	// 验证所有记录的时间戳都在范围内
	for _, record := range records {
		ts := record.Timestamp()
		if ts < 3 || ts > 7 {
			t.Errorf("Record timestamp %d is not in range [3, 7]", ts)
		}
	}

	// 查询超出范围的记录
	records = db.GetRange(15, 20)
	if len(records) != 0 {
		t.Errorf("Expected 0 records, got %d", len(records))
	}

	// 查询空数据库
	emptyDB := NewTimeSeriesDB[testRecord](10)
	records = emptyDB.GetRange(1, 10)
	if len(records) != 0 {
		t.Errorf("Expected 0 records, got %d", len(records))
	}
}

// TestGetAll 测试获取所有记录
func TestGetAll(t *testing.T) {
	db := NewTimeSeriesDB[testRecord](10)

	// 空数据库
	records := db.GetAll()
	if len(records) != 0 {
		t.Errorf("Expected 0 records, got %d", len(records))
	}

	// 添加记录
	for i := 1; i <= 5; i++ {
		db.Append(testRecord{ts: int64(i), value: i})
	}

	records = db.GetAll()
	if len(records) != 5 {
		t.Errorf("Expected 5 records, got %d", len(records))
	}

	// 验证顺序（从新到旧）
	if records[0].Timestamp() != 5 {
		t.Errorf("Expected first record timestamp 5, got %d", records[0].Timestamp())
	}
	if records[4].Timestamp() != 1 {
		t.Errorf("Expected last record timestamp 1, got %d", records[4].Timestamp())
	}
}

// TestClear 测试清空数据库
func TestClear(t *testing.T) {
	db := NewTimeSeriesDB[testRecord](10)

	// 添加记录
	for i := 1; i <= 5; i++ {
		db.Append(testRecord{ts: int64(i), value: i})
	}

	// 清空
	db.Clear()

	if db.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", db.Size())
	}

	records := db.GetAll()
	if len(records) != 0 {
		t.Errorf("Expected 0 records after clear, got %d", len(records))
	}
}

// TestConcurrentAppend 测试并发添加
func TestConcurrentAppend(t *testing.T) {
	db := NewTimeSeriesDB[testRecord](10000)
	var wg sync.WaitGroup

	numGoroutines := 10
	recordsPerGoroutine := 1000

	start := time.Now().Unix()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				db.Append(testRecord{
					ts:    start + int64(j),
					value: id*recordsPerGoroutine + j,
				})
			}
		}(i)
	}

	wg.Wait()

	// 验证总记录数
	expectedSize := numGoroutines * recordsPerGoroutine
	if db.Size() != expectedSize {
		t.Errorf("Expected size %d, got %d", expectedSize, db.Size())
	}
}

// TestConcurrentReadWrite 测试并发读写
func TestConcurrentReadWrite(t *testing.T) {
	db := NewTimeSeriesDB[testRecord](10000)
	var wg sync.WaitGroup

	stopFlag := atomic.Bool{}

	// 启动写入协程
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			j := 0
			for !stopFlag.Load() {
				db.Append(testRecord{
					ts:    time.Now().Unix(),
					value: id*1000 + j,
				})
				j++
			}
		}(i)
	}

	// 启动读取协程
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stopFlag.Load() {
				db.GetLatest(100)
				db.GetRange(time.Now().Unix()-60, time.Now().Unix())
				db.Size()
				db.GetAll()
			}
		}()
	}

	// 运行1秒
	time.Sleep(1 * time.Second)
	stopFlag.Store(true)
	wg.Wait()
}

// BenchmarkAppend 基准测试：单次追加
func BenchmarkAppend(b *testing.B) {
	db := NewTimeSeriesDB[testRecord](b.N)
	record := testRecord{ts: time.Now().Unix(), value: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Append(record)
	}
}

// BenchmarkConcurrentAppend 基准测试：并发追加
func BenchmarkConcurrentAppend(b *testing.B) {
	db := NewTimeSeriesDB[testRecord](b.N * 2)
	var wg sync.WaitGroup

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db.Append(testRecord{ts: time.Now().Unix(), value: 1})
		}()
	}
	wg.Wait()
}

// BenchmarkGetLatest 基准测试：获取最新记录
func BenchmarkGetLatest(b *testing.B) {
	db := NewTimeSeriesDB[testRecord](10000)

	// 预填充数据
	for i := 0; i < 10000; i++ {
		db.Append(testRecord{ts: int64(i), value: i})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.GetLatest(100)
	}
}

// BenchmarkGetRange 基准测试：按时间范围查询
func BenchmarkGetRange(b *testing.B) {
	db := NewTimeSeriesDB[testRecord](10000)

	// 预填充数据
	for i := 0; i < 10000; i++ {
		db.Append(testRecord{ts: int64(i), value: i})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.GetRange(5000, 6000)
	}
}

// BenchmarkConcurrentReadWrite 压力测试：并发读写
func BenchmarkConcurrentReadWrite(b *testing.B) {
	db := NewTimeSeriesDB[testRecord](10000)

	// 预填充数据
	for i := 0; i < 5000; i++ {
		db.Append(testRecord{ts: int64(i), value: i})
	}

	var wg sync.WaitGroup
	writeCount := atomic.Int64{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 30% 写入，70% 读取
		if i%10 < 3 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				db.Append(testRecord{
					ts:    time.Now().Unix(),
					value: int(writeCount.Add(1)),
				})
			}()
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				db.GetLatest(100)
				db.GetRange(1000, 2000)
				db.Size()
			}()
		}
	}
	wg.Wait()
}

// BenchmarkHighThroughput 高吞吐量测试
func BenchmarkHighThroughput(b *testing.B) {
	db := NewTimeSeriesDB[testRecord](b.N)
	var wg sync.WaitGroup

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			db.Append(testRecord{
				ts:    time.Now().Unix(),
				value: idx,
			})
		}(i)
	}
	wg.Wait()
}

// TestRealWorldUsage 真实场景测试
func TestRealWorldUsage(t *testing.T) {
	// 模拟存储最近1小时的指标数据，每秒一个数据点
	capacity := 3600 // 1小时 = 3600秒
	db := NewTimeSeriesDB[testRecord](capacity)

	startTime := time.Now().Add(-time.Hour).Unix()
	now := time.Now().Unix()

	// 模拟过去1小时的数据
	for ts := startTime; ts <= now; ts++ {
		db.Append(testRecord{
			ts:    ts,
			value: int(ts % 100),
		})
	}

	// 查询最近10分钟的数据
	tenMinutesAgo := now - 600
	records := db.GetRange(tenMinutesAgo, now)
	expectedRecords := 601 // 600秒 + 当前秒
	if len(records) != expectedRecords {
		t.Errorf("Expected %d records in last 10 minutes, got %d", expectedRecords, len(records))
	}

	// 查询最新100条
	latest := db.GetLatest(100)
	if len(latest) != 100 {
		t.Errorf("Expected 100 latest records, got %d", len(latest))
	}

	// 验证最新记录的时间戳
	if latest[0].Timestamp() != now {
		t.Errorf("Expected latest timestamp %d, got %d", now, latest[0].Timestamp())
	}

	t.Logf("Database size: %d/%d", db.Size(), db.Capacity())
}
