package entity

import (
	"strings"

	domainerr "github.com/QueerGlobal/hub-framework/core/entity/error"
)

// HTTPMethod is a string type representing HTTP methods.
type HTTPMethod string

// Enum values for HTTPMethod
const (
	HTTPMethodGET     HTTPMethod = "GET"
	HTTPMethodPOST    HTTPMethod = "POST"
	HTTPMethodPUT     HTTPMethod = "PUT"
	HTTPMethodDELETE  HTTPMethod = "DELETE"
	HTTPMethodHEAD    HTTPMethod = "HEAD"
	HTTPMethodOPTIONS HTTPMethod = "OPTIONS"
	HTTPMethodPATCH   HTTPMethod = "PATCH"
	HTTPMethodTRACE   HTTPMethod = "TRACE"
	HTTPMethodCONNECT HTTPMethod = "CONNECT"
)

func StringToHTTPMethod(method string) (HTTPMethod, error) {
	switch HTTPMethod(strings.ToUpper(method)) {
	case HTTPMethodGET, HTTPMethodPOST, HTTPMethodPUT, HTTPMethodDELETE, HTTPMethodHEAD, HTTPMethodOPTIONS, HTTPMethodPATCH, HTTPMethodTRACE, HTTPMethodCONNECT:
		return HTTPMethod(strings.ToUpper(method)), nil
	default:
		return "", domainerr.ErrUnsupportedHTTPMethod
	}
}
