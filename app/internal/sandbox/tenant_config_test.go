package sandbox

import (
	"testing"
	"time"

	"gaze/internal/types"
	"github.com/stretchr/testify/require"
)

func globalTestConfig() *Config {
	cfg := &Config{
		Type:           SandboxTypeDocker,
		DefaultTimeout: 60 * time.Second,
		DockerImage:    "global-image",
		DockerHost:     "unix:///var/run/global.sock",
		// Credentials of providers the personal edition no longer ships. A
		// named config must never inherit them.
		E2BAPIKey:        "global-key",
		E2BAPIURL:        "https://203.0.113.30",
		E2BSandboxDomain: "global.domain",
		E2BTemplate:      "global-template",
		E2BSandboxTTL:    10 * time.Minute,
		E2BHTTPTimeout:   30 * time.Second,
		CubeAPIURL:       "https://203.0.113.40",
		CubeTemplate:     "global-cube-template",
	}
	cfg.Network = resolveNetworkPolicy(nil)
	return cfg
}

// completeDockerTenantConfig is the minimum a named Docker config must carry
// now that nothing is inherited.
func completeDockerTenantConfig() *types.TenantSandboxConfig {
	return &types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker: &types.DockerSandboxConfig{
			Image: "tenant-image",
			Host:  "unix:///var/run/docker.sock",
		},
	}
}

// A nil tenant config means "the deployment default backend", which is the one
// path where the baseline is used as-is.
func TestResolveEffectiveConfigNilTenantKeepsGlobal(t *testing.T) {
	global := globalTestConfig()

	got, err := ResolveEffectiveConfig(nil, global)

	require.NoError(t, err)
	require.Equal(t, *global, *got)
}

func TestResolveEffectiveConfigNilTenantMaterializesNetworkDefaults(t *testing.T) {
	global := &Config{Type: SandboxTypeDisabled, DefaultTimeout: time.Second}

	got, err := ResolveEffectiveConfig(nil, global)

	require.NoError(t, err)
	require.NotNil(t, got.Network.AllowInternetAccess)
	require.True(t, *got.Network.AllowInternetAccess)
	require.NotNil(t, got.Network.AllowPublicTraffic)
	require.False(t, *got.Network.AllowPublicTraffic)
}

func TestDefaultConfigMaterializesNetworkDefaults(t *testing.T) {
	got := DefaultConfig()

	require.NotNil(t, got.Network.AllowInternetAccess)
	require.True(t, *got.Network.AllowInternetAccess)
	require.NotNil(t, got.Network.AllowPublicTraffic)
	require.False(t, *got.Network.AllowPublicTraffic)
}

func TestResolveEffectiveConfigDoesNotMutateGlobal(t *testing.T) {
	global := globalTestConfig()

	_, err := ResolveEffectiveConfig(completeDockerTenantConfig(), global)

	require.NoError(t, err)
	require.Equal(t, "global-image", global.DockerImage,
		"resolution must not leak tenant values into the shared global config")
	require.Equal(t, "unix:///var/run/global.sock", global.DockerHost)
	require.Equal(t, "global-key", global.E2BAPIKey)
}

// The point of the whole design: a named config never picks up the deployment's
// image, endpoint or any provider credential, so what it does not state it does
// not get.
func TestResolveEffectiveConfigDoesNotInheritProviderFields(t *testing.T) {
	got, err := ResolveEffectiveConfig(completeDockerTenantConfig(), globalTestConfig())

	require.NoError(t, err)
	require.Equal(t, "tenant-image", got.DockerImage)
	require.Equal(t, "unix:///var/run/docker.sock", got.DockerHost)
	require.Empty(t, got.DockerTLSCertPath)
	require.Empty(t, got.E2BAPIURL, "the baseline's e2b endpoint must not ride along")
	require.Empty(t, got.E2BTemplate)
	require.Empty(t, got.CubeAPIURL, "the baseline's cube endpoint must not ride along")
	require.Empty(t, got.CubeTemplate)
}

