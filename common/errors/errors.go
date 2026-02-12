package errors

import (
	"fmt"
	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/grpc/status"
	"net/http"
	"strings"
	"time"

	"encoding/json"
)

type Error struct {
	Id     string `json:"id"`
	Code   int32  `json:"code"`
	Source string `json:"source"`
	Detail string `json:"detail"`
	Status string `json:"status"`
}

func (e *Error) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// New generates a custom error.
func New(id, source, detail string, code int32) error {
	return &Error{
		Id:     id,
		Code:   code,
		Source: source,
		Detail: detail,
		Status: http.StatusText(int(code)),
	}
}

// FromError try to convert go error to *Error
func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	if verr, ok := err.(*Error); ok && verr != nil {
		return verr
	}
	if serr, ok := status.FromError(err); ok {
		return Parse(serr.Message())
	}

	return Parse(err.Error())
}

// Parse tries to parse a JSON string into an error. If that
// fails, it will set the given string as the error detail.
func Parse(err string) *Error {
	e := new(Error)
	errr := json.Unmarshal([]byte(err), e)
	if errr != nil {
		e.Detail = err
	}
	return e
}

// BadRequest generates a 400 error.
func BadRequest(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   400,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(400),
	}
}

// Unauthorized generates a 401 error.
func Unauthorized(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   401,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(401),
	}
}

// Forbidden generates a 403 error.
func Forbidden(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   403,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(403),
	}
}

// NotFound generates a 404 error.
func NotFound(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   404,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(404),
	}
}

// MethodNotAllowed generates a 405 error.
func MethodNotAllowed(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   405,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(405),
	}
}

// Timeout generates a 408 error.
func Timeout(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   408,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(408),
	}
}

// Conflict generates a 409 error.
func Conflict(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   409,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(409),
	}
}

// InternalServerError generates a 500 error.
func InternalServerError(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   500,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(500),
	}
}

// NotImplemented generates a 501 error
func NotImplemented(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   501,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(501),
	}
}

// BadGateway generates a 502 error
func BadGateway(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   502,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(502),
	}
}

// ServiceUnavailable generates a 503 error
func ServiceUnavailable(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   503,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(503),
	}
}

// GatewayTimeout generates a 504 error
func GatewayTimeout(id, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   504,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(504),
	}
}

// Equal tries to compare errors
func Equal(err1 error, err2 error) bool {
	verr1, ok1 := err1.(*Error)
	verr2, ok2 := err2.(*Error)

	if ok1 != ok2 {
		return false
	}

	if !ok1 {
		return err1 == err2
	}

	if verr1.Code != verr2.Code {
		return false
	}

	return true
}

// IsNetworkError tries to detect if error is a network error.
func IsNetworkError(err error) bool {
	s := err.Error()
	parsed := FromError(err)
	return strings.Contains(s, "context deadline exceeded") ||
		strings.Contains(s, "unexpected EOF") ||
		strings.Contains(s, "context canceled") ||
		strings.Contains(s, "can't assign requested address") ||
		strings.Contains(s, "SubConns are in TransientFailure") ||
		parsed.Id == "go.micro.client" && parsed.Code == 500 && parsed.Detail == "not found"

}

// IsContextCanceled interprets error as a "context canceled"
func IsContextCanceled(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "context canceled")
}

// ErrorCode for app
type ErrorCode int

// ErrorDetail for app
type ErrorDetail struct {
	ID     string
	Detail string
	Code   int32
}

const (
	// EC1 represents there is an error1
	EC1 ErrorCode = iota
	// EC2 represents there is an error2
	EC2
	// EC3 represents there is an error3
	EC3
	// EC4 represents there is an error4
	EC4
	// SME SendMailError
	SME
	// DBE DatabaseError
	DBE
	// PSE PubSubError
	PSE
)

