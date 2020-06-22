// Copyright 2020 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package bus

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/hexya-addons/bus/bustypes"
	"github.com/hexya-addons/bus/controllers"
	"github.com/hexya-addons/web/client"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/server"
	"github.com/hexya-erp/hexya/src/tests"
	"github.com/hexya-erp/pool/h"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMain(m *testing.M) {
	tests.RunTests(m, "bus", nil)
}

func TestBus(t *testing.T) {
	hexyaURL := url.URL{
		Scheme: "http",
		Host:   "localhost:8585",
	}
	go func() { fmt.Println("server:", server.GetServer().Run(hexyaURL.Host)) }()
	// Wait for server to come up
	time.Sleep(500 * time.Millisecond)
	cl1 := client.NewHexyaClient(hexyaURL.String())
	cl1.Login("admin", "admin")
	cl2 := client.NewHexyaClient(hexyaURL.String())
	cl2.Login("admin", "admin")
	cl3 := client.NewHexyaClient(hexyaURL.String())
	cl3.Login("admin", "admin")
	cl4 := client.NewHexyaClient(hexyaURL.String())
	cl4.Login("admin", "admin")
	Convey("Testing the IM Bus", t, func() {
		controllers.Dispatcher.Start()
		Convey("Simple notification to several clients", func() {
			ch2 := make(chan json.RawMessage)
			ch3 := make(chan json.RawMessage)
			ch4 := make(chan json.RawMessage)
			errCh2 := make(chan error)
			errCh3 := make(chan error)
			errCh4 := make(chan error)
			go func() {
				msg, err := cl2.RPC("/longpolling/poll", "call", bustypes.PollParams{
					Channels: []string{"channel1", "channel2"},
					Last:     0,
				})
				errCh2 <- err
				ch2 <- msg
			}()
			go func() {
				msg, err := cl3.RPC("/longpolling/poll", "call", bustypes.PollParams{
					Channels: []string{"channel1"},
					Last:     0,
				})
				errCh3 <- err
				ch3 <- msg
			}()
			go func() {
				msg, err := cl4.RPC("/longpolling/poll", "call", bustypes.PollParams{
					Channels: []string{"channel2"},
					Last:     0,
				})
				errCh4 <- err
				ch4 <- msg
			}()
			// Wait for our 3 listeners to be ready
			time.Sleep(200 * time.Millisecond)
			cl1.RPC("/longpolling/send", "call", bustypes.Notification{
				Channel: "channel1",
				Message: map[string]interface{}{
					"title": "Hello World!",
				},
			})
			err2 := <-errCh2
			So(err2, ShouldBeNil)
			msg2 := <-ch2
			err3 := <-errCh3
			So(err3, ShouldBeNil)
			msg3 := <-ch3
			So(string(msg2), ShouldEqual, `[{"id":1,"channel":"channel1","message":{"title":"Hello World!"}}]`)
			So(string(msg3), ShouldEqual, `[{"id":1,"channel":"channel1","message":{"title":"Hello World!"}}]`)
			cl1.RPC("/longpolling/send", "call", bustypes.Notification{
				Channel: "channel2",
				Message: map[string]interface{}{
					"title": "Hello Everyone!",
				},
			})
			err4 := <-errCh4
			So(err4, ShouldBeNil)
			msg4 := <-ch4
			So(string(msg4), ShouldEqual, `[{"id":2,"channel":"channel2","message":{"title":"Hello Everyone!"}}]`)
		})
		Convey("Several notifications at once", func() {
			ch2 := make(chan json.RawMessage)
			errCh2 := make(chan error)
			go func() {
				msg, err := cl2.RPC("/longpolling/poll", "call", bustypes.PollParams{
					Channels: []string{"channel1"},
					Last:     2,
				})
				errCh2 <- err
				ch2 <- msg
			}()
			// Wait for our listener to be ready
			time.Sleep(200 * time.Millisecond)
			err := models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				h.BusBus().NewSet(env).Sendmany([]*bustypes.Notification{
					{
						Channel: "channel1",
						Message: map[string]interface{}{
							"title": "Hello World 2!",
						},
					},
					{
						Channel: "channel1",
						Message: map[string]interface{}{
							"title": "Hello World 3!",
						},
					},
				})
			})
			So(err, ShouldBeNil)
			err2 := <-errCh2
			So(err2, ShouldBeNil)
			msg2 := <-ch2
			So(string(msg2), ShouldEqual, `[{"id":3,"channel":"channel1","message":{"title":"Hello World 2!"}},{"id":4,"channel":"channel1","message":{"title":"Hello World 3!"}}]`)
		})
		Convey("Poll timeout", func() {
			msg, err := cl2.RPC("/longpolling/poll", "call", bustypes.PollParams{
				Channels: []string{"channel1"},
				Last:     4,
				Options:  types.NewContext().WithKey("timeout", 1),
			})
			So(err, ShouldBeNil)
			So(string(msg), ShouldEqual, "[]")
		})
		Reset(func() {
			controllers.Dispatcher.Stop()
		})
	})
}
