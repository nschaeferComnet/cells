/*
 * Copyright (c) 2019-2022. Abstrium SAS <team (at) pydio.com>
 * This file is part of Pydio Cells.
 *
 * Pydio Cells is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Pydio Cells is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with Pydio Cells.  If not, see <http://www.gnu.org/licenses/>.
 *
 * The latest code can be found at <https://pydio.com>.
 */

package client

import (
	"sync"
	"time"

	pb "github.com/pydio/cells/v4/common/proto/registry"
	"github.com/pydio/cells/v4/common/registry"
)

type ServerAttributes struct {
	Name      string
	Addresses []string
	Services  []string
	Endpoints []string
}

type UpdateStateCallback func(map[string]*ServerAttributes) error

type ResolverCallback interface {
	Add(UpdateStateCallback)
	Stop()
}

type resolverCallback struct {
	reg   registry.Registry
	ml    *sync.RWMutex
	items []registry.Item

	updatedStateTimer *time.Timer
	cbs               []UpdateStateCallback

	done chan bool
	w    registry.Watcher
}

func NewResolverCallback(reg registry.Registry) (ResolverCallback, error) {
	items, err := reg.List()
	if err != nil {
		return nil, err
	}

	r := &resolverCallback{
		done: make(chan bool, 1),
	}
	r.reg = reg
	r.ml = &sync.RWMutex{}
	r.updatedStateTimer = time.NewTimer(50 * time.Millisecond)

	go r.updateState()
	go r.watch()

	r.ml.Lock()
	r.items = items
	r.ml.Unlock()

	return r, nil
}

func (r *resolverCallback) Stop() {
	if r.w != nil {
		r.w.Stop()
	}
	close(r.done)
}

func (r *resolverCallback) Add(cb UpdateStateCallback) {
	r.cbs = append(r.cbs, cb)
}

func (r *resolverCallback) watch() {
	w, err := r.reg.Watch(registry.WithAction(pb.ActionType_FULL_LIST))
	if err != nil {
		return
	}

	for {
		res, err := w.Next()
		if err != nil {
			return
		}

		r.items = res.Items()
		r.updatedStateTimer.Reset(50 * time.Millisecond)
	}
}

func (r *resolverCallback) updateState() {
	for {
		select {
		case <-r.updatedStateTimer.C:
			r.sendState()
		case <-r.done:
			return
		}
	}
}

func (r *resolverCallback) sendState() {
	var m = make(map[string]*ServerAttributes)

	r.ml.RLock()
	for _, v := range r.items {
		var srv registry.Node
		if v.As(&srv) {
			m[srv.ID()] = &ServerAttributes{
				Name:      srv.Name(),
				Addresses: srv.Address(),
				Endpoints: srv.Endpoints(),
			}
		}
	}

	for _, v := range r.items {
		var svc registry.Service
		if v.As(&svc) {
			for _, node := range svc.Nodes() {
				address, ok := m[node.ID()]
				if ok {
					address.Services = append(address.Services, svc.Name())
				}
			}
		}
	}
	r.ml.RUnlock()

	for _, cb := range r.cbs {
		cb(m)
	}
}
