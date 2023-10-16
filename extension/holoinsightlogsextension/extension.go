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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"net/http"
	"time"
)

type logsExtension struct {
	cfg    *Config
	logger *zap.Logger
	server *http.Server
	params extension.CreateSettings
	client LogServiceClient
}

func newExtension(cfg *Config, params extension.CreateSettings) (extension.Extension, error) {
	l := &logsExtension{
		cfg:    cfg,
		logger: params.Logger,
		server: &http.Server{},
		params: params,
	}
	var err error
	if l.client, err = NewLogServiceClient(cfg, l.logger); err != nil {
		return nil, err
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
		return fmt.Errorf("[holoinsight_logs] failed to create holoinsight logs server definition: %w", err)
	}
	hln, err := l.cfg.HTTP.ToListener()
	if err != nil {
		return fmt.Errorf("[holoinsight_logs] failed to create holoinsight logs listener: %w", err)
	}

	go func() {
		if err := l.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			host.ReportFatalError(fmt.Errorf("[holoinsight_logs] error starting holoinsight logs extension: %w", err))
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
			l.logger.Error(fmt.Sprintf("[holoinsight_logs] logstore: %s, unauthorized access", logstore))
			return
		}
		logstore = decryptLogstore
	}
	l.client.SetLogStore(logstore)

	datas, err := handlePayload(req)
	if err != nil {
		http.Error(w, "handler payload error", http.StatusInternalServerError)
		l.logger.Error(fmt.Sprintf("[holoinsight_logs] logstore: %s, handlePayload error: ", logstore), zap.Error(err))
		return
	}
	logs := l.dataToSLSLogs(datas)
	if l.pushLogsData(logs) != nil {
		http.Error(w, "push data error", http.StatusInternalServerError)
		l.logger.Error(fmt.Sprintf("[holoinsight_logs] logstore: %s, pushLogsData error: ", logstore), zap.Error(err))
	}
}

func (l logsExtension) dataToSLSLogs(data *Data) []*sls.Log {
	result := make([]*sls.Log, 0)
	for _, tmpLog := range data.Logs {
		log := &sls.Log{
			Time:     proto.Uint32(uint32(time.Now().Unix())),
			Contents: make([]*sls.LogContent, 0),
		}
		result = append(result, log)

		logs, _ := json.Marshal(tmpLog)
		log.Contents = append(log.Contents, &sls.LogContent{
			Key:   proto.String("__logs__"),
			Value: proto.String(string(logs)),
		})

		if data.Tags != nil {
			tags, _ := json.Marshal(data.Tags)
			log.Contents = append(log.Contents, &sls.LogContent{
				Key:   proto.String("__tags__"),
				Value: proto.String(string(tags)),
			})
		}

		if data.Topic != "" {
			l.client.SetTopic(data.Topic)
			log.Contents = append(log.Contents, &sls.LogContent{
				Key:   proto.String("__topic__"),
				Value: proto.String(data.Topic),
			})
		}

		if data.Source != "" {
			l.client.SetSource(data.Source)
			log.Contents = append(log.Contents, &sls.LogContent{
				Key:   proto.String("__source__"),
				Value: proto.String(data.Source),
			})
		}
	}
	return result
}

func (l logsExtension) pushLogsData(logs []*sls.Log) error {
	return l.client.SendLogs(logs)
}
