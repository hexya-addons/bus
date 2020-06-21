package bus

import (
	"time"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

const awayTimer = 30 * time.Minute

var disconnectionTimer = timeout + 5*time.Second

/* User Presence
Its status is 'online', 'away' or 'offline'. This model is not
attached to res_users to avoid database concurrence errors. Since the 'update' method is executed
at each poll, if the user have multiple opened tabs, concurrence errors can happend, but are 'muted-logged'.
*/

var fields_BusPresence = map[string]models.FieldDefinition{
	"User": fields.One2One{
		RelationModel: h.User(),
		String:        "Users",
		Required:      true,
		Index:         true,
		OnDelete:      `cascade`},

	"LastPoll": fields.DateTime{
		String:  "Last Poll",
		Default: func(env models.Environment) interface{} { return dates.Now() }},

	"LastPresence": fields.DateTime{
		String:  "Last Presence",
		Default: func(env models.Environment) interface{} { return dates.Now() }},

	"Status": fields.Selection{
		Selection: types.Selection{
			"online":  "Online",
			"away":    "Away",
			"offline": "Offline",
		},
		String:  "IM Status",
		Default: models.DefaultValue("offline")},
}

// Update update the last_poll and last_presence of the current user
func busPresence_Update(rs m.BusPresenceSet, inactivity_period time.Duration) {
	currentUser := h.User().NewSet(rs.Env()).CurrentUser()
	presence := h.BusPresence().Search(rs.Env(), q.BusPresence().User().Equals(currentUser))
	lastPresence := dates.Now().Add(-inactivity_period)
	values := h.BusPresence().NewData().SetLastPoll(dates.Now())
	// update the presence or a create a new one
	if presence.IsEmpty() {
		values.SetUser(currentUser).SetLastPresence(lastPresence)
		h.BusPresence().Create(rs.Env(), values)
		return
	}
	if presence.LastPresence().Lower(lastPresence) {
		values.SetLastPresence(lastPresence)
	}
	presence.Write(values)
}

func init() {
	models.NewModel("BusPresence")
	h.BusPresence().AddFields(fields_BusPresence)
	h.BusPresence().NewMethod("Update", busPresence_Update)
}
