package inet

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/djavorszky/ddn-common/logger"
	"github.com/djavorszky/ddn-common/status"
)

// JSONMessage is an interface that can hold many types of messages that
// can be json'ified. The reason we need to return an int as well is because
// I can't figure out how we could easily get the status of the JSONMessage
// without too much boilerplate code. This way, we can return the status
// in the same step and, if not needed, discard it.
type JSONMessage interface {
	Compose() []byte
}

// Message is a struct to hold a simple status-message type response
type Message struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// Compose creates a JSON formatted byte slice from the Message
func (msg Message) Compose() []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
	}

	return b
}

// ListMessage is a struct to hold a status-list of strings type response
type ListMessage struct {
	Status  int      `json:"status"`
	Message []string `json:"list"`
}

// Compose creates a JSON formatted byte slice from the ListMessage
func (msg ListMessage) Compose() []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
	}

	return b
}

// MapMessage is a struct to hold a status and a key+value type response
type MapMessage struct {
	Status  int               `json:"status"`
	Message map[string]string `json:"map"`
}

// Compose creates a JSON formatted byte slice from the Message
func (msg MapMessage) Compose() []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
	}

	return b
}

// StructMessage is a message for structs
type StructMessage struct {
	Status  int         `json:"status"`
	Message interface{} `json:"object"`
}

// Compose creates a JSON formatted byte slice from the StructMessage
func (msg StructMessage) Compose() []byte {
	b, err := json.Marshal(msg.Message)
	if err != nil {
		log.Fatal(err)
	}

	return b
}

// ErrorResponse composes a Message with an 500 response code. It should be used
// for situations where something went wrong on the server's side.
func ErrorResponse() Message {
	var errMsg Message

	errMsg.Status = status.ServerError
	errMsg.Message = "Something went wrong on the server."

	return errMsg
}

// ErrorJSONResponse composes a Message with a 400 response code. It should be used
// for situations where something was wrong with the JSON request
func ErrorJSONResponse(err error) Message {
	var msg Message

	logger.Error("json decode: %v", err)

	msg.Status = status.InvalidJSON
	msg.Message = fmt.Sprintf("Invalid JSON request, received error: %v", err)

	return msg
}

// InvalidResponse composes a Message with a 400 response code. It should be used
// for situations where the request was invalid.
func InvalidResponse() Message {
	var msg Message

	msg.Status = status.MissingParameters
	msg.Message = "One or more required fields are missing from the call"

	return msg
}

// Fireable is an empty interface. This way, custom structs can also be used. There
// is no restriction on what can be applied here, as long as it's a struct.
type Fireable interface{}

// JSONify creates a json byte slice from a given struct.
func JSONify(msg Fireable) ([]byte, error) {
	b, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return b, err
}
