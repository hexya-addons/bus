package bus

import (
	"time"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

var fields_User = map[string]models.FieldDefinition{
	"IMStatus": fields.Char{
		Compute: h.User().Methods().ComputeIMStatus()},
}

//  Compute the im_status of the users
func user_ComputeIMStatus(rs m.UserSet) m.UserData {
	presence := h.BusPresence().Search(rs.Env(), q.BusPresence().User().Equals(rs)).Limit(1)
	res := h.User().NewData().SetIMStatus("offline")
	switch {
	case time.Since(presence.LastPoll().Time) > disconnectionTimer:
		res.SetIMStatus("offline")
	case time.Since(presence.LastPresence().Time) > awayTimer:
		res.SetIMStatus("away")
	default:
		res.SetIMStatus("online")
	}
	return res
}

func init() {
	h.User().AddFields(fields_User)
	h.User().NewMethod("ComputeIMStatus", user_ComputeIMStatus)
}
