package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/robaston9/cbyge"
	"github.com/unixpickle/essentials"
)

const SessionExpiration = time.Hour / 2

type LightRequestParams struct {
	/* All properties are strings to fit in easily with the existing code for simplicity sake */
	Id         string `json:"id"`
	On         string
	Brightness string
	Color_tone string
	R          string
	G          string
	B          string
	Async      string
}

type HealthStatus struct {
	Status string
}

type MQTTSetState struct {
	State string `json:"state"`
	Brightness int `json:"brightness"`
	ColorTemp int `json:"color_temp"`
}

func main() {
	s := &Server{}

	var addr string
	var assets string
	flag.StringVar(&assets, "assets", "assets", "assets directory")
	flag.StringVar(&addr, "addr", ":8080", "address to listen on")
	flag.StringVar(&s.Email, "email", "", "C by GE account email")
	flag.StringVar(&s.Password, "password", "", "C by GE account password")
	flag.StringVar(&s.SessionInfo, "sessinfo", "", "Cync session info from 2FA login")
	flag.StringVar(&s.WebPassword, "web-password", "",
		"password for basic auth, if different than the account password")
	flag.BoolVar(&s.NoAuth, "no-auth", false, "do not require any password")
	flag.BoolVar(&s.UseMQTT, "use-mqtt", false, "integrate with mqtt")
	flag.StringVar(&s.MQTTAddress, "mqtt-address", "",
	  "MQTT broker address (hostname or ip address)")
	flag.IntVar(&s.MQTTPort, "mqtt-port", 1883, "MQTT broker port")
	flag.StringVar(&s.MQTTUsername, "mqtt-username", "", "MQTT broker username")
	flag.StringVar(&s.MQTTPassword, "mqtt-password", "", "MQTT broker password")
	flag.Parse()

	if s.SessionInfo == "" && (s.Email == "" || s.Password == "") {
		essentials.Die("Must provide -email and -password flags, or the -sessinfo flag. See -help.")
	}

	if s.WebPassword == "" {
		s.WebPassword = s.Password
	}

	if (s.UseMQTT && s.MQTTAddress == "") {
		essentials.Die("Must provide -mqtt-address when using mqtt")
	}

	if (s.UseMQTT) {
		s.SetupMQTT()
	}

	http.Handle("/", s.Auth(s.Redirect2FA(http.FileServer(http.Dir(assets)).ServeHTTP).ServeHTTP))
	http.Handle("/2fa/stage1", s.Auth(s.Handle2FAStage1))
	http.Handle("/2fa/stage2", s.Auth(s.Handle2FAStage2))
	http.Handle("/api/devices", s.Auth(s.HandleDevices))
	http.Handle("/api/device/status", s.Auth(s.HandleDeviceStatus))
	http.Handle("/api/device/set_on", s.Auth(s.HandleDeviceSetOn))
	http.Handle("/api/device/blast_on", s.Auth(s.HandleDeviceBlastOn))
	http.Handle("/api/device/set_color_tone", s.Auth(s.HandleDeviceSetColorTone))
	http.Handle("/api/device/set_rgb", s.Auth(s.HandleDeviceSetRGB))
	http.Handle("/api/device/set_brightness", s.Auth(s.HandleDeviceSetBrightness))
	http.Handle("/api/health", s.HandleHealth())
	http.ListenAndServe(addr, nil)
}

type Server struct {
	Email       string
	Password    string
	SessionInfo string

	WebPassword string
	NoAuth      bool

	UseMQTT      bool
	MQTTUsername string
	MQTTPassword string
	MQTTAddress  string
	MQTTPort     int

	devicesLock sync.Mutex
	devices     []*cbyge.ControllerDevice

	controllerLock sync.Mutex
	sessionInfo    *cbyge.SessionInfo
	controller     *cbyge.Controller

	mqttClient     mqtt.Client
}

