////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package reports

import (
	"git.xx.network/elixxir/user-reporting/messages"
	"git.xx.network/elixxir/user-reporting/storage"
	"github.com/golang/protobuf/proto"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/client/api"
	"gitlab.com/elixxir/client/interfaces/message"
	"gitlab.com/elixxir/client/interfaces/params"
)

type listener struct {
	s *storage.Storage
	c *api.Client
}

// Hear messages from users to the coupon bot & respond appropriately
func (l *listener) Hear(item message.Receive) {
	// Confirm that authenticated channels
	if !l.c.HasAuthenticatedChannel(item.Sender) {
		jww.ERROR.Printf("No authenticated channel exists to %+v", item.Sender)
	}

	responseStr := ""

	// UNPACK MESSAGE
	in := &messages.Report{}
	err := proto.Unmarshal(item.Payload, in)
	if err != nil {
		jww.ERROR.Printf("Could not unmartial message from messenger: %+v", err)
	}

	// DO STUFF HERE ~~~~
	err = l.s.StoreReport(in)
	if err != nil {
		jww.ERROR.Printf("Failed to store received report [%+v]: %+v", in, err)
	}

	// RESPOND ~~~~~
	contact, err := l.c.GetAuthenticatedChannelRequest(item.Sender)
	if err != nil {
		jww.ERROR.Printf("Could not get authenticated channel request info: %+v", err)
	}

	out := &messages.CMIXText{Text: responseStr}
	payload, err := proto.Marshal(out)
	if err != nil {
		jww.ERROR.Printf("Failed to marshal proto response: %+v", err)
	}

	// Create response message
	resp := message.Send{
		Recipient:   contact.ID,
		Payload:     payload,
		MessageType: message.Text,
	}

	// Send response message to sender over cmix
	rids, mid, t, err := l.c.SendE2E(resp, params.GetDefaultE2E())
	if err != nil {
		jww.ERROR.Printf("Failed to send message: %+v", err)
	}
	jww.INFO.Printf("Sent ... %s [%+v] to %+v on rounds %+v [%+v]", responseStr, mid, item.Sender.String(), rids, t)
}

// Name returns a name, used for debugging
func (l *listener) Name() string {
	return "User reports"
}
