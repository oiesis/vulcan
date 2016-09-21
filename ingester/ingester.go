// Copyright 2016 The Vulcan Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ingester

import (
	"sync"
	"time"

	"github.com/digitalocean/vulcan/bus"
	"github.com/digitalocean/vulcan/storage"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
)

// Ingester represents an object that consumes metrics from a bus and writes
// them to a data storage.
type Ingester struct {
	prometheus.Collector

	src bus.Source
	w   storage.SampleWriter
}

// Config provides the ingester the necessary dependencies it needs to read
// TimeSeries from the bus and write Samples.
type Config struct {
	Source bus.Source
	Writer Writer
}

// NewIngester creates a new instance of Ingester.
func NewIngester(config *Config) *Ingester {

}

// Run blocks until an error occurs and should be called only once.
func (i *Ingester) Run() error {
	once := sync.Once{}
	for n := 0; n < i.numWorkers; i++ {
		go func() {

		}()
	}
}

func (i *Ingester) worker() {
	for w := range i.work {
		t0 := time.Now()

		log.WithFields(log.Fields{"sample": w.s}).Debug("writing sample")

		err := i.sampleWriter.WriteSample(w.s)
		w.wg.Done()
		if err != nil {
			log.WithError(err).Error("error writing sample to storage")

			i.errorsTotal.WithLabelValues("write_sample").Add(1)
			continue
		}

		i.ingesterDurations.WithLabelValues("write_sample").Observe(float64(time.Since(t0).Nanoseconds()))
	}
}

// Describe implements prometheus.Collector.  Sends decriptors of the
// instance's ingesterDurations and errorsTotal to the parameter ch.
func (i *Ingester) Describe(ch chan<- *prometheus.Desc) {
	i.ingesterDurations.Describe(ch)
	i.errorsTotal.Describe(ch)
}

// Collect implements Collector.  Sends metrics collected by ingesterDurations
// and errorsTotal to the parameter ch.
func (i *Ingester) Collect(ch chan<- prometheus.Metric) {
	i.ingesterDurations.Collect(ch)
	i.errorsTotal.Collect(ch)
}

// Run starts the ingesting process by consuming from the message bus and
// writing to the data storage system.
func (i *Ingester) Run() error {
	log.Info("running...")
	ch := i.ackSource.Chan()

	for payload := range ch {
		log.WithFields(log.Fields{
			"payload": payload.SampleGroup,
		}).Debug("distributing sample group to workers")

		i.writeSampleGroup(payload.SampleGroup)
		payload.Done(nil)
	}

	return i.ackSource.Err()
}

func (i *Ingester) writeSampleGroup(sg bus.SampleGroup) {
	var (
		t0 = time.Now()
		wg = &sync.WaitGroup{}
	)

	wg.Add(len(sg))

	for _, s := range sg {
		i.work <- workPayload{
			s:  s,
			wg: wg,
		}
	}

	wg.Wait()
	i.ingesterDurations.WithLabelValues("write_sample_group").Observe(float64(time.Since(t0).Nanoseconds()))
}
