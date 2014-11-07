package controllers

import (
	"encoding/json"
	"time"

	log "github.com/cihub/seelog"
	"github.com/jonaz/go-castv2"
	"github.com/jonaz/go-castv2/api"
)

type ReceiverController struct {
	interval time.Duration
	channel  *castv2.Channel
	Incoming chan *StatusResponse
}

var getStatus = castv2.PayloadHeaders{Type: "GET_STATUS"}

func NewReceiverController(client *castv2.Client, sourceId, destinationId string) *ReceiverController {
	controller := &ReceiverController{
		channel:  client.NewChannel(sourceId, destinationId, "urn:x-cast:com.google.cast.receiver"),
		Incoming: make(chan *StatusResponse, 0),
	}

	controller.channel.OnMessage("RECEIVER_STATUS", controller.onStatus)

	return controller
}

func (c *ReceiverController) onStatus(message *api.CastMessage) {
	//spew.Dump("Got status message", message)

	response := &StatusResponse{}

	err := json.Unmarshal([]byte(*message.PayloadUtf8), response)

	if err != nil {
		log.Errorf("Failed to unmarshal status message:%s - %s", err, *message.PayloadUtf8)
		return
	}

	select {
	case c.Incoming <- response:
	default:
		log.Warnf("Incoming status, but we aren't listening. %v", response)
	}

}

type StatusResponse struct {
	Status *ReceiverStatus `json:"status,omitempty"`
}

//{\"applications\":[{\"appId\":\"E8C28D3C\",\"displayName\":\"Backdrop\",\"namespaces\":[{\"name\":\"urn:x-cast:com.google.cast.sse\"}],\"sessionId\":\"B6C1F700-50F6-04F0-8951-C0D49118E66A\",\"statusText\":\"\",\"transportId\":\"web-0\"}],\"isActiveInput\":false,\"isStandBy\":true,\"volume\":{\"level\":1.0,\"muted\":false}}

type ReceiverStatus struct {
	castv2.PayloadHeaders
	Applications  []*AppPayload  `json:"applications,omitempty"`
	IsStandBy     bool           `json:"isStandBy,omitempty"`
	IsActiveInput bool           `json:"isActiveInput,omitempty"`
	Volume        *VolumePayload `json:"volume,omitempty"`
}

type AppPayload struct {
	AppId       string              `json:"appId,omitempty"`
	DisplayName string              `json:"displayName,omitempty"`
	Namespaces  []*NamespacePayload `json:"namespaces,omitempty"`
	SessionId   string              `json:"sessionId,omitempty"`
	StatusText  string              `json:"statusText,omitempty"`
	TransportId string              `json:"transportId,omitempty"`
}

type NamespacePayload struct {
	Name string `json:"name,omitempty"`
}

type VolumePayload struct {
	Level *float64 `json:"level,omitempty"`
	Muted *bool    `json:"muted,omitempty"`
}

func (c *ReceiverController) GetStatus(timeout time.Duration) (*api.CastMessage, error) {
	return c.channel.Request(&getStatus, timeout)
}

//func (c *ReceiverController) SetVolume(volume *VolumePayload, timeout time.Duration) (*api.CastMessage, error) {
//return c.channel.Request(&ReceiverStatus{
//castv2.PayloadHeaders{Type: "SET_VOLUME"}, volume,
//}, timeout)
//}

func (c *ReceiverController) GetVolume(timeout time.Duration) (*VolumePayload, error) {
	message, err := c.GetStatus(timeout)

	if err != nil {
		return nil, err
	}

	response := StatusResponse{}

	err = json.Unmarshal([]byte(*message.PayloadUtf8), &response)

	return response.Status.Volume, err
}
