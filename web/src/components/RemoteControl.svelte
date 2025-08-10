<script lang="ts">
  import { auth, apiClient } from '../lib/auth';

  export let device: any;

  $: authState = $auth;
  $: token = authState.token;

  let feedback: string = '';
  let isLoading: boolean = false;

  async function sendRemoteAction(action: string) {
    if (!token || !device) return;
    
    isLoading = true;
    feedback = `Sending ${action.replace('_', ' ')}...`;
    
    try {
      const actionRequest = {
        type: "remote",
        action: action
      };
      
      // Show command as sent immediately, not waiting for device response
      // This gives better UX as the user knows the command was transmitted
      setTimeout(() => {
        isLoading = false;
        feedback = `${action.replace('_', ' ')} sent`;
      }, 200); // Small delay to show visual feedback
      
      // Send command in background - don't block UI on device response
      apiClient.sendDeviceAction(device.device_id, actionRequest, token)
        .then(() => {
          // Command succeeded - update feedback if still showing
          if (feedback.includes('sent')) {
            feedback = `${action.replace('_', ' ')} executed successfully`;
          }
        })
        .catch((err) => {
          // Command failed - show error
          feedback = `Failed: ${err instanceof Error ? err.message : 'Unknown error'}`;
        });
      
      // Clear feedback after 3 seconds regardless of outcome
      setTimeout(() => {
        feedback = '';
      }, 3000);
      
    } catch (err) {
      // Immediate error (before sending)
      feedback = `Error: ${err instanceof Error ? err.message : 'Unknown error'}`;
      isLoading = false;
    }
  }
</script>

<div class="remote-control">
  <div class="remote-body">
    <!-- Feedback Display -->
    {#if feedback}
      <div class="feedback" class:error={feedback.includes('Failed')}>
        {feedback}
      </div>
    {/if}

    <!-- Power Section -->
    <div class="control-section power-section">
      <h3>Power</h3>
      <div class="button-group">
        <button 
          class="remote-btn power-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('power')}
        >
          Power
        </button>
        <button 
          class="remote-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('power_on')}
        >
          On
        </button>
        <button 
          class="remote-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('power_off')}
        >
          Off
        </button>
      </div>
    </div>

    <!-- Volume Section -->
    <div class="control-section">
      <h3>Volume</h3>
      <div class="button-group">
        <button 
          class="remote-btn volume-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('volume_up')}
        >
          Vol +
        </button>
        <button 
          class="remote-btn volume-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('volume_down')}
        >
          Vol -
        </button>
        <button 
          class="remote-btn mute-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('mute')}
        >
          Mute
        </button>
      </div>
    </div>

    <!-- Channel Section -->
    <div class="control-section">
      <h3>Channel</h3>
      <div class="button-group">
        <button 
          class="remote-btn channel-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('channel_up')}
        >
          CH +
        </button>
        <button 
          class="remote-btn channel-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('channel_down')}
        >
          CH -
        </button>
      </div>
    </div>

    <!-- Navigation Section -->
    <div class="control-section navigation-section">
      <h3>Navigation</h3>
      <div class="nav-pad">
        <button 
          class="remote-btn nav-btn nav-up" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('up')}
        >
          ▲
        </button>
        <div class="nav-middle">
          <button 
            class="remote-btn nav-btn nav-left" 
            disabled={isLoading}
            on:click={() => sendRemoteAction('left')}
          >
            ◀
          </button>
          <button 
            class="remote-btn nav-btn nav-center" 
            disabled={isLoading}
            on:click={() => sendRemoteAction('confirm')}
          >
            OK
          </button>
          <button 
            class="remote-btn nav-btn nav-right" 
            disabled={isLoading}
            on:click={() => sendRemoteAction('right')}
          >
            ▶
          </button>
        </div>
        <button 
          class="remote-btn nav-btn nav-down" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('down')}
        >
          ▼
        </button>
      </div>
    </div>

    <!-- Menu Section -->
    <div class="control-section">
      <h3>Menu</h3>
      <div class="button-group">
        <button 
          class="remote-btn menu-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('home')}
        >
          Home
        </button>
        <button 
          class="remote-btn menu-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('menu')}
        >
          Menu
        </button>
        <button 
          class="remote-btn menu-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('back')}
        >
          Back
        </button>
      </div>
    </div>

    <!-- Input Section -->
    <div class="control-section">
      <h3>Input</h3>
      <div class="button-group input-grid">
        <button 
          class="remote-btn input-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('input')}
        >
          Input
        </button>
        <button 
          class="remote-btn hdmi-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('hdmi1')}
        >
          HDMI1
        </button>
        <button 
          class="remote-btn hdmi-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('hdmi2')}
        >
          HDMI2
        </button>
        <button 
          class="remote-btn hdmi-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('hdmi3')}
        >
          HDMI3
        </button>
        <button 
          class="remote-btn hdmi-btn" 
          disabled={isLoading}
          on:click={() => sendRemoteAction('hdmi4')}
        >
          HDMI4
        </button>
      </div>
    </div>
  </div>
