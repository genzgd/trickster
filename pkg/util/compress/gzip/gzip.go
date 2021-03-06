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

// Package gzip provides gzip capabilities for byte slices
package gzip

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
)

// Inflate returns the inflated version of a gzip-deflated byte slice
func Inflate(in []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewBuffer(in))
	if err != nil {
		return []byte{}, err
	}

	out, err := ioutil.ReadAll(gr)
	return out, err
}
