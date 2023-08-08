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

type Config struct {
	URL string `mapstructure:"url"`
	// You can choose whether to encrypt the apikey (the configuration provided to the agent)
	// Example: For skywalking agent SW_AGENT_AUTHENTICATION configuration item
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
