package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func ResponseCode(res int) (string, error) {
	switch res {
	case 200:
		return "200 OK", nil
	case 201:
		return "201 Created", nil
	case 204:
		return "204 No Content", nil
	case 400:
		return "400 Bad Request", nil
	case 404:
		return "404 Not Found", nil
	case 500:
		return "500 Internal Server Error", nil
	}
	return "", fmt.Errorf("unknown status code: %d", res)
}

func ResponseWriter(res int, data any) (*http.Response, error) {
	var bodyBytes []byte
	var err error

	switch v := data.(type) {
	case string:
		bodyBytes = []byte(v)
	case []byte:
		bodyBytes = v
	default:
		bodyBytes, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	statusStr, err := ResponseCode(res)
	if err != nil {
		statusStr = fmt.Sprintf("%d Status", res)
	}

	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	resp := &http.Response{
		Status:        statusStr,
		StatusCode:    res,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(bodyBytes)),
		ContentLength: int64(len(bodyBytes)),
	}

	return resp, nil
}