func TestResolveEffectiveConfigTranslatesNetworkPolicy(t *testing.T) {
	tenantCfg := completeDockerTenantConfig()
	tenantCfg.Network = &types.SandboxNetworkPolicy{
		DenyEgressByDefault: true,
		AllowPublicInbound:  false,
		AllowOut:            []string{"*.example.com"},
		DenyOut:             []string{"0.0.0.0/0"},
		E2BHostRules: []types.E2BHostRule{{
			Host:    "api.example.com",
			Headers: map[string]string{"X-Key": "secret"},
		}},
	}

	got, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())
	require.NoError(t, err)

	require.NotNil(t, got.Network.AllowInternetAccess)
	require.False(t, *got.Network.AllowInternetAccess,
		"DenyEgressByDefault must flip the provider's allow switch off")
	require.NotNil(t, got.Network.AllowPublicTraffic)
	require.False(t, *got.Network.AllowPublicTraffic,
		"inbound is always credential-required")
	require.Equal(t, []string{"*.example.com"}, got.Network.AllowOut)
	require.Equal(t, []string{"0.0.0.0/0"}, got.Network.DenyOut)
	require.Equal(t, "api.example.com", got.Network.E2BHostRules[0].Host)
	require.Equal(t, "secret", got.Network.E2BHostRules[0].Headers["X-Key"])
}

// A domain allow-list must be paired with an explicit deny-all entry for the
// translated policy to be self-consistent: the stored DenyEgressByDefault means
// exactly "install a 0.0.0.0/0 deny-all", and leaving it implied would let an
// adapter validate allow and deny independently and refuse the create.
func TestResolveEffectiveConfigMaterialisesDenyAllEntry(t *testing.T) {
	tenantCfg := completeDockerTenantConfig()
	tenantCfg.Network = &types.SandboxNetworkPolicy{
		DenyEgressByDefault: true,
		AllowOut:            []string{"*.example.com"},
	}

	got, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())

	require.NoError(t, err)
	require.Equal(t, []string{"0.0.0.0/0"}, got.Network.DenyOut)
	require.True(t, got.Network.DeniesEgressByDefault())
}

func TestResolveEffectiveConfigDoesNotDuplicateDenyAllEntry(t *testing.T) {
	for _, existing := range []string{"0.0.0.0/0", "  0.0.0.0/0  ", "1.2.3.4/0"} {
		t.Run(existing, func(t *testing.T) {
			tenantCfg := completeDockerTenantConfig()
			tenantCfg.Network = &types.SandboxNetworkPolicy{
				DenyEgressByDefault: true,
				AllowOut:            []string{"*.example.com"},
				DenyOut:             []string{existing},
			}

			got, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())

			require.NoError(t, err)
			require.Equal(t, []string{types.DenyAllIPv4}, got.Network.DenyOut,
				"any IPv4 /0 must be rewritten to the canonical deny-all, not duplicated")
		})
	}
}

func TestResolveEffectiveConfigCanonicalizesNonCanonicalDenyAll(t *testing.T) {
	tenantCfg := completeDockerTenantConfig()
	tenantCfg.Network = &types.SandboxNetworkPolicy{
		AllowOut: []string{"api.example.com"},
		DenyOut:  []string{"10.0.0.0/8", "1.2.3.4/0"},
	}

	got, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())

	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.0/8", types.DenyAllIPv4}, got.Network.DenyOut)
	require.True(t, got.Network.DeniesEgressByDefault())
}

// The entry is the resolved form of DenyEgressByDefault, so an egress-open
// policy must not acquire it.
func TestResolveEffectiveConfigLeavesDenyOutAloneWhenEgressOpen(t *testing.T) {
	tenantCfg := completeDockerTenantConfig()
	tenantCfg.Network = &types.SandboxNetworkPolicy{
		DenyOut: []string{"169.254.169.254/32"},
	}

	got, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())

	require.NoError(t, err)
	require.Equal(t, []string{"169.254.169.254/32"}, got.Network.DenyOut)
	require.False(t, got.Network.DeniesEgressByDefault())
}

func TestResolveEffectiveConfigIgnoresStoredPublicInbound(t *testing.T) {
	tenantCfg := completeDockerTenantConfig()
	tenantCfg.Network = &types.SandboxNetworkPolicy{AllowPublicInbound: true}

	got, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())

	require.NoError(t, err)
	require.NotNil(t, got.Network.AllowPublicTraffic)
	require.False(t, *got.Network.AllowPublicTraffic,
		"inbound stays credential-required even if a stored row asked for public access")
}

