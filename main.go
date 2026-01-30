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
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ovh/kmip-go"
	"github.com/ovh/kmip-go/kmipclient"
	"github.com/ovh/kmip-go/ttlv"
	"k8s.io/kms/pkg/service"
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

func getCryptoParams(alg string) (kmip.CryptographicParameters, error) {
	cryptoParams, ok := cryptoParamsPreset[alg]
	if !ok {
		return kmip.CryptographicParameters{}, fmt.Errorf("unsupported algorithm: %s", alg)
	}
	return cryptoParams, nil
}

func main() {
	sockPath := flag.String("sock", "/var/run/okms_etcd_plugin.sock", "Path to the Unix socket")
	timeout := flag.Duration("timeout", 10*time.Second, "Timeout for the gRPC server")
	kmipAddr := flag.String("kmip-addr", "", "Address of the KMIP server")
	keyId := flag.String("kmip-key-id", "", "ID of the encryption key to use")
	clientCert := flag.String("client-cert", "", "Path to the client certificate file")
	clientKey := flag.String("client-key", "", "Path to the client key file")
	debug := flag.Bool("debug", false, "Activate debug traces")

	flag.Parse()

	// Validate
	err := ValidateFlags(kmipAddr, keyId, clientCert, clientKey)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	opts := []kmipclient.Option{
		kmipclient.WithClientCertFiles(*clientCert, *clientKey),
	}
	if *debug {
		opts = append(opts, kmipclient.WithMiddlewares(
			kmipclient.DebugMiddleware(os.Stderr, ttlv.MarshalXML),
		))
	}

	svc, err := NewKmipService(*kmipAddr, *keyId, opts...)
	if err != nil {
		slog.Error("Could not create a KMIP Service", "err", err)
		os.Exit(1)
	}
	defer svc.Close()

	server := service.NewGRPCService(*sockPath, *timeout, svc)
	defer server.Close()
	go func() {
		slog.Info("Listening...")
		if err := server.ListenAndServe(); err != nil {
			slog.Error("GRPC could not listen", "err", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	slog.Info("Shutting down...")
	server.Shutdown()
}
