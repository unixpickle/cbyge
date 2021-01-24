package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/unixpickle/cbyge"
	"github.com/unixpickle/essentials"
)

const SessionExpiration = time.Hour / 2

func main() {
	s := &Server{}
	var addr string
	flag.StringVar(&addr, "addr", ":8080", "address to listen on")
	flag.StringVar(&s.Email, "email", "", "C by GE account email")
	flag.StringVar(&s.Password, "password", "", "C by GE account password")
	flag.StringVar(&s.WebPassword, "web-password", "",
		"password for basic auth, if different than the account password")
	flag.Parse()

	if s.Email == "" || s.Password == "" {
		essentials.Die("Must provide -email and -password flags. See -help.")
	}

	if s.WebPassword == "" {
		s.WebPassword = s.Password
	}

	http.Handle("/api/devices", s.Auth(s.HandleDevices))
	http.Handle("/api/device/status", s.Auth(s.HandleDeviceStatus))
	http.Handle("/api/device/set_on", s.Auth(s.HandleDeviceSetOn))
	http.Handle("/api/device/set_color_tone", s.Auth(s.HandleDeviceSetColorTone))
	http.Handle("/api/device/set_brightness", s.Auth(s.HandleDeviceSetBrightness))
	http.ListenAndServe(addr, nil)
}

type Server struct {
	Email       string
	Password    string
	WebPassword string

	devicesLock sync.Mutex
	devices     []*cbyge.ControllerDevice

	controllerLock   sync.Mutex
	controllerExpire time.Time
	controller       *cbyge.Controller
}

func (s *Server) Auth(handler http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pass := r.FormValue("auth")
		if subtle.ConstantTimeCompare([]byte(pass), []byte(s.WebPassword)) == 1 {
			s.serveError(w, http.StatusForbidden, "incorrect 'auth' parameter")
		} else {
			handler(w, r)
		}
	})
}

func (s *Server) HandleDevices(w http.ResponseWriter, r *http.Request) {
	var devs []*cbyge.ControllerDevice
	var err error
	if r.FormValue("refresh") != "" {
		devs, err = s.refreshDevices()
	} else {
		devs, err = s.getDevices()
	}
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}
	data := []map[string]interface{}{}
	for _, d := range devs {
		status := d.LastStatus()
		data = append(data, map[string]interface{}{
			"id":   d.DeviceID(),
			"name": d.Name(),
			"status": map[string]interface{}{
				"is_online":  status.IsOnline,
				"is_on":      status.IsOn,
				"brightness": status.Brightness,
				"color_tone": status.ColorTone,
			},
		})
	}
	s.serveObject(w, http.StatusOK, data)
}

func (s *Server) HandleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	dev, err := s.getDevice(r.FormValue("id"))
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ctrl, err := s.getController()
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status, err := ctrl.DeviceStatus(dev)
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.serveObject(w, http.StatusOK, map[string]interface{}{
		"is_online":  status.IsOnline,
		"is_on":      status.IsOn,
		"brightness": status.Brightness,
		"color_tone": status.ColorTone,
	})
}

func (s *Server) HandleDeviceSetOn(w http.ResponseWriter, r *http.Request) {
	s.handleSetter(w, r, func(c *cbyge.Controller, d *cbyge.ControllerDevice) error {
		return c.SetDeviceStatus(d, r.FormValue("on") == "1")
	})
}

func (s *Server) HandleDeviceSetColorTone(w http.ResponseWriter, r *http.Request) {
	tone, err := strconv.Atoi(r.FormValue("color_tone"))
	if err != nil {
		s.serveError(w, http.StatusBadRequest, err.Error())
		return
	} else if tone < 0 || tone > 100 {
		s.serveError(w, http.StatusBadRequest, "tone out of range [0, 100]")
		return
	}
	s.handleSetter(w, r, func(c *cbyge.Controller, d *cbyge.ControllerDevice) error {
		return c.SetDeviceCT(d, tone)
	})
}

func (s *Server) HandleDeviceSetBrightness(w http.ResponseWriter, r *http.Request) {
	lum, err := strconv.Atoi(r.FormValue("brightness"))
	if err != nil {
		s.serveError(w, http.StatusBadRequest, err.Error())
		return
	} else if lum < 1 || lum > 100 {
		s.serveError(w, http.StatusBadRequest, "brightness out of range [1, 100]")
		return
	}
	s.handleSetter(w, r, func(c *cbyge.Controller, d *cbyge.ControllerDevice) error {
		return c.SetDeviceLum(d, lum)
	})
}

func (s *Server) handleSetter(w http.ResponseWriter, r *http.Request,
	f func(c *cbyge.Controller, d *cbyge.ControllerDevice) error) {
	dev, err := s.getDevice(r.FormValue("id"))
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ctrl, err := s.getController()
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}
	err = f(ctrl, dev)
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Return the new device status.
	s.HandleDeviceStatus(w, r)
}

func (s *Server) serveError(w http.ResponseWriter, code int, err string) {
	obj := map[string]string{"error": err}
	s.serveObject(w, code, obj)
}

func (s *Server) serveObject(w http.ResponseWriter, code int, obj interface{}) {
	w.Header().Set("content-type", "application/json")
	data, err := json.Marshal(obj)
	if err != nil {
		obj = map[string]string{"error": err.Error()}
		code = http.StatusInternalServerError
	}
	w.WriteHeader(code)
	w.Write(data)
}

func (s *Server) getDevice(id string) (*cbyge.ControllerDevice, error) {
	devs, err := s.getDevices()
	if err != nil {
		return nil, err
	}
	for _, d := range devs {
		if d.DeviceID() == id {
			return d, nil
		}
	}
	return nil, errors.New("no device found with the given ID")
}

func (s *Server) getDevices() ([]*cbyge.ControllerDevice, error) {
	s.devicesLock.Lock()
	devs := s.devices
	s.devicesLock.Unlock()
	if devs != nil {
		return devs, nil
	}
	return s.refreshDevices()
}

func (s *Server) refreshDevices() ([]*cbyge.ControllerDevice, error) {
	ctrl, err := s.getController()
	if err != nil {
		return nil, err
	}
	devs, err := ctrl.Devices()
	if err != nil {
		return nil, err
	}
	s.devicesLock.Lock()
	s.devices = devs
	s.devicesLock.Unlock()
	return devs, nil
}

func (s *Server) getController() (*cbyge.Controller, error) {
	s.controllerLock.Lock()
	defer s.controllerLock.Unlock()

	if s.controller != nil && time.Now().Before(s.controllerExpire) {
		return s.controller, nil
	}
	var err error
	s.controller, err = cbyge.NewControllerLogin(s.Email, s.Password)
	if err == nil {
		s.controllerExpire = time.Now().Add(SessionExpiration)
	}
	return s.controller, err
}