func TestResolveEffectiveConfigDefaultsNetworkToEgressOpenInboundClosed(t *testing.T) {
	// No stored policy at all: the shipped default must still be explicit, so
	// the adapters do not have to re-derive it.
	got, err := ResolveEffectiveConfig(completeDockerTenantConfig(), globalTestConfig())
	require.NoError(t, err)

	require.NotNil(t, got.Network.AllowInternetAccess)
	require.True(t, *got.Network.AllowInternetAccess)
	require.NotNil(t, got.Network.AllowPublicTraffic)
	require.False(t, *got.Network.AllowPublicTraffic)
}

func TestResolveEffectiveConfigInvertsCubeRuleDeny(t *testing.T) {
	tenantCfg := completeDockerTenantConfig()
	tenantCfg.Network = &types.SandboxNetworkPolicy{
		CubeRules: []types.CubeEgressRule{
			{
				Name:    "allow-api",
				Scheme:  "https",
				SNI:     "api.example.com",
				Host:    "api.example.com",
				Methods: []string{"GET", "POST"},
				Path:    "/v1/*",
				Audit:   "full",
				Inject: []types.CubeHeaderInject{{
					Header: "Authorization",
					Secret: "token",
					Format: "Bearer %s",
				}},
			},
			{Name: "deny-uploads", SNI: "uploads.example.com", Deny: true},
		},
	}

	got, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())
	require.NoError(t, err)

	require.Len(t, got.Network.CubeRules, 2)
	require.True(t, got.Network.CubeRules[0].Allow,
		"a stored rule without Deny is an allow rule")
	require.Equal(t, "allow-api", got.Network.CubeRules[0].Name)
	require.Equal(t, "https", got.Network.CubeRules[0].Scheme)
	require.Equal(t, "api.example.com", got.Network.CubeRules[0].SNI)
	require.Equal(t, "api.example.com", got.Network.CubeRules[0].Host)
	require.Equal(t, []string{"GET", "POST"}, got.Network.CubeRules[0].Methods)
	require.Equal(t, "/v1/*", got.Network.CubeRules[0].Path)
	require.Equal(t, "full", got.Network.CubeRules[0].Audit)
	require.Equal(t, []RemoteHeaderInject{{
		Header: "Authorization",
		Secret: "token",
		Format: "Bearer %s",
	}}, got.Network.CubeRules[0].Inject)
	require.False(t, got.Network.CubeRules[1].Allow)
}

func TestResolveEffectiveConfigCopiesNetworkPolicyCollections(t *testing.T) {
	stored := &types.SandboxNetworkPolicy{
		AllowOut: []string{"api.example.com"},
		CubeRules: []types.CubeEgressRule{{
			Name:    "api",
			Host:    "api.example.com",
			Methods: []string{"GET"},
		}},
		E2BHostRules: []types.E2BHostRule{{
			Host:    "api.example.com",
			Headers: map[string]string{"Authorization": "stored-token"},
		}},
	}
	tenantCfg := completeDockerTenantConfig()
	tenantCfg.Network = stored

	got, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())
	require.NoError(t, err)

	got.Network.AllowOut[0] = "mutated.example.com"
	got.Network.CubeRules[0].Methods[0] = "POST"
	got.Network.E2BHostRules[0].Headers["Authorization"] = "mutated-token"

	require.Equal(t, "api.example.com", stored.AllowOut[0])
	require.Equal(t, "GET", stored.CubeRules[0].Methods[0])
	require.Equal(t, "stored-token", stored.E2BHostRules[0].Headers["Authorization"])
}

func TestResolveEffectiveConfigDoesNotInheritNetworkFromBaseline(t *testing.T) {
	// This assertion pins the resolved end state, not clearProviderFields as
	// the mechanism that produces it.
	baseline := globalTestConfig()
	inherited := true
	baseline.Network = RemoteNetworkPolicy{AllowPublicTraffic: &inherited}

	got, err := ResolveEffectiveConfig(completeDockerTenantConfig(), baseline)
	require.NoError(t, err)

	require.NotNil(t, got.Network.AllowPublicTraffic)
	require.False(t, *got.Network.AllowPublicTraffic,
		"policy belongs to the named config; the .env baseline must not leak in")
}

// A leftover sub-struct from the deployment's other provider must not survive
// either: a docker config must not answer with a removed provider's coordinates.
func TestResolveEffectiveConfigClearsInactiveProviderBaseline(t *testing.T) {
	global := globalTestConfig()

	got, err := ResolveEffectiveConfig(completeDockerTenantConfig(), global)

	require.NoError(t, err)
	require.Equal(t, SandboxTypeDocker, got.Type)
	require.Equal(t, "tenant-image", got.DockerImage)
	require.Empty(t, got.E2BAPIKey, "the baseline's e2b credentials must not ride along")
	require.Empty(t, got.E2BTemplate)
	require.Empty(t, got.CubeAPIURL)
	require.Empty(t, got.CubeTemplate)
}

