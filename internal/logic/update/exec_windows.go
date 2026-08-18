//go:build windows

package update

// execSelf Windows 无 exec 语义（在线更新仅支持 Linux），返回 false 由调用方退出。
func execSelf(bin string) bool {
	return false
}
