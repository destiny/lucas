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
  import { auth, apiClient } from '../lib/auth';
  import { onMount } from 'svelte';

  // Authentication state
  $: authState = $auth;
  
  // Hubs data
  let hubs: any[] = [];
  let loading = true;
  let error = '';

  // Claim form state
  let productKey = '';
  let claimLoading = false;
  let claimError = '';
  let claimSuccess = '';

  async function fetchHubs() {
    if (!authState.token) return;
    
    try {
      loading = true;
      error = '';
      const response = await apiClient.getUserHubs(authState.token);
      hubs = response.hubs || [];
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load hubs';
      console.error('Failed to fetch hubs:', err);
    } finally {
      loading = false;
    }
  }

  async function claimHub() {
    if (!productKey.trim()) {
      claimError = 'Product key is required';
      return;
    }

    if (!authState.token) {
      claimError = 'Authentication required';
      return;
    }

    try {
      claimLoading = true;
      claimError = '';
      claimSuccess = '';

      const result = await apiClient.claimHub(productKey.trim(), authState.token);

      claimSuccess = result.message || `Hub "${result.name || result.hub_id}" claimed successfully!`;
      productKey = '';
      
      // Refresh hubs list
      await fetchHubs();

    } catch (err) {
      claimError = err instanceof Error ? err.message : 'Failed to claim hub';
      console.error('Failed to claim hub:', err);
    } finally {
      claimLoading = false;
    }
  }

  // Fetch hubs when component mounts and when auth state changes
  $: if (authState.token) {
    fetchHubs();
  }

  onMount(() => {
    if (authState.token) {
      fetchHubs();
    }
  });
</script>

