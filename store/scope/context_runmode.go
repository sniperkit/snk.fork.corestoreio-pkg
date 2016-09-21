// Copyright 2015-2016, Cyrill @ Schumacher.fm and the CoreStore contributors
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

package scope

import (
	"context"
	"net/http"
)

// DefaultRunMode defines the default run mode if the programmer hasn't applied
// the field Mode or the function RunMode.WithContext() to specify a specific
// run mode. It indicates the fall back to the default website and its default
// store.
const DefaultRunMode TypeID = 0

// RunModeCalculater core type to initialize the run mode of the current
// request. Allows you to create a multi-site / multi-tenant setup. An
// implementation of this lives in net.runmode.WithRunMode() middleware.
//
// Your custom function allows to initialize the runMode based on parameters in
// the http.Request.
type RunModeCalculater interface {
	CalculateRunMode(*http.Request) TypeID
}

// RunModeFunc type is an adapter to allow the use of ordinary functions as
// RunModeCalculater. If f is a function with the appropriate signature,
// RunModeFunc(f) is a Handler that calls f.
type RunModeFunc func(*http.Request) TypeID

// CalculateRunMode calls f(r).
func (f RunModeFunc) CalculateRunMode(r *http.Request) TypeID {
	return f(r)
}

// WithContextRunMode sets the main run mode for the current request. It panics
// when called multiple times for the current context. This function is used in
// net/runmode together with function RunMode.CalculateMode(r, w).
// Use case for the runMode: Cache Keys and app initialization.
func WithContextRunMode(ctx context.Context, runMode TypeID) context.Context {
	if _, ok := ctx.Value(ctxRunModeKey{}).(TypeID); ok {
		panic("[scope] You are not allowed to set the runMode more than once for the current context.")
	}
	return context.WithValue(ctx, ctxRunModeKey{}, runMode)
}

// FromContextRunMode returns the run mode Hash from a context. If no entry can
// be found in the context the returned Hash has a default value. This default
// value indicates the fall back to the default website and its default store.
// Use case for the runMode: Cache Keys and app initialization.
func FromContextRunMode(ctx context.Context) TypeID {
	h, ok := ctx.Value(ctxRunModeKey{}).(TypeID)
	if !ok {
		return DefaultRunMode // indicates a fall back to a default store of the default website
	}
	return h
}

type ctxRunModeKey struct{}
