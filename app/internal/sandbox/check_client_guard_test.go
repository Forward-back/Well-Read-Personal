package sandbox

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// writeDockerCheckTLS writes a throwaway ca/cert/key trio so the check client
// can be built for a TCP daemon. The certificates only have to satisfy
// client.WithTLSClientConfig; no connection is made over TLS because the dial
// guard refuses the target first.
func writeDockerCheckTLS(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "gaze-docker-check-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ca.pem"), certPEM, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cert.pem"), certPEM, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "key.pem"), keyPEM, 0o600))
	return dir
}

// The connectivity probe dials a daemon endpoint an admin typed into an unsaved
// form, so it is the one docker client whose target is fully admin-influenced.
// Save-time validation cannot cover it: a hostname can resolve to a public
// address while the form is validated and to the cloud metadata service by the
// time the probe connects. The dial guard must therefore refuse link-local even
// under the private-endpoint opt-in that self-hosted daemons need.
func TestNewRemoteClientForCheckRefusesLinkLocalAtDial(t *testing.T) {
	isolateDockerEnabled(t)
	t.Setenv(DockerBackendEnabledEnv, "true")

	client, err := NewRemoteClientForCheck(&Config{
		Type:                  SandboxTypeDocker,
		AllowPrivateEndpoints: true,
		DockerImage:           "gaze-sandbox:test",
		DockerHost:            "tcp://169.254.169.254:2376",
		DockerTLSCertPath:     writeDockerCheckTLS(t),
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.Health(ctx)
	require.Error(t, err, "probing the metadata service must fail")
	// The guard rejects before any packet leaves, so the refusal - not a
	// connection timeout - is what has to surface.
	require.True(t,
		strings.Contains(err.Error(), ErrUnsafeOutboundURL.Error()) ||
			strings.Contains(err.Error(), "never routable"),
		"expected the outbound guard to refuse the dial, got: %v", err)
}
