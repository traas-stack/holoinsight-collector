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
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
)

type Config struct {
	HTTP      *confighttp.HTTPServerSettings `mapstructure:"http"`
	SLSConfig `mapstructure:"alibabacloud_logservice"`
	// If you want to encrypt the secretKey and iv of the holoinsight collector, it needs to be consistent with the holoinsight backend
	// The holoinsight backend encrypts the configuration, and the holoinsight collector decrypts it
	Decrypt `mapstructure:"decrypt"`
}

type Decrypt struct {
	// default: false
	Enable bool `mapstructure:"enable"`
	// A secret key is a piece of information that is used to aes encrypt and decrypt data in a symmetric encryption algorithm.
	SecretKey string `mapstructure:"secretKey"`
	// IV (Initialization Vector): An initialization vector is a random value that is used in conjunction with a secret key to
	// encrypt data in a symmetric encryption algorithm. It is used to ensure that the same plaintext message encrypted with
	// the same secret key produces a different ciphertext message each time it is encrypted. The IV is typically included
	// in the encrypted message and must be kept confidential to ensure the security of the encrypted data.
	IV string `mapstructure:"iv"`
}

type SLSConfig struct {
	// LogService's Endpoint, https://www.alibabacloud.com/help/doc-detail/29008.htm
	// for AlibabaCloud Kubernetes(or ECS), set {region-id}-intranet.log.aliyuncs.com, eg cn-hangzhou-intranet.log.aliyuncs.com;
	//  others set {region-id}.log.aliyuncs.com, eg cn-hangzhou.log.aliyuncs.com
	Endpoint string `mapstructure:"endpoint"`
	// LogService's Project Name
	Project string `mapstructure:"project"`
	// AlibabaCloud access key id
	AccessKeyID string `mapstructure:"access_key_id"`
	// AlibabaCloud access key secret
	AccessKeySecret configopaque.String `mapstructure:"access_key_secret"`
}
