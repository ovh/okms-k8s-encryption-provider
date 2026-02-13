# okms-k8s-encryption-provider

[![Go Reference](https://pkg.go.dev/badge/github.com/ovh/okms-k8s-encryption-provider.svg)](https://pkg.go.dev/github.com/ovh/okms-k8s-encryption-provider)
[![license](https://img.shields.io/badge/license-Apache%202.0-red.svg?style=flat)](https://raw.githubusercontent.com/ovh/okms-k8s-encryption-provider/master/LICENSE)
[![test](https://github.com/ovh/okms-k8s-encryption-provider/actions/workflows/test.yaml/badge.svg)](https://github.com/ovh/okms-k8s-encryption-provider/actions/workflows/test.yaml)

## 📖 Overview

`okms-k8s-encryption-provider` is an implementation of the kube-apiserver [encryption provider](https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/) enabling Kubernetes clusters to encrypt/decrypt data at rest using **OVHcloud KMS** either through the **KMIP** or the **REST** protocol. The plugin implements the `kms/v2` interface required by etcd and forwards encryption requests to a KMIP‑compatible server (OVHcloud KMS or any other KMIP server) or a REST API compatible with the OKMS Service Key API, depending on the selected protocol.

## 🚀 Features

- **Transparent encryption** for etcd data‑blocks via the standard `kms/v2` interface.
- **KMIP 1.0‑1.4 support** – works with OVHcloud KMS out‑of‑the‑box.
- **REST API support** – works with OVHcloud KMS out‑of‑the‑box.
- **Mutual TLS authentication** (client certificates) – no passwords stored in the cluster.
- **Stateless design** – the plugin does not store any secret locally; all cryptographic material stays in the KMS.

## 📦 Installation

The binary can be installed directly from go packages.

```bash
go install github.com/ovh/okms-k8s-encryption-provider@latest
```

Or you can build from sources.

```bash
git clone https://github.com/ovh/okms-k8s-encryption-provider.git
cd okms-k8s-encryption-provider
go build -o okms-k8s-encryption-provider ./cmd/okms-k8s-encryption-provider
```

## 🏃 Uses with Kubernetes

### Pre-requisites

To start using the OVHcloud KMS as an encryption provider for kubernetes you will need the following:

- An OVHcloud account with a [Key Management system (KMS)](https://www.ovh.com/manager/#/okms/key-management-service) and permissions to manage KMS KMIP keys and KMS Service Keys
- Management access to a Kubernetes API server
- Access certificate for your KMS domain
- A KMIP AES Key in your KMS. You can create one using the [okms-cli](https://github.com/ovh/okms-cli)
- An AES Service Key, with encrypt,decrypt operations set, in your KMS. You can create one using the [okms-cli](https://github.com/ovh/okms-cli)

We recommend you to read the following documentation beforehand:

- Kubernetes documentation page [Encrypting Secret Data at Rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/)
- Kubernetes documentation page [Using a KMS provider for data encryption](https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/)

### ⚙️ Provider's Configuration

The encryption provider can be run on the kube-apiserver hosts directly with the following command line:

KMIP protocol:
```bash
./okms-k8s-encryption-provider \
-protocol "kmip" \
-client-cert "~/.ovh-kms/cert.pem" \
-client-key "~/.ovh-kms/key.pem" \ 
-serv-addr "eu-west-par.okms.ovh.net:5696" \  # kmip server
-encryption-key-id "70001308-5674-43fe-93dd-6270ecac0710" # kmip key id
```

REST protocol:
```bash
./okms-k8s-encryption-provider \
-protocol "rest" \
-client-cert "~/.ovh-kms/cert.pem" \
-client-key "~/.ovh-kms/key.pem" \ 
-serv-addr "https://eu-west-rbx.okms.ovh.net" \  # okms addr
-okms-id "113d1c44-2b1d-239c-a929-c11bd1847057" \
-encryption-key-id "70001308-5674-43fe-93dd-6270ecac0710" # service key id
```

Where `cert.pem` and `key.pem` are your access certificate to your OKMS.

| Flag | Description | Default |
|------|-------------|---------|
| `--protocol` | Protocol to use. Either "kmip" or "rest". | `""` (required) |
| `--sock` | Path to the Unix socket the provider will listen on. Should be mounted inside the Kubernetes apiserver | `/var/run/okms_etcd_plugin.sock` |
| `--timeout` | Timeout for the gRPC server operations. | `10s` |
| `--serv-addr` | Address of the encryption server. Can be found in the [OVHcloud manager](https://www.ovh.com/manager) page of your KMS. (e.g `eu-west-rbx.okms.ovh.net:5696`, `https://eu-west-rbx.okms.ovh.net`) | `""` (required) |
| `--encryption-key-id` | Identifier of the encryption key to use on the KMIP/REST server. | `""` (required) |
| `--okms-id` | Identifier of your OKMS. | `""` (required if protocol="rest") |
| `--client-cert` | Path to the client certificate file for [mutual TLS authentication with the KMS](https://help.ovhcloud.com/csm/en-gb-okms-certificate-management?id=kb_article_view&sysparm_article=KB0072599). | `""` (required) |
| `--client-key` | Path to the private key file associated with the client certificate. | `""` (required)|
| `--debug` | Activate debug traces. | `false` |

### ⚙️ Kubernetes' configuration

Based on the [official Kubernetes guide for encrypting data with a KMS provider](https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/#encrypting-your-data-with-the-kms-provider) add the following flags on your kube-apiserver:

```bash
  --encryption-provider-config=<path/to>/encryption-config.yaml
  # Optional, reload the file if it is updated
  --encryption-provider-config-automatic-reload=true
```

Don't forget you'll need to mount the directory containing the unix socket that
the KMS server is listening on into the kube-apiserver.

An example of `encryption-config.yaml`:

```yaml
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
    - secrets
    providers:
    - kms:
        name: okms-encryption-provider
        endpoint: unix:///var/run/okms_etcd_plugin.sock
        cachesize: 1000
        timeout: 3s
    - identity: {}
```

### Check that the provider plugin is working

- First create a secret: `kubectl create secret generic okms-test-secret -n default --from-literal=mykey=mydata`
- Then check the contents of the secret in etcd store by running the following:

```bash
ETCDCTL_API=3 etcdctl \
    --key /rootfs/etc/kubernetes/pki/kube-apiserver/etcd-client.key \
    --cert  /rootfs/etc/kubernetes/pki/kube-apiserver/etcd-client.crt \
    --cacert /rootfs/etc/kubernetes/pki/kube-apiserver/etcd-ca.crt  \
    --endpoints "https://etcd-a.internal.${CLUSTER}:4001" get /registry/secrets/default/okms-test-secret
```

The output should be something like:

```bash
0m`�He.0�cryption-provider:�1x��%�B���#JP��J���*ȝ���΂@\n�96�^��ۦ�~0| *�H��
                    `q�*�J�.P��;&~��o#�O�8m��->8L��0�C3���A7�����~���f�V�ܬ���X��_��`�H#�D��z)+�81��qW��y��`�q��}1<LF, ��N��p����i*�aC#E�߸�s������s��l�?�a
�AźR������.��8H�4�O
```

### Rotation

To rotate your key you will need to run two encryption providers, each listening on a different unix socket.
Below is an example encryption configuration file for all API servers prior to using the new key.

```yaml
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
    - secrets
    providers:
    # provider using old key
    - kms:
        name: okms-encryption-provider
        endpoint: unix:///var/run/kmsplugin/socket.sock
        cachesize: 1000
        timeout: 3s
    # provider using new key
    - kms:
        name: okms-encryption-provider-2
        endpoint: unix:///var/run/kmsplugin/socket2.sock
        cachesize: 1000
        timeout: 3s
    - identity: {}
```

After all API servers have been restarted and are able to decrypt using the
new key, move the provider with the new key on top.
After all secrets have been re-encrypted with the new key, you can remove the old encryption provider.

## 📖 Documentation

- **KMIP protocol** – see the [OASIS KMIP v1.4 specification](https://docs.oasis-open.org/kmip/spec/v1.4/os/kmip-spec-v1.4-os.html).
- **OVHcloud KMS** – [official documentation](https://help.ovhcloud.com/csm/en-ie-kms-quick-start?id=kb_article_view&sysparm_article=KB0063362).
- **etcd KMS provider interface** – [kms provider for etcd](https://etcd.io/docs/v3.5/dev-guide/kms/)
- **Kubernetes encryption provider** -  [Using a KMS provider for data encryption](https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/)

## 🛠️ Development & Contributing

1. Fork the repository.
2. Create a feature branch.
3. Run `go test -race ./...` to ensure everything passes.
4. Submit a Pull Request.

Please follow the existing code style and run `go fmt ./...` before committing.

## 📄 License

Apache License 2.0 – see the [LICENSE](LICENSE) file.
