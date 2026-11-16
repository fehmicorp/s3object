package server

import (
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

type Params struct {
	Keys map[string]string
}

type Body struct {
	File      []byte
	Stream    io.ReadCloser
	Form      url.Values
	Multipart *multipart.Form
	RawData   []byte
}

type Object struct {
	Bkt  string
	Obj  string
	Dir  string
	Typ  string
	Bdy  Body
	Prms Params
}

func ParseS3URL(r *http.Request, slug string) Object {
	// 1. Initialize and fetch query parameters
	params := GetParams(r)
	if params.Keys == nil {
		params.Keys = make(map[string]string)
	}

	// 2. Populate the flexible Body struct based on request characteristics
	var bodyStruct Body
	bodyStruct.Stream = r.Body // Always available for streaming if needed

	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		// Parse multipart form data (e.g., file uploads via HTML forms or Postman)
		// 32 << 20 = 32MB max memory for parsing
		if err := r.ParseMultipartForm(32 << 20); err == nil {
			bodyStruct.Multipart = r.MultipartForm
		}
	} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		// Parse standard url-encoded form values
		if err := r.ParseForm(); err == nil {
			bodyStruct.Form = r.PostForm
		}
	} else {
		// For standard raw body uploads (like a PUT/POST of a binary file or JSON)
		// Note: If you want true streaming for huge files, avoid io.ReadAll here
		// and read from bodyStruct.Stream progressively inside your handlers.
		if r.Body != nil {
			if data, err := io.ReadAll(r.Body); err == nil {
				bodyStruct.File = data
				bodyStruct.RawData = data
			}
		}
	}

	cp := strings.TrimPrefix(r.URL.EscapedPath(), "/")
	if cp == "" {
		return Object{Bdy: bodyStruct, Prms: params}
	}

	pr := strings.Split(cp, "/")
	idx := 0
	cs := strings.Trim(slug, "/")
	if cs != "" && idx < len(pr) && pr[idx] == cs {
		idx++
	}
	if idx >= len(pr) {
		return Object{Bdy: bodyStruct, Prms: params}
	}

	var b string
	if u, err := urlPathUnescape(pr[idx]); err == nil {
		b = u
	} else {
		b = pr[idx]
	}
	idx++

	remaining := pr[idx:]
	if len(remaining) == 0 {
		return Object{Bkt: b, Bdy: bodyStruct, Prms: params}
	}

	uspr := make([]string, len(remaining))
	for i, p := range remaining {
		if u, err := urlPathUnescape(p); err == nil {
			uspr[i] = u
		} else {
			uspr[i] = p
		}
	}

	if len(uspr) == 1 {
		objName := uspr[0]
		ext := strings.TrimPrefix(filepath.Ext(objName), ".")
		return Object{
			Bkt:  b,
			Obj:  objName,
			Dir:  "",
			Typ:  ext,
			Bdy:  bodyStruct,
			Prms: params,
		}
	}

	objName := uspr[len(uspr)-1]
	dirStr := strings.Join(uspr[:len(uspr)-1], "/")
	ext := strings.TrimPrefix(filepath.Ext(objName), ".")

	return Object{
		Bkt:  b,
		Obj:  objName,
		Dir:  dirStr,
		Typ:  ext,
		Bdy:  bodyStruct,
		Prms: params,
	}
}

func urlPathUnescape(s string) (string, error) {
	return strings.ReplaceAll(s, "+", " "), nil // or use url.PathUnescape(s)
}

func ListingPath(path string) string {
	if slug != "" && !strings.HasPrefix(slug, "/") {
		slug = "/" + slug
	}
	return slug + path
}

func GetParams(r *http.Request) Params {
	keys := make(map[string]string)
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			keys[k] = v[0]
		}
	}
	return Params{
		Keys: keys,
	}
}

func Func(fn func() (*http.Response, error)) (func() (*http.Response, error), error) {
	return fn, nil
}

func ErrorFunct(er int, msg string) (*http.Response, error) {
	resp, err := ResponseWriter(er, msg)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
