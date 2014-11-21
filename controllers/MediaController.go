package controllers

import (
	"encoding/json"
	"time"

	log "github.com/cihub/seelog"
	"github.com/jonaz/go-castv2"
	"github.com/jonaz/go-castv2/api"
)

type MediaController struct {
	endpoint   string
	interval   time.Duration
	channel    *castv2.Channel
	connection *castv2.Channel
	Incoming   chan *MediaStatusResponse
}

func NewMediaController(client *castv2.Client, sourceId, destinationId string) *MediaController {
	controller := &MediaController{
		endpoint:   "urn:x-cast:com.google.cast.media",
		channel:    client.NewChannel(sourceId, destinationId, "urn:x-cast:com.google.cast.media"),
		connection: client.NewChannel("sender-0", destinationId, "urn:x-cast:com.google.cast.tp.connection"),
		Incoming:   make(chan *MediaStatusResponse),
	}
	// Connect to the endpoint
	controller.connection.Send(castv2.PayloadHeaders{Type: "CONNECT"})
	controller.connection.OnMessage("CONNECT", controller.connected)
	controller.connection.OnMessage("CLOSE", controller.disconnected)

	// Listen for media updates
	controller.channel.OnMessage("MEDIA_STATUS", controller.onStatus)

	return controller
}

func (c *MediaController) connected(_ *api.CastMessage) {
	log.Info("Connected to ", c.endpoint)

	// Request a status update when we are connected
	c.GetStatus(time.Second)
}

func (c *MediaController) disconnected(_ *api.CastMessage) {
	log.Info("Disconnected from ", c.endpoint)
	c.channel.Close()
	c.connection.Close()

	if c.Incoming != nil {
		select {
		case c.Incoming <- nil:
		default:
		}

		c.Incoming = nil
	}
}

func (c *MediaController) onStatus(message *api.CastMessage) {
	//spew.Dump("Got status message", message)

	response := &MediaStatusResponse{}

	err := json.Unmarshal([]byte(*message.PayloadUtf8), response)

	if err != nil {
		log.Errorf("Failed to unmarshal status message:%s - %s", err, *message.PayloadUtf8)
		return
	}

	select {
	case c.Incoming <- response: // Try to transport the message back to the APP
	default:
		log.Warnf("Incoming status, but we aren't listening. %v", response)
	}
}

type MediaStatusResponse struct {
	Status []*MediaStatus `json:"status,omitempty"`
}

type MediaStatus struct {
	MediaSessionId         int               `json:"mediaSessionId,omitempty"`
	Media                  *MediaInformation `json:"media,omitempty"`
	PlaybackRate           float32           `json:"playbackRate,omitempty"`
	PlayerState            string            `json:"playerState,omitempty"`
	IdleReason             string            `json:"idleReason,omitempty"`
	CurrentTime            float32           `json:"currentTime,omitempty"`
	SupportedMediaCommands int               `json:"supportedMediaCommands,omitempty"`
	Volume                 *VolumePayload    `json:"volume,omitempty"`
}

type MediaInformation struct {
	ContentId   string `json:"contentId,omitempty"`
	StreamType  string `json:"streamType,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	//MetaData    []string `json:"metaData,omitempty"`
	Duration float32 `json:"duration,omitempty"`
}

//type VolumePayload struct {
//Level *float64 `json:"level,omitempty"`
//Muted *bool    `json:"muted,omitempty"`
//}

func (c *MediaController) GetStatus(timeout time.Duration) (*api.CastMessage, error) {
	getStatus := castv2.PayloadHeaders{Type: "GET_STATUS"}
	return c.channel.Request(&getStatus, timeout)
}

//func (c *ReceiverController) SetVolume(volume *VolumePayload, timeout time.Duration) (*api.CastMessage, error) {
//return c.channel.Request(&ReceiverStatus{
//castv2.PayloadHeaders{Type: "SET_VOLUME"}, volume,
//}, timeout)
//}

//func (c *ReceiverController) GetVolume(timeout time.Duration) (*VolumePayload, error) {
//message, err := c.GetStatus(timeout)

//if err != nil {
//return nil, err
//}

//response := StatusResponse{}

//err = json.Unmarshal([]byte(*message.PayloadUtf8), &response)

//return response.Status.Volume, err
//}
