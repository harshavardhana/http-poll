/*
 * Minio Cloud Storage, (C) 2018 Minio, Inc.
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

package main

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
)

// HTTPClientTarget - HTTP client target.
type HTTPClientTarget struct {
	w         http.ResponseWriter
	eventCh   chan []byte
	DoneCh    chan struct{}
	stopCh    chan struct{}
	isStopped uint32
	isRunning uint32
}

func (target *HTTPClientTarget) start() {
	go func() {
		defer func() {
			atomic.AddUint32(&target.isRunning, 1)

			// Close DoneCh to indicate we are done.
			close(target.DoneCh)
		}()

		write := func(event []byte) error {
			if _, err := target.w.Write(event); err != nil {
				return err
			}

			target.w.(http.Flusher).Flush()
			return nil
		}

		keepAliveTicker := time.NewTicker(500 * time.Millisecond)
		defer keepAliveTicker.Stop()

		for {
			select {
			case <-target.stopCh:
				// We are asked to stop.
				return
			case event, ok := <-target.eventCh:
				if !ok {
					// Got read error.  Exit the goroutine.
					return
				}
				if err := write(event); err != nil {
					// Got write error to the client.  Exit the goroutine.
					return
				}
			case <-keepAliveTicker.C:
				if err := write([]byte(" ")); err != nil {
					// Got write error to the client.  Exit the goroutine.
					return
				}
			}
		}
	}()
}

// Send - sends event to HTTP client.
func (target *HTTPClientTarget) Send(eventData []byte) error {
	if atomic.LoadUint32(&target.isRunning) != 0 {
		return errors.New("closed http connection")
	}

	if !bytes.HasSuffix(eventData, []byte("\n")) {
		eventData = append(eventData, byte('\n'))
	}

	select {
	case target.eventCh <- eventData:
		return nil
	case <-target.DoneCh:
		return errors.New("error in sending event")
	}
}

// Close - closes underneath goroutine.
func (target *HTTPClientTarget) Close() {
	atomic.AddUint32(&target.isStopped, 1)
	if atomic.LoadUint32(&target.isStopped) == 1 {
		close(target.stopCh)
	}
}

// NewHTTPClientTarget - creates new HTTP client target.
func NewHTTPClientTarget(w http.ResponseWriter) (*HTTPClientTarget, error) {
	c := &HTTPClientTarget{
		w:       w,
		eventCh: make(chan []byte),
		DoneCh:  make(chan struct{}),
		stopCh:  make(chan struct{}),
	}
	c.start()
	return c, nil
}

func periodicMessage(target *HTTPClientTarget) {
	for {
		target.Send([]byte(time.Now().Format(http.TimeFormat)))
		time.Sleep(10 * time.Second)
	}
}

func eventNotification(w http.ResponseWriter, r *http.Request) {
	target, err := NewHTTPClientTarget(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	go periodicMessage(target)
	defer target.Close()

	<-target.DoneCh
}

func main() {
	// Initialize router. `SkipClean(true)` stops gorilla/mux from
	// normalizing URL path.
	router := mux.NewRouter().SkipClean(true)

	apiRouter := router.PathPrefix("/").Subrouter()
	apiRouter.Methods("GET").HandlerFunc(eventNotification)

	server := &http.Server{
		Addr:    ":8888",
		Handler: apiRouter,
	}
	log.Println("Starting to listen on http://localhost:8888")
	log.Fatalln(server.ListenAndServe())
}
