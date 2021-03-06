/**
* Copyright 2018 Comcast Cable Communications Management, LLC
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
* http://www.apache.org/licenses/LICENSE-2.0
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

// Package locks provides Named Locks functionality for manging
// mutexes by string name (e.g., cache keys).
package locks

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// NamedLocker provides a locker for handling Named Locks
type NamedLocker interface {
	Acquire(string) (NamedLock, error)
	RAcquire(string) (NamedLock, error)
}

type namedLocker struct {
	locks   map[string]*namedLock
	mapLock *sync.Mutex
}

// NewNamedLocker returns a new Named Locker
func NewNamedLocker() NamedLocker {
	return &namedLocker{
		locks:   make(map[string]*namedLock),
		mapLock: &sync.Mutex{},
	}
}

// NamedLock defines the interface for implementing Named Locks
type NamedLock interface {
	Release() error
	RRelease() error
	Upgrade() (NamedLock, error)
	WriteLockCounter() int
	WriteLockMode() bool
}

func newNamedLock(name string, locker *namedLocker) *namedLock {
	return &namedLock{
		name:    name,
		RWMutex: &sync.RWMutex{},
		locker:  locker,
	}
}

type namedLock struct {
	*sync.RWMutex
	name           string
	queueSize      int32
	writeLockMode  int32
	writeLockCount int
	locker         *namedLocker
}

// Release releases the write lock on the subject Named Lock
func (nl *namedLock) Release() error {

	if nl.name == "" {
		return errInvalidLockName(nl.name)
	}

	atomic.StoreInt32(&nl.writeLockMode, 0)
	qs := atomic.AddInt32(&nl.queueSize, -1)
	if qs == 0 {
		nl.locker.mapLock.Lock()
		delete(nl.locker.locks, nl.name)
		nl.locker.mapLock.Unlock()
	}

	nl.Unlock()
	return nil
}

// RRelease releases the read lock on the subject Named Lock
func (nl *namedLock) RRelease() error {

	if nl.name == "" {
		return errInvalidLockName(nl.name)
	}

	qs := atomic.AddInt32(&nl.queueSize, -1)
	if qs == 0 {
		nl.locker.mapLock.Lock()
		delete(nl.locker.locks, nl.name)
		nl.locker.mapLock.Unlock()
	}

	nl.RUnlock()
	return nil
}

// WriteLockCounter returns the number of write locks acquired by the namedLock
// This function should only be called by a goroutine actively holding a write lock,
// as it is otherwise not atomic
func (nl *namedLock) WriteLockCounter() int {
	return nl.writeLockCount
}

// WriteLockMode returns true if a caller is waiting for a write lock
func (nl *namedLock) WriteLockMode() bool {
	return atomic.LoadInt32(&nl.writeLockMode) == 1
}

// Upgrade will upgrade the current read-lock to a write lock without losing the reference to the
// underlying sync map, enabling goroutines, after receiving a write lock, to know how many other
// goroutines acquired a write lock (naturally or upgraded) during the time this routine released
// it's read lock and got a write lock. This helps the receiver of the write lock know if any extra
// state checks are required (e.g., re-querying a cache that might have changed) before proceeding.
func (nl *namedLock) Upgrade() (NamedLock, error) {

	ch := make(chan bool, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		atomic.AddInt32(&nl.queueSize, 1)
		ch <- true
		atomic.StoreInt32(&nl.writeLockMode, 1)
		nl.Lock()
		nl.writeLockCount++
		wg.Done()
	}()

	// once we know the write lock queueSize is incremented, we can release our read lock
	<-ch
	close(ch)
	nl.RRelease()

	// wait until write mode is set, read lock is released, and write lock is acquired
	wg.Wait()

	return nl, nil
}

// Acquire locks the named lock for writing, and blocks until the wlock is acquired
func (lk *namedLocker) Acquire(lockName string) (NamedLock, error) {
	if lockName == "" {
		return nil, errInvalidLockName(lockName)
	}

	lk.mapLock.Lock()
	nl, ok := lk.locks[lockName]
	if !ok {
		nl = newNamedLock(lockName, lk)
		lk.locks[lockName] = nl
	}
	atomic.AddInt32(&nl.queueSize, 1)
	lk.mapLock.Unlock()
	atomic.StoreInt32(&nl.writeLockMode, 1)

	nl.Lock()

	nl.writeLockCount++
	return nl, nil
}

// RAcquire locks the named lock for reading, and blocks until the rlock is acquired
func (lk *namedLocker) RAcquire(lockName string) (NamedLock, error) {
	if lockName == "" {
		return nil, errInvalidLockName(lockName)
	}

	lk.mapLock.Lock()
	nl, ok := lk.locks[lockName]
	if !ok {
		nl = newNamedLock(lockName, lk)
		lk.locks[lockName] = nl
	}

	atomic.AddInt32(&nl.queueSize, 1)
	lk.mapLock.Unlock()
	atomic.StoreInt32(&nl.writeLockMode, 0)

	nl.RLock()
	return nl, nil
}

func errInvalidLockName(name string) error {
	return fmt.Errorf("invalid lock name: %s", name)
}
