package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/unixpickle/cbyge"
	"github.com/unixpickle/essentials"
)

const SessionExpiration = time.Hour / 2

func main() {
	s := &Server{}
	var addr string
	var assets string
	flag.StringVar(&assets, "assets", "assets", "assets directory")
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

	http.Handle("/", s.Auth(http.FileServer(http.Dir(assets)).ServeHTTP))
	http.Handle("/api/devices", s.Auth(s.HandleDevices))
	http.Handle("/api/device/status", s.Auth(s.HandleDeviceStatus))
	http.Handle("/api/device/set_on", s.Auth(s.HandleDeviceSetOn))
	http.Handle("/api/device/set_color_tone", s.Auth(s.HandleDeviceSetColorTone))
	http.Handle("/api/device/set_rgb", s.Auth(s.HandleDeviceSetRGB))
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

		if pass == "" {
			// Most likely a front-end request.
			_, pass, ok := r.BasicAuth()
			if !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(s.WebPassword)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="bad credentials"`)
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Unauthorised.\n"))
				return
			}
		} else if subtle.ConstantTimeCompare([]byte(pass), []byte(s.WebPassword)) != 1 {
			s.serveError(w, http.StatusUnauthorized, "incorrect 'auth' parameter")
			return
		}
		handler(w, r)
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
	statuses := make([]cbyge.ControllerDeviceStatus, len(devs))
	for i, d := range devs {
		statuses[i] = d.LastStatus()
	}
	if r.FormValue("update_status") != "" {
		ctrl, err := s.getController()
		if err != nil {
			s.serveError(w, http.StatusInternalServerError, err.Error())
			return
		}
		statuses, _ = ctrl.DeviceStatuses(devs)
	}
	data := []map[string]interface{}{}
	for i, d := range devs {
		data = append(data, map[string]interface{}{
			"id":     d.DeviceID(),
			"name":   d.Name(),
			"status": encodeStatus(statuses[i]),
		})
	}
	s.serveObject(w, http.StatusOK, data)
}

func (s *Server) HandleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	ctrl, err := s.getController()
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}

	statuses := []map[string]interface{}{}
	for _, id := range strings.Split(r.FormValue("id"), ",") {
		dev, err := s.getDevice(id)
		if err != nil {
			s.serveError(w, http.StatusInternalServerError, err.Error())
			return
		}
		status, err := ctrl.DeviceStatus(dev)
		if err != nil {
			s.serveError(w, http.StatusInternalServerError, err.Error())
			return
		}
		statuses = append(statuses, encodeStatus(status))
	}

	s.serveObject(w, http.StatusOK, statuses)
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

func (s *Server) HandleDeviceSetRGB(w http.ResponseWriter, r *http.Request) {
	var values []uint8
	for _, k := range []string{"r", "g", "b"} {
		value, err := strconv.Atoi(r.FormValue(k))
		if err != nil {
			s.serveError(w, http.StatusBadRequest, "invalid '"+k+"': "+err.Error())
			return
		} else if value < 0 || value > 0xff {
			s.serveError(w, http.StatusBadRequest, "invalid '"+k+"': out of range")
			return
		}
		values = append(values, uint8(value))
	}
	s.handleSetter(w, r, func(c *cbyge.Controller, d *cbyge.ControllerDevice) error {
		return c.SetDeviceRGB(d, values[0], values[1], values[2])
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
	ctrl, err := s.getController()
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, id := range strings.Split(r.FormValue("id"), ",") {
		dev, err := s.getDevice(id)
		if err != nil {
			s.serveError(w, http.StatusInternalServerError, err.Error())
			return
		}
		err = f(ctrl, dev)
		if err != nil {
			s.serveError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Return the new device statuses.
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
		if !cbyge.IsAccessTokenError(err) {
			return nil, err
		}
		err = s.refreshSession()
		if err != nil {
			return nil, err
		}
		devs, err = ctrl.Devices()
		if err != nil {
			return nil, err
		}
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
	if s.controller != nil {
		err = s.controller.Login(s.Email, s.Password)
	} else {
		s.controller, err = cbyge.NewControllerLogin(s.Email, s.Password)
	}
	if err == nil {
		s.controllerExpire = time.Now().Add(SessionExpiration)
	}
	return s.controller, err
}

func (s *Server) refreshSession() error {
	s.controllerLock.Lock()
	defer s.controllerLock.Unlock()
	err := s.controller.Login(s.Email, s.Password)
	if err == nil {
		s.controllerExpire = time.Now().Add(SessionExpiration)
	}
	return err
}

func encodeStatus(s cbyge.ControllerDeviceStatus) map[string]interface{} {
	return map[string]interface{}{
		"is_online":  s.IsOnline,
		"is_on":      s.IsOn,
		"brightness": s.Brightness,
		"color_tone": s.ColorTone,
		"use_rgb":    s.UseRGB,
		"rgb":        s.RGB,
	}
}
