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

// Package testing provides functionality for use when conducting tests
package testing

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	cr "github.com/tricksterproxy/trickster/pkg/cache/registration"
	"github.com/tricksterproxy/trickster/pkg/config"
	tc "github.com/tricksterproxy/trickster/pkg/proxy/context"
	th "github.com/tricksterproxy/trickster/pkg/proxy/headers"
	oo "github.com/tricksterproxy/trickster/pkg/proxy/origins/options"
	po "github.com/tricksterproxy/trickster/pkg/proxy/paths/options"
	"github.com/tricksterproxy/trickster/pkg/proxy/request"
	"github.com/tricksterproxy/trickster/pkg/runtime"
	"github.com/tricksterproxy/trickster/pkg/tracing"
	tr "github.com/tricksterproxy/trickster/pkg/tracing/registration"
	tl "github.com/tricksterproxy/trickster/pkg/util/log"

	"github.com/tricksterproxy/mockster/pkg/testutil"
)

// this actively sets the ApplicationName for testing purposes
// do not import this package from main or any of its recursive imports
func init() {
	runtime.ApplicationName = "trickster-unit-tests"
}

// NewTestServer returns a new httptest.Server that responds with the provided code, body and headers
func NewTestServer(responseCode int, responseBody string, headers map[string]string) *httptest.Server {
	handler := func(w http.ResponseWriter, r *http.Request) {
		th.UpdateHeaders(w.Header(), headers)
		w.WriteHeader(responseCode)
		fmt.Fprint(w, responseBody)
	}
	s := httptest.NewServer(http.HandlerFunc(handler))
	return s
}

// NewTestWebClient returns a new *http.Client configured with reasonable defaults
func NewTestWebClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			Dial:                (&net.Dialer{KeepAlive: 300 * time.Second}).Dial,
			MaxIdleConns:        20,
			MaxIdleConnsPerHost: 20,
		},
	}
}

// NewTestInstance will start a trickster
func NewTestInstance(
	configFile string,
	defaultPathConfigs func(*oo.Options) map[string]*po.Options,
	respCode int, respBody string, respHeaders map[string]string,
	originType, urlPath, logLevel string,
) (*httptest.Server, *httptest.ResponseRecorder, *http.Request, *http.Client, error) {

	isBasicTestServer := false

	var ts *httptest.Server
	if originType == "promsim" {
		ts = testutil.NewTestServer()
		originType = "prometheus"
	} else if originType == "rangesim" {
		ts = testutil.NewTestServer()
		originType = "rpc"
	} else {
		isBasicTestServer = true
		ts = NewTestServer(respCode, respBody, respHeaders)
	}

	args := []string{"-origin-url", ts.URL, "-origin-type", originType, "-log-level", logLevel}
	if configFile != "" {
		args = append(args, []string{"-config", configFile}...)
	}

	conf, _, err := config.Load("trickster", "test", args)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not load configuration: %s", err.Error())
	}

	caches := cr.LoadCachesFromConfig(conf, tl.ConsoleLogger("error"))
	cache, ok := caches["default"]
	if !ok {
		return nil, nil, nil, nil, err
	}

	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", ts.URL+urlPath, nil)

	oc := conf.Origins["default"]
	p := NewTestPathConfig(oc, defaultPathConfigs, urlPath)

	var tracer *tracing.Tracer

	logger := tl.ConsoleLogger("error")

	if oc.TracingConfigName != "" {
		if tc, ok := conf.TracingConfigs[oc.TracingConfigName]; ok {
			tracer, _ = tr.GetTracer(tc, logger, true)
		}
	}

	if !isBasicTestServer && respHeaders != nil {
		p.ResponseHeaders = respHeaders
	}

	rsc := request.NewResources(oc, p, cache.Configuration(), cache, nil, tracer, logger)
	r = r.WithContext(tc.WithResources(r.Context(), rsc))

	c := NewTestWebClient()

	return ts, w, r, c, nil
}

// NewTestPathConfig returns a path config based on the provided parameters
func NewTestPathConfig(
	oc *oo.Options,
	defaultPathConfigs func(*oo.Options) map[string]*po.Options,
	urlPath string,
) *po.Options {
	var paths map[string]*po.Options
	if defaultPathConfigs != nil {
		paths = defaultPathConfigs(oc)
	}

	oc.Paths = paths

	p := &po.Options{}
	if len(paths) > 0 {
		if p2, ok := paths[urlPath]; ok {
			p = p2
		} else {
			p = paths["/"]
		}
	}

	return p
}
