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
	"errors"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/ovh/okms-sdk-go"
	"k8s.io/kms/pkg/service"
)

type RestAPIService struct {
	client         *okms.Client
	okmsUUID       uuid.UUID
	serviceKeyId   string
	serviceKeyUUID uuid.UUID
}

func NewRestAPIService(restAddr, clientCert, clientKey, serviceKeyId, okmsId string, debug bool) (*RestAPIService, error) {
	slog.Info("Create a new Rest API client")

	// Client configuration
	clientCertBytes, err := os.ReadFile(clientCert)
	if err != nil {
		return nil, err
	}
	clientKeyBytes, err := os.ReadFile(clientKey)
	if err != nil {
		return nil, err
	}
	tlsCert, err := tls.X509KeyPair(clientCertBytes, clientKeyBytes)
	if err != nil {
		return nil, err
	}
	clientCfg := okms.ClientConfig{
		TlsCfg: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{tlsCert},
		},
	}
	if debug {
		clientCfg.Middleware = okms.DebugTransport(os.Stderr)
	}

	// Request rest API client
	restClient, err := okms.NewRestAPIClient(restAddr, clientCfg)
	if err != nil {
		return nil, err
	}
	if restClient == nil {
		return nil, errors.New("retrieved nil rest api client")
	}

	// Build rest API service struct
	okmsUUID, err := uuid.Parse(okmsId)
	if err != nil {
		return nil, err
	}
	serviceKeyUUID, err := uuid.Parse(serviceKeyId)
	if err != nil {
		return nil, err
	}
	restAPIService := &RestAPIService{
		client:         restClient,
		okmsUUID:       okmsUUID,
		serviceKeyId:   serviceKeyId,
		serviceKeyUUID: serviceKeyUUID,
	}

	// Validate rest API service
	_, err = restAPIService.Validate()
	if err != nil {
		return nil, err
	}

	return restAPIService, nil
}

// Decrypt implements service.Service.
func (r *RestAPIService) Decrypt(ctx context.Context, uid string, req *service.DecryptRequest) ([]byte, error) {
	slog.Info("Decrypting content")

	decryptedData, err := r.client.Decrypt(ctx, r.okmsUUID, r.serviceKeyUUID, "", string(req.Ciphertext))
	if err != nil {
		return nil, err
	}

	return decryptedData, nil
}

// Encrypt implements service.Service.
func (r *RestAPIService) Encrypt(ctx context.Context, uid string, data []byte) (*service.EncryptResponse, error) {
	slog.Info("Encrypting content")

	cipherText, err := r.client.Encrypt(ctx, r.okmsUUID, r.serviceKeyUUID, "", data)
	if err != nil {
		return nil, err
	}

	return &service.EncryptResponse{
		Ciphertext: []byte(cipherText),
		KeyID:      r.serviceKeyId,
	}, nil
}

// Status implements service.Service.
func (r *RestAPIService) Status(ctx context.Context) (*service.StatusResponse, error) {
	slog.Info("Checking status")

	ok, err := r.Validate()
	if !ok || err != nil {
		slog.Error("Provider status : KO", "err", err, "ctx", ctx)
		return &service.StatusResponse{
			Version: "v2",
			Healthz: "ko",
			KeyID:   r.serviceKeyId,
		}, err
	}

	return &service.StatusResponse{
		Version: "v2",
		Healthz: "ok",
		KeyID:   r.serviceKeyId,
	}, nil
}

func (r *RestAPIService) Validate() (bool, error) {
	_, err := r.client.GetServiceKey(context.Background(), r.okmsUUID, r.serviceKeyUUID, nil)

	return err == nil, err
}
