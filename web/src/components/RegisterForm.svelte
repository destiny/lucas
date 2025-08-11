<script lang="ts">
  import { auth } from '../lib/auth';

  let username = '';
  let email = '';
  let password = '';
  let confirmPassword = '';
  let error = '';
  let success = '';
  let isLoading = false;

  async function handleSubmit() {
    if (!username) {
      error = 'Username is required';
      return;
    }

    if (!email) {
      error = 'Email is required';
      return;
    }

    if (!password) {
      error = 'Password is required';
      return;
    }

    if (password.length < 6) {
      error = 'Password must be at least 6 characters long';
      return;
    }

    if (password !== confirmPassword) {
      error = 'Passwords do not match';
      return;
    }

    error = '';
    success = '';
    isLoading = true;

    try {
      const result = await auth.register(username, email, password);

      if (!result.success) {
        error = result.error || 'Registration failed';
        // Form fields are preserved on error for user convenience
      } else {
        success = 'Account created successfully! You are now logged in and will be redirected to the dashboard.';
        // Form will be hidden automatically as user is now authenticated
        // Clear form fields only on success
        username = '';
        email = '';
        password = '';
        confirmPassword = '';
      }
    } catch (err) {
      error = err instanceof Error ? err.message : 'Registration failed';
    } finally {
      isLoading = false;
    }
  }

  function validatePasswordMatch() {
    if (confirmPassword && password !== confirmPassword) {
      error = 'Passwords do not match';
    } else if (error === 'Passwords do not match') {
      error = '';
    }
  }

  $: if (confirmPassword) validatePasswordMatch();
</script>

<div class="register-form">
  <h2>Create Lucas Account</h2>
  
  <form on:submit|preventDefault={handleSubmit}>
    <div class="form-group">
      <label for="username">Username:</label>
      <input
        type="text"
        id="username"
        bind:value={username}
        required
        placeholder="Choose a username"
        disabled={isLoading}
      />
    </div>

    <div class="form-group">
      <label for="email">Email:</label>
      <input
        type="email"
        id="email"
        bind:value={email}
        required
        placeholder="Enter your email"
        disabled={isLoading}
      />
    </div>

    <div class="form-group">
      <label for="password">Password:</label>
      <input
        type="password"
        id="password"
        bind:value={password}
        required
        placeholder="Choose a password (min 6 characters)"
        disabled={isLoading}
        minlength="6"
      />
    </div>

    <div class="form-group">
      <label for="confirm-password">Confirm Password:</label>
      <input
        type="password"
        id="confirm-password"
        bind:value={confirmPassword}
        required
        placeholder="Confirm your password"
        disabled={isLoading}
        minlength="6"
      />
    </div>

    {#if error}
      <div class="error">{error}</div>
    {/if}

    {#if success}
      <div class="success">{success}</div>
    {/if}

    <button type="submit" disabled={isLoading}>
      {isLoading ? 'Creating Account...' : 'Create Account'}
    </button>
  </form>
</div>

<style>
  .register-form {
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
    border-color: #4CAF50;
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

  .success {
    color: #2e7d32;
    background: #e8f5e8;
    padding: 0.75rem;
    border-radius: 4px;
    margin-bottom: 1rem;
    border-left: 4px solid #4caf50;
    text-align: center;
  }

  button[type="submit"] {
    width: 100%;
    background: #4CAF50;
    color: white;
    border: none;
    padding: 0.75rem;
    border-radius: 4px;
    font-size: 1rem;
    cursor: pointer;
    transition: background-color 0.2s;
  }

  button[type="submit"]:hover:not(:disabled) {
    background: #45a049;
  }

  button[type="submit"]:disabled {
    background: #ccc;
    cursor: not-allowed;
  }
</style>