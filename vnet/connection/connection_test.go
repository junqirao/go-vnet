package connection

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// MockAdaptor 用于测试的模拟适配器
type MockAdaptor struct {
	dialFunc       func(ctx context.Context) (io.ReadWriteCloser, error)
	onReadFunc     func(p []byte) []byte
	onWriteFunc    func(p []byte) []byte
	onErrorFunc    func(e *Error)
	dialCount      int
	readCount      int
	writeCount     int
	errorCount     int
	mu             sync.Mutex
	dialError      error
	readError      error
	writeError     error
	dataToRead     []byte
	dataReadFrom   []byte
	dataWrittenTo  []byte
	transformRead  bool
	transformWrite bool
}

func (m *MockAdaptor) OnDial(ctx context.Context) (rwc io.ReadWriteCloser, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dialCount++
	if m.dialFunc != nil {
		return m.dialFunc(ctx)
	}
	if m.dialError != nil {
		return nil, m.dialError
	}
	// 创建一个基于缓冲区的 ReadWriteCloser
	if m.dataToRead != nil {
		return &mockReadWriteCloser{
			reader: bytes.NewReader(m.dataToRead),
			writer: &bytes.Buffer{},
		}, nil
	}
	return &mockReadWriteCloser{
		reader: bytes.NewReader([]byte{}),
		writer: &bytes.Buffer{},
	}, nil
}

func (m *MockAdaptor) OnError(e *Error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCount++
	if m.onErrorFunc != nil {
		m.onErrorFunc(e)
	}
}

func (m *MockAdaptor) OnRead(p []byte) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readCount++
	if m.onReadFunc != nil {
		return m.onReadFunc(p)
	}
	if m.transformRead {
		// 简单转换：将每个字节加1
		result := make([]byte, len(p))
		for i, b := range p {
			result[i] = b + 1
		}
		return result
	}
	return p
}

func (m *MockAdaptor) OnWrite(p []byte) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCount++
	m.dataWrittenTo = append(m.dataWrittenTo, p...)
	if m.onWriteFunc != nil {
		return m.onWriteFunc(p)
	}
	if m.transformWrite {
		// 简单转换：将每个字节减1
		result := make([]byte, len(p))
		for i, b := range p {
			result[i] = b - 1
		}
		return result
	}
	return p
}

func (m *MockAdaptor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dialCount = 0
	m.readCount = 0
	m.writeCount = 0
	m.errorCount = 0
	m.dialError = nil
	m.readError = nil
	m.writeError = nil
	m.dataToRead = nil
	m.dataReadFrom = nil
	m.dataWrittenTo = nil
	m.transformRead = false
	m.transformWrite = false
}

func (m *MockAdaptor) GetDialCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dialCount
}

func (m *MockAdaptor) GetReadCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.readCount
}

func (m *MockAdaptor) GetWriteCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeCount
}

func (m *MockAdaptor) GetErrorCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errorCount
}

// mockReadWriteCloser 模拟 io.ReadWriteCloser
type mockReadWriteCloser struct {
	reader *bytes.Reader
	writer *bytes.Buffer
	closed bool
	mu     sync.Mutex
}

func (m *mockReadWriteCloser) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.reader.Read(p)
}

func (m *mockReadWriteCloser) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.writer.Write(p)
}

func (m *mockReadWriteCloser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// ========== 单元测试 ==========

func TestNewConnection(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}

	conn := NewConnection(ctx, mock)

	if conn == nil {
		t.Fatal("NewConnection returned nil")
	}

	if conn.id == "" {
		t.Error("Connection ID should not be empty")
	}

	if conn.ctx != ctx {
		t.Error("Connection context not set correctly")
	}

	if conn.Adaptor != mock {
		t.Error("Connection adaptor not set correctly")
	}

	if conn.rwc != nil {
		t.Error("Connection rwc should be nil initially")
	}
}

func TestMustDial_Success(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	err := conn.MustDial()
	if err != nil {
		t.Errorf("MustDial failed: %v", err)
	}

	if mock.GetDialCount() != 1 {
		t.Errorf("Expected dial count 1, got %d", mock.GetDialCount())
	}

	if conn.rwc == nil {
		t.Error("rwc should be set after dial")
	}
}

