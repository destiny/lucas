<script lang="ts">
    import {onMount} from 'svelte';
    import moment from 'moment';

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


<header>
    <h1>Lucas - Smart Home</h1>
</header>
<main>
    <nav>
        <ul>
            <li>
                <div>Hubs</div>
            </li>
            <li>
                <div>Devices</div>
            </li>
            <li>
                <div>Settings</div>
            </li>
        </ul>
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
                    <p><strong>Updated:</strong> {moment(gatewayStatus.timestamp).fromNow()}</p>
                </div>
            {/if}
        </section>
    </nav>
    <section class="content">

    </section>
</main>
<footer>
    <section></section>
</footer>


<style lang="scss">
  button {
    padding: 0.5rem 1rem;
    font-size: 1rem;
    border-radius: 0.5rem;
    background: #8bf;
  }
  header {
    background: #5f9efa;
  }
  footer {
    height: 1.5em;
  }
  main {
    flex: 1;
    display: flex;
    nav {
      width: 200px;
      display: flex;
      flex-direction: column;
      justify-content: space-between;
      background: #e5f5fd;
      ul {
        padding: 0.5em;
        display: flex;
        flex-direction: column;
        gap: 0.5em;
        list-style: none;
        li {
          font-weight: bold;
          font-size: 1.25em;
        }
      }
      section.status {
        height: 10em;
        background: #d4eefc;
        padding: 0.5em;
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 0.5em;
        font-size: 0.8em;
        h2 {
          font-size: 1rem;
        }
        button {
          font-size: 1.25em;
        }
      }
    }
    section.content {
      flex: 1;
    }
  }
</style>