func (s *Server) SetupMQTT() {
	var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("MQTT Message Received: %s from topic: %s\n", msg.Payload(), msg.Topic())
		slugs := strings.Split(msg.Topic(), "/")
		scope := slugs[1]
		if (scope == "gecync") {
			action := slugs[2]
			id := slugs[3]
			d, _ := s.getDevice(id)
			var cmd MQTTSetState
			json.Unmarshal([]byte(msg.Payload()), &cmd)
			switch  {
			case action == "set-state" && cmd.ColorTemp > 0:
				bottom := 153
				top := 500
				spread := top - bottom
				adjustedInput := cmd.ColorTemp - bottom
				reverse := (adjustedInput * 100) / spread
				tone := 100 - reverse
				s.controller.SetDeviceCT(d, tone)
				s.MQTTPublishColorTemp(id, tone)
			case action == "set-state" && cmd.Brightness > 0:
				s.controller.SetDeviceLum(d, cmd.Brightness)
				s.MQTTPublishBrightness(id, cmd.Brightness)
			case action == "set-state":
				s.controller.SetDeviceStatus(d, cmd.State == "ON")
				s.MQTTPublishStatus(id, cmd.State)
			}
		}
		if (scope == "status") {
			if (string(msg.Payload()) == "online") {
				s.MQTTPublishDeviceConfigAll()
			}
		}
	}

	var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
		fmt.Println("MQTT Connected")
	}

	var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
		fmt.Printf("MQTT Connect lost: %v", err)
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", s.MQTTAddress, s.MQTTPort))
	opts.SetClientID("cbyge")
	opts.SetUsername(s.MQTTUsername)
	opts.SetPassword(s.MQTTPassword)
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	s.mqttClient = mqtt.NewClient(opts)
	if token := s.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	topic1 := "homeassistant/gecync/#"
	token1 := s.mqttClient.Subscribe(topic1, 1, nil)
	token1.Wait()
	fmt.Printf("Subscribed to topic %s\n", topic1)

	topic2 := "homeassistant/status"
	token2 := s.mqttClient.Subscribe(topic2, 1, nil)
	token2.Wait()
	fmt.Printf("Subscribed to topic %s\n", topic2)

	if (s.SessionInfo != "") {
		fmt.Println("Configuring MQTT ...")
		s.MQTTPublishDeviceConfigAll()
	}
}

func (s *Server) HandleHealth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := new(HealthStatus)
		status.Status = "Healthy"

		if s.sessionInfo == nil {
			status.Status = "Server needs sign in"
		}

		s.serveObject(w, http.StatusOK, status)

		return
	})
}

func (s *Server) Auth(handler http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.NoAuth {
			handler(w, r)
			return
		}
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

func (s *Server) Redirect2FA(handler http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "" {
			handler(w, r)
			return
		}
		s.controllerLock.Lock()
		need2FA := s.SessionInfo == "" && s.sessionInfo == nil
		s.controllerLock.Unlock()
		if need2FA {
			http.Redirect(w, r, "/2fa.html", http.StatusTemporaryRedirect)
			return
		}
		handler(w, r)
	})
}

func (s *Server) Handle2FAStage1(w http.ResponseWriter, r *http.Request) {
	if err := cbyge.Login2FAStage1(s.Email, ""); err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
	} else {
		s.serveObject(w, 200, "ok")
	}
}

func (s *Server) Handle2FAStage2(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	if session, err := cbyge.Login2FAStage2(s.Email, s.Password, "", code); err != nil {
		http.Redirect(w, r, "/2fa.html?error="+url.QueryEscape(err.Error()), http.StatusTemporaryRedirect)
	} else {
		s.controllerLock.Lock()
		// log the sesion info
		data, _ := json.Marshal(session)
		fmt.Println(string(data))
		s.sessionInfo = session
		s.controllerLock.Unlock()
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}
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
	sort.Slice(devs, func(i, j int) bool {
		return strings.Compare(devs[i].DeviceID(), devs[j].DeviceID()) < 0
	})
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

		if (s.UseMQTT) {
			id := d.DeviceID()
			name := d.Name()
			s.MQTTPublishDeviceConfig(id, name)

			// Set the status through MQTT
			// Set the status through MQTT
			var status string
			if (statuses[i].IsOn) {
				status = "ON"
			} else {
				status = "OFF"
			}
			s.MQTTPublishStatus(id, status)
			s.MQTTPublishBrightness(id, int(statuses[i].Brightness))
			s.MQTTPublishColorTemp(id, int(statuses[i].ColorTone))
		}
	}
	s.serveObject(w, http.StatusOK, data)
}

func (s *Server) MQTTPublishDeviceConfigAll() {
	devs, _ := s.getDevices()
	ctrl, _ := s.getController()
	statuses, _ := ctrl.DeviceStatuses(devs)
	for i, d := range devs {
		id := d.DeviceID()
		name := d.Name()
		s.MQTTPublishDeviceConfig(id, name)

		// Set the status through MQTT
		// Set the status through MQTT
		var status string
		if (statuses[i].IsOn) {
			status = "ON"
		} else {
			status = "OFF"
		}
		s.MQTTPublishStatus(id, status)
		s.MQTTPublishBrightness(id, int(statuses[i].Brightness))
		s.MQTTPublishColorTemp(id, int(statuses[i].ColorTone))
	}
}

