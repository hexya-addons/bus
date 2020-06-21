package bus

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hexya-addons/bus/bustypes"
	"github.com/hexya-addons/bus/controllers"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	"github.com/lib/pq"
)

var timeout = 50 * time.Second

var fields_BusBus = map[string]models.FieldDefinition{
	"Channel": fields.Char{},
	"Message": fields.Char{},
}

// Gc garbage collects expired notifications, that is notifications that are older than 2 timeouts.
func busBus_Gc(rs m.BusBusSet) int64 {
	timeoutAgo := dates.Now().Add(-2 * timeout)
	return h.BusBus().NewSet(rs.Env()).Sudo().Search(q.BusBus().CreateDate().Lower(timeoutAgo)).Unlink()
}

// Sendmany sends the given notifications on the bus.
func busBus_Sendmany(rs m.BusBusSet, notifications []*bustypes.Notification) {
	channels := make(map[string]bool)
	for _, data := range notifications {
		channels[data.Channel] = true
		msgData, err := json.Marshal(data.Message)
		if err != nil {
			panic(fmt.Errorf("message '%#v' is not json serializable. error: %s", data.Message, err))
		}

		models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			// We execute in a new transaction that will be committed before we notify
			h.BusBus().Create(env, h.BusBus().NewData().
				SetChannel(data.Channel).
				SetMessage(string(msgData)))
		})
	}
	if len(channels) > 0 {
		topics := make([]string, len(channels))
		var i int
		for ch := range channels {
			topics[i] = ch
			i++
		}
		topicsJSON, err := json.Marshal(topics)
		if err != nil {
			panic(err)
		}
		query := fmt.Sprintf("NOTIFY imbus, '%s'", topicsJSON)
		rs.Env().Cr().Execute(query)
	}
}

// Sendone sends a single message on the given channel.
//
// message must be json serializable
func busBus_Sendone(rs m.BusBusSet, channel string, message interface{}) {
	rs.Sendmany([]*bustypes.Notification{{
		Channel: channel,
		Message: message,
	}})
}

// Poll returns pending notifications on the given channels
func busBus_Poll(rs m.BusBusSet, channels []string, last int64, options types.Context, force_status bool) []*bustypes.Notification {
	cond := q.BusBus().ID().Greater(last)
	if last == 0 {
		// We do not have info about last unread ID, so we send back all messages during the last timeout
		timeoutAgo := dates.Now().Add(-timeout)
		cond = q.BusBus().CreateDate().Greater(timeoutAgo)
	}
	cond = cond.And().Channel().In(channels)
	notifications := rs.Sudo().Search(cond).Load(q.BusBus().ID(), q.BusBus().Channel(), q.BusBus().Message())
	var res []*bustypes.Notification
	for _, notif := range notifications.Records() {
		var message interface{}
		err := json.Unmarshal([]byte(notif.Message()), &message)
		if err != nil {
			panic(fmt.Errorf("unable to JSON unmarshal message '%s'. err: %s", notif.Message(), err))
		}
		res = append(res, &bustypes.Notification{
			ID:      notif.ID(),
			Channel: notif.Channel(),
			Message: message,
		})
	}
	if len(res) > 0 || force_status {
		partner_ids := options.GetIntegerSlice("bus_presence_partner_ids")
		if len(partner_ids) > 0 {
			for _, p := range h.Partner().Browse(rs.Env(), partner_ids).Records() {

				res = append(res, &bustypes.Notification{
					ID:      -1,
					Channel: "bus.presence",
					Message: map[string]interface{}{
						"id":        p.ID(),
						"im_status": p.IMStatus(),
					},
				})
			}
		}
	}
	return res
}

// busDispatcher is a hub for dispatching long poll messages to clients.
type busDispatcher struct {
	sync.RWMutex
	topics   map[string]map[chan bool]bool
	started  bool
	stopChan chan struct{}
}

