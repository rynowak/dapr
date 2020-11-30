// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
// ------------------------------------------------------------

package proxy

import (
	"context"
	"net"
	"net/http"
	"sync"
)

type addressProcessor func(key string) (string, error)

type pool struct {
	mutex      sync.Mutex
	transports map[string]*http.Transport
}

func (p *pool) Get(addr string, ap addressProcessor) (*http.Transport, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.transports == nil {
		p.transports = map[string]*http.Transport{}
	}

	t, ok := p.transports[addr]
	if !ok {
		t = &http.Transport{
			// Ignore the provided address and always dial the address we have
			DialContext: func(ctx context.Context, network string, _ string) (net.Conn, error) {
				d := &net.Dialer{}
				return d.DialContext(ctx, network, addr)
			},
		}
		p.transports[addr] = t
	}

	return t, nil
}
