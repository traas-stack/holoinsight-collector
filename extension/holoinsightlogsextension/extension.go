// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package holoinsightlogsextension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/gorilla/mux"
	"github.com/traas-stack/holoinsight-collector/internal/utils"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"net/http"
	"net/url"
	"time"
)

type logsExtension struct {
	cfg         *Config
	logger      *zap.Logger
	server      *http.Server
	params      extension.CreateSettings
	clientCache map[string]LogServiceClient
}

func newExtension(cfg *Config, params extension.CreateSettings) (extension.Extension, error) {
	if cfg.ServerEndpoint == "" {
		return nil, errors.New("server endpoint not set")
	}

	l := &logsExtension{
		cfg:         cfg,
		logger:      params.Logger,
		server:      &http.Server{},
		params:      params,
		clientCache: make(map[string]LogServiceClient),
	}

	return l, nil
}

func (l logsExtension) Start(ctx context.Context, host component.Host) error {
	router := mux.NewRouter()
	router.HandleFunc("/logstores/{logstore}/track", l.handleLogs)

	var err error
	l.server, err = l.cfg.HTTP.ToServer(
		host,
		l.params.TelemetrySettings,
		router,
	)
	if err != nil {
		return fmt.Errorf("[holoinsightlogsextension] failed to create holoinsight logs server definition: %w", err)
	}
	hln, err := l.cfg.HTTP.ToListener()
	if err != nil {
		return fmt.Errorf("[holoinsightlogsextension] failed to create holoinsight logs listener: %w", err)
	}

	go func() {
		if err := l.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			host.ReportFatalError(fmt.Errorf("[holoinsightlogsextension] error starting holoinsight logs extension: %w", err))
		}
	}()
	return nil
}

func (l logsExtension) Shutdown(ctx context.Context) error {
	return l.server.Shutdown(ctx)
}

func (l logsExtension) handleLogs(w http.ResponseWriter, req *http.Request) {
	var err error
	vars := mux.Vars(req)
	logstore := vars["logstore"]
	if l.cfg.Enable && l.cfg.SecretKey != "" {
		decryptLogstore, err := AesDecrypt(logstore, l.cfg.SecretKey, l.cfg.IV)
		if err != nil {
			http.Error(w, "Unauthorized access", http.StatusUnauthorized)
			l.logger.Error(fmt.Sprintf("[holoinsightlogsextension] logstore: %s, unauthorized access", logstore))
			return
		}
		logstore = decryptLogstore
	}

	datas, err := handlePayload(req)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		l.logger.Error(fmt.Sprintf("[holoinsightlogsextension] logstore: %s, handlePayload error: ", logstore), zap.Error(err))
		return
	}

	client, err := l.getLogServiceClient(logstore)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		l.logger.Error(fmt.Sprintf("[holoinsightlogsextension] logstore: %s, get log service client error: ", logstore), zap.Error(err))
		return
	}

	logs := l.dataToSLSLogs(datas, client)
	if client.SendLogs(logs) != nil {
		http.Error(w, "push data error", http.StatusInternalServerError)
		l.logger.Error(fmt.Sprintf("[holoinsightlogsextension] logstore: %s, pushLogsData error: ", logstore), zap.Error(err))
	}
}

func (l logsExtension) getLogServiceClient(key string) (LogServiceClient, error) {
	// Get sls client from cache
	client := l.clientCache[key]
	if client == nil {
		// Get from holoinsight server
		response, err := utils.HTTPGet(l.cfg.ServerEndpoint + "/internal/customize/log/project/query?key=" + url.QueryEscape(key))
		if err != nil {
			l.logger.Error("[holoinsightlogsextension] get project from holoinsight server error: ", zap.Error(err))
			return nil, err
		}
		if response == nil {
			l.logger.Error("[holoinsightlogsextension] get empty sls config from holoinsight server: ", zap.Error(err))
			return nil, err
		}

		slsProjectConfig := make(map[string]string)
		err = json.Unmarshal(response, &slsProjectConfig)
		if err != nil {
			l.logger.Error("[holoinsightlogsextension] Unmarshal sls config error: ", zap.Error(err))
			return nil, err
		}

		slsConfig := &SLSConfig{
			Endpoint:        l.cfg.Endpoint,
			Project:         slsProjectConfig["projectName"],
			Logstore:        key,
			AccessKeyID:     slsProjectConfig["accessId"],
			AccessKeySecret: slsProjectConfig["accessKey"],
		}
		client, err = NewLogServiceClient(slsConfig, l.logger)
		if err != nil {
			l.logger.Error("[holoinsightlogsextension] new log service client error: ", zap.Error(err))
			return nil, err
		}
		l.clientCache[key] = client
	}
	return client, nil
}

func (l logsExtension) dataToSLSLogs(data *Data, client LogServiceClient) []*sls.Log {
	result := make([]*sls.Log, 0)
	log := &sls.Log{
		Time:     proto.Uint32(uint32(time.Now().Unix())),
		Contents: make([]*sls.LogContent, 0),
	}
	result = append(result, log)

	for _, tmpLog := range data.Logs {
		for k, v := range tmpLog {
			log.Contents = append(log.Contents, &sls.LogContent{
				Key:   proto.String(k),
				Value: proto.String(v),
			})
		}
	}

	for k, v := range data.Tags {
		log.Contents = append(log.Contents, &sls.LogContent{
			Key:   proto.String(k),
			Value: proto.String(v),
		})
	}

	if data.Topic != "" {
		client.SetTopic(data.Topic)
		log.Contents = append(log.Contents, &sls.LogContent{
			Key:   proto.String("__topic__"),
			Value: proto.String(data.Topic),
		})
	}

	if data.Source != "" {
		client.SetSource(data.Source)
		log.Contents = append(log.Contents, &sls.LogContent{
			Key:   proto.String("__source__"),
			Value: proto.String(data.Source),
		})
	}

	return result
}
