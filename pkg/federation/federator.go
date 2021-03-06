/*
Copyright (C) 2018 Synopsys, Inc.

Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements. See the NOTICE file
distributed with this work for additional information
regarding copyright ownership. The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied. See the License for the
specific language governing permissions and limitations
under the License.
*/

package federation

import (
	"fmt"
	//	"net"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/blackducksoftware/armada/pkg/actions"
	"github.com/blackducksoftware/armada/pkg/api"
	"github.com/blackducksoftware/armada/pkg/hub"
	"github.com/blackducksoftware/armada/pkg/util"
	"github.com/blackducksoftware/armada/pkg/webapi"
	"github.com/blackducksoftware/armada/pkg/webapi/responders"
	httpresponder "github.com/blackducksoftware/armada/pkg/webapi/responders/http"
	mockresponder "github.com/blackducksoftware/armada/pkg/webapi/responders/mock"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/gorilla/mux"

	log "github.com/sirupsen/logrus"
)

const (
	actionChannelSize = 100
)

// Federator handles federating queries across multiple hubs
type Federator struct {
	responder  responders.ResponderInterface
	router     *mux.Router
	hubCreator *HubClientCreator

	// model
	config *FederatorConfig
	hubs   map[string]*hub.Client

	// channels
	stop    chan struct{}
	actions chan actions.ActionInterface
}

// NewFederator creates a new Federator object
func NewFederator(configPath string) (*Federator, error) {
	var responder responders.ResponderInterface

	config, err := GetConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file %s: %v", configPath, err)
	}
	if config == nil {
		return nil, fmt.Errorf("expected non-nil config from path %s, but got nil", configPath)
	}

	level, err := config.GetLogLevel()
	if err != nil {
		return nil, fmt.Errorf("error setting log level: %v", err)
	}
	log.SetLevel(level)

	ip, err := util.GetOutboundIP()
	if err != nil {
		return nil, fmt.Errorf("failed to find local ip address: %v", err)
	}

	router := mux.NewRouter().StrictSlash(true)
	if config.UseMockMode {
		responder = mockresponder.NewResponder()
	} else {
		responder = httpresponder.NewResponder(ip)
	}
	webapi.SetupHTTPServer(responder, router)

	hubCreator, err := NewHubClientCreator(config.HubPasswordEnvVar, config.HubDefaults)
	if err != nil {
		return nil, fmt.Errorf("failed to recreate HubClientCreator: %v", err)
	}

	prometheus.Unregister(prometheus.NewProcessCollector(os.Getpid(), ""))
	prometheus.Unregister(prometheus.NewGoCollector())

	//	http.Handle("/metrics", prometheus.Handler())

	/*
		// dump events into 'actions' queue
		go func() {
			for {
				select {
				case a := <-responder.RequestsCh:
					actions <- a
				case d := <-hubCreator.didFinishHubCreation:
					actions <- d
				}
			}
		}()
	*/

	fed := &Federator{
		responder:  responder,
		router:     router,
		hubCreator: hubCreator,
		config:     config,
		hubs:       map[string]*hub.Client{},
		stop:       make(chan struct{}),
		actions:    make(chan actions.ActionInterface, actionChannelSize),
	}

	/*
		// process actions
		go func() {
			for {
				a := <-actions
				log.Debugf("received action %s", reflect.TypeOf(a))
				start := time.Now()
				a.Apply(fed)
				stop := time.Now()
				log.Debugf("finished processing action -- %s", stop.Sub(start))
			}
		}()
	*/

	return fed, nil
}

// Run will start the federator listening for requests
func (fed *Federator) Run(stopCh chan struct{}) {

	// dump events into 'actions' queue
	go func() {
		for {
			select {
			case a := <-fed.responder.GetRequestCh():
				log.Debugf("received action: %+v", a)
				fed.actions <- a
			case d := <-fed.hubCreator.didFinishHubCreation:
				fed.actions <- d
			}
		}
	}()

	// process actions
	go func() {
		for {
			a := <-fed.actions
			log.Debugf("processing action %s", reflect.TypeOf(a))
			start := time.Now()
			a.Execute(fed)
			stop := time.Now()
			log.Debugf("finished processing action -- %s", stop.Sub(start))
		}
	}()

	log.Infof("starting HTTP server on port %d", fed.config.Port)
	go func() {
		addr := fmt.Sprintf(":%d", fed.config.Port)
		http.ListenAndServe(addr, fed.router)
	}()
	<-stopCh
}

/*
func (fed *Federator) SetHubs(hubList *api.HubList) error {
	newHubURLs := map[string]bool{}
	hubsToCreate := api.HubList{}

	for _, newHub := range hubList.Items {
		hubURL := fmt.Sprintf("https://%s:%d", newHub.Host, newHub.Port)
		newHubURLs[hubURL] = true
		if _, ok := fed.hubs[hubURL]; !ok {
			hubsToCreate.Items = append(hubsToCreate.Items, newHub)
		}
	}

	// 1. create new hubs
	// TODO handle retries and failures intelligently
	go func() {
		fed.hubCreator.CreateClients(&hubsToCreate)
	}()

	// 2. delete removed hubs
	for hubURL, hubClient := range fed.hubs {
		if _, ok := newHubURLs[hubURL]; !ok {
			hubClient.Stop()
			delete(fed.hubs, hubURL)
			// TODO does any other clean up need to happen?
		}
	}

	return nil
}
*/

// CreateHubClients will create hub clients for the provided hubs
func (fed *Federator) CreateHubClients(hubList *api.HubList) {
	fed.hubCreator.CreateClients(hubList)
}

// AddHub will add a hub to the list of know hubs
func (fed *Federator) AddHub(url string, client *hub.Client) {
	if _, ok := fed.hubs[url]; ok {
		log.Warningf("cannot add hub %s: already present", url)
	}
	fed.hubs[url] = client
}

// DeleteHub will remove a hub from the list of know hubs
func (fed *Federator) DeleteHub(url string) {
	client, ok := fed.hubs[url]
	if !ok {
		log.Warningf("received request to delete hub %s, but it does not exist")
	} else {
		client.Stop()
		delete(fed.hubs, url)
	}
}

// GetHubs returns the hubs known to the federator
func (fed *Federator) GetHubs() map[string]*hub.Client {
	return fed.hubs
}
