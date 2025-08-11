<script lang="ts">
  import { auth, apiClient } from '../lib/auth';
  import ControlGroup from './ControlGroup.svelte';
  import { remoteCommands, getEssentialCommands, getAdvancedCommands } from '../lib/remoteCommands';
  // Remove Flowbite Toast for now - use custom toast instead

  export let device: any;

  // Authentication state - Svelte 5 compatible with store
  $: authState = $auth;
  $: token = authState.token;

  let isLoading: boolean = false;
  let showAdvanced: boolean = false;
  let isMobile: boolean = false;
  
  // Toast state
  let toastMessage: string = '';
  let toastType: 'success' | 'error' = 'success';
  let showToast: boolean = false;

  // Check if mobile based on screen size
  function checkMobile() {
    isMobile = window.innerWidth < 768;
  }

  // Initialize and add resize listener
  if (typeof window !== 'undefined') {
    checkMobile();
    window.addEventListener('resize', checkMobile);
  }

  // Get commands based on mobile/desktop view - Svelte 5 reactive pattern
  $: essentialCommands = getEssentialCommands();
  $: advancedCommands = getAdvancedCommands();
  $: displayCommands = isMobile 
    ? (showAdvanced ? remoteCommands : essentialCommands)
    : remoteCommands;

  function showToastMessage(message: string, type: 'success' | 'error' = 'success') {
    toastMessage = message;
    toastType = type;
    showToast = true;
    
    // Auto-hide after 3 seconds
    setTimeout(() => {
      showToast = false;
    }, 3000);
  }

  async function sendRemoteAction(action: string) {
    if (!token || !device) return;
    
    isLoading = true;
    const actionLabel = action.replace('_', ' ');
    
    try {
      const actionRequest = {
        type: "remote",
        action: action
      };
      
      // Show command as sent immediately for better UX
      setTimeout(() => {
        isLoading = false;
        showToastMessage(`${actionLabel} sent`, 'success');
      }, 200);
      
      // Send command in background - don't block UI on device response
      apiClient.sendDeviceAction(device.device_id, actionRequest, token)
        .then(() => {
          // Command succeeded - show success toast
          setTimeout(() => {
            showToastMessage(`${actionLabel} executed successfully`, 'success');
          }, 1000); // Delay to avoid conflicting with "sent" toast
        })
        .catch((err) => {
          // Command failed - show error toast
          setTimeout(() => {
            showToastMessage(`Failed: ${err instanceof Error ? err.message : 'Unknown error'}`, 'error');
          }, 1000);
        });
      
    } catch (err) {
      // Immediate error (before sending)
      showToastMessage(`Error: ${err instanceof Error ? err.message : 'Unknown error'}`, 'error');
      isLoading = false;
    }
  }

  // Handle button clicks from control groups
  function handleButtonClick(action: string) {
    sendRemoteAction(action);
  }

  // Toggle advanced controls on mobile
  function toggleAdvanced() {
    showAdvanced = !showAdvanced;
  }
</script>

