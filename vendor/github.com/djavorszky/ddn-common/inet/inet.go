// Package inet contains convenience features for operations on the internet.
package inet

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// WriteHeader updates the header's Content-Type to application/json and charset to
// UTF-8. Additionally, it also adds the http status to it.
func WriteHeader(w http.ResponseWriter, status int) {
	header := w.Header()
	header.Set("Content-Type", "application/json; charset=utf-8")

	if status != http.StatusOK {
		w.WriteHeader(status)
	}
}

// DownloadFile downloads the file from the url and places it into the
// `dest` folder
func DownloadFile(dest, url string) (string, error) {
	i, j := strings.LastIndex(url, "/"), len(url)
	filename := url[i+1 : j]

	filepath := fmt.Sprintf("%s/%s", dest, filename)

	out, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("could not create file: %s", err.Error())
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("couldn't get url '%s': %s", url, err.Error())
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(filepath)

		return "", fmt.Errorf("downloading file failed: %s", err.Error())
	}

	return filepath, nil
}

// AddrExists checks the URL to see if it's valid, downloadable file or not.
func AddrExists(url string) bool {
	respCode := GetResponseCode(url)

	if respCode == http.StatusOK {
		return true
	}

	return false
}

// GetResponseCode returns the response code of a HTTP call
func GetResponseCode(url string) int {
	defer func() {
		if p := recover(); p != nil {
			// panic happens, no need to log anything. It's usually a refusal.
			//log.Printf("Remote end %q refused the connection", url)
		}
	}()

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}

	resp, err := http.Get(url)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

// SendResponse composes the message, writes the header, then writes the bytes
// to the ResponseWriter
func SendResponse(w http.ResponseWriter, status int, msg JSONMessage) {
	b := msg.Compose()

	WriteHeader(w, status)

	w.Write(b)
}

// Response represents a
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   []string    `json:"error,omitempty"`
}

// Marshal marshals the response into json
func (r Response) Marshal() []byte {
	b, _ := json.Marshal(r)

	return b
}

// SendSuccess creates a JSON response of a successful API call
func SendSuccess(w http.ResponseWriter, status int, data interface{}) {
	WriteHeader(w, status)

	r := Response{
		Success: true,
		Data:    data,
	}

	w.Write(r.Marshal())
}

// SendFailure creates a JSON response of a failed API call
func SendFailure(w http.ResponseWriter, status int, errs ...string) {
	WriteHeader(w, status)

	r := Response{
		Success: false,
		Error:   errs,
	}

	w.Write(r.Marshal())
}
