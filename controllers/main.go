package controllers

import (
	"net/http"
	"time"

	"github.com/hexya-addons/bus/bustypes"
	web "github.com/hexya-addons/web/controllers"
	"github.com/hexya-erp/hexya/src/controllers"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/server"
	"github.com/hexya-erp/hexya/src/tools/logging"
	"github.com/hexya-erp/pool/h"
)

// Dispatcher is the long polling dispatching loop
var Dispatcher Poller

var log logging.Logger

// A Poller is a long poll dispatching loop
type Poller interface {
	// Poll returns the pending notification on the given channels since the last retrieved id.
	Poll([]string, int64, types.Context) []*bustypes.Notification
	// Stop the dispatching loop
	Stop()
}

// Send is the endpoint for sending a message from client side
func Send(c *server.Context) {
	uid := c.Session().Get("uid").(int64)
	web.CheckUser(uid)
	var params bustypes.Notification
	c.BindRPCParams(&params)
	err := models.ExecuteInNewEnvironment(uid, func(env models.Environment) {
		h.BusBus().NewSet(env).Sendone(params.Channel, params.Message)
	})
	c.RPC(http.StatusOK, nil, err)
}

// Poll returns the pending notification on the given channels since the last retrieved id.
func Poll(c *server.Context) {
	uid := c.Session().Get("uid").(int64)
	web.CheckUser(uid)
	var params bustypes.PollParams
	c.BindRPCParams(&params)
	if Dispatcher == nil {
		log.Warn("Bus dispatcher unavailable")
		c.RPC(http.StatusOK, []*bustypes.Notification{}, nil)
		return
	}
	// Update the user presence
	if params.Options.HasKey("bus_inactivity") {
		models.ExecuteInNewEnvironment(uid, func(env models.Environment) {
			h.BusPresence().NewSet(env).Update(time.Duration(params.Options.GetInteger("bus_inactivity")) * time.Millisecond)
		})
	}
	notifications := Dispatcher.Poll(params.Channels, params.Last, params.Options)
	if notifications == nil {
		notifications = []*bustypes.Notification{}
	}
	c.RPC(http.StatusOK, notifications)
}

func init() {
	log = logging.GetLogger("bus.controllers")
	root := controllers.Registry
	longpolling := root.AddGroup("/longpolling")
	{
		longpolling.AddMiddleWare(web.LoginRequired)
		longpolling.AddController(http.MethodPost, "/send", Send)
		longpolling.AddController(http.MethodPost, "/poll", Poll)
	}
}
