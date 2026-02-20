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
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/ovh/okms-sdk-go"
	"github.com/ovh/okms-sdk-go/types"
	"k8s.io/kms/pkg/service"

	"okms-k8s-encryption-provider/internal"
)

type RestAPIService struct {
	client          *okms.Client
	okmsUUID        uuid.UUID
	serviceKeyId    string
	serviceKeyUUID  uuid.UUID
	serviceKeyLabel *string
}

func NewRestAPIService(gRPCServerConfig internal.GRPCServerConfig, serviceKey internal.KeyAttributes, debug *bool) (*RestAPIService, error) {
	slog.Info("Create a new Rest API client")

	// Client configuration
	clientCfg, err := configureClientWithMTLS(
		*gRPCServerConfig.TlsConfig.ClientCertPath,
		*gRPCServerConfig.TlsConfig.ClientKeyPath,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create rest api client: %w", err)
	}
	if debug != nil && *debug {
		clientCfg.Middleware = okms.DebugTransport(os.Stderr)
	}

	// Request rest API client
	restClient, err := okms.NewRestAPIClient(*gRPCServerConfig.ServAddr, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("could not create rest api client: %w", err)
	}
	if restClient == nil {
		return nil, errors.New("could not create rest api client: retrieved nil rest api client")
	}

	// Build rest API service struct
	restAPIService, err := buildRestAPIService(restClient, *gRPCServerConfig.OkmsId, serviceKey)
	if err != nil {
		return nil, fmt.Errorf("could not create rest api client: %w", err)
	}

	// Validate rest API service
	_, err = restAPIService.Validate()
	if err != nil {
		return nil, err
	}

	return restAPIService, nil
}

func buildRestAPIService(restClient *okms.Client, okmsId string, serviceKey internal.KeyAttributes) (*RestAPIService, error) {
	okmsUUID, err := uuid.Parse(okmsId)
	if err != nil {
		return nil, err
	}
	serviceKeyId, serviceKeyUUID, err := retrieveServiceKeyId(restClient, okmsUUID, serviceKey)
	if err != nil {
		return nil, err
	}
	restAPIService := &RestAPIService{
		client:          restClient,
		okmsUUID:        okmsUUID,
		serviceKeyId:    serviceKeyId,
		serviceKeyUUID:  serviceKeyUUID,
		serviceKeyLabel: serviceKey.KeyLabel,
	}

	return restAPIService, nil
}

func retrieveServiceKeyId(restClient *okms.Client, okmsUUID uuid.UUID, serviceKey internal.KeyAttributes) (string, uuid.UUID, error) {
	if serviceKey.KeyLabel != nil && *serviceKey.KeyLabel != "" {
		var counter int
		var keyId openapi_types.UUID
		activeState := types.KeyStatesActive
		iter := restClient.ListAllServiceKeys(okmsUUID, nil, &activeState).Iter(context.Background())
		for key, err := range iter {
			if err != nil {
				return "", uuid.UUID{}, err
			}
			if key.Name == *serviceKey.KeyLabel {
				counter++
				keyId = key.Id
			}
		}
		if counter > 1 {
			return "", uuid.UUID{}, fmt.Errorf("Multiple service keys (%d) share the same label", counter)
		} else if counter == 0 {
			return "", uuid.UUID{}, nil
		}

		return keyId.String(), keyId, nil
	}

	serviceKeyUUID, err := uuid.Parse(*serviceKey.KeyId)
	return *serviceKey.KeyId, serviceKeyUUID, err
}

func configureClientWithMTLS(clientCertPath, clientKeyPath string) (okms.ClientConfig, error) {
	clientCertBytes, err := os.ReadFile(clientCertPath)
	if err != nil {
		return okms.ClientConfig{}, err
	}
	clientKeyBytes, err := os.ReadFile(clientKeyPath)
	if err != nil {
		return okms.ClientConfig{}, err
	}
	tlsCert, err := tls.X509KeyPair(clientCertBytes, clientKeyBytes)
	if err != nil {
		return okms.ClientConfig{}, err
	}
	clientCfg := okms.ClientConfig{
		TlsCfg: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{tlsCert},
		},
	}

	return clientCfg, nil
}

// Decrypt implements service.Service.
func (r *RestAPIService) Decrypt(ctx context.Context, uid string, req *service.DecryptRequest) ([]byte, error) {
	slog.Info("Decrypting content")

	keyId, err := uuid.Parse(req.KeyID)
	if err != nil {
		return nil, err
	}
	decryptedData, err := r.client.Decrypt(ctx, r.okmsUUID, keyId, "", string(req.Ciphertext))
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
	serviceKeyId, serviceKeyUUID, err := retrieveServiceKeyId(r.client, r.okmsUUID, internal.KeyAttributes{
		KeyId:    &r.serviceKeyId,
		KeyLabel: r.serviceKeyLabel,
	})

	if _, err := r.client.GetServiceKey(context.Background(), r.okmsUUID, serviceKeyUUID, nil); err != nil {
		slog.Error("Provider status : KO", "err", err)
		return false, err
	}

	r.serviceKeyId = serviceKeyId
	r.serviceKeyUUID = serviceKeyUUID

	return true, err
}
