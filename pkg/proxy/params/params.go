/*
 * Copyright 2018 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package params provides support for handling URL Parameters
package params

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/tricksterproxy/trickster/pkg/proxy/headers"
	"github.com/tricksterproxy/trickster/pkg/proxy/methods"
)

// UpdateParams updates the provided query parameters collection with the provided updates
func UpdateParams(params url.Values, updates map[string]string) {
	if params == nil || updates == nil || len(updates) == 0 {
		return
	}
	for k, v := range updates {
		if len(k) == 0 {
			continue
		}
		if k[0:1] == "-" {
			k = k[1:]
			params.Del(k)
			continue
		}
		if k[0:1] == "+" {
			k = k[1:]
			params.Add(k, v)
			continue
		}
		params.Set(k, v)
	}
}

// GetRequestValues returns the Query Parameters for the request
// regardless of method
func GetRequestValues(r *http.Request) (url.Values, string, bool) {
	if !methods.HasBody(r.Method) {
		return r.URL.Query(), r.URL.RawQuery, false
	}
	contentType := r.Header.Get(headers.NameContentType)
	if contentType == headers.ValueApplicationJSON {
		b, _ := ioutil.ReadAll(r.Body)
		r.Body.Close()
		r.Body = ioutil.NopCloser(bytes.NewReader(b))
		return url.Values{}, string(b), true
	}
	if contentType == headers.ValueXFormURLEncoded || contentType == headers.ValueMultipartFormData {
		r.ParseForm()
		pf := r.PostForm
		s := pf.Encode()
		r.ContentLength = int64(len(s))
		r.Body = ioutil.NopCloser(bytes.NewReader([]byte(s)))
		return pf, s, true
	}
	return url.Values{}, "", true
}

// SetRequestValues Values sets the Query Parameters for the request
// regardless of method
func SetRequestValues(r *http.Request, v url.Values) {
	s := v.Encode()
	if !methods.HasBody(r.Method) {
		r.URL.RawQuery = s
	} else if len(s) > 0 {
		// reset the body
		r.ContentLength = int64(len(s))
		r.Body = ioutil.NopCloser(bytes.NewReader([]byte(s)))
	}
}
