package cbyge

import (
	"time"

	"github.com/pkg/errors"
)

const DefaultTimeout = time.Second * 10

type ControllerDevice struct {
	switchID    uint32
	deviceIndex int
	name        string
}

func (c *ControllerDevice) Name() string {
	return c.name
}

// A Controller is a high-level API for manipulating C by GE devices.
type Controller struct {
	sessionInfo *SessionInfo
	timeout     time.Duration
}

// NewController creates a Controller using a pre-created session and a
// specified timeout.
//
// If timeout is 0, then DefaultTimeout is used.
func NewController(s *SessionInfo, timeout time.Duration) *Controller {
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	return &Controller{
		sessionInfo: s,
		timeout:     timeout,
	}
}

// NewControllerLogin creates a Controller by logging in with a username and
// password.
func NewControllerLogin(email, password string) (*Controller, error) {
	info, err := Login(email, password, "")
	if err != nil {
		return nil, errors.Wrap(err, "new controller")
	}
	return NewController(info, 0), nil
}

// Devices enumerates the devices available to the account.
func (c *Controller) Devices() ([]*ControllerDevice, error) {
	devicesResponse, err := GetDevices(c.sessionInfo.UserID, c.sessionInfo.AccessToken)
	if err != nil {
		return nil, err
	}
	var results []*ControllerDevice
	for _, dev := range devicesResponse {
		props, err := GetDeviceProperties(c.sessionInfo.AccessToken, dev.ProductID, dev.ID)
		if err != nil {
			if _, ok := err.(*RemoteError); !ok {
				return nil, err
			}
			continue
		}
		for _, bulb := range props.Bulbs {
			cd := &ControllerDevice{
				switchID: bulb.SwitchID,
				name:     bulb.DisplayName,
			}
			status, err := c.DeviceStatus(cd)
			if err != nil {
				return nil, err
			}
			cd.deviceIndex = status.Device
			results = append(results, cd)
		}
	}
	return results, nil
}

// DeviceStatus gets the status for a previously enumerated device.
func (c *Controller) DeviceStatus(d *ControllerDevice) (StatusPaginatedResponse, error) {
	var responsePacket []StatusPaginatedResponse
	var decodeErr error
	packet := NewPacketGetStatusPaginated(d.switchID, 0)
	err := c.callAndWait([]*Packet{packet}, func(p *Packet) bool {
		if IsStatusPaginatedResponse(p) {
			responsePacket, decodeErr = DecodeStatusPaginatedResponse(p)
			return true
		}
		return false
	})
	if decodeErr != nil {
		err = decodeErr
	} else if err == nil && len(responsePacket) == 0 {
		err = errors.New("lookup device status: no devices in response")
	}
	if err != nil {
		return StatusPaginatedResponse{}, errors.Wrap(err, "lookup device status")
	}
	return responsePacket[0], nil
}

// callAndWait sends packets on a new PacketConn and waits until f returns
// true on a response, or waits for a timeout.
func (c *Controller) callAndWait(p []*Packet, f func(*Packet) bool) error {
	conn, err := NewPacketConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.Auth(c.sessionInfo.Authorize); err != nil {
		return err
	}

	// Prevent the bg thread from blocking on a
	// channel send forever.
	doneChan := make(chan struct{}, 1)
	defer close(doneChan)

	packets := make(chan *Packet, 16)
	errChan := make(chan error, 1)
	go func() {
		defer close(packets)
		for {
			packet, err := conn.Read()
			if err != nil {
				errChan <- err
				return
			}
			select {
			case packets <- packet:
			case <-doneChan:
				return
			}
		}
	}()

	for _, subPacket := range p {
		if err := conn.Write(subPacket); err != nil {
			return err
		}
	}

	timeout := time.After(c.timeout)
	for {
		select {
		case packet := <-packets:
			if f(packet) {
				return nil
			}
		case err := <-errChan:
			return err
		case <-timeout:
			return errors.New("timeout waiting for response")
		}
	}
}
