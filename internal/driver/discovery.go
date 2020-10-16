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

	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	sdkModel "github.com/edgexfoundry/device-sdk-go/pkg/models"
	contract "github.com/edgexfoundry/go-mod-core-contracts/models"
	"gopkg.in/mgo.v2/bson"
)

func (d *Driver) Discover() {

	var scheme = driver.Config.DiscoverySchema
	var brokerUrl = driver.Config.DiscoveryHost
	var brokerPort = driver.Config.DiscoveryPort
	var username = driver.Config.DiscoveryUser
	var password = driver.Config.DiscoveryPassword
	var mqttClientId = driver.Config.DiscoveryClientId
	var topic = driver.Config.DiscoveryTopic

	uri := &url.URL{
		Scheme: strings.ToLower(scheme),
		Host:   fmt.Sprintf("%s:%d", brokerUrl, brokerPort),
		User:   url.UserPassword(username, password),
	}

	client, err := createClient(mqttClientId, uri, 30)
	if err != nil {
		return
	}

	defer func() {
		if client.IsConnected() {
			client.Disconnect(5000)
		}
	}()

	res, err := d.handleDiscovery(client, topic)
	if err != nil {
		driver.Logger.Info(fmt.Sprintf("Handle discovery failed: %v", err))
		return
	}

	d.DeviceCh <- res
	return
}

func (d *Driver) handleDiscovery(deviceClient mqtt.Client, topic string) ([]sdkModel.DiscoveredDevice, error) {
	var result = []sdkModel.DiscoveredDevice{}
	var err error
	var qos = byte(0)
	var retained = false

	var method = "discovery"
	var methodUuid = bson.NewObjectId().Hex()

	data := make(map[string]interface{})
	data["uuid"] = methodUuid
	data["method"] = method

	jsonData, err := json.Marshal(data)
	if err != nil {
		return result, err
	}

	deviceClient.Publish(topic, qos, retained, jsonData)

	driver.Logger.Info(fmt.Sprintf("Publish discovery: %v", string(jsonData)))

	// fetch response from MQTT broker after discovery successful
	discoveryResponse, ok := d.fetchDiscoveryResponse(methodUuid)
	if !ok {
		driver.Logger.Info(fmt.Sprintf("No new device has been discovered: method=%v", method))
		return result, nil
	}

	driver.Logger.Info(fmt.Sprintf("Parse discovery response: %v", discoveryResponse))

	result, err = newDevices(discoveryResponse)
	if err != nil {
		driver.Logger.Info(fmt.Sprintf("Failed to fetch device information: method=%v", method))
		return result, nil
	} else {
		driver.Logger.Info(fmt.Sprintf("Discovery finished: %v", result))
	}

	return result, nil
}

// fetch DiscoveryResponse use to wait and fetch response from DiscoveryResponses map
func (d *Driver) fetchDiscoveryResponse(methodUuid string) ([]string, bool) {
	var discoveryResponse interface{}
	var ok bool

	time.Sleep(time.Second * time.Duration(1) * 5)

	discoveryResponse, ok = d.DiscoveryResponses.Load(methodUuid)
	if ok {
		d.DiscoveryResponses.Delete(methodUuid)
	} else {
		return []string{}, false
	}

	return discoveryResponse.([]string), ok
}

func newDevices(response []string) ([]sdkModel.DiscoveredDevice, error) {
	var res []sdkModel.DiscoveredDevice

	for _, raw := range response {
		var fetcheddevice map[string]interface{}
		json.Unmarshal([]byte(raw), &fetcheddevice)

		proto := make(map[string]contract.ProtocolProperties)
		proto["mqtt"] = map[string]string{
			"Schema": driver.Config.DefaultCommandSchema,
			"Host": driver.Config.DefaultCommandHost,
			"Port": driver.Config.DefaultCommandPort,
			"ClientId": driver.Config.DefaultCommandClientId,
			"User": driver.Config.DefaultCommandUser,
			"Password": driver.Config.DefaultCommandPassword,
			"Topic": driver.Config.DefaultCommandTopicRoot + "/" + fetcheddevice["name"].(string),
		}

		device := sdkModel.DiscoveredDevice{
			Name:        fetcheddevice["name"].(string),
			Protocols:   proto,
			Description: fetcheddevice["description"].(string),
			Labels:      []string{"auto-discovery"},
		}

		driver.Logger.Info(fmt.Sprintf("Fetched device information: %v", device))
		res = append(res, device)
	}
	return res, nil
}