func TestResolveEffectiveConfigRejectsIncompleteDocker(t *testing.T) {
	_, err := ResolveEffectiveConfig(&types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker:      &types.DockerSandboxConfig{Host: "unix:///var/run/docker.sock"},
	}, globalTestConfig())

	require.ErrorIs(t, err, ErrSandboxConfigIncomplete)
	require.Contains(t, err.Error(), "image")
}

// The remote providers were removed from this edition; a stored row that still
// names one must be rejected rather than silently switching the workspace to a
// different execution environment.
func TestResolveEffectiveConfigRejectsRemovedProviders(t *testing.T) {
	for _, raw := range []string{"e2b", "cube"} {
		t.Run(raw, func(t *testing.T) {
			_, err := ResolveEffectiveConfig(
				&types.TenantSandboxConfig{SandboxType: raw}, globalTestConfig(),
			)

			require.ErrorIs(t, err, ErrUnsupportedSandboxType)
		})
	}
}

func TestResolveEffectiveConfigAppliesTimeoutsAndTTL(t *testing.T) {
	global := globalTestConfig()
	tenantCfg := completeDockerTenantConfig()
	tenantCfg.DefaultTimeoutSec = 90
	tenantCfg.Docker.HTTPTimeoutSec = 15
	tenantCfg.Docker.IdleTTLSeconds = 600
	tenantCfg.TerminalIdleDisconnectSec = 1200

	got, err := ResolveEffectiveConfig(tenantCfg, global)

	require.NoError(t, err)
	require.Equal(t, 90*time.Second, got.DefaultTimeout)
	require.Equal(t, 15*time.Second, got.DockerHTTPTimeout)
	require.Equal(t, 600*time.Second, got.DockerIdleTTL)
	require.Equal(t, 20*time.Minute, got.TerminalIdleDisconnect)
}

func TestResolveEffectiveConfigTerminalIdleFallsBackToBuiltIn(t *testing.T) {
	global := globalTestConfig()
	global.TerminalIdleDisconnect = 3 * time.Hour

	got, err := ResolveEffectiveConfig(completeDockerTenantConfig(), global)

	require.NoError(t, err)
	require.Equal(t, DefaultTerminalIdleDisconnect, got.TerminalIdleDisconnect)
}

// Tuning fields fall back to the built-in constants, never to the deployment's:
// "inherits nothing" would be a much weaker rule with an exception here.
func TestResolveEffectiveConfigTuningFallsBackToBuiltIns(t *testing.T) {
	global := globalTestConfig()
	global.DockerIdleTTL = 10 * time.Minute
	global.DockerHTTPTimeout = 90 * time.Second

	got, err := ResolveEffectiveConfig(completeDockerTenantConfig(), global)

	require.NoError(t, err)
	require.Equal(t, DefaultDockerIdleTTL, got.DockerIdleTTL)
	require.Equal(t, DefaultDockerHTTPTimeout, got.DockerHTTPTimeout)
	require.Equal(t, global.DefaultTimeout, got.DefaultTimeout,
		"the execution timeout is deployment policy and does carry over")
}

func TestResolveEffectiveConfigDisabled(t *testing.T) {
	tenantCfg := &types.TenantSandboxConfig{SandboxType: "disabled"}

	got, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())

	require.NoError(t, err)
	require.Equal(t, SandboxTypeDisabled, got.Type)
}

func TestResolveEffectiveConfigRejectsUnknownType(t *testing.T) {
	tenantCfg := &types.TenantSandboxConfig{SandboxType: "quantum"}

	_, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())

	require.Error(t, err)
}

func TestResolveEffectiveConfigRejectsUnsafeURL(t *testing.T) {
	tenantCfg := &types.TenantSandboxConfig{
		SandboxType:           "docker",
		AllowPrivateEndpoints: true,
		Docker: &types.DockerSandboxConfig{
			Image:       "gaze-sandbox:test",
			Host:        "tcp://169.254.169.254:2376",
			TLSCertPath: "/certs",
		},
	}

	_, err := ResolveEffectiveConfig(tenantCfg, globalTestConfig())

	require.ErrorIs(t, err, ErrUnsafeOutboundURL,
		"link-local stays blocked even under the private-endpoint opt-in")
}

