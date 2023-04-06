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

package httpforwarderauthextension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/extension/auth"

	"github.com/coocood/freecache"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"

	"github.com/traas-stack/holoinsight-collector/internal/utils"
)

type authExtension struct {
	cfg    *Config
	logger *zap.Logger
	cache  *freecache.Cache
}

const (
	Authentication     = "authentication"
	GrpcMetadataTenant = "tenant"
	CacheExpire        = 60 * 60 // 1 hour
)

var (
	errURLNotSet                      = errors.New("url not set")
	errNotAuthenticated               = errors.New("authentication didn't set")
	errCheckErrAuthentication         = errors.New("authentication check api call error")
	errAuthenticationPermissionDenied = errors.New("authentication permission denied")
)

func newExtension(cfg *Config, logger *zap.Logger) (auth.Server, error) {
	if cfg.URL == "" {
		return nil, errURLNotSet
	}

	cacheSize := 1024 * 1024 // 1KB
	cache := freecache.NewCache(cacheSize)

	e := &authExtension{
		cfg:    cfg,
		logger: logger,
		cache:  cache,
	}
	return auth.NewServer(
		auth.WithServerStart(e.start),
		auth.WithServerAuthenticate(e.authenticate),
	), nil
}

func (e *authExtension) start(context.Context, component.Host) error {
	return nil
}

// authenticate checks whether the given context contains valid auth data. Successfully authenticated calls will always return a nil error and a context with the auth data.
func (e *authExtension) authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	authHeaders := headers[Authentication]
	if len(authHeaders) == 0 || authHeaders[0] == "" {
		return ctx, errNotAuthenticated
	}

	// Get from cache
	value, _ := e.cache.Get([]byte(authHeaders[0]))
	if value != nil && len(value) != 0 { //nolint
		headers[GrpcMetadataTenant] = []string{string(value)}
		newCtx := metadata.NewIncomingContext(ctx, headers)
		return newCtx, nil
	}

	// Http get
	response, err := utils.HTTPGet(e.cfg.URL + "?apikey=" + authHeaders[0])
	if err != nil {
		e.logger.Error("[httpforwarderauthextension] authentication check error: ", zap.Error(err))
		return ctx, errCheckErrAuthentication
	}

	if len(response) == 0 {
		e.logger.Warn(fmt.Sprintf("[httpforwarderauthextension] authentication %s permission denied!", authHeaders[0]))
		return ctx, errAuthenticationPermissionDenied
	}

	m := make(map[string]string)
	json.Unmarshal(response, &m) //nolint

	e.cache.Set([]byte(authHeaders[0]), []byte(m[GrpcMetadataTenant]), CacheExpire) //nolint

	headers[GrpcMetadataTenant] = []string{m[GrpcMetadataTenant]}
	newCtx := metadata.NewIncomingContext(ctx, headers)

	return newCtx, nil
}
