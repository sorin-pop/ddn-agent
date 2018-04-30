// Package notif is a simple library that makes it easy to send JSON messages
// to a specified address:port/endpoint location. The messages are sent as plain
// text.
package notif

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/djavorszky/ddn-common/inet"
)

// Dest should be a valid URL on the remote host. It should be the full package,
// meaning, it should contain everything from schema till the URI. This is
// used primarily, except if the method specifies otherwise.
var Dest = "http://localhost:80/message"

// New returns a channel through which it awaits messages to be sent to the address
func New(ID int, address string) chan Y {
	ch := make(chan Y)

	go watch(ch, ID, address)

	return ch
}

// Y should be used to send messages through the channel to notif.
type Y struct {
	StatusCode int
	Msg        string
}

// Msg is the simplest type of message that can be sent. It has an ID field which
// can be used to identify the message if it belongs to a conversation, a StatusID
// which can correspond to statuses, similar to that of the HTTP response codes,
// and a Message that can contain any text, aimed to have a human readable message.
type Msg struct {
	ID, StatusID int
	Message      string
}

// Fireable is an empty interface. This way, custom structs can also be used. There
// is no restriction on what can be applied here, as long as it's a struct.
type Fireable interface{}

func watch(ch chan Y, ID int, address string) {
	for y := range ch {
		m := Msg{ID, y.StatusCode, y.Msg}

		_, err := SndLoc(m, address)
		if err != nil {
			log.Println(err)
		}
	}
}

// snd JSONifies the message, then sends it as a POST request to the DefaultDest.
func snd(msg Fireable) (string, error) {
	return SndLoc(msg, Dest)
}

// SndLoc JSONifies the message, then sends it as a POST request to the specified destination.
func SndLoc(msg Fireable, dest string) (resp string, err error) {
	jMsg, err := inet.JSONify(msg)
	if err != nil {
		return "", err
	}

	statusCode, resp, err := sendReq(dest, jMsg)
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("got non-200 response '%d' and message: %s", statusCode, resp)
	}

	return resp, nil
}

func sendReq(dest string, msg []byte) (status int, message string, err error) {
	req, err := http.NewRequest(http.MethodPost, dest, bytes.NewBuffer(msg))
	if err != nil {
		return 0, "", err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("sending request failed: %s", err.Error())
	}
	defer resp.Body.Close()

	var buf bytes.Buffer

	io.Copy(&buf, resp.Body)

	return resp.StatusCode, buf.String(), nil
}
