//go:build windows

package update

// signalSelfTerm Windows 无自信号机制，返回 false 由调用方直接退出
func signalSelfTerm() bool {
	return false
}
