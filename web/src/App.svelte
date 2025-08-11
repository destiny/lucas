<script lang="ts">
    import { onMount } from 'svelte';
    import moment from 'moment';
    import { auth, apiClient } from './lib/auth';
    import LoginForm from './components/LoginForm.svelte';
    import RegisterForm from './components/RegisterForm.svelte';
    import Dashboard from './components/Dashboard.svelte';
    import RemoteControl from './components/RemoteControl.svelte';
    
    // Svelte 5 compatible - using traditional reactive statements for now

    // Types
    type AuthView = 'login' | 'register';
    type MainView = 'dashboard' | 'settings' | 'remote';

    // Gateway status state
    let gatewayStatus: any = null;
    let loading = true;
    let error: string | null = null;

    // Authentication state using Svelte 5 store integration
    $: authState = $auth;
    $: isAuthenticated = authState.isAuthenticated;
    $: authLoading = authState.isLoading;

    // View state
    let currentAuthView: AuthView = 'login';
    let currentMainView: MainView = 'dashboard';

    // User data
    let userHubCount = 0;
    let userDeviceCount = 0;
    let devices: any[] = [];
    let hubs: any[] = [];
    let selectedDevice: any = null;

    // Version numbering - YY.WW.N format
    function generateVersion(): string {
        const now = new Date();
        const year = now.getFullYear().toString().slice(-2); // Last 2 digits of year
        
        // Calculate week number (ISO 8601 week numbering)
        const startOfYear = new Date(now.getFullYear(), 0, 1);
        const pastDaysOfYear = Math.floor((now.getTime() - startOfYear.getTime()) / (1000 * 60 * 60 * 24));
        const weekNum = Math.ceil((pastDaysOfYear + startOfYear.getDay() + 1) / 7);
        
        // Build number within the week (1-7, based on day of week, Monday=1)
        const dayOfWeek = now.getDay();
        const buildNum = dayOfWeek === 0 ? 7 : dayOfWeek; // Sunday=7, Monday=1
        
        return `${year}.${weekNum}.${buildNum}`;
    }

    let appVersion = generateVersion();

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

    // Fetch user-specific data when authenticated
    async function fetchUserData() {
        if (!authState.token) return;
        
        try {
            const [hubsResponse, devicesResponse] = await Promise.all([
                apiClient.getUserHubs(authState.token),
                apiClient.getUserDevices(authState.token)
            ]);
            
            hubs = hubsResponse.hubs || [];
            devices = devicesResponse.devices || [];
            userHubCount = hubs.length;
            userDeviceCount = devices.length;
        } catch (err) {
            console.error('Failed to fetch user data:', err);
        }
    }

    // Navigation functions
    function setMainView(view: MainView) {
        currentMainView = view;
    }

    function showAuthView(view: AuthView) {
        currentAuthView = view;
    }

    function handleLogout() {
        auth.logout();
        currentAuthView = 'login';
        currentMainView = 'dashboard';
        selectedDevice = null;
    }

    // Device selection functions
    function selectDevice(device: any) {
        selectedDevice = device;
        currentMainView = 'remote';
    }

    function goBackToDashboard() {
        selectedDevice = null;
        currentMainView = 'dashboard';
    }

    // Watch for authentication changes
    $: if (isAuthenticated && authState?.token) {
        fetchUserData();
    }

    onMount(async () => {
        await auth.init();
        fetchGatewayStatus();
    });
</script>


