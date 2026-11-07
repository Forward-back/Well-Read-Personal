package router

import (
	"gaze/internal/handler"
	"gaze/internal/sandbox"
)

func deploymentCapabilitiesFromRouter(params RouterParams) handler.DeploymentCapabilitiesData {
	return handler.BuildDeploymentCapabilities(handler.Edition, handler.DeploymentFeatureAvailability{
		Agents:        false,
		IM:            false,
		API:           params.TenantAPIKeyService != nil,
		MCP:           false,
		WebSearch:     false,
		Storage:       params.StorageBackendHandler != nil,
		Sandbox:       false,
		SandboxDocker: sandbox.DockerBackendEnabled(),
	})
}
