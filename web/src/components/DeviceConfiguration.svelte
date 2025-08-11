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

  export let selectedHubId: string = '';
  export let onClose: () => void = () => {};

  $: token = $auth.token;
  
  let hubs: any[] = [];
  let currentDevices: any[] = [];
  let newDevices: any[] = [];
  let loading = true;
  let saving = false;
  let error: string | null = null;
  let success: string | null = null;

  // Device types and their default configurations
  const deviceTypes = [
    {
      id: 'bravia',
      name: 'Sony Bravia TV',
      defaultCapabilities: ['remote_control', 'system_control', 'audio_control', 'content_control']
    },
    {
      id: 'generic_tv',
      name: 'Generic TV',
      defaultCapabilities: ['remote_control', 'system_control', 'audio_control']
    }
  ];

  async function loadData() {
    if (!token) return;
    
    loading = true;
    error = null;

    try {
      const hubsResponse = await apiClient.getUserHubs(token);
      hubs = hubsResponse.hubs || [];
      
      if (selectedHubId && hubs.find(h => h.hub_id === selectedHubId)) {
        await loadHubDevices();
      }
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load data';
    } finally {
      loading = false;
    }
  }

  async function loadHubDevices() {
    if (!selectedHubId || !token) return;
    
    try {
      const response = await apiClient.getHubDevices(selectedHubId, token);
      currentDevices = response.data?.devices || [];
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load hub devices';
    }
  }

  function addNewDevice() {
    newDevices = [...newDevices, {
      id: '',
      type: 'bravia',
      model: '',
      address: '',
      credential: '',
      capabilities: [...deviceTypes[0].defaultCapabilities]
    }];
  }

  function removeDevice(index: number) {
    newDevices = newDevices.filter((_, i) => i !== index);
  }

  function updateDeviceType(index: number, typeId: string) {
    const deviceType = deviceTypes.find(dt => dt.id === typeId);
    if (deviceType) {
      newDevices[index] = {
        ...newDevices[index],
        type: typeId,
        capabilities: [...deviceType.defaultCapabilities]
      };
      newDevices = [...newDevices];
    }
  }

  function toggleCapability(deviceIndex: number, capability: string) {
    const device = newDevices[deviceIndex];
    if (device.capabilities.includes(capability)) {
      device.capabilities = device.capabilities.filter(c => c !== capability);
    } else {
      device.capabilities = [...device.capabilities, capability];
    }
    newDevices = [...newDevices];
  }

  async function saveConfiguration() {
    if (!selectedHubId || !token) return;
    
    saving = true;
    error = null;
    success = null;

    try {
      // Combine current devices with new devices
      const allDevices = [...currentDevices, ...newDevices];
      
      // Validate devices
      for (let i = 0; i < allDevices.length; i++) {
        const device = allDevices[i];
        if (!device.id || !device.type || !device.address) {
          throw new Error(`Device ${i + 1}: ID, type, and address are required`);
        }
      }

      await apiClient.configureHubDevices(selectedHubId, allDevices, token);
      
      success = 'Device configuration saved successfully!';
      newDevices = [];
      await loadHubDevices();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to save configuration';
    } finally {
      saving = false;
    }
  }

  async function reloadDevices() {
    if (!selectedHubId || !token) return;
    
    try {
      await apiClient.reloadHubDevices(selectedHubId, token);
      success = 'Devices reloaded successfully!';
      await loadHubDevices();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to reload devices';
    }
  }

  onMount(() => {
    loadData();
  });

  $: if (selectedHubId) {
    loadHubDevices();
  }
</script>

