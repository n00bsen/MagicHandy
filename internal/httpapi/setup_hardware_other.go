//go:build !windows

package httpapi

// displayAdapters is Windows-only; managed local chat targets Windows x64.
func displayAdapters() []displayAdapter { return nil }