</div>

<style>
  .remote-control {
    max-width: 400px;
    margin: 0 auto;
    background: linear-gradient(145deg, #2c3e50, #34495e);
    border-radius: 2rem;
    padding: 2rem;
    box-shadow: 
      0 10px 30px rgba(0,0,0,0.3),
      inset 0 1px 0 rgba(255,255,255,0.1);
  }

  .remote-body {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }

  .feedback {
    background: #27ae60;
    color: white;
    padding: 0.75rem;
    border-radius: 0.5rem;
    text-align: center;
    font-size: 0.9rem;
    animation: fadeIn 0.3s ease-in;
  }

  .feedback.error {
    background: #e74c3c;
  }

  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(-10px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .control-section h3 {
    color: #ecf0f1;
    font-size: 0.9rem;
    margin: 0 0 1rem 0;
    text-align: center;
    text-transform: uppercase;
    letter-spacing: 1px;
  }

  .button-group {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
    justify-content: center;
  }

  .input-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.5rem;
  }

  .remote-btn {
    background: linear-gradient(145deg, #3498db, #2980b9);
    color: white;
    border: none;
    border-radius: 0.75rem;
    padding: 0.75rem 1rem;
    font-size: 0.9rem;
    font-weight: bold;
    cursor: pointer;
    transition: all 0.2s ease;
    box-shadow: 
      0 4px 8px rgba(0,0,0,0.2),
      inset 0 1px 0 rgba(255,255,255,0.2);
    min-width: 60px;
    text-align: center;
  }

  .remote-btn:hover:not(:disabled) {
    background: linear-gradient(145deg, #2980b9, #21618c);
    transform: translateY(-1px);
    box-shadow: 
      0 6px 12px rgba(0,0,0,0.3),
      inset 0 1px 0 rgba(255,255,255,0.2);
  }

  .remote-btn:active:not(:disabled) {
    transform: translateY(0);
    box-shadow: 
      0 2px 4px rgba(0,0,0,0.2),
      inset 0 1px 0 rgba(255,255,255,0.2);
  }

  .remote-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  /* Specific button styles */
  .power-btn {
    background: linear-gradient(145deg, #e74c3c, #c0392b);
  }

  .power-btn:hover:not(:disabled) {
    background: linear-gradient(145deg, #c0392b, #a93226);
  }

  .volume-btn, .channel-btn {
    background: linear-gradient(145deg, #27ae60, #229954);
  }

  .volume-btn:hover:not(:disabled), .channel-btn:hover:not(:disabled) {
    background: linear-gradient(145deg, #229954, #1e8449);
  }

  .mute-btn {
    background: linear-gradient(145deg, #f39c12, #e67e22);
  }

  .mute-btn:hover:not(:disabled) {
    background: linear-gradient(145deg, #e67e22, #d35400);
  }

  .hdmi-btn {
    background: linear-gradient(145deg, #9b59b6, #8e44ad);
    font-size: 0.8rem;
  }

  .hdmi-btn:hover:not(:disabled) {
    background: linear-gradient(145deg, #8e44ad, #7d3c98);
  }

  /* Navigation pad styles */
  .navigation-section {
    margin: 2rem 0;
  }

  .nav-pad {
    display: grid;
    grid-template-rows: auto auto auto;
    grid-template-columns: 1fr;
    gap: 0.5rem;
    max-width: 200px;
    margin: 0 auto;
  }

  .nav-middle {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 0.5rem;
    align-items: center;
  }

  .nav-btn {
    width: 60px;
    height: 60px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.2rem;
    font-weight: bold;
  }

  .nav-up, .nav-down {
    justify-self: center;
  }

  .nav-center {
    background: linear-gradient(145deg, #f39c12, #e67e22);
    font-size: 1rem;
  }

  .nav-center:hover:not(:disabled) {
    background: linear-gradient(145deg, #e67e22, #d35400);
  }

  /* Mobile responsiveness */
  @media (max-width: 480px) {
    .remote-control {
      padding: 1.5rem;
      margin: 0 1rem;
    }

    .remote-btn {
      padding: 0.6rem 0.8rem;
      font-size: 0.8rem;
      min-width: 50px;
    }

    .nav-btn {
      width: 50px;
      height: 50px;
      font-size: 1rem;
    }

    .input-grid {
      grid-template-columns: repeat(2, 1fr);
    }

    .control-section h3 {
      font-size: 0.8rem;
    }
  }
</style>