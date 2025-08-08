<script lang="ts">
  import { onMount } from 'svelte';
  import { auth, apiClient } from '../lib/auth';

  $: authState = $auth;
  $: user = authState.user;
  
  let gatewayStatus: any = null;
  let devices: any[] = [];
  let hubs: any[] = [];
  let loading = true;
  let error: string | null = null;

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
        <h2>Your Hubs ({hubs.length})</h2>
        {#if hubs.length === 0}
          <p class="empty-state">No hubs found. Connect a hub to get started.</p>
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
                  {#if device.device_type === 'tv'}
                    <button on:click={() => sendDeviceAction(device.device_id, { type: 'power', action: 'toggle' })}>
                      Power Toggle
                    </button>
                    <button on:click={() => sendDeviceAction(device.device_id, { type: 'volume', action: 'up' })}>
                      Volume Up
                    </button>
                    <button on:click={() => sendDeviceAction(device.device_id, { type: 'volume', action: 'down' })}>
                      Volume Down
                    </button>
                  {:else}
                    <button on:click={() => sendDeviceAction(device.device_id, { type: 'generic', action: 'status' })}>
                      Get Status
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

  .empty-state {
    text-align: center;
    color: #888;
    font-style: italic;
    padding: 2rem;
  }
</style>