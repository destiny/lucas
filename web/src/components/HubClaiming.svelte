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

  export let onHubClaimed: () => void = () => {};
  export let onClose: () => void = () => {};
  export let showAsModal: boolean = true;

  $: token = $auth.token;
  
  let productKey = '';
  let claiming = false;
  let error: string | null = null;
  let success: string | null = null;

  async function claimHub() {
    if (!token || !productKey.trim()) {
      error = 'Product key is required';
      return;
    }
    
    claiming = true;
    error = null;
    success = null;

    try {
      const response = await apiClient.claimHub(productKey.trim(), token);
      success = `Successfully claimed hub: ${response.hub_id}`;
      productKey = '';
      
      // Call callback to refresh hub list
      setTimeout(() => {
        onHubClaimed();
        if (showAsModal) {
          onClose();
        }
      }, 2000);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to claim hub';
    } finally {
      claiming = false;
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      claimHub();
    } else if (event.key === 'Escape' && showAsModal) {
      onClose();
    }
  }

  function clearMessages() {
    error = null;
    success = null;
  }
</script>

{#if showAsModal}
  <div class="hub-claiming modal" on:keydown={handleKeydown}>
    <div class="modal-backdrop" on:click={onClose}></div>
    <div class="modal-content">
      <div class="modal-header">
        <h2>Claim New Hub</h2>
        <button class="close-btn" on:click={onClose} aria-label="Close">×</button>
      </div>
      <div class="modal-body">
        <div class="claim-form">
          <div class="instructions">
            <p>Enter your hub's product key to claim it and add it to your account.</p>
            <p class="help-text">You can find the product key on your hub device or in its configuration documentation.</p>
          </div>

          <div class="form-group">
            <label for="product-key">Product Key</label>
            <input 
              id="product-key"
              type="text"
              bind:value={productKey}
              on:input={clearMessages}
              placeholder="e.g., 96123960-caf7-4706-9f5a-a072b0699723"
              disabled={claiming}
              class:error={error}
              autocomplete="off"
            />
          </div>

          {#if error}
            <div class="error-message">
              {error}
            </div>
          {/if}

          {#if success}
            <div class="success-message">
              {success}
            </div>
          {/if}

          <div class="form-actions">
            <button 
              class="claim-btn" 
              on:click={claimHub}
              disabled={claiming || !productKey.trim()}
            >
              {claiming ? 'Claiming...' : 'Claim Hub'}
            </button>
            
            <button class="cancel-btn" on:click={onClose} disabled={claiming}>
              Cancel
            </button>
          </div>

          <div class="additional-info">
            <details>
              <summary>Need help finding your product key?</summary>
              <div class="help-content">
                <p>The product key is a unique identifier for your Lucas hub. You can find it:</p>
                <ul>
                  <li>On a label attached to your hub device</li>
                  <li>In the hub's configuration file (<code>hub.yml</code>)</li>
                  <li>In the hub's setup documentation</li>
                  <li>On the hub's display screen (if available)</li>
                </ul>
                <p>It typically looks like: <code>12345678-1234-1234-1234-123456789abc</code></p>
              </div>
            </details>
          </div>
        </div>
      </div>
    </div>
  </div>
{:else}
  <div class="hub-claiming" on:keydown={handleKeydown}>
    <div class="claim-content">
      <h2>Claim Your Hub</h2>
      
      <div class="claim-form">
        <div class="instructions">
          <p>Enter your hub's product key to claim it and add it to your account.</p>
          <p class="help-text">You can find the product key on your hub device or in its configuration documentation.</p>
        </div>

        <div class="form-group">
          <label for="product-key-standalone">Product Key</label>
          <input 
            id="product-key-standalone"
            type="text"
            bind:value={productKey}
            on:input={clearMessages}
            placeholder="e.g., 96123960-caf7-4706-9f5a-a072b0699723"
            disabled={claiming}
            class:error={error}
            autocomplete="off"
          />
        </div>

        {#if error}
          <div class="error-message">
            {error}
          </div>
        {/if}

        {#if success}
          <div class="success-message">
            {success}
          </div>
        {/if}

        <div class="form-actions">
          <button 
            class="claim-btn" 
            on:click={claimHub}
            disabled={claiming || !productKey.trim()}
          >
            {claiming ? 'Claiming...' : 'Claim Hub'}
          </button>
        </div>

        <div class="additional-info">
          <details>
            <summary>Need help finding your product key?</summary>
            <div class="help-content">
              <p>The product key is a unique identifier for your Lucas hub. You can find it:</p>
              <ul>
                <li>On a label attached to your hub device</li>
                <li>In the hub's configuration file (<code>hub.yml</code>)</li>
                <li>In the hub's setup documentation</li>
                <li>On the hub's display screen (if available)</li>
              </ul>
              <p>It typically looks like: <code>12345678-1234-1234-1234-123456789abc</code></p>
            </div>
          </details>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .hub-claiming {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  }

  .hub-claiming.modal {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .modal-backdrop {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.6);
  }

  .modal-content {
    position: relative;
    background: white;
    border-radius: 12px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
    max-width: 500px;
    width: 90vw;
    max-height: 80vh;
    overflow-y: auto;
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.5rem;
    border-bottom: 1px solid #e0e0e0;
    background: #f8f9fa;
    border-radius: 12px 12px 0 0;
  }

  .modal-header h2 {
    margin: 0;
    color: #333;
    font-size: 1.25rem;
  }

  .close-btn {
    background: none;
    border: none;
    font-size: 1.5rem;
    cursor: pointer;
    color: #666;
    padding: 0.5rem;
    border-radius: 4px;
    line-height: 1;
  }

  .close-btn:hover {
    background: #e0e0e0;
    color: #333;
  }

  .modal-body {
    padding: 1.5rem;
  }

  .claim-content {
    max-width: 600px;
    margin: 0 auto;
    padding: 2rem;
  }

  .claim-content h2 {
    color: #333;
    margin-bottom: 1.5rem;
    text-align: center;
  }

  .claim-form {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  .instructions {
    text-align: center;
    margin-bottom: 1rem;
  }

  .instructions p {
    margin: 0.5rem 0;
    color: #555;
  }

  .help-text {
    font-size: 0.9rem;
    color: #777;
    font-style: italic;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .form-group label {
    font-weight: 600;
    color: #333;
    font-size: 1rem;
  }

  .form-group input {
    padding: 0.75rem;
    border: 2px solid #ddd;
    border-radius: 8px;
    font-size: 1rem;
    transition: border-color 0.2s, box-shadow 0.2s;
    font-family: 'SF Mono', 'Monaco', 'Consolas', monospace;
  }

  .form-group input:focus {
    outline: none;
    border-color: #2196F3;
    box-shadow: 0 0 0 3px rgba(33, 150, 243, 0.1);
  }

  .form-group input.error {
    border-color: #f44336;
    box-shadow: 0 0 0 3px rgba(244, 67, 54, 0.1);
  }

  .form-group input:disabled {
    background: #f5f5f5;
    cursor: not-allowed;
  }

  .error-message {
    background: #ffebee;
    color: #d32f2f;
    padding: 0.75rem;
    border-radius: 6px;
    border-left: 4px solid #f44336;
    font-size: 0.9rem;
  }

  .success-message {
    background: #e8f5e8;
    color: #2e7d32;
    padding: 0.75rem;
    border-radius: 6px;
    border-left: 4px solid #4caf50;
    font-size: 0.9rem;
  }

  .form-actions {
    display: flex;
    gap: 1rem;
    justify-content: center;
    flex-wrap: wrap;
  }

  .claim-btn, .cancel-btn {
    padding: 0.75rem 1.5rem;
    border-radius: 8px;
    font-size: 1rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
    border: none;
    min-width: 120px;
  }

  .claim-btn {
    background: #2196F3;
    color: white;
  }

  .claim-btn:hover:not(:disabled) {
    background: #1976D2;
    transform: translateY(-1px);
    box-shadow: 0 2px 8px rgba(33, 150, 243, 0.3);
  }

  .claim-btn:disabled {
    background: #ccc;
    cursor: not-allowed;
    transform: none;
    box-shadow: none;
  }

  .cancel-btn {
    background: #f5f5f5;
    color: #666;
    border: 1px solid #ddd;
  }

  .cancel-btn:hover:not(:disabled) {
    background: #e0e0e0;
    color: #333;
  }

  .additional-info {
    margin-top: 2rem;
    border-top: 1px solid #eee;
    padding-top: 1.5rem;
  }

  .additional-info details {
    cursor: pointer;
  }

  .additional-info summary {
    color: #2196F3;
    font-weight: 500;
    padding: 0.5rem 0;
    outline: none;
  }

  .additional-info summary:hover {
    color: #1976D2;
  }

  .help-content {
    padding: 1rem 0;
    color: #555;
    line-height: 1.6;
  }

  .help-content ul {
    margin: 0.5rem 0;
    padding-left: 1.5rem;
  }

  .help-content li {
    margin: 0.25rem 0;
  }

  .help-content code {
    background: #f5f5f5;
    padding: 0.2rem 0.4rem;
    border-radius: 3px;
    font-family: 'SF Mono', 'Monaco', 'Consolas', monospace;
    font-size: 0.9em;
    color: #666;
  }

  /* Responsive design */
  @media (max-width: 600px) {
    .modal-content {
      width: 95vw;
      margin: 1rem;
    }

    .modal-header,
    .modal-body {
      padding: 1rem;
    }

    .claim-content {
      padding: 1rem;
    }

    .form-actions {
      flex-direction: column;
    }

    .claim-btn, .cancel-btn {
      width: 100%;
    }
  }
</style>