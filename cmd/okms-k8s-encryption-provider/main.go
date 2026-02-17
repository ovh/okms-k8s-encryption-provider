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
	"log/slog"
	"os"
	"time"

	"okms-k8s-encryption-provider/pkg/kmip"
	"okms-k8s-encryption-provider/pkg/rest"
	"okms-k8s-encryption-provider/pkg/validate"
)

func main() {
	sockPath := flag.String("sock", "/var/run/okms_etcd_plugin.sock", "Path to the Unix socket")
	timeout := flag.Duration("timeout", 10*time.Second, "Timeout for the gRPC server")
	protocol := flag.String("protocol", "", "Protocol to use for encryption (rest|kmip)")
	servAddr := flag.String("serv-addr", "", "Address of the KMIP server")
	keyId := flag.String("encryption-key-id", "", "ID of the encryption key to use")
	okmsId := flag.String("okms-id", "", "Only needed if --protocol is rest\nID of your OKMS domain")
	clientCert := flag.String("client-cert", "", "Path to the client certificate file")
	clientKey := flag.String("client-key", "", "Path to the client key file")
	debug := flag.Bool("debug", false, "Activate debug traces")

	flag.Parse()

	// Validate
	err := validate.ValidateFlags(protocol, servAddr, keyId, okmsId, clientCert, clientKey)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	switch *protocol {
	case "kmip":
		kmip.KmipEncryption(*servAddr, *clientCert, *clientKey, *keyId, *sockPath, *timeout, *debug)
	case "rest":
		rest.RestEncryption(*servAddr, *clientCert, *clientKey, *keyId, *okmsId, *sockPath, *timeout, *debug)
	}
}
