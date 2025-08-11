/*
 * Copyright 2025 Arion Yau
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

<script lang="ts">
  import { onMount } from 'svelte';
  import { auth, apiClient } from '../lib/auth';
  import HubClaiming from './HubClaiming.svelte';

  // Props
  export let onSelectDevice: (device: any) => void = () => {};
  export let onConfigureHub: (hubId: string) => void = () => {};

  $: authState = $auth;
  $: user = authState.user;
  
  let gatewayStatus: any = null;
  let devices: any[] = [];
  let hubs: any[] = [];
  let loading = true;
  let error: string | null = null;
  let showClaimModal = false;

  $: token = $auth.token;

  async function loadData() {
    if (!token) return;
    
    loading = true;
    error = null;

    try {
      // Load gateway status and user data in parallel
      const [statusResponse, devicesResponse, hubsResponse] = await Promise.all([
        apiClient.getGatewayStatus(),
        apiClient.getUserDevices(token),
        apiClient.getUserHubs(token),
      ]);

      gatewayStatus = statusResponse;
      devices = devicesResponse.devices || [];
      hubs = hubsResponse.hubs || [];
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load data';
    } finally {
      loading = false;
    }
  }

  async function sendDeviceAction(deviceId: string, action: any) {
    if (!token) return;
    
    try {
      await apiClient.sendDeviceAction(deviceId, action, token);
      // Reload device data to reflect changes
      await loadData();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to send device action';
    }
  }

  function openClaimModal() {
    showClaimModal = true;
  }

  function closeClaimModal() {
    showClaimModal = false;
  }

  function handleHubClaimed() {
    loadData(); // Refresh the hub list
  }

  onMount(() => {
    loadData();
  });
</script>

<div class="dashboard">

  {#if loading}
    <div class="loading">Loading dashboard...</div>
  {:else if error}
    <div class="error">
      <p>Error: {error}</p>
      <button on:click={loadData}>Retry</button>
    </div>
  {:else}
    <div class="dashboard-content">
      <!-- Gateway Status Section -->
      <section class="status-section">
        <h2>Gateway Status</h2>
        {#if gatewayStatus}
          <div class="status-info">
            <div class="status-item">
              <strong>Status:</strong> {gatewayStatus.status}
            </div>
            <div class="status-item">
              <strong>Active Hubs:</strong> {gatewayStatus.active_hubs}
            </div>
            <div class="status-item">
              <strong>Version:</strong> {gatewayStatus.version}
            </div>
          </div>
        {/if}
      </section>

      <!-- Hubs Section -->
      <section class="hubs-section">
        <div class="section-header">
          <h2>Your Hubs ({hubs.length})</h2>
          <button class="claim-hub-btn" on:click={openClaimModal}>
            + Claim Hub
          </button>
        </div>
        {#if hubs.length === 0}
          <div class="empty-state-card">
            <div class="empty-content">
              <h3>No hubs claimed yet</h3>
              <p>Claim your first Lucas hub to start managing your smart home devices.</p>
              <button class="primary-claim-btn" on:click={openClaimModal}>
                Claim Your First Hub
              </button>
            </div>
          </div>
        {:else}
          <div class="hubs-grid">
            {#each hubs as hub}
              <div class="hub-card">
                <h3>{hub.name || hub.hub_id}</h3>
                <p><strong>Status:</strong> {hub.status}</p>
                <p><strong>Hub ID:</strong> {hub.hub_id}</p>
                {#if hub.last_seen}
                  <p><strong>Last Seen:</strong> {new Date(hub.last_seen).toLocaleString()}</p>
                {/if}
                <div class="hub-actions">
                  <button class="config-btn" on:click={() => onConfigureHub(hub.hub_id)}>
                    Configure Devices
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </section>

      <!-- Devices Section -->
      <section class="devices-section">
        <h2>Your Devices ({devices.length})</h2>
        {#if devices.length === 0}
          <p class="empty-state">No devices found. Add devices through your hubs to control them here.</p>
        {:else}
          <div class="devices-grid">
            {#each devices as device}
              <div class="device-card">
                <h3>{device.name || device.device_id}</h3>
                <p><strong>Type:</strong> {device.device_type}</p>
                <p><strong>Status:</strong> {device.status}</p>
                {#if device.model}
                  <p><strong>Model:</strong> {device.model}</p>
                {/if}
                
                <div class="device-actions">
                  <button class="remote-btn" on:click={() => onSelectDevice(device)}>
                    Remote Control
                  </button>
                  
                  {#if device.device_type === 'tv'}
                    <button on:click={() => sendDeviceAction(device.device_id, { type: 'remote', action: 'power' })}>
                      Power Toggle
                    </button>
                    <button on:click={() => sendDeviceAction(device.device_id, { type: 'remote', action: 'volume_up' })}>
                      Volume Up
                    </button>
                    <button on:click={() => sendDeviceAction(device.device_id, { type: 'remote', action: 'volume_down' })}>
                      Volume Down
                    </button>
                  {:else}
                    <button on:click={() => sendDeviceAction(device.device_id, { type: 'remote', action: 'power' })}>
                      Power
                    </button>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </section>
    </div>
  {/if}

  <!-- Hub Claiming Modal -->
  {#if showClaimModal}
    <HubClaiming 
      onHubClaimed={handleHubClaimed}
      onClose={closeClaimModal}
      showAsModal={true}
    />
  {/if}
</div>

<style>
  .dashboard {
    max-width: 1200px;
    margin: 0 auto;
    padding: 1rem;
  }


  .loading, .error {
    text-align: center;
    padding: 2rem;
  }

  .error {
    color: #d32f2f;
    background: #ffebee;
    border-radius: 4px;
    border-left: 4px solid #d32f2f;
  }

  .error button {
    margin-top: 1rem;
    background: #2196F3;
    color: white;
    border: none;
    padding: 0.5rem 1rem;
    border-radius: 4px;
    cursor: pointer;
  }

  .dashboard-content section {
    margin-bottom: 2rem;
  }

  .dashboard-content h2 {
    color: #555;
    border-bottom: 2px solid #e0e0e0;
    padding-bottom: 0.5rem;
    margin-bottom: 1rem;
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .section-header h2 {
    margin: 0;
    border: none;
    padding: 0;
  }

  .claim-hub-btn {
    background: #4CAF50;
    color: white;
    border: none;
    padding: 0.5rem 1rem;
    border-radius: 6px;
    cursor: pointer;
    font-weight: 500;
    font-size: 0.9rem;
    transition: all 0.2s;
  }

  .claim-hub-btn:hover {
    background: #45a049;
    transform: translateY(-1px);
    box-shadow: 0 2px 8px rgba(76, 175, 80, 0.3);
  }

  .empty-state-card {
    background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
    border: 2px dashed #dee2e6;
    border-radius: 12px;
    padding: 3rem 2rem;
    text-align: center;
    margin: 2rem 0;
  }

  .empty-content h3 {
    color: #495057;
    margin: 0 0 1rem 0;
    font-size: 1.25rem;
  }

  .empty-content p {
    color: #6c757d;
    margin: 0 0 2rem 0;
    font-size: 1rem;
    line-height: 1.5;
  }

  .primary-claim-btn {
    background: #2196F3;
    color: white;
    border: none;
    padding: 1rem 2rem;
    border-radius: 8px;
    cursor: pointer;
    font-weight: 600;
    font-size: 1rem;
    transition: all 0.2s;
    box-shadow: 0 2px 4px rgba(33, 150, 243, 0.2);
  }

  .primary-claim-btn:hover {
    background: #1976D2;
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(33, 150, 243, 0.4);
  }

  .status-info {
    background: #f5f5f5;
    padding: 1rem;
    border-radius: 4px;
    border-left: 4px solid #4CAF50;
  }

  .status-item {
    margin: 0.5rem 0;
  }

  .hubs-grid, .devices-grid {
    display: grid;
    gap: 1rem;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  }

  .hub-card, .device-card {
    background: white;
    padding: 1.5rem;
    border-radius: 8px;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
    border: 1px solid #e0e0e0;
  }

  .hub-card h3, .device-card h3 {
    margin-top: 0;
    color: #333;
  }

  .hub-card p, .device-card p {
    margin: 0.5rem 0;
    color: #666;
  }

  .hub-actions {
    margin-top: 1rem;
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .config-btn {
    background: #4CAF50;
    color: white;
    border: none;
    padding: 0.5rem 1rem;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.9rem;
    font-weight: 500;
    transition: background-color 0.2s;
  }

  .config-btn:hover {
    background: #45a049;
  }

  .device-actions {
    margin-top: 1rem;
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .device-actions button {
    background: #2196F3;
    color: white;
    border: none;
    padding: 0.5rem 1rem;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.9rem;
    transition: background-color 0.2s;
  }

  .device-actions button:hover {
    background: #1976D2;
  }

  .remote-btn {
    background: #9C27B0 !important;
    font-weight: bold;
  }

  .remote-btn:hover {
    background: #7B1FA2 !important;
  }

  .empty-state {
    text-align: center;
    color: #888;
    font-style: italic;
    padding: 2rem;
  }
</style>