func TestMustDial_Repeated(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	// 第一次 dial
	err := conn.MustDial()
	if err != nil {
		t.Errorf("First MustDial failed: %v", err)
	}

	// 第二次 dial - 不应该再次调用 adaptor 的 OnDial
	err = conn.MustDial()
	if err != nil {
		t.Errorf("Second MustDial failed: %v", err)
	}

	if mock.GetDialCount() != 1 {
		t.Errorf("Expected dial count 1 (not repeated), got %d", mock.GetDialCount())
	}
}

func TestMustDial_Error(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{
		dialError: errors.New("dial failed"),
	}
	conn := NewConnection(ctx, mock)

	err := conn.MustDial()
	if err == nil {
		t.Error("MustDial should return error when dial fails")
	}

	if mock.GetErrorCount() == 0 {
		t.Error("OnError should be called when dial fails")
	}
}

func TestRead_Success(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{
		dataToRead: []byte("test data"),
	}
	conn := NewConnection(ctx, mock)

	buf := make([]byte, 100)
	n, err := conn.Read(buf)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}

	if n != len("test data") {
		t.Errorf("Expected to read %d bytes, got %d", len("test data"), n)
	}

	if string(buf[:n]) != "test data" {
		t.Errorf("Expected to read 'test data', got '%s'", string(buf[:n]))
	}

	if mock.GetDialCount() != 1 {
		t.Errorf("Expected dial count 1, got %d", mock.GetDialCount())
	}

	if mock.GetReadCount() != 1 {
		t.Errorf("Expected read count 1, got %d", mock.GetReadCount())
	}
}

func TestRead_AutoDial(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{
		dataToRead: []byte("test"),
	}
	conn := NewConnection(ctx, mock)

	// 不显式调用 MustDial，直接 Read
	buf := make([]byte, 10)
	n, err := conn.Read(buf)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}

	if n != 4 {
		t.Errorf("Expected to read 4 bytes, got %d", n)
	}

	if mock.GetDialCount() != 1 {
		t.Errorf("Read should auto-dial, expected dial count 1, got %d", mock.GetDialCount())
	}
}

func TestRead_TransformData(t *testing.T) {
	ctx := context.Background()
	onReadCalled := false
	mock := &MockAdaptor{
		dataToRead: []byte("abc"),
		onReadFunc: func(p []byte) []byte {
			onReadCalled = true
			// OnRead 应该返回一个 buffer 供 rwc.Read 写入
			// 这里我们直接返回输入 buffer，不做任何修改
			return p
		},
	}
	conn := NewConnection(ctx, mock)

	buf := make([]byte, 10)
	n, err := conn.Read(buf)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}

	// 验证 OnRead 被调用
	if !onReadCalled {
		t.Error("OnRead should be called")
	}

	// rwc.Read 会将 dataToRead ("abc") 写入 buf
	expected := []byte{'a', 'b', 'c'}
	for i := 0; i < n; i++ {
		if buf[i] != expected[i] {
			t.Errorf("Byte at position %d: expected %d (%c), got %d (%c)", i, expected[i], expected[i], buf[i], buf[i])
		}
	}
}

func TestRead_Error(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return &errorReadWriteCloser{readErr: errors.New("read error")}, nil
		},
	}
	conn := NewConnection(ctx, mock)

	buf := make([]byte, 10)
	_, err := conn.Read(buf)
	if err == nil {
		t.Error("Read should return error when read fails")
	}

	if mock.GetErrorCount() == 0 {
		t.Error("OnError should be called when read fails")
	}
}

func TestWrite_Success(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	data := []byte("test data")
	n, err := conn.Write(data)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	if n != len(data) {
		t.Errorf("Expected to write %d bytes, got %d", len(data), n)
	}

	if mock.GetDialCount() != 1 {
		t.Errorf("Expected dial count 1, got %d", mock.GetDialCount())
	}

	if mock.GetWriteCount() != 1 {
		t.Errorf("Expected write count 1, got %d", mock.GetWriteCount())
	}

	if !bytes.Equal(mock.dataWrittenTo, data) {
		t.Errorf("Expected to write '%s', got '%s'", string(data), string(mock.dataWrittenTo))
	}
}