// newBusDispatcher returns a pointer to a new instance of busDispatcher
func newBusDispatcher() *busDispatcher {
	bd := busDispatcher{
		topics: make(map[string]map[chan bool]bool),
	}
	return &bd
}

func (bd *busDispatcher) channels(topics []string) []chan bool {
	bd.RLock()
	defer bd.RUnlock()
	chans := make(map[chan bool]bool)
	for _, topic := range topics {
		for ch := range bd.topics[topic] {
			chans[ch] = true
		}
	}
	res := make([]chan bool, len(chans))
	var i int
	for ch := range chans {
		res[i] = ch
		i++
	}
	return res
}

func (bd *busDispatcher) addChannel(topic string, ch chan bool) {
	bd.Lock()
	defer bd.Unlock()
	if bd.topics[topic] == nil {
		bd.topics[topic] = make(map[chan bool]bool)
	}
	bd.topics[topic][ch] = true
}

func (bd *busDispatcher) removeChannel(topic string, ch chan bool) {
	bd.Lock()
	defer bd.Unlock()
	delete(bd.topics[topic], ch)
}

// Poll returns the pending notification on the given channels since the last retrieved id.
func (bd *busDispatcher) Poll(channels []string, last int64, options types.Context) []*bustypes.Notification {
	var notifications []*bustypes.Notification
	models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		notifications = h.BusBus().NewSet(env).Poll(channels, last, options, false)
	})
	if options.GetBool("peek") {
		return notifications
	}
	if len(notifications) == 0 {
		if !bd.started {
			// Lazy start of events listener
			bd.start()
		}
		notifyChan := make(chan bool)
		for _, channel := range channels {
			bd.addChannel(channel, notifyChan)
		}
		select {
		case <-notifyChan:
			models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				notifications = h.BusBus().NewSet(env).Poll(channels, last, options, true)
			})
		case <-time.After(timeout):
		}
		// gc channels
		for _, channel := range channels {
			bd.removeChannel(channel, notifyChan)
		}
	}
	return notifications
}

// loop dispatches DB notifications to the relevant polling goroutine
func (bd *busDispatcher) loop() {
	connStr := models.DBParams().ConnectionString()
	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Warn("error in listener", "error", err)
		}
	}
	l := pq.NewListener(connStr, 10*time.Second, 1*time.Minute, reportProblem)
	defer l.Close()
	err := l.Listen("imbus")
	if err != nil {
		log.Warn("error when starting listen imbus", "error", err)
		return
	}
	for {
		select {
		case notification := <-l.Notify:
			if notification == nil {
				continue
			}
			// Notifiy each connection through its notification channel
			var topics []string
			err := json.Unmarshal([]byte(notification.Extra), &topics)
			if err != nil {
				log.Warn("error when reading topics", "error", err)
				return
			}
			for _, ch := range bd.channels(topics) {
				go func(c chan bool) {
					c <- true
				}(ch)
			}
		case <-time.After(timeout):
		case <-bd.stopChan:
			return
		}
	}
}

// run starts the loop, restarting it when it fails
func (bd *busDispatcher) run() {
	for {
		bd.loop()
		log.Warn("Bus.loop error, sleep and retry")
		select {
		case <-time.After(timeout):
		case <-bd.stopChan:
			return
		}
	}
}

// start the bus dispatcher loop in its own goroutine
func (bd *busDispatcher) start() {
	bd.started = true
	bd.stopChan = make(chan struct{})
	go bd.run()
}

// Stop the busDispatcher loop
func (bd *busDispatcher) Stop() {
	if bd.started {
		close(bd.stopChan)
		bd.started = false
	}
}

func init() {
	models.NewModel("BusBus")
	h.BusBus().AddFields(fields_BusBus)
	h.BusBus().NewMethod("Gc", busBus_Gc)
	h.BusBus().NewMethod("Sendmany", busBus_Sendmany)
	h.BusBus().NewMethod("Sendone", busBus_Sendone)
	h.BusBus().NewMethod("Poll", busBus_Poll)

	controllers.Dispatcher = newBusDispatcher()
}
