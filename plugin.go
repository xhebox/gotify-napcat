package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/gotify/plugin-api"
)

func GetGotifyPluginInfo() plugin.Info {
	return plugin.Info{
		ModulePath:  "github.com/xhebox/gotify-napcat",
		Version:     "1.0.0",
		Author:      "xhe",
		Description: "Bridge message to napcat",
		License:     "MIT",
		Name:        "xhebox/gotify-napcat",
	}
}

type GotifyMessage struct {
	Id       uint32
	Appid    uint32
	Message  string
	Title    string
	Priority uint32
	Date     string
}

type NapcatMessageData struct {
	Text string `json:"text"`
}

type NapcatMessage struct {
	Type string            `json:"type"`
	Data NapcatMessageData `json:"data"`
}

type NapcatGroupMessage struct {
	GroupID string          `json:"group_id"`
	Message []NapcatMessage `json:"message"`
}

type NapcatPlugin struct {
	sync.Mutex
	logger *log.Logger
	ws     *websocket.Conn
	cancel func()
}

func (c *NapcatPlugin) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if c.ws == nil {
				var err error
				addr := fmt.Sprintf("%s/stream", os.Getenv("NAPCAT_GOTIFY_URL"))
				hdr := make(http.Header)
				hdr.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("NAPCAT_GOTIFY_TOKEN")))
				c.ws, _, err = websocket.DefaultDialer.DialContext(ctx, addr, hdr)
				if err != nil {
					c.logger.Printf("can not dial %s, %s", addr, err)
					time.Sleep(time.Second)
					continue
				}
			}

			msg := &GotifyMessage{}
			if err := c.ws.ReadJSON(msg); err != nil {
				if websocket.IsCloseError(err) {
					c.logger.Printf("read error %s", err)
					c.ws = nil
				} else {
					c.logger.Printf("read error %s, close it %s", err, c.ws.Close())
				}
				continue
			}

			nmsg := NapcatGroupMessage{
				GroupID: os.Getenv("NAPCAT_GROUP_ID"),
				Message: []NapcatMessage{
					{
						Type: "text",
						Data: NapcatMessageData{
							Text: fmt.Sprintf("[%s] %s", msg.Title, msg.Message),
						},
					},
				},
			}
			f, err := json.Marshal(nmsg)
			if err != nil {
				c.logger.Printf("marshal error %s", err)
				continue
			}
			res, err := http.DefaultClient.Post(fmt.Sprintf("%s/send_group_msg", os.Getenv("NAPCAT_URL")), "application/json", bytes.NewReader(f))
			if err != nil {
				c.logger.Printf("post napcat error %s", err)
				continue
			}
			resbody, _ := io.ReadAll(res.Body)
			if strings.Contains(string(resbody), "failed") {
				c.logger.Printf("received msg %+v", msg)
				c.logger.Printf("napcat req %s, res %s, close error %s", f, resbody, res.Body.Close())
			}
		}
	}
}

// Enable enables the plugin.
func (c *NapcatPlugin) Enable() error {
	c.Lock()
	if c.cancel != nil {
		c.Unlock()
		return nil
	}

	var ctx context.Context
	ctx, c.cancel = context.WithCancel(context.Background())
	c.Unlock()

	go c.loop(ctx)
	return nil
}

// Disable disables the plugin.
func (c *NapcatPlugin) Disable() error {
	c.Lock()
	if c.cancel != nil {
		if c.ws != nil {
			c.logger.Printf("disable plugin %s", c.ws.Close())
		}
		c.cancel()
	}
	c.Unlock()
	return nil
}

// RegisterWebhook implements plugin.Webhooker.
func (c *NapcatPlugin) RegisterWebhook(basePath string, g *gin.RouterGroup) {
}

// NewGotifyPluginInstance creates a plugin instance for a user context.
func NewGotifyPluginInstance(ctx plugin.UserContext) plugin.Plugin {
	return &NapcatPlugin{logger: log.Default()}
}

func main() {
	panic("this should be built as go plugin")
}
