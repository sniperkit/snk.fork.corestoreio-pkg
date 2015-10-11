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

/*
Package mail provides functions and services for sending html or text
emails via encrypted or unencrypted connections.

Features: Attachments, Embedded images, HTML and text templates, Automatic
encoding of special characters, SSL and TLS, Sending multiple emails with
the same SMTP connection.

Daemon Manager

The daemon manager handles for each store view a mail daemon... @todo

Running one daemon

To run one daemon for a specific store view:

	import (
		"github.com/corestoreio/csfw/utils/log"
		"github.com/corestoreio/csfw/utils/mail"
		"github.com/corestoreio/csfw/store"
	)

	sm := store.NewManager(
		store.NewStorageOption(),
	)
	sm.ReInit(dbrSess)

	...

	d := mail.NewDaemon(
		mail.SetSMTPTimeout(20),
		mail.SetScope(sm.Store())
	)
	go func(){
		if err := d.Worker(); err != nil {
			log.Error("daemon.Worker", "err", err)
		}
	}()

	// send emails for customer registration, order confirmation, etc

	d.Send(*gomail.Message)

	// stop the daemon whenever you wish
	d.Stop()


Running multiple daemons

When you have configured different stores with different SMTP server, the daemon
will check to only create one new Dialer to a SMTP server, called DialerPool.

Running 3rd party APIs

Currently supported is only the Mandrill API. Other providers will be added on
request: AmazonSES, Mailgun, MailJet, SendGrid, PostMark, etc.

	d := mail.NewDaemon(
		mail.SetSMTPTimeout(20),
		mail.SetScope(sm.Store())
		mail.SetMandrill(),
	)

The API key must be stored in path: mail.PathSmtpMandrillAPIKey

Offline sending

If SMTP has been disabled via config key mail.PathSmtpDisable all emails will
be send to a custom logger.

@todo Instead of sending the emails to a logger, we can use a web interface like
mailcatcher.me to read the emails.

*/
package email