func TestWrite_AutoDial(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	// 不显式调用 MustDial，直接 Write
	data := []byte("test")
	n, err := conn.Write(data)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	if n != 4 {
		t.Errorf("Expected to write 4 bytes, got %d", n)
	}

	if mock.GetDialCount() != 1 {
		t.Errorf("Write should auto-dial, expected dial count 1, got %d", mock.GetDialCount())
	}
}

func TestWrite_TransformData(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{
		transformWrite: true, // OnWrite 会将每个字节-1
	}
	conn := NewConnection(ctx, mock)

	data := []byte{99, 100, 101} // "cde"
	n, err := conn.Write(data)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	if n != 3 {
		t.Errorf("Expected to write 3 bytes, got %d", n)
	}

	// OnWrite 会将数据转换成 {98, 99, 100} ("bcd")
	// 所以实际写入 rwc 的是 "bcd"
	// 但 MockAdaptor 记录的是原始输入 data (cde)
	expected := []byte{99, 100, 101} // "cde" - 这是 MockAdaptor 记录的数据
	if !bytes.Equal(mock.dataWrittenTo, expected) {
		t.Errorf("Expected dataWrittenTo '%v', got '%v'", expected, mock.dataWrittenTo)
	}
}

func TestWrite_Error(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return &errorReadWriteCloser{writeErr: errors.New("write error")}, nil
		},
	}
	conn := NewConnection(ctx, mock)

	_, err := conn.Write([]byte("test"))
	if err == nil {
		t.Error("Write should return error when write fails")
	}

	if mock.GetErrorCount() == 0 {
		t.Error("OnError should be called when write fails")
	}
}

func TestClose_Success(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	conn.MustDial()

	err := conn.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestClose_NilRWC(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	// 不调用 MustDial，rwc 为 nil
	err := conn.Close()
	if err != nil {
		t.Errorf("Close should not error when rwc is nil: %v", err)
	}
}

func TestClose_Repeated(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	conn.MustDial()

	err1 := conn.Close()
	err2 := conn.Close()

	if err1 != nil {
		t.Errorf("First close failed: %v", err1)
	}

	// 第二次关闭应该成功（因为内部检查 rwc == nil）
	if err2 != nil {
		t.Errorf("Second close failed: %v", err2)
	}
}

func TestError_Format(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	err := &Error{
		Conn:     conn,
		Position: PosRead,
		Err:      errors.New("some error"),
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error string should not be empty")
	}

	expectedStr := fmt.Sprintf("connection %s error caused at %s", conn.id, PosRead)
	if errStr[:len(expectedStr)] != expectedStr {
		t.Errorf("Error string format incorrect, expected prefix '%s', got '%s'", expectedStr, errStr[:len(expectedStr)])
	}
}

func TestError_DifferentPositions(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	positions := []string{PosDial, PosRead, PosWrite}
	for _, pos := range positions {
		err := &Error{
			Conn:     conn,
			Position: pos,
			Err:      errors.New("test error"),
		}

		errStr := err.Error()
		if pos == PosDial && !contains(errStr, "dial") {
			t.Errorf("Error for position %s should contain 'dial'", pos)
		}
		if pos == PosRead && !contains(errStr, "read") {
			t.Errorf("Error for position %s should contain 'read'", pos)
		}
		if pos == PosWrite && !contains(errStr, "write") {
			t.Errorf("Error for position %s should contain 'write'", pos)
		}
	}
}

func TestConcurrentRead(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{
		dataToRead: []byte("concurrent test data"),
	}
	conn := NewConnection(ctx, mock)

	var wg sync.WaitGroup
	numGoroutines := 10
	errorCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 10)
			_, err := conn.Read(buf)
			// EOF 是正常的，因为数据会被读完
			if err != nil && err != io.EOF {
				errorCh <- err
			}
		}()
	}

	wg.Wait()
	close(errorCh)

	for err := range errorCh {
		t.Errorf("Concurrent read error: %v", err)
	}
}

