package castv2

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jonaz/go-castv2/api"
)

type Channel struct {
	client        *Client
	sourceId      string
	destinationId string
	namespace     string
	//requestId     int
	inFlight  map[int]chan *api.CastMessage
	listeners []channelListener
}

type channelListener struct {
	responseType string
	callback     func(*api.CastMessage)
}

type hasRequestId interface {
	setRequestId(id int)
	getRequestId() int
}

func (c *Channel) Close() {
	c.client.CloseChannel(c)
}

func (c *Channel) message(message *api.CastMessage, headers *PayloadHeaders) bool {

	//spew.Dump("RAW MESSAGE", message)

	if *message.DestinationId != "*" && (*message.SourceId != c.destinationId || *message.DestinationId != c.sourceId || *message.Namespace != c.namespace) {
		return false
	}

	deliverd := false

	if *message.DestinationId != "*" && headers.RequestId != nil {
		listener, ok := c.inFlight[*headers.RequestId]
		if !ok {
			log.Printf("Warning: Unknown incoming response id: %d to destination:%s", *headers.RequestId, c.destinationId)
			return false
		}

		listener <- message
		delete(c.inFlight, *headers.RequestId)

		deliverd = true
	}

	if headers.Type == "" {
		if !deliverd {
			log.Printf("Warning: No message type. Don't know what to do. headers:%v message:%v", headers, message)
		}

		return deliverd
	}

	for _, listener := range c.listeners {
		if listener.responseType == headers.Type {
			listener.callback(message)
			deliverd = true
		}
	}

	return deliverd
}

func (c *Channel) OnMessage(responseType string, cb func(*api.CastMessage)) {
	c.listeners = append(c.listeners, channelListener{responseType, cb})
}

func (c *Channel) Send(payload interface{}) error {

	payloadJson, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	payloadString := string(payloadJson)

	message := &api.CastMessage{
		ProtocolVersion: api.CastMessage_CASTV2_1_0.Enum(),
		SourceId:        &c.sourceId,
		DestinationId:   &c.destinationId,
		Namespace:       &c.namespace,
		PayloadType:     api.CastMessage_STRING.Enum(),
		PayloadUtf8:     &payloadString,
	}

	return c.client.Send(message)
}

func (c *Channel) Request(payload hasRequestId, timeout time.Duration) (*api.CastMessage, error) {

	// TODO: Need locking here
	c.client.requestId++
	//c.requestId++

	//payload.setRequestId(c.requestId)
	payload.setRequestId(c.client.requestId)

	response := make(chan *api.CastMessage)

	c.inFlight[payload.getRequestId()] = response

	err := c.Send(payload)

	if err != nil {
		delete(c.inFlight, payload.getRequestId())
		return nil, err
	}

	select {
	case reply := <-response:
		return reply, nil
	case <-time.After(timeout):
		delete(c.inFlight, payload.getRequestId())
		return nil, fmt.Errorf("Call to cast channel %s, id %d - timed out after %d seconds", c.destinationId, payload.getRequestId(), timeout/time.Second)
	}

}
