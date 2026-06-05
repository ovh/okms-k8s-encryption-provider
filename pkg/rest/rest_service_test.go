// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package rest

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/ovh/okms-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/kms/pkg/service"

	"okms-k8s-encryption-provider/internal"
)

const (
	testOkmsId = "11111111-1111-1111-1111-111111111111"
	testKeyId  = "22222222-2222-2222-2222-222222222222"
	testToken  = "my-secret-pat-token"
)

// testHandler returns a handler that routes by URL suffix and records the Authorization header.
func testHandler(capturedAuth *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capturedAuth != nil {
			*capturedAuth = r.Header.Get("Authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/encrypt"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"ciphertext":"test-ciphertext"}`)
		case strings.HasSuffix(r.URL.Path, "/decrypt"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"plaintext":"dGVzdA=="}`) // base64("test")
		default:
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"id":%q,"attributes":null}`, testKeyId)
		}
	})
}

// newTestService creates a RestAPIService backed by the given httptest server and optionally
// configured with a PAT token.
func newTestService(t *testing.T, handler http.Handler, accessToken string) *RestAPIService {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	client, err := okms.NewRestAPIClientWithHttp(server.URL, server.Client())
	require.NoError(t, err)

	if accessToken != "" {
		client.SetCustomHeader("Authorization", "Bearer "+accessToken)
	}

	okmsId := uuid.MustParse(testOkmsId)
	keyId := uuid.MustParse(testKeyId)
	return &RestAPIService{
		client:         client,
		okmsUUID:       okmsId,
		serviceKeyId:   testKeyId,
		serviceKeyUUID: keyId,
	}
}

func TestConfigureClientWithPAT(t *testing.T) {
	cfg := configureClientWithPAT()

	require.NotNil(t, cfg.TlsCfg)
	assert.Empty(t, cfg.TlsCfg.Certificates, "PAT config must not include client certificates")
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.TlsCfg.MinVersion)
}

func TestPATHeaderSentOnRequests(t *testing.T) {
	var capturedAuth string
	svc := newTestService(t, testHandler(&capturedAuth), testToken)

	_, _ = svc.client.GetServiceKey(context.Background(), svc.okmsUUID, svc.serviceKeyUUID, nil)

	assert.Equal(t, "Bearer "+testToken, capturedAuth)
}

func TestNoAuthHeaderWithoutPAT(t *testing.T) {
	var capturedAuth string
	svc := newTestService(t, testHandler(&capturedAuth), "")

	_, _ = svc.client.GetServiceKey(context.Background(), svc.okmsUUID, svc.serviceKeyUUID, nil)

	assert.Empty(t, capturedAuth)
}

func TestEncrypt(t *testing.T) {
	svc := newTestService(t, testHandler(nil), testToken)

	resp, err := svc.Encrypt(context.Background(), "uid-1", []byte("plaintext"))

	require.NoError(t, err)
	assert.Equal(t, []byte("test-ciphertext"), resp.Ciphertext)
	assert.Equal(t, testKeyId, resp.KeyID)
}

func TestDecrypt(t *testing.T) {
	svc := newTestService(t, testHandler(nil), testToken)

	req := &service.DecryptRequest{
		KeyID:      testKeyId,
		Ciphertext: []byte("some-jwe-ciphertext"),
	}
	plaintext, err := svc.Decrypt(context.Background(), "uid-1", req)

	require.NoError(t, err)
	assert.Equal(t, []byte("test"), plaintext) // base64("dGVzdA==") = "test"
}

func TestStatus_OK(t *testing.T) {
	svc := newTestService(t, testHandler(nil), testToken)

	resp, err := svc.Status(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Healthz)
	assert.Equal(t, "v2", resp.Version)
	assert.Equal(t, testKeyId, resp.KeyID)
}

func TestStatus_Error(t *testing.T) {
	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	})
	svc := newTestService(t, errorHandler, testToken)

	resp, err := svc.Status(context.Background())

	assert.Error(t, err)
	assert.Equal(t, "ko", resp.Healthz)
	assert.Equal(t, "v2", resp.Version)
}

func TestNewRestAPIService_PAT(t *testing.T) {
	var capturedAuth string
	server := httptest.NewTLSServer(testHandler(&capturedAuth))
	t.Cleanup(server.Close)

	token := testToken
	okmsId := testOkmsId
	keyId := testKeyId
	keyAttr := internal.KeyAttributes{KeyId: &keyId}

	// NewRestAPIService creates a real TLS client — point it at the test server using its cert pool.
	// We need InsecureSkipVerify since the test cert is not trusted by the system pool.
	origCfg := configureClientWithPAT()
	origCfg.TlsCfg.InsecureSkipVerify = true
	testClient, err := okms.NewRestAPIClient(server.URL, origCfg)
	require.NoError(t, err)
	testClient.SetCustomHeader("Authorization", "Bearer "+token)

	svc, err := buildRestAPIService(testClient, okmsId, keyAttr)
	require.NoError(t, err)

	_, _ = svc.Validate()
	assert.Equal(t, "Bearer "+testToken, capturedAuth)
}
