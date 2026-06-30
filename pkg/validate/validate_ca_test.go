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
	"strings"
	"testing"

	"okms-k8s-encryption-provider/internal"
)

func TestValidateFlags_WithOptionalCA(t *testing.T) {
	setup()

	protocol := "kmip"
	servAddr := "localhost:5696"
	okmsID := "11111111-1111-1111-1111-111111111111"
	keyID := "3d588782-dbe5-40ad-852b-78f029ae88db"
	clientCert := "./build/dummy_cert.pem"
	clientKey := "./build/dummy_key.pem"
	caCert := "./build/dummy_cert.pem"

	cfg := internal.GRPCServerConfig{
		Protocol: &protocol,
		ServAddr: &servAddr,
		OkmsId:   &okmsID,
		TlsConfig: internal.TlsConfig{
			ClientCertPath: &clientCert,
			ClientKeyPath:  &clientKey,
			CACertPath:     &caCert,
		},
	}
	attrs := internal.KeyAttributes{KeyId: &keyID}

	if err := ValidateFlags(cfg, attrs); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateFlags_InvalidCA(t *testing.T) {
	setup()

	protocol := "kmip"
	servAddr := "localhost:5696"
	okmsID := "11111111-1111-1111-1111-111111111111"
	keyID := "3d588782-dbe5-40ad-852b-78f029ae88db"
	clientCert := "./build/dummy_cert.pem"
	clientKey := "./build/dummy_key.pem"
	caCert := "./build/dummy_key.pem"

	cfg := internal.GRPCServerConfig{
		Protocol: &protocol,
		ServAddr: &servAddr,
		OkmsId:   &okmsID,
		TlsConfig: internal.TlsConfig{
			ClientCertPath: &clientCert,
			ClientKeyPath:  &clientKey,
			CACertPath:     &caCert,
		},
	}
	attrs := internal.KeyAttributes{KeyId: &keyID}

	err := ValidateFlags(cfg, attrs)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Could not load CA certificate from --ca") {
		t.Fatalf("expected --ca error, got %v", err)
	}
}
