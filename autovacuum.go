package bus

import (
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

// PowerOn executes a vacuum of internal resources.
func autoVacuum_PowerOn(rs m.AutoVacuumSet) {
	h.BusBus().NewSet(rs.Env()).Gc()
	rs.Super().PowerOn()
}

func init() {
	h.AutoVacuum().Methods().PowerOn().Extend(autoVacuum_PowerOn)
}
