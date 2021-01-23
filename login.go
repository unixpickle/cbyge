package cbyge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// DefaultCorpID is the corporation ID used by the C by GE app.
const DefaultCorpID = "1007d2ad150c4000"

const (
	authURL           = "https://api-ge.xlink.cn/v2/user_auth"
	userInfoURL       = "https://api2.xlink.cn/v2/user/%d"
	devicesURL        = "https://api2.xlink.cn/v2/user/%d/subscribe/devices"
	devicePropertyURL = "https://api2.xlink.cn/v2/product/%s/device/%d/property"
)

type SessionInfo struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int64  `json:"user_id"`
	ExpireIn     int    `json:"expire_in"`
	Authorize    string `json:"authorize"`
}

type UserInfo struct {
	Gender          int        `json:"gender"`
	ActiveDate      *time.Time `json:"active_date"`
	Source          int        `json:"source"`
	PasswordInited  bool       `json:"passwd_inited"`
	IsValid         bool       `json:"is_valid"`
	Nickname        string     `json:"nickname"`
	ID              int        `json:"id"`
	CreateDate      *time.Time `json:"create_date"`
	Email           string     `json:"email"`
	RegionID        int        `json:"region_id"`
	AuthorizeCode   string     `json:"authorize_code"`
	CertificateNo   string     `json:"certificate_no"`
	CertificateType int        `json:"certificate_type"`
	CorpID          string     `json:"corp_id"`
	PrivacyCode     string     `json:"privacy_code"`
	Account         string     `json:"account"`
	Age             int        `json:"age"`
	Status          int        `json:"status"`
}

type DeviceInfo struct {
	AccessKey       int64     `json:"access_key"`
	ActiveCode      string    `json:"active_code"`
	ActiveDate      time.Time `json:"active_date"`
	AuthorizeCode   string    `json:"authorize_code"`
	FirmwareVersion int       `json:"firmware_version"`
	Groups          string    `json:"groups"`
	ID              uint32    `json:"id"`
	IsActive        bool      `json:"is_active"`
	IsOnline        bool      `json:"is_online"`
	LastLogin       string    `json:"last_login"`
	MAC             string    `json:"mac"`
	MCUVersion      int       `json:"mcu_version"`
	Name            string    `json:"name"`
	ProductID       string    `json:"product_id"`
	Role            int       `json:"role"`
	Source          int       `json:"source"`
	SubscribeDate   string    `json:"subscribe_date"`
}

type DeviceProperties struct {
	Bulbs []struct {
		DeviceID    string `json:"deviceID"`
		DisplayName string `json:"displayName"`
		SwitchID    uint32 `json:"switchID"`
	} `json:"bulbsArray"`
}

type devicePropertiesResponse struct {
	DeviceProperties
	Error *RemoteError `json:"error"`
}

type RemoteError struct {
	Msg     string `json:"msg"`
	Code    int    `json:"code"`
	Context string
}

func (l *RemoteError) Error() string {
	return l.Context + ": " + l.Msg
}

type loginResponse struct {
	SessionInfo
	Error *RemoteError `json:"error"`
}

// Login authenticates with the server to create a new session.
//
// If the login fails because of incorrect credentials, then the error is of
// type *RemoteError.
//
// If corpID is "", then DefaultCorpID is used.
func Login(email, password, corpID string) (*SessionInfo, error) {
	if corpID == "" {
		corpID = DefaultCorpID
	}
	jsonObj := map[string]string{"email": email, "password": password, "corp_id": corpID}
	data, _ := json.Marshal(jsonObj)
	resp, err := http.Post(authURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, errors.Wrap(err, "login")
	}
	defer resp.Body.Close()
	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "login")
	}
	response := loginResponse{}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, errors.Wrap(err, "login")
	}
	if response.Error != nil {
		response.Error.Context = "login"
		return nil, response.Error
	}
	return &response.SessionInfo, nil
}

// GetUserInfo gets UserInfo using information from Login.
func GetUserInfo(userID int64, accessToken string) (*UserInfo, error) {
	urlStr := fmt.Sprintf(userInfoURL, userID)
	var response UserInfo
	if err := makeAPICall(urlStr, accessToken, &response); err != nil {
		return nil, errors.Wrap(err, "get user info")
	}
	return &response, nil
}

// GetDevices gets the devices using information from Login.
func GetDevices(userID int64, accessToken string) ([]*DeviceInfo, error) {
	urlStr := fmt.Sprintf(devicesURL, userID)
	var response []*DeviceInfo
	if err := makeAPICall(urlStr, accessToken, &response); err != nil {
		return nil, errors.Wrap(err, "get devices")
	}
	return response, nil
}

// GetDeviceProperties gets extended device information.
//
// If the request fails because of an error returned by the server, then the
// returned error is of type *RemoteError.
func GetDeviceProperties(accessToken, productID string, deviceID uint32) (*DeviceProperties, error) {
	urlStr := fmt.Sprintf(devicePropertyURL, productID, deviceID)
	var response devicePropertiesResponse
	if err := makeAPICall(urlStr, accessToken, &response); err != nil {
		return nil, errors.Wrap(err, "get device properties")
	}
	if response.Error != nil {
		response.Error.Context = "get device properties"
		return nil, response.Error
	}
	return &response.DeviceProperties, nil
}

func makeAPICall(urlStr, accessToken string, response interface{}) error {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Access-Token", accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, response); err != nil {
		return err
	}
	return nil
}
