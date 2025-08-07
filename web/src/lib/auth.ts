import { writable } from 'svelte/store';

export interface User {
  id: number;
  username: string;
  email: string;
  api_key: string;
  created_at: string;
}

export interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

// Create the auth store
function createAuthStore() {
  const { subscribe, set, update } = writable<AuthState>({
    user: null,
    token: localStorage.getItem('auth_token'),
    isAuthenticated: false,
    isLoading: false,
  });

  return {
    subscribe,
    
    // Initialize authentication state from stored token
    async init() {
      update(state => ({ ...state, isLoading: true }));
      
      const token = localStorage.getItem('auth_token');
      if (token) {
        try {
          const user = await apiClient.getCurrentUser(token);
          set({
            user,
            token,
            isAuthenticated: true,
            isLoading: false,
          });
        } catch (error) {
          // Token is invalid, clear it
          localStorage.removeItem('auth_token');
          set({
            user: null,
            token: null,
            isAuthenticated: false,
            isLoading: false,
          });
        }
      } else {
        update(state => ({ ...state, isLoading: false }));
      }
    },

    // Login user
    async login(username: string, email: string, password: string) {
      update(state => ({ ...state, isLoading: true }));
      
      try {
        const response = await apiClient.login(username, email, password);
        const { user, token } = response;
        
        localStorage.setItem('auth_token', token);
        set({
          user,
          token,
          isAuthenticated: true,
          isLoading: false,
        });
        
        return { success: true };
      } catch (error) {
        update(state => ({ ...state, isLoading: false }));
        return { 
          success: false, 
          error: error instanceof Error ? error.message : 'Login failed' 
        };
      }
    },

    // Register user
    async register(username: string, email: string, password: string) {
      update(state => ({ ...state, isLoading: true }));
      
      try {
        const response = await apiClient.register(username, email, password);
        const { user, token } = response;
        
        localStorage.setItem('auth_token', token);
        set({
          user,
          token,
          isAuthenticated: true,
          isLoading: false,
        });
        
        return { success: true };
      } catch (error) {
        update(state => ({ ...state, isLoading: false }));
        return { 
          success: false, 
          error: error instanceof Error ? error.message : 'Registration failed' 
        };
      }
    },

    // Logout user
    logout() {
      localStorage.removeItem('auth_token');
      set({
        user: null,
        token: null,
        isAuthenticated: false,
        isLoading: false,
      });
    },
  };
}

export const auth = createAuthStore();

// API Client
class ApiClient {
  private baseUrl = '/api/v1';

  private async request(endpoint: string, options: RequestInit = {}) {
    const url = `${this.baseUrl}${endpoint}`;
    
    const response = await fetch(url, {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      ...options,
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      throw new Error(errorData.message || `HTTP ${response.status}: ${response.statusText}`);
    }

    return response.json();
  }

  private async authenticatedRequest(endpoint: string, token: string, options: RequestInit = {}) {
    return this.request(endpoint, {
      ...options,
      headers: {
        'Authorization': `Bearer ${token}`,
        ...options.headers,
      },
    });
  }

  async login(username: string, email: string, password: string) {
    const requestData: any = { password };
    if (username) requestData.username = username;
    if (email) requestData.email = email;

    return this.request('/auth/login', {
      method: 'POST',
      body: JSON.stringify(requestData),
    });
  }

  async register(username: string, email: string, password: string) {
    return this.request('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, email, password }),
    });
  }

  async getCurrentUser(token: string): Promise<User> {
    const response = await this.authenticatedRequest('/auth/me', token);
    return response.user;
  }

  async getGatewayStatus() {
    return this.request('/gateway/status');
  }

  async getUserDevices(userId: number, token: string) {
    return this.authenticatedRequest(`/users/${userId}/devices`, token);
  }

  async getUserHubs(userId: number, token: string) {
    return this.authenticatedRequest(`/users/${userId}/hubs`, token);
  }

  async sendDeviceAction(userId: number, deviceId: string, action: any, token: string) {
    return this.authenticatedRequest(`/users/${userId}/devices/${deviceId}/action`, token, {
      method: 'POST',
      body: JSON.stringify(action),
    });
  }
}

export const apiClient = new ApiClient();