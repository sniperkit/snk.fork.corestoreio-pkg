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

package signed

import (
	"net/http"

	"github.com/corestoreio/csfw/log"
	"github.com/corestoreio/csfw/util/errors"
)

func (s *Service) WithResponseSignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scpCfg := s.configByContext(r.Context())
		if err := scpCfg.IsValid(); err != nil {
			s.Log.Info("signed.Service.WithRateLimit.configByContext.Error", log.Err(err))
			if s.Log.IsDebug() {
				s.Log.Debug("signed.Service.WithRateLimit.configByContext", log.Err(err), log.HTTPRequest("request", r))
			}
			s.ErrorHandler(errors.Wrap(err, "signed.Service.WithRateLimit.configFromContext")).ServeHTTP(w, r)
			return
		}
		if scpCfg.Disabled {
			if s.Log.IsDebug() {
				s.Log.Debug("signed.Service.WithRateLimit.Disabled", log.Stringer("scope", scpCfg.ScopeHash), log.Object("scpCfg", scpCfg), log.HTTPRequest("request", r))
			}
			next.ServeHTTP(w, r)
			return
		}

		if scpCfg.InTrailer {
			// direct output to the client and the signature will be inserted
			// after the body has been written. ideal for streaming but not all
			// clients can process a trailer.
			scpCfg.writeTrailer(next, w, r)
			return
		}
		// the write to w gets buffered and we calculate the checksum of the
		// buffer and then flush the buffer to the client.
		scpCfg.writeBuffered(next, w, r)
	})
}

func (s *Service) WithRequestSignatureValidation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scpCfg := s.configByContext(r.Context())
		if err := scpCfg.IsValid(); err != nil {
			s.Log.Info("signed.Service.WithRateLimit.configByContext.Error", log.Err(err))
			if s.Log.IsDebug() {
				s.Log.Debug("signed.Service.WithRateLimit.configByContext", log.Err(err), log.HTTPRequest("request", r))
			}
			s.ErrorHandler(errors.Wrap(err, "signed.Service.WithRateLimit.configFromContext")).ServeHTTP(w, r)
			return
		}
		if scpCfg.Disabled {
			if s.Log.IsDebug() {
				s.Log.Debug("signed.Service.WithRateLimit.Disabled", log.Stringer("scope", scpCfg.ScopeHash), log.Object("scpCfg", scpCfg), log.HTTPRequest("request", r))
			}
			next.ServeHTTP(w, r)
			return
		}

		if err := scpCfg.ValidateBody(r); err != nil {
			scpCfg.ErrorHandler(err).ServeHTTP(w, r)
			return
		}
		// signature valid
		next.ServeHTTP(w, r)
	})
}
