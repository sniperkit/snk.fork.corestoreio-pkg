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

	"bytes"
	"fmt"

	"github.com/corestoreio/csfw/net"
	"github.com/corestoreio/csfw/util/bufferpool"
	"github.com/corestoreio/csfw/util/errors"
)

const signatureDefaultSeparator = ','

// Signature represents an HTTP Header or Trailer entry.
type Signature struct {
	// KeyID field is an opaque string that the server/client can use to look up
	// the component they need to validate the signature. It could be an SSH key
	// fingerprint, an LDAP DN, etc. REQUIRED.
	KeyID string
	// Algorithm parameter is used if the client and server agree on a
	// non-standard digital signature algorithm.  The full list of supported
	// signature mechanisms is listed below. REQUIRED.
	Algorithm string
	// Signature parameter is an encoded digital signature generated by the
	// client.  The client uses the `algorithm` and `headers` request parameters
	// to form a canonicalized `signing string`.  This `signing string` is then
	// signed with the key associated with `keyId` and the algorithm
	// corresponding to `algorithm`.  The `signature` parameter is then set to
	// the encoding of the signature.
	Signature []byte
	// Separator defines the field separator and defaults to colon.
	Separator rune
}

// IsValid checks if the signature is valid. Returns nil on success.
func (s *Signature) IsValid() error {
	switch {
	case s.KeyID == "":
		return errors.NewNotValidf("[signed] Empty KeyID")
	case s.Algorithm == "":
		return errors.NewNotValidf("[signed] Empty Algorithm")
	case len(s.Signature) == 0:
		return errors.NewNotValidf("[signed] Empty Signature")
	}
	return nil
}

// WriteHTTPContentSignature writes the content signature header using an
// encoder, which can be hex or base64.
// 	Content-Signature: keyId="rsa-key-1",algorithm="rsa-sha256",signature="Hex|Base64(RSA-SHA256(signing string))"
// 	Content-Signature: keyId="hmac-key-1",algorithm="hmac-sha1",signature="Hex|Base64(HMAC-SHA1(signing string))"
func (s Signature) Write(w http.ResponseWriter, encoder func(src []byte) string) error {
	if s.Separator == 0 {
		s.Separator = signatureDefaultSeparator
	}

	buf := bufferpool.Get()
	buf.WriteString(`keyId="` + s.KeyID + `"`)
	buf.WriteRune(s.Separator)
	buf.WriteString(`algorithm="` + s.Algorithm + `"`)
	buf.WriteRune(s.Separator)
	buf.WriteString(`signature="`)
	buf.WriteString(encoder(s.Signature))
	buf.WriteRune('"')
	w.Header().Set(net.ContentSignature, buf.String())
	bufferpool.Put(buf)
	return nil
}

// Parse parses the header or trailer Content-Signature into the struct.
// Returns an error notFound, notValid behaviour or nil on success.
func (s *Signature) Parse(r *http.Request, decoder func(s string) ([]byte, error)) error {
	if s.Separator == 0 {
		s.Separator = signatureDefaultSeparator
	}
	raw := r.Header.Get(net.ContentSignature)
	if raw == "" {
		raw = r.Trailer.Get(net.ContentSignature)
	}
	if raw == "" {
		return errors.NewNotFoundf("[signed]")
	}

	var fields [3]bytes.Buffer
	var idx int
	for _, r := range raw {
		if r == s.Separator {
			idx++
			continue
		}
		fields[idx].WriteRune(r)
	}

	fmt.Printf("%s\n", fields[0].String())
	fmt.Printf("%s\n", fields[1].String())
	fmt.Printf("%s\n", fields[2].String())

	return nil
}
