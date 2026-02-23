package embed

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
	"time"
)

//go:embed data/ui.zip
var UIZip embed.FS

var (
	// 缓存的文件系统实例，避免重复创建
	cachedFS *zipFS
	once     sync.Once
)

// GetUIFS 返回 UI 文件的 fs.FS，使用缓存避免重复读取
func GetUIFS() (fs.FS, error) {
	var initErr error

	once.Do(func() {
		// 读取 embed 的 ui.zip 文件到内存（只读取一次）
		zipData, err := UIZip.ReadFile("data/ui.zip")
		if err != nil {
			initErr = err
			return
		}

		// 创建 zip reader
		zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
		if err != nil {
			initErr = err
			return
		}

		// 创建带索引的 zipFS
		cachedFS = newZipFS(zipReader)
	})

	if initErr != nil {
		return nil, initErr
	}

	// 直接返回，zip 第一层就是 index.html 所在层级
	return cachedFS, nil
}

// zipFS 实现了 fs.FS 接口，使用 map 加速文件查找
type zipFS struct {
	reader *zip.Reader
	// 使用 map 缓存文件索引，避免每次线性遍历
	files map[string]*zip.File
}

func newZipFS(reader *zip.Reader) *zipFS {
	fs := &zipFS{
		reader: reader,
		files:  make(map[string]*zip.File),
	}

	// 构建文件索引
	for _, f := range reader.File {
		fs.files[f.Name] = f
	}

	return fs
}

func (z *zipFS) Stat(name string) (fs.FileInfo, error) {
	// 确保路径不以 / 开头
	name = strings.TrimPrefix(name, "/")

	// 处理根目录
	if name == "" || name == "." {
		return &dirInfo{name: "", mode: fs.ModeDir}, nil
	}

	// 查找目录（检查是否有以 name/ 开头的文件）
	dirName := name + "/"
	for fName := range z.files {
		if strings.HasPrefix(fName, dirName) {
			return &dirInfo{name: name, mode: fs.ModeDir}, nil
		}
	}

	// 查找文件
	if file, ok := z.files[name]; ok {
		return file.FileInfo(), nil
	}

	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

// dirInfo 表示目录的文件信息
type dirInfo struct {
	name string
	mode fs.FileMode
}

func (d *dirInfo) Name() string       { return d.name }
func (d *dirInfo) Size() int64        { return 0 }
func (d *dirInfo) Mode() fs.FileMode  { return d.mode }
func (d *dirInfo) ModTime() time.Time { return time.Time{} }
func (d *dirInfo) IsDir() bool        { return true }
func (d *dirInfo) Sys() interface{}   { return nil }

// zipFile 包装了 zip 文件以实现 fs.File 接口
type zipFile struct {
	rc   io.ReadCloser
	info fs.FileInfo
}

func (zf *zipFile) Read(p []byte) (n int, err error) {
	return zf.rc.Read(p)
}

func (zf *zipFile) Close() error {
	return zf.rc.Close()
}

func (zf *zipFile) Stat() (fs.FileInfo, error) {
	return zf.info, nil
}

func (z *zipFS) Open(name string) (fs.File, error) {
	// 确保路径不以 / 开头（zip 内部路径通常不包含前导 /）
	name = strings.TrimPrefix(name, "/")

	// 处理根目录，返回目录文件，http.FileServer 会自动查找 index.html
	if name == "" || name == "." {
		return &dirFile{dirInfo: dirInfo{name: "", mode: fs.ModeDir}}, nil
	}

	// 处理目录请求，返回目录文件
	if strings.HasSuffix(name, "/") {
		name = strings.TrimSuffix(name, "/")
	}
	dirName := name + "/"
	for fName := range z.files {
		if strings.HasPrefix(fName, dirName) {
			return &dirFile{dirInfo: dirInfo{name: name, mode: fs.ModeDir}}, nil
		}
	}

	// 使用 map 直接查找文件，O(1) 复杂度
	if file, ok := z.files[name]; ok {
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		return &zipFile{rc: rc, info: file.FileInfo()}, nil
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (z *zipFS) ReadDir(name string) ([]fs.DirEntry, error) {
	// 确保路径不以 / 开头
	name = strings.TrimPrefix(name, "/")

	// 处理根目录
	if name == "" || name == "." {
		name = ""
	} else {
		name = name + "/"
	}

	var entries []fs.DirEntry
	seen := make(map[string]bool)

	for fName, file := range z.files {
		// 只收集直接子文件/目录
		if strings.HasPrefix(fName, name) {
			relativePath := strings.TrimPrefix(fName, name)
			if relativePath == "" {
				continue
			}

			// 获取第一级路径
			parts := strings.SplitN(relativePath, "/", 2)
			firstPart := parts[0]

			if !seen[firstPart] {
				seen[firstPart] = true
				entries = append(entries, &dirEntry{
					name:  firstPart,
					info:  file.FileInfo(),
					isDir: len(parts) > 1,
				})
			}
		}
	}

	return entries, nil
}

// dirEntry 实现了 fs.DirEntry 接口
type dirEntry struct {
	name  string
	info  fs.FileInfo
	isDir bool
}

func (d *dirEntry) Name() string {
	return d.name
}

func (d *dirEntry) IsDir() bool {
	return d.isDir
}

func (d *dirEntry) Type() fs.FileMode {
	return d.info.Mode().Type()
}

func (d *dirEntry) Info() (fs.FileInfo, error) {
	return d.info, nil
}

// dirFile 实现了 fs.File 接口来表示目录
type dirFile struct {
	dirInfo
}

func (d *dirFile) Read(p []byte) (n int, err error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: fmt.Errorf("is a directory")}
}

func (d *dirFile) Close() error {
	return nil
}

func (d *dirFile) Stat() (fs.FileInfo, error) {
	return &d.dirInfo, nil
}
