// Copyright 2020-2021 Nao Yonashiro
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
	Now func() time.Time

	resMu     sync.RWMutex
	res       interface{}
	expiresAt time.Time

	callMu sync.Mutex
	call   *call
}

func (g *Group) Do(ttl time.Duration, fn func() (interface{}, error)) (interface{}, error) {
	g.resMu.RLock()
	res := g.res
	expiresAt := g.expiresAt
	g.resMu.RUnlock()
	t := g.now()
	if res != nil && t.Before(expiresAt) {
		return res, nil
	}

	g.callMu.Lock()
	c := g.call
	if c == nil || t.After(c.expiresAt) {
		t := g.now()
		c = &call{ready: make(chan struct{}), expiresAt: t.Add(ttl)}
		g.call = c
		g.callMu.Unlock()

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
		g.callMu.Unlock()
		<-c.ready
	}
	return c.res, c.err
}

func (g *Group) Reset() {
	g.resMu.Lock()
	defer g.resMu.Unlock()
	g.callMu.Lock()
	defer g.callMu.Unlock()
	g.call = nil
	g.res = nil
}

func (g *Group) now() time.Time {
	if g.Now == nil {
		return time.Now()
	}
	return g.Now()
}
