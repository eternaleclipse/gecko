package inspect

// Snapshot is not implemented on Windows yet; foreground detection there
// relies on shell integration and window titles only.
func Snapshot() (*Table, error) { return newTable(nil), nil }

// Cwd is unknown on Windows.
func Cwd(pid int) string { return "" }
