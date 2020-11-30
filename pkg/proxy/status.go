// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
// ------------------------------------------------------------

package proxy

import "sync"

// ServerStatus is the status of the proxy server.
type ServerStatus struct {
	Enabled bool
	Port    int
}

// StatusMonitor provides access to the proxy status.
type StatusMonitor interface {
	SetStatus(status ServerStatus)
	Status() ServerStatus
}

// NewStatusMonitor returns a new StatusMonitor.
func NewStatusMonitor() StatusMonitor {
	return &statusMonitor{}
}

type statusMonitor struct {
	statusMutex sync.Mutex
	status      ServerStatus
}

func (s *statusMonitor) SetStatus(status ServerStatus) {
	s.statusMutex.Lock()
	s.status = status
	s.statusMutex.Unlock()
}

func (s *statusMonitor) Status() ServerStatus {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	return s.status
}