func TestResolveEffectiveConfigRejectsDockerHostNetwork(t *testing.T) {
	_, err := ResolveEffectiveConfig(&types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker: &types.DockerSandboxConfig{
			Image:       "gaze:test",
			NetworkMode: "host",
		},
	}, DefaultConfig())
	require.Error(t, err)
}

// A blank host must come out of config resolution already pointing at the
// daemon the Docker CLI would use, because that resolved value is what the
// connectivity check dials. Leaving it empty here is what made the check fail
// on a Colima or Docker Desktop host, where /var/run/docker.sock is absent.
func TestResolveEffectiveConfigDetectsLocalDockerHostWhenBlank(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/from-env.sock")

	effective, err := ResolveEffectiveConfig(&types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker:      &types.DockerSandboxConfig{Image: "gaze:test"},
	}, DefaultConfig())

	require.NoError(t, err)
	require.Equal(t, "unix:///tmp/from-env.sock", effective.DockerHost)
}

func TestResolveEffectiveConfigMapsDockerNoneToDeniedEgress(t *testing.T) {
	effective, err := ResolveEffectiveConfig(&types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker: &types.DockerSandboxConfig{
			Image:       "gaze:test",
			NetworkMode: "none",
		},
	}, DefaultConfig())
	require.NoError(t, err)
	require.True(t, effective.Network.DeniesEgressByDefault(),
		"docker network_mode=none is this backend's deny-all switch")
}

func TestResolveEffectiveConfigKeepsDockerBridgeEgressOpen(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///tmp/from-env.sock")
	effective, err := ResolveEffectiveConfig(&types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker: &types.DockerSandboxConfig{
			Image:       "gaze:test",
			NetworkMode: "bridge",
		},
	}, DefaultConfig())
	require.NoError(t, err)
	require.False(t, effective.Network.DeniesEgressByDefault())
}

func TestResolveEffectiveConfigRejectsPlaintextDockerTCP(t *testing.T) {
	_, err := ResolveEffectiveConfig(&types.TenantSandboxConfig{
		SandboxType:           "docker",
		AllowPrivateEndpoints: true,
		Docker: &types.DockerSandboxConfig{
			Image: "gaze:test",
			Host:  "tcp://10.0.0.5:2376",
		},
	}, DefaultConfig())
	require.Error(t, err)
}

// The same bar has to apply to a host nobody typed. A blank field is filled in
// from DOCKER_HOST, and on a deployment pointed at a plaintext daemon that
// resolved value is what gets dialled — so validating only the stored string
// would accept a config that cannot work and say so only at the first sandbox.
func TestResolveEffectiveConfigRejectsResolvedPlaintextDockerTCP(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://10.0.0.5:2375")

	_, err := ResolveEffectiveConfig(&types.TenantSandboxConfig{
		SandboxType:           "docker",
		AllowPrivateEndpoints: true,
		Docker:                &types.DockerSandboxConfig{Image: "gaze:test"},
	}, DefaultConfig())
	require.Error(t, err)
}

// A resolved host is still only a daemon endpoint, so it answers to the same
// outbound policy an explicitly typed one does.
func TestResolveEffectiveConfigRejectsResolvedPrivateDockerHostWithoutOptIn(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://10.0.0.5:2376")

	_, err := ResolveEffectiveConfig(&types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker: &types.DockerSandboxConfig{
			Image:       "gaze:test",
			TLSCertPath: "/etc/gaze/docker-certs",
		},
	}, DefaultConfig())
	require.ErrorIs(t, err, ErrUnsafeOutboundURL)
}

// A missing image is a field the admin can see and fix in the form, so it must
// still be reported ahead of anything about the daemon endpoint.
func TestResolveEffectiveConfigReportsMissingImageBeforeHostProblems(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://10.0.0.5:2375")

	_, err := ResolveEffectiveConfig(&types.TenantSandboxConfig{
		SandboxType: "docker",
	}, DefaultConfig())
	require.ErrorIs(t, err, ErrSandboxConfigIncomplete)
}