var appErrors = map[ErrorCode]ErrorDetail{
	EC1: {"EC1", "not good", 500},
	EC2: {"EC2", "not valid", 500},
	EC3: {"EC3", "not valid", 500},
	EC4: {"EC4", "not valid", 500},
	SME: {"SME", "unable to send email: %v", 500},
	DBE: {"DBE", "database error: %v", 500},
	PSE: {"PSE", "broker publish error: %v", 500},
}

// TODO: Should I use https://github.com/grpc/grpc/blob/master/doc/http-grpc-status-mapping.md
// https://github.com/avinassh/grpc-errors/blob/master/go/server.go
// http://avi.im/grpc-errors/

// AppError - App specific Error
func AppError(errorCode ErrorCode, a ...interface{}) error {
	return &Error{
		Id:     appErrors[errorCode].ID,
		Code:   appErrors[errorCode].Code,
		Detail: fmt.Sprintf(appErrors[errorCode].Detail, a...),
		Status: http.StatusText(500),
	}
}

// ValidationError - Unprocessable Entity
func ValidationError(id, source, format string, a ...interface{}) error {
	return &Error{
		Id:     id,
		Code:   422,
		Source: source,
		Detail: fmt.Sprintf(format, a...),
		Status: http.StatusText(422),
	}
}

// SrvError defines the string type relating to all the global errors
type SrvError string

const (
	srvNoStartTxt               SrvError = "Unable to start %s server. Error: %v \n"
	srvNoHandlerTxt             SrvError = "Unable to register service handler. Error: %v"
	dbNoConnectionTxt           SrvError = "Unable to connect to DB %s. Error: %v\n"
	dbNoConnectionStringTxt     SrvError = "Unable to find DB connection string. Please set environment variable %s \n"
	dbConnectRetry              SrvError = "Attempting to connect to DB again. Retry number: %d. Previous error: %v\n"
	dtProtoTimeStampToTimeStamp SrvError = "Unable to convert proto timestamp %v to timestamp. Error: %v \n"
	dtTimeStampToProtoTimeStamp SrvError = "Unable to convert timestamp %v to proto timestamp. Error: %v \n"
	dtInvalidValidityDates      SrvError = "The valid thru date (%v) must take place after the valid from date (%v)\n"
	missingField                SrvError = "%s must not be empty\n"
	authNoMetaData              SrvError = "Unable to read meta-date for end point: %s\n"
	authInvalidToken            SrvError = "invalid token\n"
	authNilToken                SrvError = "Invalid nil user token\n"
	authNilClaim                SrvError = "invalid nil %s claim\n"
	authInvalidClaim            SrvError = "Invalid %s claim\n"
	authNoUserInToken           SrvError = "unable to get logged in user from metadata. Error: %v\n"
	brkBadMarshall              SrvError = "Unable to marshall object %v. Error: %v\n"
	brkNoMessageSent            SrvError = "Unable to send message to broker for topic %s. Error: %v\n"
	brkNoConnection             SrvError = "Unable to connect to broker: Error: %v\n"
	brkUnableToSetSubs          SrvError = "Unable to setup broker subscription for topic %s. Error: %v\n"
	brkBadUnMarshall            SrvError = "Unable to unmarshall received object for topic %s. Message received: %v. Error: %v\n"
	audFailureSending           SrvError = "Unable to send %s audit information for record %s. Error: %v\n"
)

const (
	marshalFullMap          SrvError = "Unable to marshal full map. Error: %v\n"
	unMarshalByteFullMap    SrvError = "Unable to unmarshal byte full version map to struct. Error: %v\n"
	marshalPartialMap       SrvError = "Unable to marshal partial  map. Error: %v\n"
	unMarshalBytePartialMap SrvError = "Unable to unmarshal byte partial version of map to proto struct. Error: %v\n"
)

