package server

import (
	"fmt"
	"net/http"
)

func GetListObjects(p Object) (*http.Response, error) {
	objects := "Blanks"
	bucket := p.Bkt
	data := map[string]any{"bucket": fmt.Sprintf("%s", bucket), "objects": fmt.Sprintf("%s", objects)}
	resp, err := ResponseWriter(http.StatusOK, data)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func GetObject(p Object) (*http.Response, error) {
	objectName := p.Obj
	data := map[string]any{"object": fmt.Sprintf("%s", objectName)}
	resp, err := ResponseWriter(http.StatusOK, data)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (p Params) CreateObject() (*http.Response, error) {
	data := map[string]any{
		"action": "create_bucket",
		"params": p.Keys,
	}
	resp, err := ResponseWriter(http.StatusOK, data)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (o Object) UploadObject() (*http.Response, error) {
	if len(o.Bdy.File) > 0 {
		// process o.Bdy.File
	}
	// if o.Bdy.Multipart != nil && len(o.Bdy.Multipart.File) > 0 {
	// 	files := o.Bdy.Multipart.File["upload_file_key"]
	// 	for _, fileHeader := range files {
	// 		file, err := fileHeader.Open()
	// 		if err != nil {
	// 			continue
	// 		}
	// 		defer file.Close()
	// 	}
	// }
	data := map[string]any{
		"action": "uploaded_successfully",
		"bucket": o.Bkt,
		"object": o.Obj,
	}
	return ResponseWriter(http.StatusOK, data)
}

func (p Params) DeleteObject() (*http.Response, error) {
	data := map[string]any{
		"action": "delete_bucket",
		"params": p.Keys,
	}
	resp, err := ResponseWriter(http.StatusOK, data)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
