package cbyge

import (
	"encoding/binary"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const DefaultTimeout = time.Second * 10

type ControllerDeviceStatus struct {
	StatusPaginatedResponse

	// If IsOnline is false, all other fields are invalid.
	// This means that the device could not be reached.
	IsOnline bool
}

type ControllerDevice struct {
	deviceID string
	switchID uint32
	name     string

	lastStatus     ControllerDeviceStatus
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
func (c *ControllerDevice) LastStatus() ControllerDeviceStatus {
	c.lastStatusLock.RLock()
	defer c.lastStatusLock.RUnlock()
	return c.lastStatus
}

// A Controller is a high-level API for manipulating C by GE devices.
type Controller struct {
	sessionInfoLock sync.RWMutex
	sessionInfo     *SessionInfo
	timeout         time.Duration

	deviceIndicesLock sync.RWMutex
	deviceIndices     map[string]int

	// Prevent multiple PacketConns at once, since the server boots
	// off one connection when anoher is made.
	packetConnLock sync.Mutex
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

		deviceIndices: map[string]int{},
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

// Login creates a new authentication token on the session using the username
// and password.
func (c *Controller) Login(email, password string) error {
	info, err := Login(email, password, "")
	if err != nil {
		return errors.Wrap(err, "login controller")
	}
	c.sessionInfoLock.Lock()
	c.sessionInfo = info
	c.sessionInfoLock.Unlock()
	return nil
}

// Devices enumerates the devices available to the account.
//
// Each device's status is available through its LastStatus() method.
func (c *Controller) Devices() ([]*ControllerDevice, error) {
	sessInfo := c.getSessionInfo()
	devicesResponse, err := GetDevices(sessInfo.UserID, sessInfo.AccessToken)
	if err != nil {
		return nil, err
	}
	var results []*ControllerDevice
	for _, dev := range devicesResponse {
		props, err := GetDeviceProperties(sessInfo.AccessToken, dev.ProductID, dev.ID)
		if err != nil {
			if !IsPropertyNotExistsError(err) {
				return nil, err
			}
			continue
		}
		for _, bulb := range props.Bulbs {
			cd := &ControllerDevice{
				deviceID: strconv.Itoa(bulb.DeviceID),
				switchID: bulb.SwitchID,
				name:     bulb.DisplayName,
			}
			results = append(results, cd)
		}
	}
	// Update device status. If this fails, we swallow the error
	// because the device(s) are automatically marked offline.
	c.DeviceStatuses(results)
	return results, nil
}

// DeviceStatus gets the status for a previously enumerated device.
//
// If no error occurs, the status is updated in d.LastStatus() in addition to
// being returned.
func (c *Controller) DeviceStatus(d *ControllerDevice) (ControllerDeviceStatus, error) {
	var responsePacket []StatusPaginatedResponse
	var decodeErr error
	packet := NewPacketGetStatusPaginated(d.switchID, 0)
	err := c.callAndWait([]*Packet{packet}, true, func(p *Packet) bool {
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
		return ControllerDeviceStatus{}, errors.Wrap(err, "lookup device status")
	}

	c.deviceIndicesLock.Lock()
	c.deviceIndices[d.deviceID] = responsePacket[0].Device
	c.deviceIndicesLock.Unlock()

	status := ControllerDeviceStatus{
		StatusPaginatedResponse: responsePacket[0],
		IsOnline:                true,
	}
	d.lastStatusLock.Lock()
	d.lastStatus = status
	d.lastStatusLock.Unlock()
	return status, nil
}

// DeviceStatuses gets the status for previously enumerated devices.
//
// Each device will have its own status, and can have an independent error
// when fetching the status.
//
// Each device's status is updated in d.LastStatus() if no error occurred for
// that device.
func (c *Controller) DeviceStatuses(devs []*ControllerDevice) ([]ControllerDeviceStatus, []error) {
	deviceStatuses := make([]ControllerDeviceStatus, len(devs))
	deviceErrors := make([]error, len(devs))
	hasResponses := make([]bool, len(devs))

	packets := make([]*Packet, len(devs))
	idToIndex := map[uint32]int{}
	for i, d := range devs {
		idToIndex[d.switchID] = i
		packets[i] = NewPacketGetStatusPaginated(d.switchID, uint16(i))
	}
	err := c.callAndWait(packets, false, func(p *Packet) bool {
		if IsStatusPaginatedResponse(p) {
			deviceID := binary.BigEndian.Uint32(p.Data[:4])
			devIdx, ok := idToIndex[deviceID]
			if !ok || hasResponses[devIdx] {
				return false
			}
			hasResponses[devIdx] = true
			response, err := DecodeStatusPaginatedResponse(p)
			if err == nil && len(response) == 0 {
				err = errors.New("lookup device status: no devices in response")
			}
			status := ControllerDeviceStatus{IsOnline: false}
			if err != nil {
				deviceErrors[devIdx] = err
			} else {
				status.IsOnline = true
				status.StatusPaginatedResponse = response[0]
				c.deviceIndicesLock.Lock()
				c.deviceIndices[devs[devIdx].DeviceID()] = response[0].Device
				c.deviceIndicesLock.Unlock()
			}
			deviceStatuses[devIdx] = status
		} else if p.IsResponse && len(p.Data) >= 4 && p.Data[len(p.Data)-1] != 0 {
			// This is an error response.
			deviceID := binary.BigEndian.Uint32(p.Data[:4])
			devIdx, ok := idToIndex[deviceID]
			if ok && !hasResponses[devIdx] {
				hasResponses[devIdx] = true
				deviceErrors[devIdx] = RemoteCallError
			}
		}
		for _, hasResponse := range hasResponses {
			if !hasResponse {
				return false
			}
		}
		return true
	})
	if err != nil {
		for i, hasResponse := range hasResponses {
			if !hasResponse {
				deviceErrors[i] = err
			}
		}
	}
	// Update statuses for online devices.
	for i, status := range deviceStatuses {
		if deviceErrors[i] == nil {
			devs[i].lastStatusLock.Lock()
			devs[i].lastStatus = status
			devs[i].lastStatusLock.Unlock()
		}
	}
	return deviceStatuses, deviceErrors
}

// SetDeviceStatus turns on or off a device.
func (c *Controller) SetDeviceStatus(d *ControllerDevice, status bool) error {
	index, err := c.getDeviceIndex(d)
	if err != nil {
		return errors.Wrap(err, "set device status")
	}
	statusInt := 0
	if status {
		statusInt = 1
	}
	packet := NewPacketSetDeviceStatus(d.switchID, 123, index, statusInt)
	return c.callAndWaitSimple(packet, "set device status")
}

// SetDeviceLum changes a device's brightness.
//
// Brightness values are in [1, 100].
func (c *Controller) SetDeviceLum(d *ControllerDevice, lum int) error {
	index, err := c.getDeviceIndex(d)
	if err != nil {
		return errors.Wrap(err, "set device luminance")
	}
	packet := NewPacketSetLum(d.switchID, 123, index, lum)
	return c.callAndWaitSimple(packet, "set device luminance")
}

// SetDeviceLum changes a device's RGB.
func (c *Controller) SetDeviceRGB(d *ControllerDevice, r, g, b uint8) error {
	index, err := c.getDeviceIndex(d)
	if err != nil {
		return errors.Wrap(err, "set device RGB")
	}
	packet := NewPacketSetRGB(d.switchID, 123, index, r, g, b)
	return c.callAndWaitSimple(packet, "set device RGB")
}

// SetDeviceCT changes a device's color tone.
//
// Color tone values are in [0, 100].
func (c *Controller) SetDeviceCT(d *ControllerDevice, ct int) error {
	index, err := c.getDeviceIndex(d)
	if err != nil {
		return errors.Wrap(err, "set device color tone")
	}
	packet := NewPacketSetCT(d.switchID, 123, index, ct)
	return c.callAndWaitSimple(packet, "set device color tone")
}

func (c *Controller) getDeviceIndex(d *ControllerDevice) (int, error) {
	c.deviceIndicesLock.Lock()
	index, ok := c.deviceIndices[d.deviceID]
	c.deviceIndicesLock.Unlock()
	if ok {
		// The device was already online.
		return index, nil
	}

	// Getting the device status forces us to lookup the device index.
	// If it succeeds, then the device has come online.
	_, err := c.DeviceStatus(d)
	if err != nil {
		return 0, err
	}

	c.deviceIndicesLock.Lock()
	defer c.deviceIndicesLock.Unlock()
	return c.deviceIndices[d.deviceID], nil
}

func (c *Controller) callAndWaitSimple(p *Packet, context string) error {
	err := c.callAndWait([]*Packet{p}, true, func(p *Packet) bool {
		return p.Type == PacketTypePipe && p.IsResponse
	})
	if err != nil {
		return errors.Wrap(err, context)
	}
	return nil
}

// callAndWait sends packets on a new PacketConn and waits until f returns
// true on a response, or waits for a timeout.
func (c *Controller) callAndWait(p []*Packet, checkError bool, f func(*Packet) bool) error {
	c.packetConnLock.Lock()
	defer c.packetConnLock.Unlock()

	conn, err := NewPacketConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.Auth(c.getSessionInfo().Authorize, c.timeout); err != nil {
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
			if checkError && packet.IsResponse {
				if len(packet.Data) > 0 {
					if packet.Data[len(packet.Data)-1] != 0 {
						errChan <- RemoteCallError
						return
					}
				}
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
		case packet, ok := <-packets:
			if !ok {
				// Could be a race condition between packets and errChan.
				select {
				case err := <-errChan:
					return err
				default:
					return errors.New("connection closed")
				}
			}
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

func (c *Controller) getSessionInfo() *SessionInfo {
	c.sessionInfoLock.RLock()
	defer c.sessionInfoLock.RUnlock()
	return c.sessionInfo
}