const (
	cacheUnableToWrite       SrvError = "Unable to write to cache with key %s. Error: %v\n"
	cacheDBNameNotSet        SrvError = "Cache Database Name is not set. Please provide a value\n"
	cacheEnvVarAddressNotSet SrvError = "Cache Address not found. PLease provide a MICRO_STORE_ADDRESS environment variable with the address"
	cacheUnableToReadVal     SrvError = "Unable to read key %v. Error: %v\n"
	cacheUnableToDeleteVal   SrvError = "Unable to delete key %v from cache. Error %v\n"
	cacheTooManyValuesToList SrvError = "Requested too many keys to list from cache. Max number is %d\n"
	cacheListError           SrvError = "Unable to list cache Keys .Error %v\n"
)

/*
Functions that return the error message formatted with the information passed in as arguments to the individual functions
*/

// SrvNoStart returns relevant error based on the provided parameters
func (ge *SrvError) SrvNoStart(serviceName string, err error) string {
	return fmt.Sprintf(string(srvNoStartTxt), serviceName, err)
}

// DbNoConnection returns relevant error based on the provided parameters
func (ge *SrvError) DbNoConnection(dbName string, err error) string {
	return fmt.Sprintf(string(dbNoConnectionTxt), dbName, err)
}

// DbNoConnectionString returns relevant error based on the provided parameters
func (ge *SrvError) DbNoConnectionString(envVarName string) string {
	return fmt.Sprintf(string(dbNoConnectionStringTxt), envVarName)
}

// DbConnectRetry returns relevant error based on the provided parameters
func (ge *SrvError) DbConnectRetry(RetryNum int, err error) string {
	return fmt.Sprintf(string(dbConnectRetry), RetryNum, err)
}

// SrvNoHandler returns relevant error based on the provided parameters
func (ge *SrvError) SrvNoHandler(err error) string {
	return fmt.Sprintf(string(srvNoHandlerTxt), err)
}

// DtProtoTimeStampToTimeStamp returns relevant error based on the provided parameters
func (ge *SrvError) DtProtoTimeStampToTimeStamp(currTimeStamp *timestamp.Timestamp, err error) string {
	return fmt.Sprintf(string(dtProtoTimeStampToTimeStamp), currTimeStamp, err)
}

// DtTimeStampToProtoTimeStamp returns relevant error based on the provided parameters
func (ge *SrvError) DtTimeStampToProtoTimeStamp(currentTime time.Time, err error) string {
	return fmt.Sprintf(string(dtTimeStampToProtoTimeStamp), currentTime, err)
}

// MissingField returns relevant error based on the provided parameters
func (ge *SrvError) MissingField(fieldName string) string {
	return fmt.Sprintf(string(missingField), fieldName)
}

// DtInvalidValidityDates returns relevant error based on the provided parameters
func (ge *SrvError) DtInvalidValidityDates(validFrom, validThru time.Time) string {
	return fmt.Sprintf(string(dtInvalidValidityDates), validFrom, validThru)
}

// AuthNoMetaData returns relevant error based on the provided parameters
func (ge *SrvError) AuthNoMetaData(endpoint string) string {
	return fmt.Sprintf(string(authNoMetaData), endpoint)
}

// AuthInvalidToken returns relevant error based on the provided parameters
func (ge *SrvError) AuthInvalidToken() string {
	return fmt.Sprintf(string(authInvalidToken))
}

// AuthNilToken returns relevant error based on the provided parameters
func (ge *SrvError) AuthNilToken() string {
	return fmt.Sprintf(string(authNilToken))
}

// AuthNilClaim returns relevant error based on the provided parameters
func (ge *SrvError) AuthNilClaim(claimType string) string {
	return fmt.Sprintf(string(authNilClaim), claimType)
}

// AuthInvalidClaim returns relevant error based on the provided parameters
func (ge *SrvError) AuthInvalidClaim(claimType string) string {
	return fmt.Sprintf(string(authInvalidClaim), claimType)
}

