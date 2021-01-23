package cbyge

import (
	"sync"
	"time"

	"github.com/pkg/errors"
)

const DefaultTimeout = time.Second * 10

type ControllerDevice struct {
	deviceID    string
	switchID    uint32
	deviceIndex int
	name        string

	lastStatus     StatusPaginatedResponse
	lastStatusLock sync.RWMutex
}

// DeviceID gets a unique identifier for the device.
func (c *ControllerDevice) DeviceID() string {
	return c.deviceID
}

// Name gets the user-assigned name of the device.
func (c *ControllerDevice) Name() string {
	return c.name
}

// LastStatus gets the last known status of the device.
//
// This is not updated automatically, but it will be updated on a device
// object when Controller.DeviceStatus() is called.
func (c *ControllerDevice) LastStatus() StatusPaginatedResponse {
	c.lastStatusLock.RLock()
	defer c.lastStatusLock.RUnlock()
	return c.lastStatus
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
//
// Each device's status is available through its LastStatus() method.
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
				deviceID: bulb.DeviceID,
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
//
// If no error occurs, the status is updated in d.LastStatus() in addition to
// being returned.
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
	d.lastStatusLock.Lock()
	d.lastStatus = responsePacket[0]
	d.lastStatusLock.Unlock()
	return responsePacket[0], nil
}

// SetDeviceStatus turns on or off a device.
func (c *Controller) SetDeviceStatus(d *ControllerDevice, status bool) error {
	statusInt := 0
	if status {
		statusInt = 1
	}
	packet := NewPacketSetDeviceStatus(d.switchID, 123, d.deviceIndex, statusInt)
	return c.callAndWaitSimple(packet, "set device status")
}

// SetDeviceLum changes a device's brightness.
//
// Brightness values are in [1, 100].
func (c *Controller) SetDeviceLum(d *ControllerDevice, lum int) error {
	packet := NewPacketSetLum(d.switchID, 123, d.deviceIndex, lum)
	return c.callAndWaitSimple(packet, "set device luminance")
}

// SetDeviceCT changes a device's color tone.
//
// Color tone values are in [0, 100].
func (c *Controller) SetDeviceCT(d *ControllerDevice, ct int) error {
	packet := NewPacketSetCT(d.switchID, 123, d.deviceIndex, ct)
	return c.callAndWaitSimple(packet, "set device color tone")
}

func (c *Controller) callAndWaitSimple(p *Packet, context string) error {
	err := c.callAndWait([]*Packet{p}, func(p *Packet) bool {
		return p.Type == PacketTypePipe && p.IsResponse
	})
	if err != nil {
		return errors.Wrap(err, context)
	}
	return nil
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