func (s *Server) MQTTPublishDeviceConfig(id string, name string) {
	topic := "homeassistant/light/gecync/" + id + "/config"

	var suppColorModes [1]string
	suppColorModes[0] = "color_temp"
	// suppColorModes[1] = "rgb"

	var identifiers [2]string
	identifiers[0] = id
	identifiers[1] = name

	device := map[string]interface{}{
		"identifiers": identifiers,
		"name": name,
		"model": "GE Cync Direct Connect Light Bulb",
		"manufacturer": "GE",
		"sw_version": "4.XX",
	}

	mqttPayload := map[string]interface{}{
		"name": name,
		"unique_id": id,
		"schema": "json",
		"state_topic": "homeassistant/gecync/state/" + id,
		"command_topic": "homeassistant/gecync/set-state/" + id,
		"brightness_state_topic": "homeassistant/gecync/brightness/" + id,
		"brightness_command_topic":
			"homeassistant/gecync/set-brightness/" + id,
		"color_temp_state_topic": "homeassistant/gecync/color-temp/" + id,
		"color_temp_command_topic": "homeassistant/gecync/set-color-temp/" + id,
		"brightness": true,
		"brightness_scale": 100,
		"color_mode": true,
		"supported_color_modes": suppColorModes,
		"device": device,
	};

	data, _ := json.Marshal(mqttPayload)
	token := s.mqttClient.Publish(topic, 0, false, data)
	token.Wait()
}

func (s *Server) MQTTPublishStatus(id string, status string) {
	state := map[string]interface{}{ "state": status }
	topic := "homeassistant/gecync/state/" + id
	data, _ := json.Marshal(state)
	token := s.mqttClient.Publish(topic, 0, false, data)
	token.Wait()
}

func (s *Server) MQTTPublishBrightness(id string, brightness int) {
	topic := "homeassistant/gecync/brightness/" + id
	data, _ := json.Marshal(brightness)
	token := s.mqttClient.Publish(topic, 0, false, data)
	token.Wait()
}

func (s *Server) MQTTPublishColorTemp(id string, colorTempPercent int) {
	topic := "homeassistant/gecync/color-temp/" + id
	top := 500
	bottom := 153
	colorTemp := (colorTempPercent * (top - bottom) / 100) + bottom
	data, _ := json.Marshal(colorTemp)
	token := s.mqttClient.Publish(topic, 0, false, data)
	token.Wait()
}

func (s *Server) HandleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	requestParams := s.extractRequestParams(r)
	s.GetDeviceStatus(w, r, requestParams)
}

func (s *Server) GetDeviceStatus(w http.ResponseWriter, r *http.Request, requestParams LightRequestParams) {
	ctrl, err := s.getController()
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}

	statuses := []map[string]interface{}{}
	for _, id := range strings.Split(requestParams.Id, ",") {
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
	requestParams := s.extractRequestParams(r)
	s.handleSetter(w, r, requestParams, func(c *cbyge.Controller, d *cbyge.ControllerDevice, async bool) error {
		if async {
			return c.SetDeviceStatusAsync(d, requestParams.On == "1")
		}
		return c.SetDeviceStatus(d, requestParams.On == "1")
	})
}

func (s *Server) HandleDeviceBlastOn(w http.ResponseWriter, r *http.Request) {
	ids := strings.Split(r.FormValue("id"), ",")
	status := r.FormValue("on") == "1"
	numSwitches := 3

	if r.FormValue("switches") != "" {
		n, err := strconv.Atoi(r.FormValue("switches"))
		if err != nil || n < 1 {
			s.serveError(w, http.StatusBadRequest, "invalid 'switches' argument")
			return
		}
		numSwitches = n
	}

	runFunc := func() error {
		ctrl, err := s.getController()
		if err != nil {
			return err
		}
		var devs []*cbyge.ControllerDevice
		var statuses []bool
		for _, id := range ids {
			dev, err := s.getDevice(id)
			if err != nil {
				return err
			}
			devs = append(devs, dev)
			statuses = append(statuses, status)
		}
		return ctrl.BlastDeviceStatuses(devs, statuses, numSwitches)
	}
	if r.FormValue("async") == "1" {
		go runFunc()
		s.serveObject(w, http.StatusOK, map[string]interface{}{})
	} else {
		err := runFunc()
		if err != nil {
			s.serveError(w, http.StatusInternalServerError, err.Error())
		} else {
			s.serveObject(w, http.StatusOK, map[string]interface{}{})
		}
	}
}

