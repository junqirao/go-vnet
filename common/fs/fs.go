package fs

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// 初始化 MIME 类型
func init() {
	// 手动添加一些可能缺失的 MIME 类型
	if mime.TypeByExtension(".js") == "" {
		_ = mime.AddExtensionType(".js", "application/javascript")
	}
	if mime.TypeByExtension(".mjs") == "" {
		_ = mime.AddExtensionType(".mjs", "application/javascript")
	}
	if mime.TypeByExtension(".wasm") == "" {
		_ = mime.AddExtensionType(".wasm", "application/wasm")
	}
}

// NewZipFS 从 zip.Reader 创建文件系统实例
func NewZipFS(reader *zip.Reader) fs.FS {
	return newZipFS(reader)
}

// NewZipFSFromBytes 从字节数据创建 zip 文件系统实例
func NewZipFSFromBytes(data []byte) (fs.FS, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	return newZipFS(zipReader), nil
}

// NewZipFSFromReader 从 io.ReaderAt 创建 zip 文件系统实例
func NewZipFSFromReader(reader io.ReaderAt, size int64) (fs.FS, error) {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, err
	}
	return newZipFS(zipReader), nil
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

	// 查找文件
	if file, ok := z.files[name]; ok {
		return file.FileInfo(), nil
	}

	// 查找目录（检查是否有以 name/ 开头的文件）
	dirName := strings.TrimSuffix(name, "/") + "/"
	for fName := range z.files {
		if strings.HasPrefix(fName, dirName) {
			return &dirInfo{name: dirName, mode: fs.ModeDir}, nil
		}
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
func (d *dirInfo) Mode() fs.FileMode  { return d.mode | 0o555 } // 确保目录可读可执行
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

	// 使用 map 直接查找文件，O(1) 复杂度
	if file, ok := z.files[name]; ok {
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		return &zipFile{rc: rc, info: file.FileInfo()}, nil
	}

	// 处理目录请求（如 "login/" 或检查是否是目录）
	dirName := strings.TrimSuffix(name, "/") + "/"
	for fName := range z.files {
		if strings.HasPrefix(fName, dirName) {
			return &dirFile{dirInfo: dirInfo{name: dirName, mode: fs.ModeDir}}, nil
		}
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

// SpaHandler 处理 SPA 请求
type SpaHandler struct {
	FileSystem fs.FS
}

func (h *SpaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 获取请求路径
	requestPath := r.URL.Path

	// 尝试打开文件
	f, err := h.FileSystem.Open(requestPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// 文件不存在，返回 index.html
		f, err = h.FileSystem.Open("index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	defer f.Close()

	// 获取文件信息
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 如果是目录，返回 index.html
	if stat.IsDir() {
		f.Close()
		f, err = h.FileSystem.Open("index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()
		stat, _ = f.Stat()
	}

	// 设置 Content-Type
	contentType := h.getMimeType(stat.Name())
	w.Header().Set("Content-Type", contentType)

	// 写入文件内容
	_, _ = io.Copy(w, f)
}

// getMimeType 根据文件名获取 MIME 类型
func (h *SpaHandler) getMimeType(fileName string) string {
	// 默认 HTML
	if fileName == "" || fileName == "index.html" {
		return "text/html; charset=utf-8"
	}

	// 获取扩展名
	ext := path.Ext(fileName)
	if ext == "" {
		return "application/octet-stream"
	}

	// 移除前导点
	ext = strings.TrimPrefix(ext, ".")

	// 根据扩展名返回 MIME 类型
	switch strings.ToLower(ext) {
	case "js", "mjs":
		return "application/javascript"
	case "css":
		return "text/css"
	case "html":
		return "text/html; charset=utf-8"
	case "json":
		return "application/json"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "svg":
		return "image/svg+xml"
	case "ico":
		return "image/x-icon"
	case "woff":
		return "font/woff"
	case "woff2":
		return "font/woff2"
	case "ttf":
		return "font/ttf"
	case "eot":
		return "application/vnd.ms-fontobject"
	case "wasm":
		return "application/wasm"
	default:
		// 使用 mime 包的默认映射
		if ct := mime.TypeByExtension("." + ext); ct != "" {
			return ct
		}
		return "application/octet-stream"
	}
}
