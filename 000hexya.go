package bus

import (
	_ "github.com/hexya-addons/base"
	"github.com/hexya-addons/bus/controllers"
	_ "github.com/hexya-addons/web"
	"github.com/hexya-erp/hexya/src/server"
	"github.com/hexya-erp/hexya/src/tools/logging"
)

const MODULE_NAME string = "bus"

var log logging.Logger

func init() {
	log = logging.GetLogger("bus")
	server.RegisterModule(&server.Module{
		Name:    MODULE_NAME,
		PreInit: func() {},
		PostInit: func() {
			controllers.Dispatcher.Start()
		},
	})
}
