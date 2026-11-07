import assert from 'node:assert/strict'
import test from 'node:test'

import {
  SETTINGS_SECTION_CAPABILITY,
  isDeploymentCapabilitySupported,
  type DeploymentCapabilityMap,
} from './deploymentCapabilities'

test('capability filtering is fail-open unless backend explicitly disables a feature', () => {
  assert.equal(isDeploymentCapabilitySupported({}, 'agents'), true)

  const capabilities: DeploymentCapabilityMap = {
    agents: { supported: false, reason: 'disabled_by_deployment' },
  }
  assert.equal(isDeploymentCapabilitySupported(capabilities, 'agents'), false)
})

test('only route-backed settings sections require deployment capabilities', () => {
  assert.equal(SETTINGS_SECTION_CAPABILITY.mcp, undefined)
  assert.equal(SETTINGS_SECTION_CAPABILITY.storage, 'settings.storage')
  assert.equal(SETTINGS_SECTION_CAPABILITY.parser, undefined)
  assert.equal(SETTINGS_SECTION_CAPABILITY.vectorstore, undefined)
  assert.equal(SETTINGS_SECTION_CAPABILITY['runtime-queues'], undefined)
})

test('skill credentials follow the sandbox capability rather than a key of their own', () => {
  // The values are injected into a skill script's process, so a deployment with
  // no sandbox support has nowhere to put them and the page could only ever show
  // its empty state.
  assert.equal(SETTINGS_SECTION_CAPABILITY.envvars, 'settings.sandbox')
  assert.equal(SETTINGS_SECTION_CAPABILITY.envvars, SETTINGS_SECTION_CAPABILITY.sandbox)
})



test('docker sandbox stays hidden unless the deployment explicitly enables it', () => {
  assert.equal(isDeploymentCapabilitySupported({}, 'settings.sandbox.docker'), false)
  assert.equal(
    isDeploymentCapabilitySupported(
      { 'settings.sandbox.docker': { supported: false, reason: 'docker_backend_disabled' } },
      'settings.sandbox.docker',
    ),
    false,
  )
  assert.equal(
    isDeploymentCapabilitySupported(
      { 'settings.sandbox.docker': { supported: true } },
      'settings.sandbox.docker',
    ),
    true,
  )
})