func TestConcurrentWrite(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	var wg sync.WaitGroup
	numGoroutines := 10
	errorCh := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := []byte(fmt.Sprintf("test-%d", i))
			_, err := conn.Write(data)
			if err != nil {
				errorCh <- err
			}
		}()
	}

	wg.Wait()
	close(errorCh)

	for err := range errorCh {
		t.Errorf("Concurrent write error: %v", err)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	ctx := context.Background()
	mock := &MockAdaptor{
		dataToRead: bytes.Repeat([]byte("rw"), 1000),
	}
	conn := NewConnection(ctx, mock)

	var wg sync.WaitGroup
	errorCh := make(chan error, 20)

	// 10 个 goroutines 读取
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 100)
			_, err := conn.Read(buf)
			if err != nil && err != io.EOF {
				errorCh <- err
			}
		}()
	}

	// 10 个 goroutines 写入
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := []byte(fmt.Sprintf("write-%d", i))
			_, err := conn.Write(data)
			if err != nil {
				errorCh <- err
			}
		}()
	}

	wg.Wait()
	close(errorCh)

	for err := range errorCh {
		t.Errorf("Concurrent read/write error: %v", err)
	}
}

func TestOnReadCalled(t *testing.T) {
	ctx := context.Background()
	onReadCalled := false
	mock := &MockAdaptor{
		dataToRead: []byte("test"),
		onReadFunc: func(p []byte) []byte {
			onReadCalled = true
			return p
		},
	}
	conn := NewConnection(ctx, mock)

	buf := make([]byte, 10)
	conn.Read(buf)

	if !onReadCalled {
		t.Error("OnRead should be called during Read")
	}
}

func TestOnWriteCalled(t *testing.T) {
	ctx := context.Background()
	writeCalled := false
	mock := &MockAdaptor{
		onWriteFunc: func(p []byte) []byte {
			writeCalled = true
			return p
		},
	}
	conn := NewConnection(ctx, mock)

	conn.Write([]byte("test"))

	if !writeCalled {
		t.Error("OnWrite should be called during Write")
	}
}

func TestOnErrorCalled(t *testing.T) {
	ctx := context.Background()
	capturedErrors := make([]*Error, 0)
	mock := &MockAdaptor{
		onErrorFunc: func(e *Error) {
			capturedErrors = append(capturedErrors, e)
		},
		dialError: errors.New("dial error"),
	}
	conn := NewConnection(ctx, mock)

	conn.MustDial()

	if len(capturedErrors) == 0 {
		t.Error("OnError should be called when error occurs")
	}

	if capturedErrors[0].Position != PosDial {
		t.Errorf("Expected position %s, got %s", PosDial, capturedErrors[0].Position)
	}

	if capturedErrors[0].Conn != conn {
		t.Error("Error connection should match the connection")
	}
}

func TestConnectionWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	mock := &MockAdaptor{
		dialFunc: func(dialCtx context.Context) (io.ReadWriteCloser, error) {
			select {
			case <-dialCtx.Done():
				return nil, dialCtx.Err()
			case <-time.After(200 * time.Millisecond):
				return nil, nil
			}
		},
	}
	conn := NewConnection(ctx, mock)

	err := conn.MustDial()
	if err == nil {
		t.Error("Expected error when dial times out")
	}

	if mock.GetErrorCount() == 0 {
		t.Error("OnError should be called when dial times out")
	}
}

// ========== 辅助函数 ==========

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[0:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// errorReadWriteCloser 总是返回错误
type errorReadWriteCloser struct {
	readErr  error
	writeErr error
}

func (e *errorReadWriteCloser) Read(p []byte) (n int, err error) {
	return 0, e.readErr
}

func (e *errorReadWriteCloser) Write(p []byte) (n int, err error) {
	return 0, e.writeErr
}

func (e *errorReadWriteCloser) Close() error {
	return nil
}
