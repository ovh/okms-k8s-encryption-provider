// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kmip

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"

	"github.com/ovh/kmip-go"
	"github.com/ovh/kmip-go/kmipclient"
	"github.com/ovh/kmip-go/ttlv"
	"k8s.io/kms/pkg/service"

	"okms-k8s-encryption-provider/internal"
)

const (
	ANNOT_IV  = "iv.okms.ovh.com"
	ANNOT_ALG = "alg.okms.ovh.com"
)

var cryptoParamsPreset = map[string]kmip.CryptographicParameters{
	"AES_GCM": {
		CryptographicAlgorithm: kmip.CryptographicAlgorithmAES,
		BlockCipherMode:        kmip.BlockCipherModeGCM,
		TagLength:              16,
		IVLength:               12,
	},
}

type KmipService struct {
	client   *kmipclient.Client
	keyID    string
	keyLabel string
	alg      string
}

var _ service.Service = (*KmipService)(nil)

func NewKmipService(addr string, kmipKey internal.KeyAttributes, opts ...kmipclient.Option) (*KmipService, error) {
	slog.Info("Create a new KMIP client")
	client, err := kmipclient.Dial(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("Could not connect to KMIP server")
	}
	if ttlv.CompareVersions(client.Version(), kmip.V1_2) < 0 {
		return nil, fmt.Errorf("unsupported KMIP version: %s", client.Version())
	}

	keyId, err := retrieveKmipKeyId(client, kmipKey)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve kmip key id: %#v", err)
	}
	kmipService := &KmipService{
		client:   client,
		keyID:    keyId,
		keyLabel: *kmipKey.KeyLabel,
		alg:      "AES_GCM", // Only supported scheme for now
	}

	_, err = kmipService.Validate()
	if err != nil {
		return nil, err
	}

	return kmipService, nil
}

func retrieveKmipKeyId(client *kmipclient.Client, kmipKey internal.KeyAttributes) (string, error) {
	if kmipKey.KeyLabel != nil && *kmipKey.KeyLabel != "" {
		locateResp, err := client.Locate().WithName(*kmipKey.KeyLabel).Exec()
		if err != nil {
			return "", err
		}
		if locateResp == nil || len(locateResp.UniqueIdentifier) == 0 {
			return "", fmt.Errorf("no key ID found for label %q", *kmipKey.KeyLabel)
		}

		return locateResp.UniqueIdentifier[0], nil
	}
	return *kmipKey.KeyId, nil
}

func getCryptoParams(alg string) (kmip.CryptographicParameters, error) {
	cryptoParams, ok := cryptoParamsPreset[alg]
	if !ok {
		return kmip.CryptographicParameters{}, fmt.Errorf("unsupported algorithm: %s", alg)
	}
	return cryptoParams, nil
}

// Decrypt implements service.Service.
func (k *KmipService) Decrypt(ctx context.Context, uid string, req *service.DecryptRequest) ([]byte, error) {
	slog.Info("Decrypting content")
	alg := k.alg
	if a, ok := req.Annotations[ANNOT_ALG]; ok && a != nil {
		alg = string(a)
	}
	cryptoParams, err := getCryptoParams(alg)
	if err != nil {
		slog.Error("Decryption failed while trying to retrieve cryptographic parameters", "err", err, "ctx", ctx)
		return nil, err
	}

	bld := k.client.Decrypt(req.KeyID).
		WithCryptographicParameters(cryptoParams).
		WithIvCounterNonce(req.Annotations[ANNOT_IV])

	if taglen := int(cryptoParams.TagLength); taglen > 0 && ttlv.CompareVersions(k.client.Version(), kmip.V1_4) >= 0 {
		bld = bld.WithAuthTag(req.Ciphertext[len(req.Ciphertext)-taglen:])
		req.Ciphertext = req.Ciphertext[:len(req.Ciphertext)-taglen]
	}

	resp, err := bld.Data(req.Ciphertext).ExecContext(ctx)
	if err != nil {
		slog.Error("Decryption failed, the kmip client returned an error", "err", err, "ctx", ctx)
		return nil, err
	}
	return resp.Data, nil
}

