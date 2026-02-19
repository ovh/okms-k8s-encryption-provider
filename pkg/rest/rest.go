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
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/kms/pkg/service"

	keyAttr "okms-k8s-encryption-provider/internal"
)

func RestEncryption(restAddr, clientCert, clientKey, okmsId, sockPath string, serviceKey keyAttr.KeyAttributes, timeout time.Duration, debug bool) {
	svc, err := NewRestAPIService(restAddr, clientCert, clientKey, okmsId, serviceKey, debug)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	server := service.NewGRPCService(sockPath, timeout, svc)
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
