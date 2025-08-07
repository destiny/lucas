<script lang="ts">
  import { auth } from '$lib/auth';

  let username = '';
  let email = '';
  let password = '';
  let error = '';
  let isLoading = false;
  let useEmail = false; // Toggle between username and email login

  async function handleSubmit() {
    if (!password) {
      error = 'Password is required';
      return;
    }

    if (!useEmail && !username) {
      error = 'Username is required';
      return;
    }

    if (useEmail && !email) {
      error = 'Email is required';
      return;
    }

    error = '';
    isLoading = true;

    try {
      const result = await auth.login(
        useEmail ? '' : username,
        useEmail ? email : '',
        password
      );

      if (!result.success) {
        error = result.error || 'Login failed';
      }
    } catch (err) {
      error = err instanceof Error ? err.message : 'Login failed';
    } finally {
      isLoading = false;
    }
  }

  function toggleLoginMethod() {
    useEmail = !useEmail;
    error = '';
  }
</script>

<div class="login-form">
  <h2>Login to Lucas</h2>
  
  <form on:submit|preventDefault={handleSubmit}>
    <div class="form-group">
      <label for="login-field">
        {useEmail ? 'Email' : 'Username'}:
      </label>
      {#if useEmail}
        <input
          type="email"
          id="login-field"
          bind:value={email}
          required
          placeholder="Enter your email"
          disabled={isLoading}
        />
      {:else}
        <input
          type="text"
          id="login-field"
          bind:value={username}
          required
          placeholder="Enter your username"
          disabled={isLoading}
        />
      {/if}
    </div>

    <div class="form-group">
      <label for="password">Password:</label>
      <input
        type="password"
        id="password"
        bind:value={password}
        required
        placeholder="Enter your password"
        disabled={isLoading}
        minlength="6"
      />
    </div>

    {#if error}
      <div class="error">{error}</div>
    {/if}

    <button type="submit" disabled={isLoading}>
      {isLoading ? 'Logging in...' : 'Login'}
    </button>

    <div class="form-options">
      <button type="button" class="link" on:click={toggleLoginMethod}>
        Use {useEmail ? 'Username' : 'Email'} instead
      </button>
    </div>
  </form>
</div>

<style>
  .login-form {
    max-width: 400px;
    margin: 2rem auto;
    padding: 2rem;
    background: white;
    border-radius: 8px;
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
  }

  h2 {
    text-align: center;
    margin-bottom: 1.5rem;
    color: #333;
  }

  .form-group {
    margin-bottom: 1rem;
  }

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
  }

  input:focus {
    outline: none;
    border-color: #2196F3;
  }

  input:disabled {
    background-color: #f5f5f5;
    cursor: not-allowed;
  }

  .error {
    color: #d32f2f;
    background: #ffebee;
    padding: 0.75rem;
    border-radius: 4px;
    margin-bottom: 1rem;
    border-left: 4px solid #d32f2f;
  }

  button[type="submit"] {
    width: 100%;
    background: #2196F3;
    color: white;
    border: none;
    padding: 0.75rem;
    border-radius: 4px;
    font-size: 1rem;
    cursor: pointer;
    transition: background-color 0.2s;
  }

  button[type="submit"]:hover:not(:disabled) {
    background: #1976D2;
  }

  button[type="submit"]:disabled {
    background: #ccc;
    cursor: not-allowed;
  }

  .form-options {
    text-align: center;
    margin-top: 1rem;
  }

  .link {
    background: none;
    border: none;
    color: #2196F3;
    text-decoration: underline;
    cursor: pointer;
    font-size: 0.9rem;
  }

  .link:hover {
    color: #1976D2;
  }
</style>