package cbyge

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// DefaultCorpID is the corporation ID used by the C by GE app.
const DefaultCorpID = "1007d2ad150c4000"

const authURL = "https://api-ge.xlink.cn/v2/user_auth"

type SessionInfo struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
	ExpireIn     int    `json:"expire_in"`
	Authorize    string `json:"authorize"`
}

type UserInfo struct {
	Gender          int       `json:"gender"`
	ActiveDate      time.Time `json:"active_date"`
	Source          int       `json:"source"`
	PasswordInited  bool      `json:"passwd_inited"`
	IsValid         bool      `json:"is_valid"`
	Nickname        string    `json:"nickname"`
	ID              int       `json:"id"`
	CreateDate      time.Time `json:"create_date"`
	Email           string    `json:"email"`
	RegionID        int       `json:"region_id"`
	AuthorizeCode   string    `json:"authorize_code"`
	CertificateNo   string    `json:"certificate_no"`
	CertificateType int       `json:"certificate_type"`
	CorpID          string    `json:"corp_id"`
	PrivacyCode     string    `json:"privacy_code"`
	Account         string    `json:"account"`
	Age             int       `json:"age"`
	Status          int       `json:"status"`
}

type LoginError struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
}

func (l *LoginError) Error() string {
	return "login: " + l.Msg
}

type loginResponse struct {
	SessionInfo
	Error *LoginError `json:"error"`
}

// Login authenticates with the server to create a new session.
//
// If the login fails because of incorrect credentials, then the error is of
// type *LoginError.
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
		return nil, response.Error
	}
	return &response.SessionInfo, nil
}
