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

package directory

import (
	"github.com/corestoreio/csfw/config/valuelabel"
	"golang.org/x/text/currency"
)

// CurrencyCollection static representation of all currencies available
var CurrencyCollection valuelabel.Slice

// InitCurrencyCollection sets the Options() on all PathCurrency* configuration
// global variables.
func InitCurrencyCollection() error {

	CurrencyCollection = valuelabel.NewByStringValue(currency.All()...)

	PathSystemCurrencyInstalled.ValueLabel = CurrencyCollection
	PathCurrencyOptionsBase.ValueLabel = CurrencyCollection
	PathCurrencyOptionsAllow.ValueLabel = CurrencyCollection
	PathCurrencyOptionsDefault.ValueLabel = CurrencyCollection
	return nil
}

// Currency represents a corestore currency type which may add more features.
type Currency struct {
	currency.Unit
}
