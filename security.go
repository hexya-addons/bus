package bus

import (
	"github.com/hexya-addons/base"
	"github.com/hexya-erp/pool/h"
)

func init() {
	h.BusPresence().Methods().AllowAllToGroup(base.GroupUser)
	h.BusPresence().Methods().AllowAllToGroup(base.GroupPortal)
}
