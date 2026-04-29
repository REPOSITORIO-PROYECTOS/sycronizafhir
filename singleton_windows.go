//go:build windows

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	mutexName             = "Global\\sycronizafhir-singleton"
	errorAlreadyExists    = 183
	pingExitTimeoutMS     = 4000
	pingExitWaitIntervals = 8
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutexW     = kernel32.NewProc("CreateMutexW")
	procGetLastError     = kernel32.NewProc("GetLastError")
	procReleaseMutex     = kernel32.NewProc("ReleaseMutex")
	procCloseHandle      = kernel32.NewProc("CloseHandle")
)

type instanceLock struct {
	handle syscall.Handle
}

func (l *instanceLock) release() {
	if l == nil || l.handle == 0 {
		return
	}
	procReleaseMutex.Call(uintptr(l.handle))
	procCloseHandle.Call(uintptr(l.handle))
	l.handle = 0
}

func acquireMutex(name string) (*instanceLock, bool, error) {
	utf16Name, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, false, err
	}

	handle, _, _ := procCreateMutexW.Call(
		0,
		0,
		uintptr(unsafe.Pointer(utf16Name)),
	)
	if handle == 0 {
		return nil, false, errors.New("CreateMutexW returned NULL")
	}

	lastErr, _, _ := procGetLastError.Call()
	alreadyExists := lastErr == errorAlreadyExists

	return &instanceLock{handle: syscall.Handle(handle)}, alreadyExists, nil
}

// ensureBackgroundReleased intenta cerrar instancias background previas
// usando taskkill para liberar el mutex global. Devuelve true si el mutex
// quedo libre.
func ensureBackgroundReleased() bool {
	for attempt := 0; attempt < pingExitWaitIntervals; attempt++ {
		lock, exists, err := acquireMutex(mutexName)
		if err == nil && !exists {
			lock.release()
			return true
		}
		if lock != nil {
			lock.release()
		}

		if attempt == 0 {
			killBackgroundInstances()
		}

		time.Sleep(time.Duration(pingExitTimeoutMS/pingExitWaitIntervals) * time.Millisecond)
	}
	return false
}

func killBackgroundInstances() {
	currentPID := fmt.Sprintf("%d", syscall.Getpid())
	cmd := exec.Command(
		"taskkill",
		"/F",
		"/IM", "sycronizafhir.exe",
		"/FI", fmt.Sprintf("PID ne %s", currentPID),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		// taskkill devuelve exit 128 cuando no encuentra procesos; es OK
		if !strings.Contains(string(out), "No hay") &&
			!strings.Contains(strings.ToLower(string(out)), "no tasks") {
			return
		}
	}
}
