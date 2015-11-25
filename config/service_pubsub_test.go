// Copyright 2015, Cyrill @ Schumacher.fm and the CoreStore contributors
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

package config_test

import (
	std "log"
	"testing"
	"time"

	"errors"

	"sync"

	"github.com/corestoreio/csfw/config"
	"github.com/corestoreio/csfw/config/scope"
	"github.com/corestoreio/csfw/utils/log"
	"github.com/stretchr/testify/assert"
)

var errLogBuf = new(log.MutexBuffer)

func init() {
	config.PkgLog = log.NewStdLogger(
		log.SetStdDebug(errLogBuf, "testErr: ", std.Lshortfile),
	)
	config.PkgLog.SetLevel(log.StdLevelDebug)
}

var _ config.MessageReceiver = (*testSubscriber)(nil)

type testSubscriber struct {
	f func(path string, sg scope.Scope, id int64) error
}

func (ts *testSubscriber) MessageConfig(path string, sg scope.Scope, id int64) error {
	//ts.t.Logf("Message: %s Group %d Scope %d", path, sg, s.scope.ID())
	return ts.f(path, sg, id)
}

func TestPubSubBubbling(t *testing.T) {
	defer errLogBuf.Reset()
	testPath := "a/b/c"

	m := config.NewService()

	_, err := m.Subscribe("", nil)
	assert.EqualError(t, err, config.ErrPathEmpty.Error())

	subID, err := m.Subscribe(testPath, &testSubscriber{
		f: func(path string, sg scope.Scope, id int64) error {
			assert.Equal(t, testPath, path)
			if sg == scope.DefaultID {
				assert.Equal(t, int64(0), id)
			} else {
				assert.Equal(t, int64(123), id)
			}
			return nil
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, subID, "The very first subscription ID should be 1")

	assert.NoError(t, m.Write(config.Value(1), config.Path(testPath), config.Scope(scope.WebsiteID, 123)))

	assert.NoError(t, m.Close())
	time.Sleep(time.Millisecond * 10) // wait for goroutine to close

	// send on closed channel
	assert.NoError(t, m.Write(config.Value(1), config.Path(testPath+"Doh"), config.Scope(scope.WebsiteID, 3)))
	assert.EqualError(t, m.Close(), config.ErrPublisherClosed.Error())
}

func TestPubSubPanicSimple(t *testing.T) {
	defer errLogBuf.Reset()
	testPath := "x/y/z"

	m := config.NewService()
	subID, err := m.Subscribe(testPath, &testSubscriber{
		f: func(path string, sg scope.Scope, id int64) error {
			panic("Don't panic!")
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, subID, "The very first subscription ID should be 1")
	assert.NoError(t, m.Write(config.Value(321), config.Path(testPath), config.ScopeStore(123)))
	assert.NoError(t, m.Close())
	time.Sleep(time.Millisecond * 10) // wait for goroutine to close
	assert.Contains(t, errLogBuf.String(), `config.pubSub.publish.recover.r recover: "Don't panic!"`)
}

func TestPubSubPanicError(t *testing.T) {
	defer errLogBuf.Reset()
	testPath := "™/ö/º"

	var pErr = errors.New("OMG! Panic!")
	m := config.NewService()
	subID, err := m.Subscribe(testPath, &testSubscriber{
		f: func(path string, sg scope.Scope, id int64) error {
			panic(pErr)
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, subID, "The very first subscription ID should be 1")
	assert.NoError(t, m.Write(config.Value(321), config.Path(testPath), config.ScopeStore(123)))
	// not closing channel to let the Goroutine around egging aka. herumeiern.
	time.Sleep(time.Millisecond * 10) // wait for goroutine ...
	assert.Contains(t, errLogBuf.String(), `config.pubSub.publish.recover.err err: OMG! Panic!`)
	assert.NoError(t, m.Close())
}

func TestPubSubPanicMultiple(t *testing.T) {
	defer errLogBuf.Reset()
	m := config.NewService()

	subID, err := m.Subscribe("x", &testSubscriber{
		f: func(path string, sg scope.Scope, id int64) error {
			assert.Equal(t, "x/y/z", path)
			panic("One: Don't panic!")
		},
	})
	assert.NoError(t, err)
	assert.True(t, subID > 0)

	subID, err = m.Subscribe("x/y", &testSubscriber{
		f: func(path string, sg scope.Scope, id int64) error {
			assert.Equal(t, "x/y/z", path)
			panic("Two: Don't panic!")
		},
	})
	assert.NoError(t, err)
	assert.True(t, subID > 0)

	subID, err = m.Subscribe("x/y/z", &testSubscriber{
		f: func(path string, sg scope.Scope, id int64) error {
			assert.Equal(t, "x/y/z", path)
			panic("Three: Don't panic!")
		},
	})
	assert.NoError(t, err)
	assert.True(t, subID > 0)

	assert.NoError(t, m.Write(config.Value(789), config.Path("x/y/z"), config.ScopeStore(987)))
	assert.NoError(t, m.Close())
	time.Sleep(time.Millisecond * 30) // wait for goroutine to close
	assert.Contains(t, errLogBuf.String(), `config.pubSub.publish.recover.r recover: "One: Don't panic!`)
	assert.Contains(t, errLogBuf.String(), `config.pubSub.publish.recover.r recover: "Two: Don't panic!"`)
	assert.Contains(t, errLogBuf.String(), `config.pubSub.publish.recover.r recover: "Three: Don't panic!"`)
}

func TestPubSubUnsubscribe(t *testing.T) {
	defer errLogBuf.Reset()

	var pErr = errors.New("WTF? Panic!")
	m := config.NewService()
	subID, err := m.Subscribe("x/y/z", &testSubscriber{
		f: func(path string, sg scope.Scope, id int64) error {
			panic(pErr)
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, subID, "The very first subscription ID should be 1")
	assert.NoError(t, m.Unsubscribe(subID))
	assert.NoError(t, m.Write(config.Value(321), config.Path("x/y/z"), config.ScopeStore(123)))
	time.Sleep(time.Millisecond) // wait for goroutine ...
	assert.Contains(t, errLogBuf.String(), `config.Service.Write path: "stores/123/x/y/z" val: 321`)
	assert.NoError(t, m.Close())
}

type levelCalls struct {
	sync.Mutex
	level2Calls int
	level3Calls int
}

func TestPubSubEvict(t *testing.T) {
	defer errLogBuf.Reset()

	levelCall := new(levelCalls)

	var pErr = errors.New("WTF Eviction? Panic!")
	m := config.NewService()
	subID, err := m.Subscribe("x/y", &testSubscriber{
		f: func(path string, sg scope.Scope, id int64) error {
			assert.Contains(t, path, "x/y")
			// this function gets called 3 times
			levelCall.Lock()
			levelCall.level2Calls++
			levelCall.Unlock()
			return nil
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, subID)

	subID, err = m.Subscribe("x/y/z", &testSubscriber{
		f: func(path string, sg scope.Scope, id int64) error {
			levelCall.Lock()
			levelCall.level3Calls++
			levelCall.Unlock()
			// this function gets called 1 times and then gets removed
			panic(pErr)
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, subID)

	assert.NoError(t, m.Write(config.Value(321), config.Path("x/y/z"), config.ScopeStore(123)))
	assert.NoError(t, m.Write(config.Value(321), config.Path("x/y/a"), config.ScopeStore(123)))
	assert.NoError(t, m.Write(config.Value(321), config.Path("x/y/z"), config.ScopeStore(123)))

	time.Sleep(time.Millisecond * 20) // wait for goroutine ...

	assert.Contains(t, errLogBuf.String(), "config.pubSub.publish.recover.err err: WTF Eviction? Panic!")

	levelCall.Lock()
	assert.Equal(t, 3, levelCall.level2Calls)
	assert.Equal(t, 1, levelCall.level3Calls)
	levelCall.Unlock()
	assert.NoError(t, m.Close())
}