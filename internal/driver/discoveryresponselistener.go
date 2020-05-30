// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018-2019 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func startDiscoveryResponseListening() error {
	var scheme = driver.Config.DiscoveryResponseSchema
	var brokerUrl = driver.Config.DiscoveryResponseHost
	var brokerPort = driver.Config.DiscoveryResponsePort
	var username = driver.Config.DiscoveryResponseUser
	var password = driver.Config.DiscoveryResponsePassword
	var mqttClientId = driver.Config.DiscoveryResponseClientId
	var qos = byte(driver.Config.DiscoveryResponseQos)
	var keepAlive = driver.Config.DiscoveryResponseKeepAlive
	var topic = driver.Config.DiscoveryResponseTopic

	uri := &url.URL{
		Scheme: strings.ToLower(scheme),
		Host:   fmt.Sprintf("%s:%d", brokerUrl, brokerPort),
		User:   url.UserPassword(username, password),
	}

	client, err := createClient(mqttClientId, uri, keepAlive)
	if err != nil {
		return err
	}

	defer func() {
		if client.IsConnected() {
			client.Disconnect(5000)
		}
	}()

	token := client.Subscribe(topic, qos, onDiscoveryResponseReceived)
	if token.Wait() && token.Error() != nil {
		driver.Logger.Info(fmt.Sprintf("[Discovery Response listener] Stop discovery response listening. Cause:%v", token.Error()))
		return token.Error()
	}

	driver.Logger.Info("[Discovery Response listener] Start discovery response listening. ")
	select {}
}

func onDiscoveryResponseReceived(client mqtt.Client, message mqtt.Message) {
	var response map[string]interface{}

	json.Unmarshal(message.Payload(), &response)
	uuid, ok := response["uuid"].(string)
	if !ok {
		driver.Logger.Warn(fmt.Sprintf("[Discovery Response listener] Discovery response ignored. No UUID found in the message: topic=%v msg=%v", message.Topic(), string(message.Payload())))
		return
	}

	existingDiscoveryResponses, ok := driver.DiscoveryResponses.Load(uuid)
	if ok {
		margedDiscoveryResponses := append(existingDiscoveryResponses.([]string), string(message.Payload()))
		driver.DiscoveryResponses.Store(uuid, margedDiscoveryResponses)
	} else {
		driver.DiscoveryResponses.Store(uuid, []string{string(message.Payload())})
	}

	driver.Logger.Info(fmt.Sprintf("[Discovery Response listener] Discovery response received: topic=%v uuid=%v msg=%v", message.Topic(), uuid, string(message.Payload())))
}
