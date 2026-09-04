//go:build darwin && cgo

// Wails uses UTType in native dialogs. Link its framework for direct Go builds
// as well as builds through the Wails CLI.
package main

/*
#cgo LDFLAGS: -framework UniformTypeIdentifiers
*/
import "C"
