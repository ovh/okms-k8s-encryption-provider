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
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	privateKey string = "-----BEGIN PRIVATE KEY-----\n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\ndushR3h733tAdYeILd1NMkEGxmPUhhR9\n-----END PRIVATE KEY-----"
	publicKey  string = "-----BEGIN CERTIFICATE-----\nMIIFCDCCAvCgAwIBAgIUe4wDXTwgX0M41GohSIUI1+dLc4MwDQYJKoZIhvcNAQEN\nBQAwFjEUMBIGA1UEAwwLZXhhbXBsZS5jb20wHhcNMjUxMjA1MDkyOTQ3WhcNMzUx\nMjAzMDkyOTQ3WjAWMRQwEgYDVQQDDAtleGFtcGxlLmNvbTCCAiIwDQYJKoZIhvcN\nAQEBBQADggIPADCCAgoCggIBAMc2JCYpr1Ym6ArhYMVxaQJEGxavWpTW1zrDxFCb\nPhs/cKHTCy6RzYtAgpNE8t2saR76OEbCEN7Vo2KTXu1D/7bg4t9UJMdISMh4WsSJ\n5557ES4Uf6T9OZN85XnaBaMujPfNMp6dhglbcmQFTLx+chxQOE6QYMah4BWOSWso\nyuZ3AXktfRQXwYYiGCU8a6VAklSB69vOpTtJ3JOGAJTOxs4zpfVuL08lSWqTQHMF\nsNOG/tilnNKPvcvgtgcTgkhngn6cU/Rc0+NvJTDMqKf4hScQ5Lh0P5Xk/ocyK3Lj\nI9JMTM9HUifLUDUC4tGlwp1uBW0ppQIbj4BKm/erz07H75ikM2vZaIdydI/xEK8d\ncyAuILriP8u2Q2nc+JbZEI6pq7syHkdmgXCsex+N3vJuhUGZIR0wlMBS5cGwQUat\nP7pW2+wZCo2MzN3eXFxhyaOXuUU6ZVDbFMoxq5YswdCu4mQjOXtKdnSCoMD5Thti\nJuyKAu58l2kNs9HZHt4NC+SWA6fZFqGIfQ4My+HnIVVRfIsEGSlithklgsVTNM/U\nXyeDUOVPl1d8Kn7QNKcmm6/Gg4SpV9SubWEujkSAoxm+j6YDpj5xNTPm9vmGpqBE\nXY8c1Om/3OFauJjznWNB/ef0lKFqdgDUveYZbBVpMH/VxpA72f6GlI010tuNpwfr\n44apAgMBAAGjTjBMMCsGA1UdEQQkMCKCC2V4YW1wbGUuY29tgg0qLmV4YW1wbGUu\nY29thwQKAAABMB0GA1UdDgQWBBQ/YYLAY7pgJQVe0d3+RWJr+chgLzANBgkqhkiG\n9w0BAQ0FAAOCAgEARNXzTQR5HY0fgi0bHinLdGGNjO1iIpfwAJOx5+aRyA7rIA0e\n48hdHOrrUX5OOAWfOFA9gaZCC5cF0dyxOsFt1MvDc3mVS6qtSiOSHlfvQdR9cpf5\ncZL1FfcHzJHcbn/HolfGpeG44K+abAX8u+87Ic1MsDJOWahZrtXjY/pL4VKmur1V\nWNYjdDuzI++WNq09uVQmsEFmN/KIYl1iRU1H5p3sU9KiWB4NzXVBY1UdWoy0mvlG\nxXSHb6Cc+9Mb3uCAYYVEoOf3AchnkG/DSExJF9H6xG2osD/WmU0Eaib29rQ1IJ+s\n5KxOTsD9WhyR0ByFcw5sv5C18gmU9PCPFsCP8n0nFl6rAvTIkgeDYOCsnNnatzHs\nxs8lNHMFOT7M5208+OBI2ytr/HLqpOf/c8oFWIiEw2iQarQGgYbYFofzGfr75l9B\nyCelTKva2BJkfBgoI1W64aEE/hnV/Vcfmb18FEMWcbiQHWCo9eAhk5TsYepG2MPr\nveS/cJ0KUw+f+nYd/5Agd0z04APJg06r3DjgdJxuQij1HMKQRVJbZgLc3Exud/XO\nR7hlV0uK0Kgvn4yUwLOUW0PnT+1Se3s0oDTaKR3woBgHXvEeRxyp80oMqlKLKxb0\n6uH+colrcJaOLWrnGjqKm8h4AF60z+icpvtjPuRHIS9LrUViZmceD626tm0=\n-----END CERTIFICATE-----"
)

func setup() {
	err := os.Mkdir("./build/", 0700)
	if err != nil {
		slog.Warn(err.Error())
	}
	err = os.WriteFile("./build/dummy_cert.pem", []byte(publicKey), 0755)
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile("./build/dummy_key.pem", []byte(privateKey), 0755)
	if err != nil {
		log.Fatal(err)
	}
}

func TestValidateFlags(t *testing.T) {
	setup()
	defaultKmipAddr := "localhost:5696"
	defaultkeyId := "3d588782-dbe5-40ad-852b-78f029ae88db"
	defaultClientCert := "./build/dummy_cert.pem"
	defaultClientKey := "./build/dummy_key.pem"

	tcs := []struct {
		name          string
		kmipAddr      string
		keyId         string
		clientCert    string
		clientKey     string
		expectedError error
	}{
		{
			name:          "Everything OK",
			kmipAddr:      defaultKmipAddr,
			keyId:         defaultkeyId,
			clientCert:    defaultClientCert,
			clientKey:     defaultClientKey,
			expectedError: nil,
		},
		{
			name:          "kmip addr missing",
			kmipAddr:      "",
			keyId:         defaultkeyId,
			clientCert:    defaultClientCert,
			clientKey:     defaultClientKey,
			expectedError: fmt.Errorf("Missing address of the KMIP server: kmip-addr"),
		},
		{
			name:          "key id missing",
			kmipAddr:      defaultKmipAddr,
			keyId:         "",
			clientCert:    defaultClientCert,
			clientKey:     defaultClientKey,
			expectedError: fmt.Errorf("Missing key Id : kmip-key-id"),
		},
		{
			name:          "cert  missing",
			kmipAddr:      defaultKmipAddr,
			keyId:         defaultkeyId,
			clientCert:    "",
			clientKey:     defaultClientKey,
			expectedError: fmt.Errorf("Missing certificates: client-cert, client-key"),
		},
		{
			name:          "wrong cert",
			kmipAddr:      defaultKmipAddr,
			keyId:         defaultkeyId,
			clientCert:    defaultClientKey,
			clientKey:     defaultClientKey,
			expectedError: fmt.Errorf("Could not load certificate: tls: failed to find certificate PEM data in certificate input, but did find a private key; PEM inputs may have been switched"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateFlags(&tc.kmipAddr, &tc.keyId, &tc.clientCert, &tc.clientKey)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