<div class="remote-control" class:mobile={isMobile}>
  <div class="remote-body">
    <!-- Device Header -->
    <div class="device-header">
      <h2 class="device-title">{device?.name || 'Remote Control'}</h2>
      <div class="device-status" class:online={device?.status === 'online'}>
        {device?.status || 'unknown'}
      </div>
    </div>

    <!-- Custom Toast Notification -->
    {#if showToast}
      <div 
        class="custom-toast toast-{toastType}"
        role="alert"
        aria-live="polite"
      >
        <div class="toast-icon">
          {#if toastType === 'success'}
            ✓
          {:else}
            ✗
          {/if}
        </div>
        <div class="toast-message">
          {toastMessage}
        </div>
      </div>
    {/if}

    <!-- Control Groups -->
    <div class="control-groups" class:desktop={!isMobile}>
      {#each displayCommands as command}
        <ControlGroup 
          config={command} 
          disabled={isLoading} 
          onButtonClick={handleButtonClick} 
        />
      {/each}
    </div>

    <!-- Mobile Advanced Toggle -->
    {#if isMobile && advancedCommands.length > 0}
      <div class="advanced-toggle">
        <button 
          class="toggle-btn"
          on:click={toggleAdvanced}
          aria-label="Toggle advanced controls"
        >
          <span class="toggle-text">
            {showAdvanced ? 'Hide' : 'Show'} Advanced Controls
          </span>
          <span class="toggle-icon" class:rotated={showAdvanced}>▼</span>
        </button>
      </div>
    {/if}
  </div>
</div>

<style>
  .remote-control {
    max-width: 500px;
    margin: 0 auto;
    background: linear-gradient(145deg, #2c3e50, #34495e);
    border-radius: 1.5rem;
    padding: 1.5rem;
    box-shadow: 
      0 10px 30px rgba(0,0,0,0.3),
      inset 0 1px 0 rgba(255,255,255,0.1);
  }

  .remote-control.mobile {
    max-width: 100%;
    margin: 0;
    padding: 1rem;
    border-radius: 1rem;
  }

  .remote-body {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  /* Device Header */
  .device-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
    padding: 0.75rem 1rem;
    background: rgba(255,255,255,0.05);
    border-radius: 0.75rem;
  }

  .device-title {
    color: #ecf0f1;
    font-size: 1.2rem;
    margin: 0;
    font-weight: 600;
  }

  .device-status {
    color: #95a5a6;
    font-size: 0.8rem;
    text-transform: uppercase;
    padding: 0.25rem 0.5rem;
    border-radius: 0.5rem;
    background: rgba(149, 165, 166, 0.2);
    letter-spacing: 0.5px;
  }

  .device-status.online {
    color: #27ae60;
    background: rgba(39, 174, 96, 0.2);
  }

  /* Custom Toast Notifications */
  .custom-toast {
    position: fixed;
    top: 1rem;
    right: 1rem;
    z-index: 1000;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    border-radius: 0.5rem;
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    animation: toast-slide-in 0.3s ease-out;
    font-size: 0.9rem;
    font-weight: 500;
    max-width: 300px;
  }
  
  .toast-success {
    background: #10b981;
    color: white;
    border-left: 4px solid #059669;
  }
  
  .toast-error {
    background: #ef4444;
    color: white;
    border-left: 4px solid #dc2626;
  }
  
  .toast-icon {
    font-weight: bold;
    font-size: 1rem;
  }
  
  .toast-message {
    flex: 1;
  }
  
  @keyframes toast-slide-in {
    from {
      opacity: 0;
      transform: translateX(100%);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  /* Control Groups Container */
  .control-groups {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .control-groups.desktop {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 1rem;
  }

  /* Advanced Toggle for Mobile */
  .advanced-toggle {
    margin-top: 1rem;
    text-align: center;
  }

  .toggle-btn {
    background: linear-gradient(145deg, #34495e, #2c3e50);
    color: #ecf0f1;
    border: 2px solid rgba(255,255,255,0.1);
    border-radius: 0.75rem;
    padding: 0.75rem 1.25rem;
    font-size: 0.9rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s ease;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    margin: 0 auto;
    box-shadow: 0 4px 8px rgba(0,0,0,0.2);
  }

  .toggle-btn:hover {
    background: linear-gradient(145deg, #3e5063, #2e3a47);
    border-color: rgba(255,255,255,0.2);
    transform: translateY(-1px);
    box-shadow: 0 6px 12px rgba(0,0,0,0.3);
  }

  .toggle-text {
    font-size: 0.85rem;
    letter-spacing: 0.5px;
  }

  .toggle-icon {
    transition: transform 0.2s ease;
    font-size: 0.8rem;
  }

  .toggle-icon.rotated {
    transform: rotate(180deg);
  }

  /* Desktop Layout Enhancements */
  @media (min-width: 768px) {
    .remote-control {
      max-width: 800px;
      padding: 2rem;
    }

    .device-header {
      margin-bottom: 1.5rem;
    }

    .device-title {
      font-size: 1.4rem;
    }

    .control-groups.desktop {
      grid-template-columns: repeat(2, 1fr);
      gap: 1.5rem;
    }

    .remote-body {
      gap: 1.5rem;
    }
  }

  /* Large Desktop Layout */
  @media (min-width: 1024px) {
    .remote-control {
      max-width: 1000px;
      padding: 2.5rem;
    }

    .control-groups.desktop {
      grid-template-columns: repeat(3, 1fr);
      gap: 2rem;
    }
  }

  /* Mobile Portrait Optimizations */
  @media (max-width: 767px) {
    .remote-control {
      padding: 0.75rem;
      border-radius: 0.75rem;
      box-shadow: 
        0 5px 15px rgba(0,0,0,0.2),
        inset 0 1px 0 rgba(255,255,255,0.1);
    }

    .device-header {
      padding: 0.5rem 0.75rem;
      margin-bottom: 0.75rem;
    }

    .device-title {
      font-size: 1rem;
    }

    .device-status {
      font-size: 0.7rem;
    }


    .control-groups {
      gap: 0.5rem;
    }

    .remote-body {
      gap: 0.75rem;
    }
  }

  /* Extra small mobile devices */
  @media (max-width: 480px) {
    .remote-control {
      padding: 0.5rem;
    }
    
    .device-header {
      flex-direction: column;
      gap: 0.25rem;
      text-align: center;
    }
    
    .device-title {
      font-size: 0.9rem;
    }
  }

  /* Touch-friendly mobile enhancements */
  @media (max-width: 767px) and (pointer: coarse) {
    .toggle-btn {
      padding: 1rem 1.5rem;
      font-size: 1rem;
      min-height: 44px; /* iOS recommended touch target */
    }
  }

  /* Landscape mobile optimizations */
  @media (max-width: 1023px) and (orientation: landscape) {
    .remote-control {
      max-width: 90vw;
    }
    
    .control-groups {
      display: grid;
      grid-template-columns: repeat(2, 1fr);
      gap: 1rem;
    }
  }
</style>