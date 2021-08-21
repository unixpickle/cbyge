package cbyge

import (
	"encoding/json"

	"github.com/pkg/errors"
)

const (
	RemoteErrorCodeAccessTokenRefresh = 4031022
	RemoteErrorCodePasswordError      = 4001007
	RemoteErrorCodeUserNotExists      = 4041011
	RemoteErrorCodePropertyNotExists  = 4041009
)

// A RemoteCallError is triggered when the packet server returns an
// unspecified error.
var RemoteCallError = errors.New("the server returned with an error")

// An UnreachableError is triggered when a device cannot be reached
// through any wifi-connected switch.
var UnreachableError = errors.New("the device cannot be reached")

// A RemoteError is an error message returned by the HTTPS API server.
type RemoteError struct {
	Msg     string `json:"msg"`
	Code    int    `json:"code"`
	Context string
}

func decodeRemoteError(data []byte, context string) *RemoteError {
	var res struct {
		Error *RemoteError `json:"error"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil
	}
	if res.Error != nil {
		res.Error.Context = context
	}
	return res.Error
}

func (l *RemoteError) Error() string {
	if l.Context == "" {
		return l.Msg
	}
	return l.Context + ": " + l.Msg
}

// IsAccessTokenError returns true if the error is an API error that can be
// solved by refreshing the access token.
func IsAccessTokenError(err error) bool {
	return isErrorWithCode(err, RemoteErrorCodeAccessTokenRefresh)
}

// IsCredentialsError returns true if the error was the result of a bad
// username or password.
func IsCredentialsError(err error) bool {
	return isErrorWithCode(err, RemoteErrorCodePasswordError, RemoteErrorCodeUserNotExists)
}

// IsPropertyNotExistsError returns true if an error was the result of looking
// up properties for a device without properties.
func IsPropertyNotExistsError(err error) bool {
	return isErrorWithCode(err, RemoteErrorCodePropertyNotExists)
}

func isErrorWithCode(err error, codes ...int) bool {
	var re *RemoteError
	if !errors.As(err, &re) {
		return false
	}
	for _, code := range codes {
		if re.Code == code {
			return true
		}
	}
	return false
}
