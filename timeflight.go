// Copyright 2020 Nao Yonashiro
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package timeflight

import (
	"sync"
	"time"
)

type call struct {
	res       interface{}
	err       error
	expiresAt time.Time
	ready     chan struct{}
}

type Group struct {
	CacheDuration time.Duration

	resMu     sync.RWMutex
	res       interface{}
	expiresAt time.Time

	callsMu sync.Mutex
	calls   map[int64]*call
	callID  int64
}

func (g *Group) Do(t time.Time, fn func() (interface{}, error)) (interface{}, error) {
	g.resMu.RLock()
	res := g.res
	expiresAt := g.expiresAt
	g.resMu.RUnlock()
	if res != nil && t.Before(expiresAt) {
		return res, nil
	}

	g.callsMu.Lock()
	if g.calls == nil {
		g.calls = make(map[int64]*call)
	}
	c := g.calls[g.callID]
	if c == nil || t.After(c.expiresAt) {
		delete(g.calls, g.callID)
		g.callID++
		c = &call{ready: make(chan struct{}), expiresAt: t.Add(g.CacheDuration)}
		g.calls[g.callID] = c
		g.callsMu.Unlock()

		c.res, c.err = fn()
		if c.err == nil {
			g.resMu.Lock()
			if c.expiresAt.After(g.expiresAt) {
				g.expiresAt = c.expiresAt
				g.res = c.res
			}
			g.resMu.Unlock()
		}
		close(c.ready)
	} else {
		g.callsMu.Unlock()
		<-c.ready
	}
	return c.res, c.err
}

func (g *Group) Reset() {
	g.resMu.Lock()
	g.callsMu.Lock()
	g.calls = nil
	g.callID = 0
	g.res = nil
	g.callsMu.Unlock()
	g.resMu.Unlock()
}
