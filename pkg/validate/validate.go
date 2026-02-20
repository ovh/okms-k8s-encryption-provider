// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package validate

import (
	"crypto/tls"
	"fmt"

	"okms-k8s-encryption-provider/internal"
)

func ValidateFlags(gRPCServerConfig internal.GRPCServerConfig, keyAttr internal.KeyAttributes) error {
	if err := validateProtocol(gRPCServerConfig.Protocol, gRPCServerConfig.OkmsId); err != nil {
		return err
	}
	if err := validateMTLS(gRPCServerConfig.TlsConfig.ClientCertPath, gRPCServerConfig.TlsConfig.ClientKeyPath); err != nil {
		return err
	}
	if err := validateEncryptionKey(keyAttr); err != nil {
		return err
	}
	if err := validateServerAddress(gRPCServerConfig.ServAddr); err != nil {
		return err
	}

	return nil
}

func validateProtocol(protocol, okmsId *string) error {
	if protocol == nil {
		return fmt.Errorf("Missing protocol: protocol")
	}

	switch *protocol {
	case "rest":
		if okmsId == nil || *okmsId == "" {
			return fmt.Errorf("Missing okmsId: okms-id")
		}
	case "kmip":
		// nothing to do
	default:
		return fmt.Errorf("Invalid protocol: %s", *protocol)
	}
	return nil
}

func validateMTLS(clientCert, clientKey *string) error {
	if clientCert == nil || *clientCert == "" {
		return fmt.Errorf("Missing client certificate: client-cert")
	}
	if clientKey == nil || *clientKey == "" {
		return fmt.Errorf("Missing client key: client-key")
	}

	_, err := tls.LoadX509KeyPair(*clientCert, *clientKey)
	if err != nil {
		return fmt.Errorf("Could not load certificate: %v", err)
	}
	return nil
}

// validateEncryptionKey checks whether a key ID or key label was provided.
// It returns an error if neither or both are set.
func validateEncryptionKey(keyAttr internal.KeyAttributes) error {
	if (keyAttr.KeyId == nil || *keyAttr.KeyId == "") &&
		(keyAttr.KeyLabel == nil || *keyAttr.KeyLabel == "") {
		return fmt.Errorf("Missing required key: encryption-key-id | encryption-key-label")
	} else if keyAttr.KeyId != nil && *keyAttr.KeyId != "" &&
		keyAttr.KeyLabel != nil && *keyAttr.KeyLabel != "" {
		return fmt.Errorf("Encryption key conflict: only one key parameter allowed (encryption-key-id | encryption-key-label)")
	}
	return nil
}

func validateServerAddress(servAddr *string) error {
	if servAddr == nil || *servAddr == "" {
		return fmt.Errorf("Missing address of the encryption server: serv-addr")
	}
	return nil
}