<div class="device-config">
  <div class="config-header">
    <h2>Device Configuration</h2>
    <button class="close-btn" on:click={onClose}>×</button>
  </div>

  {#if loading}
    <div class="loading">Loading device configuration...</div>
  {:else if error}
    <div class="error">
      <p>Error: {error}</p>
      <button on:click={loadData}>Retry</button>
    </div>
  {:else}
    <div class="config-content">
      <!-- Hub Selection -->
      <div class="section">
        <h3>Select Hub</h3>
        <select bind:value={selectedHubId} on:change={loadHubDevices}>
          <option value="">Choose a hub...</option>
          {#each hubs as hub}
            <option value={hub.hub_id}>{hub.name || hub.hub_id} ({hub.status})</option>
          {/each}
        </select>
      </div>

      {#if selectedHubId}
        <!-- Current Devices -->
        <div class="section">
          <div class="section-header">
            <h3>Current Devices ({currentDevices.length})</h3>
            <button class="reload-btn" on:click={reloadDevices}>Reload Devices</button>
          </div>
          
          {#if currentDevices.length === 0}
            <p class="empty-state">No devices configured for this hub.</p>
          {:else}
            <div class="devices-list">
              {#each currentDevices as device}
                <div class="device-item">
                  <div class="device-info">
                    <strong>{device.id}</strong>
                    <span class="device-type">{device.type}</span>
                    <span class="device-address">{device.address}</span>
                  </div>
                  <div class="device-capabilities">
                    {#each device.capabilities as capability}
                      <span class="capability-tag">{capability}</span>
                    {/each}
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Add New Devices -->
        <div class="section">
          <div class="section-header">
            <h3>Add New Devices</h3>
            <button class="add-btn" on:click={addNewDevice}>+ Add Device</button>
          </div>

          {#if newDevices.length === 0}
            <p class="empty-state">Click "Add Device" to configure new devices.</p>
          {:else}
            <div class="new-devices-list">
              {#each newDevices as device, index}
                <div class="device-form">
                  <div class="form-header">
                    <h4>Device {index + 1}</h4>
                    <button class="remove-btn" on:click={() => removeDevice(index)}>Remove</button>
                  </div>
                  
                  <div class="form-grid">
                    <div class="form-group">
                      <label for="device-id-{index}">Device ID</label>
                      <input 
                        id="device-id-{index}"
                        bind:value={device.id}
                        placeholder="e.g., living_room_tv"
                        required
                      />
                    </div>

                    <div class="form-group">
                      <label for="device-type-{index}">Type</label>
                      <select 
                        id="device-type-{index}"
                        bind:value={device.type}
                        on:change={() => updateDeviceType(index, device.type)}
                      >
                        {#each deviceTypes as deviceType}
                          <option value={deviceType.id}>{deviceType.name}</option>
                        {/each}
                      </select>
                    </div>

                    <div class="form-group">
                      <label for="device-model-{index}">Model</label>
                      <input 
                        id="device-model-{index}"
                        bind:value={device.model}
                        placeholder="e.g., Sony Bravia X90J"
                      />
                    </div>

                    <div class="form-group">
                      <label for="device-address-{index}">IP Address</label>
                      <input 
                        id="device-address-{index}"
                        bind:value={device.address}
                        placeholder="e.g., 192.168.1.100"
                        required
                      />
                    </div>

                    <div class="form-group">
                      <label for="device-credential-{index}">Credential/PSK</label>
                      <input 
                        id="device-credential-{index}"
                        bind:value={device.credential}
                        type="password"
                        placeholder="PSK key or credential"
                      />
                    </div>

                    <div class="form-group capabilities">
                      <label>Capabilities</label>
                      <div class="capabilities-list">
                        {#each ['remote_control', 'system_control', 'audio_control', 'content_control'] as capability}
                          <label class="capability-checkbox">
                            <input 
                              type="checkbox"
                              checked={device.capabilities.includes(capability)}
                              on:change={() => toggleCapability(index, capability)}
                            />
                            {capability.replace('_', ' ')}
                          </label>
                        {/each}
                      </div>
                    </div>
                  </div>
                </div>
              {/each}
            </div>

            <div class="actions">
              <button 
                class="save-btn" 
                on:click={saveConfiguration}
                disabled={saving || newDevices.length === 0}
              >
                {saving ? 'Saving...' : 'Save Configuration'}
              </button>
            </div>
          {/if}
        </div>
      {/if}

      {#if success}
        <div class="success">
          {success}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .device-config {
    background: white;
    border-radius: 8px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    max-width: 1000px;
    margin: 2rem auto;
    max-height: 90vh;
    overflow-y: auto;
  }

  .config-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.5rem;
    border-bottom: 1px solid #e0e0e0;
    background: #f8f9fa;
    border-radius: 8px 8px 0 0;
  }

  .config-header h2 {
    margin: 0;
    color: #333;
  }

  .close-btn {
    background: none;
    border: none;
    font-size: 1.5rem;
    cursor: pointer;
    color: #666;
    padding: 0.5rem;
    border-radius: 4px;
  }

  .close-btn:hover {
    background: #e0e0e0;
    color: #333;
  }

  .config-content {
    padding: 1.5rem;
  }

  .loading {
    text-align: center;
    padding: 2rem;
    color: #666;
  }

  .error {
    background: #ffebee;
    color: #d32f2f;
    padding: 1rem;
    border-radius: 4px;
    border-left: 4px solid #d32f2f;
    margin-bottom: 1rem;
  }

  .success {
    background: #e8f5e8;
    color: #2e7d32;
    padding: 1rem;
    border-radius: 4px;
    border-left: 4px solid #4caf50;
    margin-top: 1rem;
  }

  .section {
    margin-bottom: 2rem;
  }

  .section h3 {
    color: #333;
    margin-bottom: 1rem;
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .section-header h3 {
    margin: 0;
  }

  select, input {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid #ddd;
    border-radius: 4px;
    font-size: 1rem;
  }

  .devices-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .device-item {
    background: #f8f9fa;
    padding: 1rem;
    border-radius: 4px;
    border-left: 4px solid #2196f3;
  }

  .device-info {
    display: flex;
    gap: 1rem;
    align-items: center;
    margin-bottom: 0.5rem;
  }

  .device-type, .device-address {
    color: #666;
    font-size: 0.9rem;
  }

  .device-capabilities {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .capability-tag {
    background: #e3f2fd;
    color: #1976d2;
    padding: 0.2rem 0.5rem;
    border-radius: 12px;
    font-size: 0.8rem;
  }

  .new-devices-list {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  .device-form {
    background: #f8f9fa;
    padding: 1.5rem;
    border-radius: 8px;
    border: 1px solid #e0e0e0;
  }

  .form-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .form-header h4 {
    margin: 0;
    color: #333;
  }

  .form-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 1rem;
  }

  .form-group {
    display: flex;
    flex-direction: column;
  }

  .form-group.capabilities {
    grid-column: 1 / -1;
  }

  .form-group label {
    font-weight: 500;
    color: #333;
    margin-bottom: 0.5rem;
  }

  .capabilities-list {
    display: flex;
    gap: 1rem;
    flex-wrap: wrap;
    margin-top: 0.5rem;
  }

  .capability-checkbox {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
    font-weight: normal;
  }

  .capability-checkbox input {
    width: auto;
  }

  .empty-state {
    text-align: center;
    color: #888;
    font-style: italic;
    padding: 2rem;
  }

  .add-btn, .save-btn, .reload-btn, .remove-btn {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 500;
    transition: background-color 0.2s;
  }

  .add-btn {
    background: #4caf50;
    color: white;
  }

  .add-btn:hover {
    background: #45a049;
  }

  .save-btn {
    background: #2196f3;
    color: white;
    font-size: 1rem;
    padding: 0.75rem 1.5rem;
  }

  .save-btn:hover:not(:disabled) {
    background: #1976d2;
  }

  .save-btn:disabled {
    background: #ccc;
    cursor: not-allowed;
  }

  .reload-btn {
    background: #ff9800;
    color: white;
  }

  .reload-btn:hover {
    background: #f57c00;
  }

  .remove-btn {
    background: #f44336;
    color: white;
  }

  .remove-btn:hover {
    background: #d32f2f;
  }

  .actions {
    text-align: center;
    margin-top: 2rem;
    padding-top: 1rem;
    border-top: 1px solid #e0e0e0;
  }
</style>