func TestEffectiveTemplateIDPerProvider(t *testing.T) {
	require.Equal(t, "img-1", EffectiveTemplateID(&Config{
		Type: SandboxTypeDocker, DockerImage: "img-1",
		E2BTemplate: "e2b-tpl", CubeTemplate: "cube-tpl",
	}), "docker's template ID is its image")
	require.Empty(t, EffectiveTemplateID(&Config{Type: SandboxTypeDisabled, DockerImage: "img-1"}))
	require.Empty(t, EffectiveTemplateID(&Config{Type: "e2b", E2BTemplate: "e2b-tpl"}),
		"removed providers expose no template")
	require.Empty(t, EffectiveTemplateID(nil))
}

func TestResolveEffectiveConfigUsesSkillSnapshotAsDockerImage(t *testing.T) {
	global := DefaultConfig()
	base := &types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker: &types.DockerSandboxConfig{
			Image: "gaze/sandbox:base",
			Host:  "unix:///var/run/docker.sock",
		},
	}
	fp := SkillImageFingerprint("docker", "", "unix:///var/run/docker.sock")

	t.Run("usable snapshot overrides the base image", func(t *testing.T) {
		cfg := *base
		cfg.Docker = &types.DockerSandboxConfig{
			Image: "gaze/sandbox:base", Host: "unix:///var/run/docker.sock",
		}
		cfg.SkillImage = &types.SkillImageConfig{
			SnapshotID: "gaze-skill/gaze-sk-cfg1-g1", OwnerFingerprint: fp,
		}

		eff, err := ResolveEffectiveConfig(&cfg, global)

		require.NoError(t, err)
		require.Equal(t, "gaze-skill/gaze-sk-cfg1-g1", eff.DockerImage)
	})

	t.Run("fingerprint mismatch falls back to the base image", func(t *testing.T) {
		cfg := *base
		cfg.Docker = &types.DockerSandboxConfig{
			Image: "gaze/sandbox:base", Host: "unix:///var/run/docker.sock",
		}
		cfg.SkillImage = &types.SkillImageConfig{
			SnapshotID: "gaze-skill/gaze-sk-cfg1-g1", OwnerFingerprint: "another-daemon",
		}

		eff, err := ResolveEffectiveConfig(&cfg, global)

		require.NoError(t, err)
		require.Equal(t, "gaze/sandbox:base", eff.DockerImage,
			"a snapshot from another daemon is invisible; the session must still boot")
	})

	// A blank host follows the environment, so binding the fingerprint to the
	// resolved daemon would make "switch docker context" indistinguishable
	// from "rotate the credentials": sessions would silently lose every
	// skill, installs would be refused, and the snapshot prune would skip the
	// config forever. The images are on the same disk either way.
	t.Run("blank host survives a change of local daemon", func(t *testing.T) {
		newConfig := func() *types.TenantSandboxConfig {
			return &types.TenantSandboxConfig{
				SandboxType: "docker",
				Docker:      &types.DockerSandboxConfig{Image: "gaze/sandbox:base"},
				SkillImage: &types.SkillImageConfig{
					SnapshotID: "gaze-skill/gaze-sk-cfg1-g1",
					OwnerFingerprint: SkillOwnerFingerprint(&types.TenantSandboxConfig{
						SandboxType: "docker",
						Docker:      &types.DockerSandboxConfig{Image: "gaze/sandbox:base"},
					}),
				},
			}
		}

		for _, host := range []string{
			"unix:///tmp/colima.sock",
			"unix:///tmp/orbstack.sock",
		} {
			t.Setenv("DOCKER_HOST", host)
			cfg := newConfig()

			eff, err := ResolveEffectiveConfig(cfg, global)

			require.NoError(t, err)
			require.Equal(t, "gaze-skill/gaze-sk-cfg1-g1", eff.DockerImage,
				"DOCKER_HOST=%s must not retire the config's skill image", host)
			require.True(t, SkillImageActive(cfg))
		}
	})

	// An explicit host is still part of the identity: only an admin editing
	// the config can change it, and it may well be another machine.
	t.Run("an explicit host change retires the image", func(t *testing.T) {
		cfg := *base
		cfg.Docker = &types.DockerSandboxConfig{
			Image: "gaze/sandbox:base", Host: "tcp://198.51.100.10:2376",
			TLSCertPath: "/certs",
		}
		cfg.SkillImage = &types.SkillImageConfig{
			SnapshotID: "gaze-skill/gaze-sk-cfg1-g1", OwnerFingerprint: fp,
		}

		eff, err := ResolveEffectiveConfig(&cfg, global)

		require.NoError(t, err)
		require.Equal(t, "gaze/sandbox:base", eff.DockerImage)
	})
}