func (s *Server) HandleDeviceSetColorTone(w http.ResponseWriter, r *http.Request) {
	requestParams := s.extractRequestParams(r)
	tone, err := strconv.Atoi(requestParams.Color_tone)
	if err != nil {
		s.serveError(w, http.StatusBadRequest, err.Error())
		return
	} else if tone < 0 || tone > 100 {
		s.serveError(w, http.StatusBadRequest, "tone out of range [0, 100]")
		return
	}
	s.handleSetter(w, r, requestParams, func(c *cbyge.Controller, d *cbyge.ControllerDevice, async bool) error {
		if async {
			return c.SetDeviceCTAsync(d, tone)
		}
		return c.SetDeviceCT(d, tone)
	})
}

func (s *Server) HandleDeviceSetRGB(w http.ResponseWriter, r *http.Request) {
	requestParams := s.extractRequestParams(r)
	var values []uint8
	for _, k := range []string{requestParams.R, requestParams.G, requestParams.B} {
		value, err := strconv.Atoi(k)
		if err != nil {
			s.serveError(w, http.StatusBadRequest, "invalid rgb value:'"+k+"': "+err.Error())
			return
		} else if value < 0 || value > 0xff {
			s.serveError(w, http.StatusBadRequest, "invalid rgb value'"+k+"': out of range")
			return
		}
		values = append(values, uint8(value))
	}
	s.handleSetter(w, r, requestParams, func(c *cbyge.Controller, d *cbyge.ControllerDevice, async bool) error {
		if async {
			return c.SetDeviceRGBAsync(d, values[0], values[1], values[2])
		}
		return c.SetDeviceRGB(d, values[0], values[1], values[2])
	})
}

func (s *Server) HandleDeviceSetBrightness(w http.ResponseWriter, r *http.Request) {
	requestParams := s.extractRequestParams(r)
	lum, err := strconv.Atoi(requestParams.Brightness)
	if err != nil {
		s.serveError(w, http.StatusBadRequest, err.Error())
		return
	} else if lum < 1 || lum > 100 {
		s.serveError(w, http.StatusBadRequest, "brightness out of range [1, 100]")
		return
	}
	s.handleSetter(w, r, requestParams, func(c *cbyge.Controller, d *cbyge.ControllerDevice, async bool) error {
		if async {
			return c.SetDeviceLumAsync(d, lum)
		}
		return c.SetDeviceLum(d, lum)
	})
}

func (s *Server) extractRequestParams(r *http.Request) LightRequestParams {
	var requestParams LightRequestParams
	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&requestParams)

		if err != nil {
			println("error occured reading body")
		}
	} else {
		// requestParams = new(LightRequestParams)
		requestParams.Id = r.FormValue("id")
		requestParams.On = r.FormValue("on")
		requestParams.Brightness = r.FormValue("brightness")
		requestParams.Color_tone = r.FormValue("color_tone")
		requestParams.R = r.FormValue("r")
		requestParams.G = r.FormValue("g")
		requestParams.B = r.FormValue("b")
		requestParams.Async = r.FormValue("async")
	}

	return requestParams
}

func (s *Server) handleSetter(w http.ResponseWriter, r *http.Request, requestParams LightRequestParams,
	f func(c *cbyge.Controller, d *cbyge.ControllerDevice, async bool) error) {

	if requestParams.Async == "1" {
		ids := strings.Split(requestParams.Id, ",")
		go func() {
			ctrl, err := s.getController()
			if err != nil {
				return
			}
			for _, id := range ids {
				// Ignore errors; apply the change to as many
				// devices as possible in async mode.
				dev, err := s.getDevice(id)
				if err == nil {
					f(ctrl, dev, true)
				}
			}
		}()
		s.serveObject(w, http.StatusOK, []interface{}{})
		return
	}

	ctrl, err := s.getController()
	if err != nil {
		s.serveError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, id := range strings.Split(requestParams.Id, ",") {
		dev, err := s.getDevice(id)
		if err != nil {
			s.serveError(w, http.StatusInternalServerError, err.Error())
			return
		}
		err = f(ctrl, dev, false)
		if err != nil {
			s.serveError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Return the new device statuses.
	s.GetDeviceStatus(w, r, requestParams)
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

	if s.controller != nil {
		return s.controller, nil
	}

	if s.sessionInfo == nil {
		if s.SessionInfo == "" {
			return nil, errors.New("user must authenticate with 2FA first")
		} else {
			err := json.Unmarshal([]byte(s.SessionInfo), &s.sessionInfo)
			if err != nil {
				fmt.Fprintf(os.Stderr, "The session info JSON, passed via -sessinfo, is invalid. "+
					"Encountered parse error: "+err.Error()+". The offending data is: %#v\n", s.SessionInfo)
				return nil, errors.New("invalid -sessinfo argument")
			}
		}
	}

	s.controller = cbyge.NewController(s.sessionInfo, 0)
	return s.controller, nil
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
