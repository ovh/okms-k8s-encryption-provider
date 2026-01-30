// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package main

import (
	"crypto/tls"
	"fmt"
)

func ValidateFlags(kmipAddr, keyId, clientCert, clientKey *string) error {
	if clientCert == nil || clientKey == nil || *clientCert == "" || *clientKey == "" {
		return fmt.Errorf("Missing certificates: client-cert, client-key")
	}
	_, err := tls.LoadX509KeyPair(*clientCert, *clientKey)
	if err != nil {
		return fmt.Errorf("Could not load certificate: %v", err)
	}

	if kmipAddr == nil || *kmipAddr == "" {
		return fmt.Errorf("Missing address of the KMIP server: kmip-addr")
	}

	if keyId == nil || *keyId == "" {
		return fmt.Errorf("Missing key Id : kmip-key-id")
	}
	return nil
}