// BrkBadMarshall returns relevant error based on the provided parameters
func (ge *SrvError) BrkBadMarshall(objToMarshal string, err error) string {
	return fmt.Sprintf(string(brkBadMarshall), objToMarshal, err)
}

// BrkNoMessageSent returns relevant error based on the provided parameters
func (ge *SrvError) BrkNoMessageSent(objToMarshal string, err error) string {
	return fmt.Sprintf(string(brkNoMessageSent), objToMarshal, err)
}

// BrkNoConnection returns relevant error based on the provided parameters
func (ge *SrvError) BrkNoConnection(err error) string {
	return fmt.Sprintf(string(brkNoConnection), err)
}

// BrkUnableToSetSubs returns relevant error based on the provided parameters
func (ge *SrvError) BrkUnableToSetSubs(topic string, err error) string {
	return fmt.Sprintf(string(brkUnableToSetSubs), topic, err)
}

// BrkBadUnMarshall returns relevant error based on the provided parameters
func (ge *SrvError) BrkBadUnMarshall(topic string, message []byte, err error) string {
	return fmt.Sprintf(string(brkBadUnMarshall), topic, message, err)
}

// AudFailureSending returns relevant error based on the provided parameters
func (ge *SrvError) AudFailureSending(operation string, id string, err error) string {
	return fmt.Sprintf(string(audFailureSending), operation, id, err)
}

// AuthNoUserInToken returns relevant error based on the provided parameters
func (ge *SrvError) AuthNoUserInToken(err error) string {
	return fmt.Sprintf(string(authNoUserInToken), err)
}

// MarshalFullMap returns relevant error based on the provided parameters
func (ge *SrvError) MarshalFullMap(err error) string {
	return fmt.Sprintf(string(marshalFullMap), err)
}

// UnMarshalByteFullMap returns relevant error based on the provided parameters
func (ge *SrvError) UnMarshalByteFullMap(err error) string {
	return fmt.Sprintf(string(unMarshalByteFullMap), err)
}

// MarshalPartialMap returns relevant error based on the provided parameters
func (ge *SrvError) MarshalPartialMap(err error) string {
	return fmt.Sprintf(string(marshalPartialMap), err)
}

// UnMarshalBytePartialMap returns relevant error based on the provided parameters
func (ge *SrvError) UnMarshalBytePartialMap(err error) string {
	return fmt.Sprintf(string(unMarshalBytePartialMap), err)
}

// CacheUnableToWrite returns relevant error based on the provided parameters
func (ge *SrvError) CacheUnableToWrite(key string, err error) string {
	return fmt.Sprintf(string(cacheUnableToWrite), key, err)
}

// CacheDBNameNotSet returns relevant error based on the provided parameters
func (ge *SrvError) CacheDBNameNotSet() string {
	return fmt.Sprintf(string(cacheDBNameNotSet))
}

// CacheUnableToReadVal returns relevant error based on the provided parameters
func (ge *SrvError) CacheUnableToReadVal(key string, err error) string {
	return fmt.Sprintf(string(cacheUnableToReadVal), key, err)
}

// CacheUnableToDeleteVal returns relevant error based on the provided parameters
func (ge *SrvError) CacheUnableToDeleteVal(key string, err error) string {
	return fmt.Sprintf(string(cacheUnableToDeleteVal), key, err)
}

// CacheTooManyValuesToList returns relevant error based on the provided parameters
func (ge *SrvError) CacheTooManyValuesToList(maxValues int) string {
	return fmt.Sprintf(string(cacheTooManyValuesToList), maxValues)
}

// CacheListError returns relevant error based on the provided parameters
func (ge *SrvError) CacheListError(err error) string {
	return fmt.Sprintf(string(cacheListError), err)
}

// CacheEnvVarAddressNotSet returns relevant error based on the provided parameters
func (ge *SrvError) CacheEnvVarAddressNotSet() string {
	return fmt.Sprintf(string(cacheEnvVarAddressNotSet))
}
