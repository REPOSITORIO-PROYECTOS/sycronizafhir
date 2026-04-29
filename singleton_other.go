//go:build !windows

package main

type instanceLock struct{}

func (l *instanceLock) release() {}

func acquireMutex(_ string) (*instanceLock, bool, error) {
	return &instanceLock{}, false, nil
}

func ensureBackgroundReleased() bool {
	return true
}
