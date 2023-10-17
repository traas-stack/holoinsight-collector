// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package holoinsightlogsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

import (
	"errors"
	"fmt"
	"net"
	"os"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/producer"
	"go.uber.org/zap"
)

// LogServiceClient log Service's client wrapper
type LogServiceClient interface {
	// SendLogs send message to LogService
	SendLogs(logs []*sls.Log) error
	SetLogStore(logstore string)
	SetTopic(topic string)
	SetSource(source string)
}

type logServiceClientImpl struct {
	clientInstance *producer.Producer
	project        string
	logstore       string
	topic          string
	source         string
	logger         *zap.Logger
}

func getIPAddress() (ipAddress string, err error) {
	as, err := net.InterfaceAddrs()
	for _, a := range as {
		if in, ok := a.(*net.IPNet); ok && !in.IP.IsLoopback() {
			if in.IP.To4() != nil {
				ipAddress = in.IP.String()
			}
		}
	}
	return ipAddress, err
}

// NewLogServiceClient Create Log Service client
func NewLogServiceClient(config *SLSConfig, logger *zap.Logger) (LogServiceClient, error) {
	if config == nil || config.Endpoint == "" || config.Project == "" {
		return nil, errors.New("[holoinsightlogsextension] missing logservice params: Endpoint, Project")
	}

	producerConfig := producer.GetDefaultProducerConfig()
	producerConfig.Endpoint = config.Endpoint
	producerConfig.AccessKeyID = config.AccessKeyID
	producerConfig.AccessKeySecret = config.AccessKeySecret

	c := &logServiceClientImpl{
		project:        config.Project,
		clientInstance: producer.InitProducer(producerConfig),
		logger:         logger,
		logstore:       config.Logstore,
	}
	c.clientInstance.Start()
	// do not return error if get hostname or ip address fail
	c.topic, _ = os.Hostname()
	c.source, _ = getIPAddress()
	logger.Info("[holoinsightlogsextension] Create LogService client success", zap.String("project", config.Project), zap.String("logstore", config.Logstore))
	return c, nil
}

// SendLogs send message to LogService
func (c *logServiceClientImpl) SendLogs(logs []*sls.Log) error {
	if c.logstore == "" {
		return errors.New("[holoinsightlogsextension] missing logservice params: LogStore")
	}
	return c.clientInstance.SendLogListWithCallBack(c.project, c.logstore, c.topic, c.source, logs, c)
}

// Success is impl of producer.CallBack
func (c *logServiceClientImpl) Success(*producer.Result) {
	c.logger.Info(fmt.Sprintf("Write successed, project: %s, logstore: %s", c.project, c.logstore))
}

// Fail is impl of producer.CallBack
func (c *logServiceClientImpl) Fail(result *producer.Result) {
	c.logger.Warn("[holoinsightlogsextension] Send to LogService failed",
		zap.String("project", c.project),
		zap.String("store", c.logstore),
		zap.String("code", result.GetErrorCode()),
		zap.String("error_message", result.GetErrorMessage()),
		zap.String("request_id", result.GetRequestId()))
}

func (c *logServiceClientImpl) SetLogStore(logstore string) {
	c.logstore = logstore
}

func (c *logServiceClientImpl) SetTopic(topic string) {
	c.topic = topic
}

func (c *logServiceClientImpl) SetSource(source string) {
	c.source = source
}
