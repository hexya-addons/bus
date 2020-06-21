package bus

import (
	"time"

	"github.com/hexya-addons/bus/bustypes"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

var fields_Partner = map[string]models.FieldDefinition{
	"IMStatus": fields.Char{
		Compute: h.Partner().Methods().ComputeIMStatus()},
}

// ComputeIMStatus computes the IM status of the partner
func partner_ComputeIMStatus(rs m.PartnerSet) m.PartnerData {
	presence := h.BusPresence().Search(rs.Env(), q.BusPresence().UserFilteredOn(q.User().Partner().Equals(rs))).Limit(1)
	res := h.Partner().NewData().SetIMStatus("offline")
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

// IMSearch search partner with a name and return its id, name and im_status.
// Note : the user must be logged
// - name : the partner name to search
// - limit : the limit of result to return
func partner_IMSearch(rs m.PartnerSet, name string, limit int) []bustypes.IMSearchResult {
	users := h.User().Search(rs.Env(), q.User().Name().ILike(name).And().ID().NotEquals(rs.Env().Uid())).Limit(limit)
	var res []bustypes.IMSearchResult
	for _, user := range users.Records() {
		res = append(res, bustypes.IMSearchResult{
			ID:       user.Partner().ID(),
			Name:     user.Name(),
			IMStatus: user.IMStatus(),
		})
	}
	return res
}
func init() {
	h.Partner().AddFields(fields_Partner)
	h.Partner().NewMethod("ComputeIMStatus", partner_ComputeIMStatus)
	h.Partner().NewMethod("IMSearch", partner_IMSearch)
}
