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
  import type { ControlGroupConfig, ButtonConfig } from '../lib/remoteCommands';

  export let config: ControlGroupConfig;
  export let disabled: boolean = false;
  export let onButtonClick: (action: string) => void;

  // Reactive variables for responsive behavior
  let isMobile = false;
  let collapsed = false;

  // Check if mobile based on screen size
  function checkMobile() {
    isMobile = window.innerWidth < 768; // Mobile breakpoint
  }

  // Initialize and add resize listener
  if (typeof window !== 'undefined') {
    checkMobile();
    window.addEventListener('resize', checkMobile);
  }

  // Get current layout based on screen size
  $: currentLayout = isMobile ? config.responsive.mobile.layout : config.responsive.desktop.layout;
  $: currentColumns = isMobile ? config.responsive.mobile.columns : config.responsive.desktop.columns;
  
  // Toggle collapse state
  function toggleCollapse() {
    collapsed = !collapsed;
  }

  // Handle button click
  function handleButtonClick(button: ButtonConfig) {
    if (!disabled) {
      onButtonClick(button.action);
    }
  }

  // Generate CSS classes for layout
  function getLayoutClasses(layout: string, columns?: number): string {
    const baseClass = 'control-group-buttons';
    
    switch (layout) {
      case 'horizontal':
        return `${baseClass} horizontal`;
      case 'vertical':
        return `${baseClass} vertical`;
      case 'grid':
        return `${baseClass} grid grid-cols-${columns || 3}`;
      case 'navigation':
        return `${baseClass} navigation`;
      default:
        return baseClass;
    }
  }

  // Generate button classes
  function getButtonClasses(button: ButtonConfig): string {
    let classes = 'control-btn';
    
    if (button.style) {
      classes += ` ${button.style}`;
    }
    
    if (button.size) {
      classes += ` btn-${button.size}`;
    }

    if (disabled) {
      classes += ' btn-disabled';
    }
    
    return classes;
  }
</script>