<header>
    <h1>Lucas - Smart Home</h1>
    {#if isAuthenticated && authState.user}
        <div class="user-info">
            <span>Welcome, {authState.user.username}</span>
            <button class="logout-btn" on:click={handleLogout}>Logout</button>
        </div>
    {/if}
</header>
<main>
    <nav>
        <ul>
            <li>
                <button 
                    class="nav-item"
                    class:active={currentMainView === 'dashboard'}
                    on:click={() => setMainView('dashboard')}
                >
                    Dashboard
                </button>
            </li>
            <li>
                <button 
                    class="nav-item"
                    class:active={currentMainView === 'settings'}
                    on:click={() => setMainView('settings')}
                >
                    Settings
                </button>
            </li>
        </ul>
        <section class="status">
            <h2>Status</h2>

            {#if isAuthenticated}
                <div class="user-stats">
                    <p><strong>Your Hubs:</strong> {userHubCount}</p>
                    <p><strong>Your Devices:</strong> {userDeviceCount}</p>
                </div>
                <hr />
            {/if}

            {#if isAuthenticated && devices.length > 0}
                <div class="device-list">
                    <h3>Devices</h3>
                    {#each hubs as hub}
                        {#if devices.some(device => device.hub_id === hub.hub_id)}
                            <div class="hub-group">
                                <h4>{hub.name || hub.hub_id}</h4>
                                {#each devices.filter(device => device.hub_id === hub.hub_id) as device}
                                    <button 
                                        class="device-item"
                                        class:selected={selectedDevice?.device_id === device.device_id}
                                        on:click={() => selectDevice(device)}
                                    >
                                        {device.name || device.device_id}
                                    </button>
                                {/each}
                            </div>
                        {/if}
                    {/each}
                </div>
                <hr />
            {/if}

            <div class="gateway-status">
                <h3>Gateway</h3>
                {#if loading}
                    <p>Loading...</p>
                {:else if error}
                    <p class="error">Error: {error}</p>
                    <button on:click={fetchGatewayStatus}>Retry</button>
                {:else if gatewayStatus}
                    <div class="status-info">
                        <p><strong>Status:</strong> {gatewayStatus.status}</p>
                        <p><strong>Version:</strong> {gatewayStatus.version}</p>
                        <p><strong>Updated:</strong> {moment(gatewayStatus.timestamp).fromNow()}</p>
                    </div>
                {/if}
            </div>
        </section>
    </nav>
    <section class="content">
        {#if authLoading}
            <div class="loading-content">
                <h2>Loading...</h2>
                <p>Initializing Lucas Smart Home</p>
            </div>
        {:else if !isAuthenticated}
            <div class="auth-content">
                {#if currentAuthView === 'login'}
                    <LoginForm />
                    <div class="auth-switch">
                        <p>Don't have an account? 
                            <button class="link" on:click={() => showAuthView('register')}>Create one</button>
                        </p>
                    </div>
                {:else if currentAuthView === 'register'}
                    <RegisterForm />
                    <div class="auth-switch">
                        <p>Already have an account? 
                            <button class="link" on:click={() => showAuthView('login')}>Sign in</button>
                        </p>
                    </div>
                {/if}
            </div>
        {:else}
            <div class="main-content">
                {#if currentMainView === 'dashboard'}
                    <Dashboard onSelectDevice={selectDevice} />
                {:else if currentMainView === 'remote' && selectedDevice}
                    <div class="remote-view">
                        <div class="remote-header">
                            <button class="back-btn" on:click={goBackToDashboard}>← Back</button>
                            <h2>{selectedDevice.name || selectedDevice.device_id} Remote</h2>
                        </div>
                        <RemoteControl device={selectedDevice} />
                    </div>
                {:else if currentMainView === 'settings'}
                    <div class="view-placeholder">
                        <h2>Settings</h2>
                        <p>User: {authState.user?.username}</p>
                        <p>Settings component will be implemented here</p>
                    </div>
                {/if}
            </div>
        {/if}
    </section>
</main>
<footer>
    <div class="footer-content">
        <span class="app-version">Lucas v{appVersion}</span>
        {#if gatewayStatus}
            <span class="gateway-info">Gateway: {gatewayStatus.status}</span>
        {/if}
        <span class="build-info">Build: {appVersion}</span>
    </div>
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
    padding: 1rem 2rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    
    h1 {
      margin: 0;
      color: white;
    }
    
    .user-info {
      display: flex;
      align-items: center;
      gap: 1rem;
      color: white;
      
      .logout-btn {
        background: rgba(255,255,255,0.2);
        color: white;
        border: 1px solid rgba(255,255,255,0.3);
        
        &:hover {
          background: rgba(255,255,255,0.3);
        }
      }
    }
  }
  footer {
    background: #f8f9fa;
    padding: 0.5rem 1rem;
    border-top: 1px solid #e9ecef;
    font-size: 0.8rem;
    
    .footer-content {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 1rem;
      
      span {
        color: #6c757d;
      }
      
      .app-version {
        font-weight: bold;
        color: #495057;
      }
    }
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
          .nav-item {
            width: 100%;
            padding: 0.75rem 1rem;
            background: none;
            border: none;
            text-align: left;
            font-size: 1rem;
            font-weight: bold;
            cursor: pointer;
            border-radius: 0.5rem;
            transition: background-color 0.2s;
            color: #333;
            
            &:hover {
              background: rgba(95, 158, 250, 0.1);
            }
            
            &.active {
              background: #5f9efa;
              color: white;
            }
          }
        }
      }
      section.status {
        background: #d4eefc;
        padding: 1rem;
        font-size: 0.8em;
        
        h2, h3 {
          font-size: 1rem;
          margin: 0 0 0.5rem 0;
          text-align: center;
          color: #333;
        }
        
        h3 {
          font-size: 0.9rem;
          margin-top: 0.5rem;
        }
        
        .user-stats {
          margin-bottom: 0.5rem;
          
          p {
            margin: 0.25rem 0;
            text-align: center;
            color: #555;
          }
        }
        
        .gateway-status {
          p {
            margin: 0.25rem 0;
            text-align: center;
            color: #555;
          }
        }
        
        hr {
          border: none;
          border-top: 1px solid #bde4f7;
          margin: 0.5rem 0;
        }
        
        button {
          font-size: 0.75rem;
          padding: 0.25rem 0.5rem;
        }
      }
      
      .device-list {
        margin-top: 0.5rem;
        
        h3 {
          font-size: 1rem;
          margin: 0 0 0.5rem 0;
          text-align: center;
          color: #333;
        }
        
        .hub-group {
          margin-bottom: 1rem;
          
          h4 {
            font-size: 0.85rem;
            margin: 0 0 0.25rem 0;
            color: #555;
            font-weight: bold;
          }
          
          .device-item {
            width: 100%;
            padding: 0.5rem 0.75rem;
            background: rgba(95, 158, 250, 0.05);
            border: 1px solid rgba(95, 158, 250, 0.2);
            text-align: left;
            font-size: 0.8rem;
            cursor: pointer;
            border-radius: 0.25rem;
            margin-bottom: 0.25rem;
            transition: all 0.2s;
            color: #333;
            
            &:hover {
              background: rgba(95, 158, 250, 0.1);
              border-color: rgba(95, 158, 250, 0.3);
            }
            
            &.selected {
              background: #5f9efa;
              color: white;
              border-color: #4a8bc2;
            }
          }
        }
      }
    }
    section.content {
      flex: 1;
      padding: 1rem;
      
      .loading-content {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: 100%;
        
        h2 {
          color: #333;
          margin-bottom: 1rem;
        }
        
        p {
          color: #666;
        }
      }
      
      .auth-content {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: 100%;
        
        .auth-switch {
          text-align: center;
          margin-top: 1rem;
          
          p {
            color: #666;
            margin: 0;
          }
          
          .link {
            background: none;
            border: none;
            color: #5f9efa;
            text-decoration: underline;
            cursor: pointer;
            font-size: 1rem;
            
            &:hover {
              color: #4a8bc2;
            }
          }
        }
      }
      
      .main-content {
        height: 100%;
        
        .view-placeholder {
          background: white;
          padding: 2rem;
          border-radius: 0.5rem;
          box-shadow: 0 1px 3px rgba(0,0,0,0.1);
          
          h2 {
            color: #333;
            margin-bottom: 1rem;
          }
          
          p {
            color: #666;
            margin: 0.5rem 0;
          }
        }
      }
      
      .remote-view {
        height: 100%;
        
        .remote-header {
          display: flex;
          align-items: center;
          gap: 1rem;
          margin-bottom: 2rem;
          
          .back-btn {
            background: #5f9efa;
            color: white;
            border: none;
            padding: 0.5rem 1rem;
            border-radius: 0.5rem;
            cursor: pointer;
            font-size: 1rem;
            
            &:hover {
              background: #4a8bc2;
            }
          }
          
          h2 {
            color: #333;
            margin: 0;
          }
        }
        
      }
    }
  }
</style>