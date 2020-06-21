// Copyright 2020 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package bustypes

import "github.com/hexya-erp/hexya/src/models/types"

// A Notification is a message that is sent/received on a channel over the message bus.
// Message must be JSON serializable.
type Notification struct {
	ID      int64       `json:"id"`
	Channel string      `json:"channel"`
	Message interface{} `json:"message"`
}

// PollParams are the parameters of a long poll
type PollParams struct {
	Channels []string      `json:"channels"`
	Last     int64         `json:"last"`
	Options  types.Context `json:"options"`
}

// An IMSearchResult is returned by Partner's IMSearch method
type IMSearchResult struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IMStatus string `json:"im_status"`
}
