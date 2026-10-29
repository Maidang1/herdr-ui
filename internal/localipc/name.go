// Package localipc connects to the local transport used by the Herdr daemon.
package localipc

// WindowsPipeName mirrors interprocess 2.4.2 GenericNamespaced: the daemon
// prepends the local pipe namespace to the exact logical socket path. Do not
// clean, hash, or replace separators here; that would address another pipe.
func WindowsPipeName(path string) string { return `\\.\pipe\` + path }
