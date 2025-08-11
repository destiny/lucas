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

// Remote Control Command Configuration System
// Centralized definitions for button layouts and command mappings

export interface ButtonConfig {
  id: string;
  label: string;
  action: string;
  icon?: string;
  style?: string;
  size?: 'small' | 'medium' | 'large';
}

export interface ResponsiveConfig {
  mobile: {
    layout: 'horizontal' | 'vertical' | 'grid' | 'navigation';
    columns?: number;
    priority: number; // Lower number = higher priority (shown first on mobile)
  };
  desktop: {
    layout: 'horizontal' | 'vertical' | 'grid' | 'navigation';
    columns?: number;
  };
}

export interface ControlGroupConfig {
  title: string;
  layout: 'horizontal' | 'vertical' | 'grid' | 'navigation';
  buttons: ButtonConfig[];
  responsive: ResponsiveConfig;
  collapsible?: boolean; // Can be collapsed on mobile
}

// Command definitions for Sony Bravia TV remote
export const remoteCommands: ControlGroupConfig[] = [
  // Power Controls - Most important
  {
    title: "Power",
    layout: "horizontal",
    buttons: [
      {
        id: "power",
        label: "Power",
        action: "power",
        style: "power-btn",
        size: "large"
      },
      {
        id: "power_on",
        label: "On",
        action: "power_on",
        size: "medium"
      },
      {
        id: "power_off",
        label: "Off", 
        action: "power_off",
        size: "medium"
      }
    ],
    responsive: {
      mobile: {
        layout: "horizontal",
        priority: 1
      },
      desktop: {
        layout: "horizontal"
      }
    }
  },

  // Navigation - Essential for mobile
  {
    title: "Navigation",
    layout: "navigation",
    buttons: [
      {
        id: "up",
        label: "▲",
        action: "up",
        style: "nav-btn"
      },
      {
        id: "left", 
        label: "◀",
        action: "left",
        style: "nav-btn"
      },
      {
        id: "confirm",
        label: "OK",
        action: "confirm", 
        style: "nav-btn nav-center",
        size: "large"
      },
      {
        id: "right",
        label: "▶", 
        action: "right",
        style: "nav-btn"
      },
      {
        id: "down",
        label: "▼",
        action: "down",
        style: "nav-btn"
      }
    ],
    responsive: {
      mobile: {
        layout: "navigation",
        priority: 2
      },
      desktop: {
        layout: "navigation"
      }
    }
  },

  // Volume Controls - Essential
  {
    title: "Volume", 
    layout: "horizontal",
    buttons: [
      {
        id: "volume_up",
        label: "Vol +",
        action: "volume_up",
        style: "volume-btn"
      },
      {
        id: "volume_down", 
        label: "Vol -",
        action: "volume_down",
        style: "volume-btn"
      },
      {
        id: "mute",
        label: "Mute",
        action: "mute",
        style: "mute-btn"
      }
    ],
    responsive: {
      mobile: {
        layout: "horizontal",
        priority: 3
      },
      desktop: {
        layout: "horizontal"
      }
    }
  },

  // Channel Controls
  {
    title: "Channel",
    layout: "horizontal", 
    buttons: [
      {
        id: "channel_up",
        label: "CH +",
        action: "channel_up",
        style: "channel-btn"
      },
      {
        id: "channel_down",
        label: "CH -", 
        action: "channel_down",
        style: "channel-btn"
      }
    ],
    responsive: {
      mobile: {
        layout: "horizontal",
        priority: 4
      },
      desktop: {
        layout: "horizontal"
      }
    },
    collapsible: true
  },

  // Menu Controls
  {
    title: "Menu",
    layout: "horizontal",
    buttons: [
      {
        id: "home",
        label: "Home",
        action: "home", 
        style: "menu-btn"
      },
      {
        id: "menu",
        label: "Menu",
        action: "menu",
        style: "menu-btn"
      },
      {
        id: "back",
        label: "Back",
        action: "back",
        style: "menu-btn"
      }
    ],
    responsive: {
      mobile: {
        layout: "horizontal",
        priority: 5
      },
      desktop: {
        layout: "horizontal"
      }
    },
    collapsible: true
  },

  // Input Controls - Less important on mobile
  {
    title: "Input",
    layout: "grid",
    buttons: [
      {
        id: "input",
        label: "Input",
        action: "input",
        style: "input-btn"
      },
      {
        id: "hdmi1",
        label: "HDMI1", 
        action: "hdmi1",
        style: "hdmi-btn",
        size: "small"
      },
      {
        id: "hdmi2",
        label: "HDMI2",
        action: "hdmi2", 
        style: "hdmi-btn",
        size: "small"
      },
      {
        id: "hdmi3", 
        label: "HDMI3",
        action: "hdmi3",
        style: "hdmi-btn", 
        size: "small"
      },
      {
        id: "hdmi4",
        label: "HDMI4",
        action: "hdmi4",
        style: "hdmi-btn",
        size: "small"
      }
    ],
    responsive: {
      mobile: {
        layout: "grid",
        columns: 3,
        priority: 6
      },
      desktop: {
        layout: "grid", 
        columns: 3
      }
    },
    collapsible: true
  }
];

// Helper function to get commands by priority for mobile
export function getCommandsByPriority(): ControlGroupConfig[] {
  return [...remoteCommands].sort((a, b) => a.responsive.mobile.priority - b.responsive.mobile.priority);
}

// Helper function to get essential commands (priority 1-3)
export function getEssentialCommands(): ControlGroupConfig[] {
  return remoteCommands.filter(cmd => cmd.responsive.mobile.priority <= 3);
}

// Helper function to get advanced commands (priority 4+)
export function getAdvancedCommands(): ControlGroupConfig[] {
  return remoteCommands.filter(cmd => cmd.responsive.mobile.priority > 3);
}