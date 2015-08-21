package castv2

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/gogo/protobuf/proto"

	log "github.com/cihub/seelog"
	"github.com/davecgh/go-spew/spew"
	"github.com/jonaz/go-castv2/api"
)

type Client struct {
	conn      *packetStream
	channels  []*Channel
	requestId int

	Connection *Channel
	HeartBeat  *Channel

	heartBeatTicker *time.Ticker
}

type PayloadHeaders struct {
	Type      string `json:"type"`
	RequestId *int   `json:"requestId,omitempty"`
}

func (h *PayloadHeaders) setRequestId(id int) {
	h.RequestId = &id
}

func (h *PayloadHeaders) getRequestId() int {
	return *h.RequestId
}

func NewClient(host net.IP, port int) (*Client, error) {

	log.Infof("connecting to %s:%d ...", host, port)

	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", host, port), &tls.Config{
		InsecureSkipVerify: true,
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to connect to Chromecast. Error:%s", err)
	}

	wrapper := NewPacketStream(conn)

	client := &Client{
		conn:     wrapper,
		channels: make([]*Channel, 0),
	}

	go func() {
		for {
			packet := wrapper.Read()

			message := &api.CastMessage{}
			err = proto.Unmarshal(*packet, message)
			if err != nil {
				log.Errorf("Failed to unmarshal CastMessage: %s", err)
				continue
			}

			var headers PayloadHeaders

			err := json.Unmarshal([]byte(*message.PayloadUtf8), &headers)

			if err != nil {
				log.Errorf("Failed to unmarshal message: %s", err)
				continue
			}

			log.Trace("Channels: ", len(client.channels), " Got message in namepspace: "+*message.Namespace)

			deliverd := false
			for _, channel := range client.channels {
				deliverd = deliverd || channel.message(message, &headers)
			}

			if !deliverd {
				spew.Dump("Lost message (no one is lisening):", message)
			}
		}
	}()

	client.Connection = client.NewChannel("sender-0", "receiver-0", "urn:x-cast:com.google.cast.tp.connection")
	client.Connection.Send(PayloadHeaders{Type: "CONNECT"})

	client.HeartBeat = client.NewChannel("sender-0", "receiver-0", "urn:x-cast:com.google.cast.tp.heartbeat")
	client.HeartBeat.OnMessage("PING", func(_ *api.CastMessage) {
		client.HeartBeat.Send(PayloadHeaders{Type: "PONG"})
	})
	client.HeartBeat.OnMessage("PONG", func(_ *api.CastMessage) {

	})

	client.heartBeatTicker = time.NewTicker(time.Second * 5)
	go func() {
		for {
			<-client.heartBeatTicker.C
			client.HeartBeat.Send(PayloadHeaders{Type: "PING"})
		}
	}()

	return client, nil
}

func (c *Client) NewChannel(sourceId, destinationId, namespace string) *Channel {
	channel := &Channel{
		client:        c,
		sourceId:      sourceId,
		destinationId: destinationId,
		namespace:     namespace,
		listeners:     make([]channelListener, 0),
		inFlight:      make(map[int]chan *api.CastMessage),
	}

	c.channels = append(c.channels, channel)

	return channel
}

func (c *Client) CloseChannel(channel *Channel) {
	log.Warn("CLOSE channel, len:", len(c.channels))

	for i, item := range c.channels {
		if item == channel {

			if len(c.channels) > i {
				log.Error("FOUND channel in the middle, deleting")
				c.channels = append(c.channels[:i], c.channels[i+1:]...)
			} else {
				log.Error("FOUND channel in the end, deleting")
				c.channels = c.channels[:i]
			}
		}
	}
	log.Warn("CLOSE channel, after:", len(c.channels))
}

func (c *Client) Send(message *api.CastMessage) error {

	proto.SetDefaults(message)

	data, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	//spew.Dump("Writing", message)
	//log.Trace("SEND: ", message)

	_, err = c.conn.Write(&data)

	return err

}
