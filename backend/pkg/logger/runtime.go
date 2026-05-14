// pkg/logger/runtime.go
package logger

import "runtime"

// runtimeCallers resolves a program counter to a runtime.Frames iterator.
// Returns nil if the PC is invalid.
func runtimeCallers(pc uintptr) *runtime.Frames {
	if pc == 0 {
		return nil
	}
	pcs := []uintptr{pc}
	return runtime.CallersFrames(pcs)
}
