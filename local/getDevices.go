package local

import (
	"context"
	"fmt"

	"github.com/go-ocf/sdk/schema"

	gocoap "github.com/go-ocf/go-coap"
)

// DeviceHandler conveys device connections and errors during discovery.
type DeviceHandler interface {
	// Handle gets a device connection and is responsible for closing it.
	Handle(ctx context.Context, device *Device, deviceLinks schema.ResourceLinks)
	// Error gets errors during discovery.
	Error(err error)
}

// GetDevices discovers devices using a CoAP multicast request via UDP.
// Device resources can be queried in DeviceHandler using device.Client,
func (c *Client) GetDevices(ctx context.Context, handler DeviceHandler) error {
	multicastConn := DialDiscoveryAddresses(ctx, c.discoveryConfiguration, c.errFunc)
	defer func() {
		for _, conn := range multicastConn {
			conn.Close()
		}
	}()
	return DiscoverDevices(ctx, multicastConn, newDiscoveryHandler(c.getDeviceConfiguration(), handler))
}

func newDiscoveryHandler(
	deviceCfg deviceConfiguration,
	h DeviceHandler,
) *discoveryHandler {
	return &discoveryHandler{
		deviceCfg: deviceCfg,
		handler:   h,
	}
}

type discoveryHandler struct {
	deviceCfg deviceConfiguration
	handler   DeviceHandler
}

func (h *discoveryHandler) Handle(ctx context.Context, conn *gocoap.ClientConn, links schema.ResourceLinks) {
	conn.Close()

	link, err := GetResourceLink(links, "/oic/d")
	if err != nil {
		h.handler.Error(err)
		return
	}
	deviceID := link.GetDeviceID()
	if deviceID == "" {
		h.handler.Error(fmt.Errorf("cannot determine deviceID"))
		return
	}
	if len(link.ResourceTypes) == 0 {
		h.handler.Error(fmt.Errorf("cannot get resource types for %v: is empty", deviceID))
		return
	}
	d := NewDevice(h.deviceCfg, deviceID, link.ResourceTypes)

	h.handler.Handle(ctx, d, links)
}

func (h *discoveryHandler) Error(err error) {
	h.handler.Error(err)
}
