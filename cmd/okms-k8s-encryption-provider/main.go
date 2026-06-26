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
	"os"
	"time"

	"okms-k8s-encryption-provider/internal"
	"okms-k8s-encryption-provider/pkg/kmip"
	"okms-k8s-encryption-provider/pkg/rest"
	"okms-k8s-encryption-provider/pkg/validate"
)

var version = "dev"

// envVars maps each flag name to its environment variable fallback.
var envVars = []struct{ flag, env string }{
	{"protocol", "OKMS_PROTOCOL"},
	{"serv-addr", "OKMS_SERV_ADDR"},
	{"sock", "OKMS_SOCK"},
	{"timeout", "OKMS_TIMEOUT"},
	{"okms-id", "OKMS_ID"},
	{"encryption-key-id", "OKMS_KEY_ID"},
	{"encryption-key-label", "OKMS_KEY_LABEL"},
	{"client-cert", "OKMS_CLIENT_CERT"},
	{"client-key", "OKMS_CLIENT_KEY"},
	{"ca", "OKMS_CA_CERT"},
	{"debug", "OKMS_DEBUG"},
}

// applyEnvVars sets flag values from environment variables for any flag that
// was not explicitly provided on the command line.
func applyEnvVars() error {
	explicitly := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicitly[f.Name] = true })

	for _, m := range envVars {
		if explicitly[m.flag] {
			continue
		}
		if val, ok := os.LookupEnv(m.env); ok {
			if err := flag.Set(m.flag, val); err != nil {
				return fmt.Errorf("invalid value for %s (%s): %v", m.env, m.flag, err)
			}
		}
	}
	return nil
}

type flagGroup struct {
	header string
	note   string
	names  []string
}

var flagGroups = []flagGroup{
	{
		header: "SERVER",
		names:  []string{"protocol", "serv-addr", "sock", "timeout"},
	},
	{
		header: "KEY",
		note:   "provide exactly one of --encryption-key-id or --encryption-key-label",
		names:  []string{"encryption-key-id", "encryption-key-label"},
	},
	{
		header: "REST ONLY (required when --protocol rest)",
		names:  []string{"okms-id"},
	},
	{
		header: "AUTHENTICATION",
		names:  []string{"client-cert", "client-key", "ca"},
	},
	{
		header: "ADVANCED",
		names:  []string{"debug", "version"},
	},
}

func printFlagLine(f *flag.Flag, env string) {
	typeName, usage := flag.UnquoteUsage(f)
	line := fmt.Sprintf("  --%s", f.Name)
	if typeName != "" {
		line += " " + typeName
	}
	line += "\n        " + usage
	if env != "" {
		line += fmt.Sprintf(" [env: %s]", env)
	}
	if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" && f.DefValue != "0s" {
		line += fmt.Sprintf(" (default %q)", f.DefValue)
	}
	fmt.Fprintln(os.Stderr, line)
}

func usage() {
	// build env lookup map
	envOf := map[string]string{}
	for _, m := range envVars {
		envOf[m.flag] = m.env
	}

	fmt.Fprintf(os.Stderr, "okms-k8s-encryption-provider - Kubernetes KMS plugin for OVHcloud KMS\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n  okms-k8s-encryption-provider --protocol <rest|kmip> --serv-addr <addr> [flags]\n\n  Run with -h or --help to list all flags.\n\n")

	for _, g := range flagGroups {
		fmt.Fprintf(os.Stderr, "%s\n", g.header)
		for _, name := range g.names {
			f := flag.Lookup(name)
			if f == nil {
				continue
			}
			printFlagLine(f, envOf[name])
		}
		if g.note != "" {
			fmt.Fprintf(os.Stderr, "  * %s\n", g.note)
		}
		fmt.Fprintln(os.Stderr)
	}

	fmt.Fprintf(os.Stderr, `Examples:
  # KMIP protocol
  okms-k8s-encryption-provider \
    --protocol kmip \
    --serv-addr eu-west-rbx.okms.ovh.net:5696 \
    --encryption-key-id <key-id> \
    --client-cert /path/to/client.crt \
		--client-key /path/to/client.key \
		--ca /path/to/ca.crt

  # REST protocol
  okms-k8s-encryption-provider \
    --protocol rest \
    --serv-addr https://eu-west-rbx.okms.ovh.net \
    --okms-id <okms-domain-id> \
    --encryption-key-id <key-id> \
    --client-cert /path/to/client.crt \
		--client-key /path/to/client.key \
		--ca /path/to/ca.crt
`)
}

func main() {
	keyAttr := internal.KeyAttributes{}
	gRPCServerConfig := internal.GRPCServerConfig{}

	showVersion := flag.Bool("version", false, "Print version and exit")
	gRPCServerConfig.Protocol = flag.String("protocol", "", "Protocol to use for encryption (rest|kmip) (required)")
	gRPCServerConfig.ServAddr = flag.String("serv-addr", "", "Address of the encryption server (required)")
	gRPCServerConfig.SockPath = flag.String("sock", "/var/run/okms_etcd_plugin.sock", "Path to the Unix socket")
	gRPCServerConfig.Timeout = flag.Duration("timeout", 10*time.Second, "Timeout for the gRPC server")
	keyAttr.KeyId = flag.String("encryption-key-id", "", "ID of the encryption key to use")
	keyAttr.KeyLabel = flag.String("encryption-key-label", "", "Label of the encryption key to use")
	gRPCServerConfig.OkmsId = flag.String("okms-id", "", "ID of your OKMS domain")
	gRPCServerConfig.TlsConfig.ClientCertPath = flag.String("client-cert", "", "Path to the client certificate file")
	gRPCServerConfig.TlsConfig.ClientKeyPath = flag.String("client-key", "", "Path to the client key file")
	gRPCServerConfig.TlsConfig.CACertPath = flag.String("ca", "", "Path to a PEM-encoded CA certificate used to verify the KMS server certificate")
	debug := flag.Bool("debug", false, "Activate debug traces")

	flag.Usage = usage
	flag.Parse()

	if len(os.Args) == 1 {
		usage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("okms-k8s-encryption-provider %s\n", version)
		os.Exit(0)
	}

	if err := applyEnvVars(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\nRun '%s --help' for usage.\n", err, os.Args[0])
		os.Exit(1)
	}

	// Validate
	err := validate.ValidateFlags(gRPCServerConfig, keyAttr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\nRun '%s --help' for usage.\n", err, os.Args[0])
		os.Exit(1)
	}

	switch *gRPCServerConfig.Protocol {
	case "kmip":
		kmip.KmipEncryption(gRPCServerConfig, keyAttr, debug)
	case "rest":
		rest.RestEncryption(gRPCServerConfig, keyAttr, debug)
	}
}
