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

package store

import "github.com/corestoreio/csfw/util/errors"

// GroupOption can be used as an argument in NewGroup to configure a group.
type GroupOption func(*Group) error

// SetGroupWebsite assigns a website to a group. If website ID does not match
// the group website ID then add error will be generated.
func SetGroupWebsite(tw *TableWebsite) GroupOption {
	return func(g *Group) error {
		if g.Data.WebsiteID != tw.WebsiteID {
			return errors.NewNotFoundf(errGroupWebsiteNotFound)
		}
		var err error
		g.Website, err = NewWebsite(g.baseConfig, tw)
		return errors.Wrap(err, "[store] SetGroupWebsite.NewWebsite")
	}
}

// SetGroupStores uses the full store collection to extract the stores which are
// assigned to a group. Either Website must be set before calling SetGroupStores() or
// the second argument may not be nil. Does nothing if tss variable is nil.
func SetGroupStores(tss TableStoreSlice, w *TableWebsite) GroupOption {
	return func(g *Group) error {
		if w == nil {
			w = g.Website.Data
		}
		if w.WebsiteID != g.Data.WebsiteID {
			return errors.NewNotValidf(errGroupWebsiteIntegrityFailed)
		}
		for _, s := range tss.FilterByGroupID(g.Data.GroupID) {
			ns, err := NewStore(g.baseConfig, s, w, g.Data)
			if err != nil {
				return errors.Wrapf(err, "[store] SetGroupStores.FilterByGroupID.NewStore. StoreID %d WebsiteID %d Group %v", s.StoreID, w.WebsiteID, g.Data)
			}
			g.Stores = append(g.Stores, ns)
		}
		return nil
	}
}