func TestSkillOwnerFingerprintForDocker(t *testing.T) {
	require.Empty(t, SkillOwnerFingerprint(&types.TenantSandboxConfig{SandboxType: "docker"}),
		"a docker type with no daemon block cannot own a snapshot")
	require.Empty(t, SkillOwnerFingerprint(&types.TenantSandboxConfig{SandboxType: "disabled"}))

	got := SkillOwnerFingerprint(&types.TenantSandboxConfig{
		SandboxType: "docker",
		Docker: &types.DockerSandboxConfig{
			Image: "img", Host: "unix:///var/run/docker.sock", TLSCertPath: "/certs",
		},
	})
	require.Equal(t, SkillImageFingerprint("docker", "/certs", "unix:///var/run/docker.sock"), got)
}

// SkillImageActive is what the agent side asks before telling a model about an
// installed skill, so it must agree with the template ResolveEffectiveConfig
// actually boots. Any disagreement means either skills that are announced and
// cannot run, or skills that are in the image and hidden.
func TestSkillImageActiveAgreesWithTheResolvedTemplate(t *testing.T) {
	global := DefaultConfig()
	docker := func() *types.TenantSandboxConfig {
		return &types.TenantSandboxConfig{
			SandboxType: "docker",
			Docker: &types.DockerSandboxConfig{
				Image: "gaze/sandbox:base",
				Host:  "unix:///var/run/docker.sock",
			},
		}
	}
	fp := SkillImageFingerprint("docker", "", "unix:///var/run/docker.sock")

	cases := map[string]struct {
		config *types.TenantSandboxConfig
		want   bool
	}{
		"snapshot owned by the live daemon": {
			config: func() *types.TenantSandboxConfig {
				cfg := docker()
				cfg.SkillImage = &types.SkillImageConfig{
					SnapshotID: "gaze-skill/gaze-sk-cfg1-g1", OwnerFingerprint: fp,
				}
				return cfg
			}(),
			want: true,
		},
		"snapshot from another daemon": {
			config: func() *types.TenantSandboxConfig {
				cfg := docker()
				cfg.SkillImage = &types.SkillImageConfig{
					SnapshotID: "gaze-skill/gaze-sk-cfg1-g1", OwnerFingerprint: "another-daemon",
				}
				return cfg
			}(),
			want: false,
		},
		"snapshot with no recorded owner": {
			config: func() *types.TenantSandboxConfig {
				cfg := docker()
				cfg.SkillImage = &types.SkillImageConfig{SnapshotID: "gaze-skill/gaze-sk-cfg1-g1"}
				return cfg
			}(),
			want: false,
		},
		"no snapshot yet": {
			config: docker(),
			want:   false,
		},
		"backend that cannot snapshot": {
			config: &types.TenantSandboxConfig{
				SandboxType: "disabled",
				SkillImage: &types.SkillImageConfig{
					SnapshotID: "gaze-skill/gaze-sk-cfg1-g1", OwnerFingerprint: fp,
				},
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			active := SkillImageActive(tc.config)
			require.Equal(t, tc.want, active)

			eff, err := ResolveEffectiveConfig(tc.config, global)
			require.NoError(t, err)
			booted := EffectiveTemplateID(eff)
			if active {
				require.Equal(t, tc.config.SkillImage.SnapshotID, booted)
				return
			}
			if tc.config.SkillImage != nil && tc.config.SkillImage.SnapshotID != "" {
				require.NotEqual(t, tc.config.SkillImage.SnapshotID, booted,
					"a skill declared unusable must not be the image the session boots")
			}
		})
	}

	require.False(t, SkillImageActive(nil))
}

func TestSkillImageFingerprintIsStableAndDiscriminating(t *testing.T) {
	a := SkillImageFingerprint("docker", "daemon-a", "unix:///var/run/docker.sock")
	require.Equal(t, a, SkillImageFingerprint("docker", "daemon-a", "unix:///var/run/docker.sock"))
	require.NotEqual(t, a, SkillImageFingerprint("docker", "daemon-b", "unix:///var/run/docker.sock"))
	require.NotEqual(t, a, SkillImageFingerprint("disabled", "daemon-a", "unix:///var/run/docker.sock"))
}
