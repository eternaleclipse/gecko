//go:build !windows

package attach

// enableVT is a no-op outside Windows, where terminals always interpret
// escape sequences.
func enableVT() (restore func()) { return func() {} }