// Encrypt implements service.Service.
func (k *KmipService) Encrypt(ctx context.Context, uid string, data []byte) (*service.EncryptResponse, error) {
	slog.Info("Encrypting content")
	cryptoParams, err := getCryptoParams(k.alg)
	if err != nil {
		slog.Error("Encryption failed while trying to retrieve cryptographic parameters", "err", err, "ctx", ctx)
		return nil, err
	}

	var iv []byte
	if ivlen := cryptoParams.IVLength; ivlen > 0 {
		iv = make([]byte, ivlen)
		_, err := rand.Read(iv)
		if err != nil {
			slog.Error("Encryption failed, could not parse the IV ", "err", err, "ctx", ctx)

			return nil, err
		}
	}

	encResp, err := k.client.Encrypt(k.keyID).
		WithCryptographicParameters(cryptoParams).
		WithIvCounterNonce(iv).
		Data(data).
		ExecContext(ctx)
	if err != nil {
		slog.Error("Encryption failed, the kmip client returned an error", "err", err, "ctx", ctx)
		return nil, err
	}

	// Append authenticated tag to the end of ciphertext
	ciphertext := encResp.Data
	ciphertext = append(ciphertext, encResp.AuthenticatedEncryptionTag...)

	resp := &service.EncryptResponse{
		Ciphertext: ciphertext,
		KeyID:      k.keyID,
		Annotations: map[string][]byte{
			ANNOT_ALG: []byte(k.alg),
		},
	}
	if len(iv) > 0 {
		resp.Annotations[ANNOT_IV] = iv
	}
	return resp, nil
}

// Status implements service.Service.
func (k *KmipService) Status(ctx context.Context) (*service.StatusResponse, error) {
	slog.Info("Checking status")

	ok, err := k.Validate()
	if !ok || err != nil {
		slog.Error("Provider status : KO", "err", err, "ctx", ctx)
		return &service.StatusResponse{
			Version: "v2",
			Healthz: "ko",
			KeyID:   k.keyID,
		}, err
	}

	return &service.StatusResponse{
		Version: "v2",
		Healthz: "ok",
		KeyID:   k.keyID,
	}, nil
}

func (k *KmipService) Close() error {
	slog.Info("Closing")
	if k.client != nil {
		return k.client.Close()
	}
	return nil
}

func (k *KmipService) Validate() (bool, error) {
	// In case of service key label rotation
	keyID, err := retrieveKmipKeyId(k.client, internal.KeyAttributes{
		KeyId:    &k.keyID,
		KeyLabel: &k.keyLabel,
	})
	if err != nil {
		return false, err
	}
	getKey, err := k.client.Get(keyID).Exec()
	if err != nil {
		return false, err
	}
	if getKey == nil {
		return false, fmt.Errorf("No key found with the keyID : %v", k.keyID)
	}
	k.keyID = keyID

	keyAttributes, err := k.client.GetAttributes(k.keyID, kmip.AttributeNameCryptographicAlgorithm, kmip.AttributeNameCryptographicUsageMask).Exec()
	if err != nil {
		return false, err
	}
	if keyAttributes == nil || len(keyAttributes.Attribute) != 2 {
		return false, fmt.Errorf("CryptographicAttributes should be set to AES for this keyID: %v", k.keyID)
	}

	for _, attr := range keyAttributes.Attribute {
		switch attr.AttributeName {
		case kmip.AttributeNameCryptographicAlgorithm:
			alg, ok := attr.AttributeValue.(kmip.CryptographicAlgorithm)
			if !ok || alg != kmip.CryptographicAlgorithmAES {
				return false, fmt.Errorf("CryptographicAttributes should be set to AES for this keyID: %v", k.keyID)
			}

		case kmip.AttributeNameCryptographicUsageMask:
			usg, ok := attr.AttributeValue.(kmip.CryptographicUsageMask)
			if !ok || usg&kmip.CryptographicUsageDecrypt == 0 || usg&kmip.CryptographicUsageEncrypt == 0 {
				return false, fmt.Errorf("CryptographicUsageMask should be set to encrypt/decrypt for this keyID: %v", k.keyID)
			}
		default:
			return false, fmt.Errorf("Unexpected attribute %v received when validating the key", attr.AttributeName)
		}
	}

	return true, nil
}
