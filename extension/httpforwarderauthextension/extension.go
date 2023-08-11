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
	"strings"

	"github.com/coocood/freecache"
	"github.com/traas-stack/holoinsight-collector/internal/utils"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

type authExtension struct {
	cfg    *Config
	logger *zap.Logger
	cache  *freecache.Cache
}

const (
	Authentication             = "authentication"
	ExtendAuthenticationPrefix = "extend"
	ExtendTags                 = "extend_tags"
	GrpcMetadataTenant         = "tenant"
	CacheExpire                = 60 * 60 // 1 hour
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
	apikey := authHeaders[0]
	var err error
	if e.cfg.Enable && e.cfg.SecretKey != "" {
		apikey, err = AesDecrypt(authHeaders[0], e.cfg.SecretKey, e.cfg.IV)
		if err != nil {
			e.logger.Debug("[httpforwarderauthextension] aes decrypt error: ", zap.Error(err))
		}
	}

	// extend{"authentication":"xx", "custom_tag1":"xx", "custom_tag2":"xx"}
	// authentication is required, custom tags will be added to span tags
	if strings.HasPrefix(apikey, ExtendAuthenticationPrefix) {
		split := strings.Split(apikey, ExtendAuthenticationPrefix)
		m := make(map[string]string)
		err = json.Unmarshal([]byte(split[1]), &m)
		if err != nil {
			e.logger.Error("[httpforwarderauthextension] extend authentication unmarshal error: ", zap.Error(err))
			return nil, err
		}
		apikey = m[Authentication]
		delete(m, Authentication)
		ctx = context.WithValue(ctx, ExtendTags, m)
	}

	// Get from cache
	value, _ := e.cache.Get([]byte(apikey))
	if len(value) != 0 {
		ctx = context.WithValue(ctx, GrpcMetadataTenant, string(value))
		newCtx := metadata.NewIncomingContext(ctx, headers)
		return newCtx, nil
	}

	// Http get
	response, err := utils.HTTPGet(e.cfg.URL + "?apikey=" + apikey)
	if err != nil {
		e.logger.Error("[httpforwarderauthextension] authentication check error: ", zap.Error(err))
		return ctx, errCheckErrAuthentication
	}

	if len(response) == 0 {
		e.logger.Warn(fmt.Sprintf("[httpforwarderauthextension] authentication %s permission denied!", apikey))
		return ctx, errAuthenticationPermissionDenied
	}

	m := make(map[string]any)
	err = json.Unmarshal(response, &m)
	if err != nil {
		e.logger.Error("[httpforwarderauthextension] authentication unmarshal error: ", zap.Error(err))
		return nil, err
	}

	err = e.cache.Set([]byte(authHeaders[0]), []byte(m[GrpcMetadataTenant].(string)), CacheExpire)
	if err != nil {
		e.logger.Error("[httpforwarderauthextension] cache error: ", zap.Error(err))
		return ctx, errCheckErrAuthentication
	}

	ctx = context.WithValue(ctx, GrpcMetadataTenant, m[GrpcMetadataTenant].(string))
	newCtx := metadata.NewIncomingContext(ctx, headers)
	return newCtx, nil
}
