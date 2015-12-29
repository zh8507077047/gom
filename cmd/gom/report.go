// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/rakyll/gom/internal/pprof/fetch"
	"github.com/rakyll/gom/internal/pprof/profile"
	"github.com/rakyll/gom/internal/pprof/report"
	"github.com/rakyll/gom/internal/pprof/symbolz"
)

type Report struct {
	mu sync.Mutex
	p  *profile.Profile

	name string
	secs int
}

func (r *Report) Inited() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.p != nil
}

// Fetch fetches the current profile and the symbols from the target program.
func (r *Report) Fetch(secs int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if secs == 0 {
		secs = r.secs
	}
	// TODO(jbd): Set timeout according to the seonds parameter.
	url := fmt.Sprintf("%s/debug/pprof/%s?seconds=%d", *target, r.name, secs)
	p, err := fetch.FetchProfile(url, 60*time.Second)
	if err != nil {
		return err
	}
	if err := symbolz.Symbolize(fmt.Sprintf("%s/debug/pprof/symbol", *target), fetch.PostURL, p); err != nil {
		return err
	}
	r.p = p
	return nil
}

// Filter filters the report with a focus regex. If no focus is provided,
// it reports back with the entire set of calls.
// Focus regex works on the package, type and function names. Filtered
// results will include parent samples from the call graph.
func (r *Report) Filter(w io.Writer, cum bool, focus *regexp.Regexp) {
	// TODO(jbd): Support ignore and hide regex parameters.
	if r.p == nil {
		return
	}
	c := r.p.Copy()
	c.FilterSamplesByName(focus, nil, nil)
	rpt := report.NewDefault(c, report.Options{
		OutputFormat:   report.JSON,
		CumSort:        cum,
		PrintAddresses: true,
	})
	report.Generate(w, rpt, nil)
}

func (r *Report) Draw(w io.Writer, cum bool, focus *regexp.Regexp) error {
	// TODO(jbd): Support ignore and hide regex parameters.
	if r.p == nil {
		return errors.New("no such profile")
	}
	c := r.p.Copy()
	c.FilterSamplesByName(focus, nil, nil)
	rpt := report.NewDefault(c, report.Options{
		OutputFormat: report.Dot,
		CumSort:      cum,
	})
	data := bytes.NewBuffer(nil)
	report.Generate(data, rpt, nil)
	cmd := exec.Command("dot", "-Tsvg")
	in, _ := cmd.StdinPipe()
	_, err := io.Copy(in, data)
	if err != nil {
		return err
	}
	in.Close()
	out, err := cmd.Output()
	_, err = w.Write(out)
	return err
}