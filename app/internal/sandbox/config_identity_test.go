package sandbox

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gaze/internal/types"
)

// Identity comes from the stored row alone. Nothing about the deployment can
// change it, which is what makes "did this edit strand a sandbox" answerable
// without reaching for process-level sandbox configuration.
func TestIdentityOfReadsStoredFieldsOnly(t *testing.T) {
	identity := IdentityOf(&types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker: &types.DockerSandboxConfig{
			Image:       "gaze-sandbox:test",
			Host:        "tcp://203.0.113.10:2376",
			TLSCertPath: "/certs",
		},
	})

	// The daemon endpoint is both planes at once for Docker, so the host maps
	// onto APIURL and the certificate directory onto APIKey.
	require.Equal(t, SandboxIdentity{
		Provider: "docker",
		APIURL:   "tcp://203.0.113.10:2376",
		APIKey:   "/certs",
	}, identity)
}

// A blank field is blank, not "whatever the deployment runs". Two configs that
// differ only in what they spell out are therefore different identities.
func TestIdentityOfDoesNotInherit(t *testing.T) {
	blank := IdentityOf(&types.TenantSandboxConfig{SandboxType: "docker"})
	spelledOut := IdentityOf(&types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker: &types.DockerSandboxConfig{
			Image: "gaze-sandbox:test",
			Host:  "unix:///var/run/docker.sock",
		},
	})

	require.Equal(t, SandboxIdentity{Provider: "docker"}, blank)
	require.NotEqual(t, blank, spelledOut)
	require.Equal(t, "unix:///var/run/docker.sock", spelledOut.APIURL)
}

// Only the active provider describes where the sandboxes are; sub-structs left
// behind by an earlier provider switch say nothing about today's sandboxes.
func TestIdentityOfReadsOnlyActiveProvider(t *testing.T) {
	cfg := &types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker:      &types.DockerSandboxConfig{Host: "unix:///var/run/docker.sock"},
		E2B:         &types.E2BSandboxConfig{APIKey: "stale-e2b-key"},
		Cube:        &types.CubeSandboxConfig{APIKey: "stale-cube-key"},
	}

	identity := IdentityOf(cfg)

	require.Equal(t, "unix:///var/run/docker.sock", identity.APIURL)
	require.Empty(t, identity.ProxyURL, "a removed provider's data plane is irrelevant")
}

// Backends that hold no remote resources carry no credentials, but switching
// between them still has to register as a change.
func TestIdentityOfDockerWithoutHostCarriesProviderOnly(t *testing.T) {
	docker := IdentityOf(&types.TenantSandboxConfig{SandboxType: "docker"})
	disabled := IdentityOf(&types.TenantSandboxConfig{SandboxType: "disabled"})

	require.Equal(t, SandboxIdentity{Provider: "docker"}, docker)
	require.NotEqual(t, docker, disabled)
}

// A nil config must not panic: Update compares against it when creating.
func TestIdentityOfToleratesNilConfig(t *testing.T) {
	require.Equal(t, SandboxIdentity{}, IdentityOf(nil))
}
