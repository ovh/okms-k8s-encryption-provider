//go:build integration

// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package integration_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ovh/kmip-go/kmipclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	kmsapi "k8s.io/kms/apis/v2"
	"k8s.io/kms/pkg/service"

	"okms-k8s-encryption-provider/internal"
	"okms-k8s-encryption-provider/pkg/kmip"
	"okms-k8s-encryption-provider/pkg/rest"
)

// skipIfMissing skips the test when any of the listed environment variables are unset or empty.
func skipIfMissing(t *testing.T, vars ...string) {
	t.Helper()
	for _, v := range vars {
		if os.Getenv(v) == "" {
			t.Skipf("skipping integration test: env var %s not set", v)
		}
	}
}

// writeTempCert base64-decodes the value of envVar and writes it to a temp file.
// Returns the file path. The file is removed when the test ends.
func writeTempCert(t *testing.T, envVar string) string {
	t.Helper()
	raw := os.Getenv(envVar)
	data, err := base64.StdEncoding.DecodeString(raw)
	require.NoError(t, err, "failed to base64-decode %s", envVar)
	f, err := os.CreateTemp(t.TempDir(), "okms-cert-*")
	require.NoError(t, err)
	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// testEncryptDecryptRoundTrip verifies that encrypting a plaintext and then decrypting
// the result returns the original bytes. Works with any service.Service implementation.
func testEncryptDecryptRoundTrip(t *testing.T, svc service.Service) {
	t.Helper()
	ctx := context.Background()
	plaintext := []byte("integration-test-secret-data")

	encResp, err := svc.Encrypt(ctx, "integ-uid-enc", plaintext)
	require.NoError(t, err)
	require.NotEmpty(t, encResp.Ciphertext)
	require.NotEmpty(t, encResp.KeyID)

	decrypted, err := svc.Decrypt(ctx, "integ-uid-dec", &service.DecryptRequest{
		KeyID:      encResp.KeyID,
		Ciphertext: encResp.Ciphertext,
	})
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestREST_PAT tests the REST service using Personal Access Token authentication.
func TestREST_PAT(t *testing.T) {
	skipIfMissing(t, "OKMS_REST_SERV_ADDR", "OKMS_REST_ID", "OKMS_REST_KEY_ID", "OKMS_REST_ACCESS_TOKEN")

	servAddr := os.Getenv("OKMS_REST_SERV_ADDR")
	okmsId := os.Getenv("OKMS_REST_ID")
	keyId := os.Getenv("OKMS_REST_KEY_ID")
	token := os.Getenv("OKMS_REST_ACCESS_TOKEN")
	debug := false

	svc, err := rest.NewRestAPIService(internal.GRPCServerConfig{
		ServAddr:    &servAddr,
		OkmsId:      &okmsId,
		AccessToken: &token,
	}, internal.KeyAttributes{KeyId: &keyId}, &debug)
	require.NoError(t, err)

	testEncryptDecryptRoundTrip(t, svc)
}

// TestREST_MTLS tests the REST service using mutual TLS certificate authentication.
func TestREST_MTLS(t *testing.T) {
	skipIfMissing(t, "OKMS_REST_SERV_ADDR", "OKMS_REST_ID", "OKMS_REST_KEY_ID",
		"OKMS_CLIENT_CERT_B64", "OKMS_CLIENT_KEY_B64")

	certPath := writeTempCert(t, "OKMS_CLIENT_CERT_B64")
	keyPath := writeTempCert(t, "OKMS_CLIENT_KEY_B64")

	servAddr := os.Getenv("OKMS_REST_SERV_ADDR")
	okmsId := os.Getenv("OKMS_REST_ID")
	keyId := os.Getenv("OKMS_REST_KEY_ID")
	debug := false

	svc, err := rest.NewRestAPIService(internal.GRPCServerConfig{
		ServAddr: &servAddr,
		OkmsId:   &okmsId,
		TlsConfig: internal.TlsConfig{
			ClientCertPath: &certPath,
			ClientKeyPath:  &keyPath,
		},
	}, internal.KeyAttributes{KeyId: &keyId}, &debug)
	require.NoError(t, err)

	testEncryptDecryptRoundTrip(t, svc)
}

// TestKMIP_MTLS tests the KMIP service using mutual TLS certificate authentication.
func TestKMIP_MTLS(t *testing.T) {
	skipIfMissing(t, "OKMS_KMIP_SERV_ADDR", "OKMS_KMIP_KEY_ID",
		"OKMS_CLIENT_CERT_B64", "OKMS_CLIENT_KEY_B64")

	certPath := writeTempCert(t, "OKMS_CLIENT_CERT_B64")
	keyPath := writeTempCert(t, "OKMS_CLIENT_KEY_B64")

	servAddr := os.Getenv("OKMS_KMIP_SERV_ADDR")
	keyId := os.Getenv("OKMS_KMIP_KEY_ID")
	emptyLabel := "" // NewKmipService dereferences KeyLabel; must not be nil

	svc, err := kmip.NewKmipService(
		servAddr,
		internal.KeyAttributes{KeyId: &keyId, KeyLabel: &emptyLabel},
		kmipclient.WithClientCertFiles(certPath, keyPath),
	)
	require.NoError(t, err)
	t.Cleanup(func() { svc.Close() })

	testEncryptDecryptRoundTrip(t, svc)
}

// TestKubernetesGRPCProtocol starts the full KMS gRPC server backed by a real OKMS domain
// and exercises it via the same gRPC protocol that the Kubernetes API server uses.
func TestKubernetesGRPCProtocol(t *testing.T) {
	skipIfMissing(t, "OKMS_REST_SERV_ADDR", "OKMS_REST_ID", "OKMS_REST_KEY_ID", "OKMS_REST_ACCESS_TOKEN")

	servAddr := os.Getenv("OKMS_REST_SERV_ADDR")
	okmsId := os.Getenv("OKMS_REST_ID")
	keyId := os.Getenv("OKMS_REST_KEY_ID")
	token := os.Getenv("OKMS_REST_ACCESS_TOKEN")
	debug := false

	// 1. Create the real REST service.
	restSvc, err := rest.NewRestAPIService(internal.GRPCServerConfig{
		ServAddr:    &servAddr,
		OkmsId:      &okmsId,
		AccessToken: &token,
	}, internal.KeyAttributes{KeyId: &keyId}, &debug)
	require.NoError(t, err)

	// 2. Start it behind the KMS gRPC server on a Unix socket.
	// Use a short path under os.TempDir() to stay well within the 108-char Unix socket limit.
	sockPath := filepath.Join(os.TempDir(), fmt.Sprintf("okms-integ-%d.sock", os.Getpid()))
	t.Cleanup(func() { os.Remove(sockPath) })

	grpcServer := service.NewGRPCService(sockPath, 30*time.Second, restSvc)
	go func() { _ = grpcServer.ListenAndServe() }()
	t.Cleanup(grpcServer.Shutdown)

	// 3. Connect like Kubernetes does: gRPC over a Unix socket, no TLS on the transport.
	conn, err := grpc.NewClient(
		sockPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", addr)
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	kmsClient := kmsapi.NewKeyManagementServiceClient(conn)

	// 4. Wait for the server to be ready (mirrors how kubelet polls the plugin).
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.Eventually(t, func() bool {
		resp, err := kmsClient.Status(ctx, &kmsapi.StatusRequest{})
		return err == nil && resp.GetHealthz() == "ok"
	}, 30*time.Second, 500*time.Millisecond, "KMS gRPC server did not become ready in time")

	// 5. Encrypt then decrypt via the gRPC protocol — the full Kubernetes round-trip.
	plaintext := []byte("kubernetes-integration-test-secret")

	encResp, err := kmsClient.Encrypt(ctx, &kmsapi.EncryptRequest{
		Uid:       "k8s-test-uid-enc",
		Plaintext: plaintext,
	})
	require.NoError(t, err)
	require.NotEmpty(t, encResp.GetCiphertext())
	require.NotEmpty(t, encResp.GetKeyId())

	decResp, err := kmsClient.Decrypt(ctx, &kmsapi.DecryptRequest{
		Uid:         "k8s-test-uid-dec",
		Ciphertext:  encResp.GetCiphertext(),
		KeyId:       encResp.GetKeyId(),
		Annotations: encResp.GetAnnotations(),
	})
	require.NoError(t, err)
	assert.Equal(t, plaintext, decResp.GetPlaintext())
}