<div class="control-group" class:mobile={isMobile} class:collapsed>
  <!-- Group Header -->
  <div class="group-header">
    <h3 class="group-title">{config.title}</h3>
    {#if config.collapsible && isMobile}
      <button 
        class="collapse-btn"
        on:click={toggleCollapse}
        aria-label="Toggle {config.title} controls"
      >
        <span class="collapse-icon" class:rotated={collapsed}>▼</span>
      </button>
    {/if}
  </div>

  <!-- Button Container -->
  {#if !collapsed || !isMobile}
    <div class={getLayoutClasses(currentLayout, currentColumns)}>
      {#if currentLayout === 'navigation'}
        <!-- Special navigation layout -->
        <div class="nav-grid">
          <div class="nav-row">
            {#each config.buttons as button}
              {#if button.id === 'up'}
                <button
                  class={getButtonClasses(button)}
                  {disabled}
                  on:click={() => handleButtonClick(button)}
                  aria-label={button.label}
                >
                  {button.label}
                </button>
              {/if}
            {/each}
          </div>
          
          <div class="nav-row nav-middle">
            {#each config.buttons as button}
              {#if ['left', 'confirm', 'right'].includes(button.id)}
                <button
                  class={getButtonClasses(button)}
                  {disabled}
                  on:click={() => handleButtonClick(button)}
                  aria-label={button.label}
                >
                  {button.label}
                </button>
              {/if}
            {/each}
          </div>
          
          <div class="nav-row">
            {#each config.buttons as button}
              {#if button.id === 'down'}
                <button
                  class={getButtonClasses(button)}
                  {disabled}
                  on:click={() => handleButtonClick(button)}
                  aria-label={button.label}
                >
                  {button.label}
                </button>
              {/if}
            {/each}
          </div>
        </div>
      {:else}
        <!-- Standard button layout -->
        {#each config.buttons as button}
          <button
            class={getButtonClasses(button)}
            {disabled}
            on:click={() => handleButtonClick(button)}
            aria-label={button.label}
          >
            {#if button.icon}
              <span class="btn-icon">{button.icon}</span>
            {/if}
            <span class="btn-label">{button.label}</span>
          </button>
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
  .control-group {
    background: linear-gradient(145deg, #34495e, #2c3e50);
    border-radius: 1rem;
    padding: 1rem;
    margin-bottom: 1rem;
    box-shadow: 0 4px 8px rgba(0,0,0,0.2);
    transition: all 0.3s ease;
  }

  .control-group.mobile {
    margin-bottom: 0.75rem;
    padding: 0.75rem;
  }

  .control-group.collapsed {
    padding-bottom: 0.75rem;
  }

  .group-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }

  .group-title {
    color: #ecf0f1;
    font-size: 0.9rem;
    margin: 0;
    text-transform: uppercase;
    letter-spacing: 1px;
    font-weight: 600;
  }

  .collapse-btn {
    background: transparent;
    border: none;
    color: #bdc3c7;
    cursor: pointer;
    padding: 0.25rem;
    border-radius: 0.25rem;
    transition: color 0.2s ease;
  }

  .collapse-btn:hover {
    color: #ecf0f1;
  }

  .collapse-icon {
    display: inline-block;
    transition: transform 0.2s ease;
    font-size: 0.8rem;
  }

  .collapse-icon.rotated {
    transform: rotate(-90deg);
  }

  /* Button Containers */
  .control-group-buttons {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .control-group-buttons.horizontal {
    flex-direction: row;
    justify-content: center;
  }

  .control-group-buttons.vertical {
    flex-direction: column;
    align-items: center;
  }

  .control-group-buttons.grid {
    display: grid;
    gap: 0.5rem;
  }

  .grid-cols-2 { grid-template-columns: repeat(2, 1fr); }
  .grid-cols-3 { grid-template-columns: repeat(3, 1fr); }
  .grid-cols-4 { grid-template-columns: repeat(4, 1fr); }

  /* Navigation Layout */
  .control-group-buttons.navigation {
    justify-content: center;
  }

  .nav-grid {
    display: grid;
    grid-template-rows: repeat(3, auto);
    gap: 0.5rem;
    max-width: 200px;
  }

  .nav-row {
    display: grid;
    grid-template-columns: 1fr;
    justify-items: center;
  }

  .nav-middle {
    display: grid !important;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 0.5rem;
    align-items: center;
  }

  /* Button Styles */
  .control-btn {
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
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.25rem;
  }

  .control-btn:hover:not(.btn-disabled) {
    background: linear-gradient(145deg, #2980b9, #21618c);
    transform: translateY(-1px);
    box-shadow: 
      0 6px 12px rgba(0,0,0,0.3),
      inset 0 1px 0 rgba(255,255,255,0.2);
  }

  .control-btn:active:not(.btn-disabled) {
    transform: translateY(0);
    box-shadow: 
      0 2px 4px rgba(0,0,0,0.2),
      inset 0 1px 0 rgba(255,255,255,0.2);
  }

  .control-btn.btn-disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  /* Button Sizes */
  .btn-small {
    padding: 0.5rem 0.75rem;
    font-size: 0.8rem;
    min-width: 50px;
  }

  .btn-medium {
    padding: 0.75rem 1rem;
    font-size: 0.9rem;
    min-width: 60px;
  }

  .btn-large {
    padding: 1rem 1.25rem;
    font-size: 1rem;
    min-width: 80px;
  }

  /* Specific Button Styles */
  .power-btn {
    background: linear-gradient(145deg, #e74c3c, #c0392b);
  }

  .power-btn:hover:not(.btn-disabled) {
    background: linear-gradient(145deg, #c0392b, #a93226);
  }

  .volume-btn, .channel-btn {
    background: linear-gradient(145deg, #27ae60, #229954);
  }

  .volume-btn:hover:not(.btn-disabled), .channel-btn:hover:not(.btn-disabled) {
    background: linear-gradient(145deg, #229954, #1e8449);
  }

  .mute-btn {
    background: linear-gradient(145deg, #f39c12, #e67e22);
  }

  .mute-btn:hover:not(.btn-disabled) {
    background: linear-gradient(145deg, #e67e22, #d35400);
  }

  .hdmi-btn {
    background: linear-gradient(145deg, #9b59b6, #8e44ad);
  }

  .hdmi-btn:hover:not(.btn-disabled) {
    background: linear-gradient(145deg, #8e44ad, #7d3c98);
  }

  .nav-btn {
    width: 60px;
    height: 60px;
    border-radius: 50%;
    font-size: 1.2rem;
    font-weight: bold;
    min-width: unset;
  }

  .nav-center {
    background: linear-gradient(145deg, #f39c12, #e67e22);
    font-size: 1rem;
  }

  .nav-center:hover:not(.btn-disabled) {
    background: linear-gradient(145deg, #e67e22, #d35400);
  }

  /* Mobile Responsive */
  @media (max-width: 767px) {
    .control-group {
      padding: 0.75rem;
      border-radius: 0.75rem;
    }

    .group-title {
      font-size: 0.8rem;
    }

    .control-btn {
      padding: 0.6rem 0.8rem;
      font-size: 0.85rem;
      min-width: 55px;
    }

    .btn-small {
      padding: 0.5rem 0.6rem;
      font-size: 0.75rem;
      min-width: 45px;
    }

    .btn-large {
      padding: 0.75rem 1rem;
      font-size: 0.9rem;
      min-width: 70px;
    }

    .nav-btn {
      width: 50px;
      height: 50px;
      font-size: 1rem;
    }

    .control-group-buttons {
      gap: 0.5rem;
    }

    .nav-grid {
      max-width: 180px;
    }
  }

  /* Touch targets for better mobile experience */
  @media (max-width: 767px) {
    .control-btn {
      min-height: 44px; /* iOS recommended touch target */
      touch-action: manipulation; /* Prevent zoom on double-tap */
    }
  }
</style>