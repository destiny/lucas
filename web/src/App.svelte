<script lang="ts">
  import { onMount } from 'svelte';

  let gatewayStatus: any = null;
  let loading = true;
  let error: string | null = null;

  async function fetchGatewayStatus() {
    try {
      const response = await fetch('/api/v1/gateway/status');
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      gatewayStatus = await response.json();
      loading = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Unknown error';
      loading = false;
    }
  }

  onMount(() => {
    fetchGatewayStatus();
  });
</script>

<main>
  <h1>Lucas Control Panel</h1>
  
  <section class="status">
    <h2>Gateway Status</h2>
    
    {#if loading}
      <p>Loading gateway status...</p>
    {:else if error}
      <p class="error">Error: {error}</p>
      <button on:click={fetchGatewayStatus}>Retry</button>
    {:else if gatewayStatus}
      <div class="status-info">
        <p><strong>Status:</strong> {gatewayStatus.status}</p>
        <p><strong>Active Hubs:</strong> {gatewayStatus.active_hubs}</p>
        <p><strong>Version:</strong> {gatewayStatus.version}</p>
        <p><strong>Last Updated:</strong> {new Date(gatewayStatus.timestamp).toLocaleString()}</p>
      </div>
    {/if}
  </section>

  <section class="actions">
    <h2>Quick Actions</h2>
    <div class="action-buttons">
      <button>View Hubs</button>
      <button>View Devices</button>
      <button>User Settings</button>
    </div>
  </section>
</main>

<style>
  main {
    max-width: 800px;
    margin: 0 auto;
    padding: 2rem;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  }

  h1 {
    color: #333;
    text-align: center;
    margin-bottom: 2rem;
  }

  h2 {
    color: #555;
    border-bottom: 2px solid #e0e0e0;
    padding-bottom: 0.5rem;
  }

  section {
    margin-bottom: 2rem;
  }

  .status-info {
    background: #f5f5f5;
    padding: 1rem;
    border-radius: 4px;
    border-left: 4px solid #4CAF50;
  }

  .status-info p {
    margin: 0.5rem 0;
  }

  .error {
    color: #d32f2f;
    background: #ffebee;
    padding: 1rem;
    border-radius: 4px;
    border-left: 4px solid #d32f2f;
  }

  .action-buttons {
    display: flex;
    gap: 1rem;
    flex-wrap: wrap;
  }

  button {
    background: #2196F3;
    color: white;
    border: none;
    padding: 0.75rem 1.5rem;
    border-radius: 4px;
    cursor: pointer;
    font-size: 1rem;
    transition: background-color 0.2s;
  }

  button:hover {
    background: #1976D2;
  }

  button:active {
    transform: translateY(1px);
  }
</style>