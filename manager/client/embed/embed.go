package embed

import (
	"embed"
	"io/fs"
	"sync"

	f "go-vnet/common/fs"
)

//go:embed data/ui.zip
var UIZip embed.FS

var (
	// 缓存的文件系统实例，避免重复创建
	cachedFS fs.FS
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

		// 使用公共实现创建 zip 文件系统
		cachedFS, initErr = f.NewZipFSFromBytes(zipData)
	})

	if initErr != nil {
		return nil, initErr
	}

	return cachedFS, nil
}
