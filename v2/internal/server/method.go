package server

import (
	"fmt"
	"net/http"
)

func (h *Health) HttpMethods(w http.ResponseWriter, r *http.Request, slug string) (func() (*http.Response, error), error) {
	m := r.Method
	pr := ParseS3URL(r, slug)
	switch m {
	case http.MethodGet:
		if pr.Bkt == "" {
			return Func(GetListBuckets)
		} else if pr.Obj == "" {
			return Func(func() (*http.Response, error) {
				return GetListObjects(pr)
			})
		} else {
			return Func(func() (*http.Response, error) {
				return GetObject(pr)
			})
		}
	case http.MethodPut:
		if pr.Bkt == "" && pr.Prms.Keys["bucket"] != "" {
			// return Func(pr.Prms.CreateBucket)
		}
		if pr.Bkt != "" && pr.Obj != "" && pr.Prms.Keys["object"] != "" {
			return Func(pr.Prms.CreateObject)
		}
	case http.MethodPost:
		if pr.Bkt != "" && pr.Obj != "" && len(pr.Bdy.File) > 0 {
			return Func(pr.UploadObject)
		}
	case http.MethodDelete:
		if pr.Bkt != "" {
			return Func(pr.Prms.DeleteBucket)
		}
		if pr.Bkt != "" && pr.Obj != "" {
			return Func(pr.Prms.DeleteObject)
		}
	default:
		return func() (*http.Response, error) {
			return ErrorFunct(http.StatusMethodNotAllowed, "Method not allowed")
		}, nil
	}
	return nil, fmt.Errorf("Something went wrong")
}