<div class="hubs-container">
  <h2>Hub Management</h2>
  
  <!-- Claim New Hub Section -->
  <div class="claim-section">
    <h3>Claim New Hub</h3>
    <div class="claim-form">
      <div class="form-group">
        <label for="product-key">Product Key:</label>
        <input
          type="text"
          id="product-key"
          bind:value={productKey}
          placeholder="Enter hub product key"
          disabled={claimLoading}
        />
      </div>
      
      {#if claimError}
        <div class="error">{claimError}</div>
      {/if}
      
      {#if claimSuccess}
        <div class="success">{claimSuccess}</div>
      {/if}
      
      <button 
        type="button" 
        on:click={claimHub} 
        disabled={claimLoading || !productKey.trim()}
        class="claim-btn"
      >
        {claimLoading ? 'Claiming...' : 'Claim Hub'}
      </button>
    </div>
  </div>

  <!-- Existing Hubs Section -->
  <div class="hubs-list">
    <h3>Your Hubs ({hubs.length})</h3>
    
    {#if loading}
      <div class="loading">
        <p>Loading your hubs...</p>
      </div>
    {:else if error}
      <div class="error-section">
        <p class="error">Error: {error}</p>
        <button on:click={fetchHubs} class="retry-btn">Retry</button>
      </div>
    {:else if hubs.length === 0}
      <div class="empty-state">
        <p>You don't have any hubs yet.</p>
        <p>Use the form above to claim your first hub with a product key.</p>
      </div>
    {:else}
      <div class="hubs-grid">
        {#each hubs as hub}
          <div class="hub-card">
            <h4>{hub.name || `Hub ${hub.id}`}</h4>
            <div class="hub-info">
              <p><strong>ID:</strong> {hub.id}</p>
              <p><strong>Status:</strong> 
                <span class="status" class:online={hub.status === 'online'} class:offline={hub.status !== 'online'}>
                  {hub.status || 'Unknown'}
                </span>
              </p>
              {#if hub.location}
                <p><strong>Location:</strong> {hub.location}</p>
              {/if}
              {#if hub.device_count !== undefined}
                <p><strong>Devices:</strong> {hub.device_count}</p>
              {/if}
              {#if hub.last_seen}
                <p><strong>Last Seen:</strong> {new Date(hub.last_seen).toLocaleString()}</p>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style lang="scss">
  .hubs-container {
    padding: 1rem;
    max-width: 1200px;
    margin: 0 auto;

    h2 {
      color: #333;
      margin-bottom: 2rem;
      text-align: center;
    }

    h3 {
      color: #555;
      margin-bottom: 1rem;
      border-bottom: 2px solid #e9ecef;
      padding-bottom: 0.5rem;
    }
  }

  .claim-section {
    background: white;
    padding: 2rem;
    border-radius: 0.5rem;
    box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    margin-bottom: 2rem;

    .claim-form {
      max-width: 400px;

      .form-group {
        margin-bottom: 1rem;

        label {
          display: block;
          margin-bottom: 0.5rem;
          font-weight: 500;
          color: #555;
        }

        input {
          width: 100%;
          padding: 0.75rem;
          border: 2px solid #ddd;
          border-radius: 4px;
          font-size: 1rem;
          transition: border-color 0.2s;

          &:focus {
            outline: none;
            border-color: #5f9efa;
          }

          &:disabled {
            background-color: #f5f5f5;
            cursor: not-allowed;
          }
        }
      }

      .claim-btn {
        background: #28a745;
        color: white;
        border: none;
        padding: 0.75rem 1.5rem;
        border-radius: 4px;
        font-size: 1rem;
        cursor: pointer;
        transition: background-color 0.2s;

        &:hover:not(:disabled) {
          background: #218838;
        }

        &:disabled {
          background: #ccc;
          cursor: not-allowed;
        }
      }
    }
  }

  .hubs-list {
    background: white;
    padding: 2rem;
    border-radius: 0.5rem;
    box-shadow: 0 1px 3px rgba(0,0,0,0.1);
  }

  .loading, .empty-state {
    text-align: center;
    padding: 3rem 2rem;
    color: #666;

    p {
      margin: 0.5rem 0;
    }
  }

  .error-section {
    text-align: center;
    padding: 2rem;

    .retry-btn {
      background: #6c757d;
      color: white;
      border: none;
      padding: 0.5rem 1rem;
      border-radius: 4px;
      cursor: pointer;
      margin-top: 1rem;

      &:hover {
        background: #5a6268;
      }
    }
  }

  .error {
    color: #d32f2f;
    background: #ffebee;
    padding: 0.75rem;
    border-radius: 4px;
    margin-bottom: 1rem;
    border-left: 4px solid #d32f2f;
  }

  .success {
    color: #2e7d32;
    background: #e8f5e8;
    padding: 0.75rem;
    border-radius: 4px;
    margin-bottom: 1rem;
    border-left: 4px solid #4caf50;
  }

  .hubs-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 1.5rem;
    margin-top: 1rem;
  }

  .hub-card {
    border: 1px solid #e9ecef;
    border-radius: 0.5rem;
    padding: 1.5rem;
    background: #f8f9fa;
    transition: transform 0.2s, box-shadow 0.2s;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 12px rgba(0,0,0,0.1);
    }

    h4 {
      margin: 0 0 1rem 0;
      color: #333;
      font-size: 1.2rem;
    }

    .hub-info {
      p {
        margin: 0.5rem 0;
        font-size: 0.9rem;
        color: #666;

        strong {
          color: #333;
        }
      }

      .status {
        padding: 0.2rem 0.5rem;
        border-radius: 3px;
        font-size: 0.8rem;
        font-weight: bold;

        &.online {
          background: #d4edda;
          color: #155724;
        }

        &.offline {
          background: #f8d7da;
          color: #721c24;
        }
      }
    }
  }

  // Responsive design
  @media (max-width: 768px) {
    .hubs-container {
      padding: 0.5rem;
    }

    .claim-section, .hubs-list {
      padding: 1rem;
      margin-bottom: 1rem;
    }

    .hubs-grid {
      grid-template-columns: 1fr;
      gap: 1rem;
    }
  }
</style>