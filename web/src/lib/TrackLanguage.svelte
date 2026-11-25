<script lang="ts">
  import { languageLabel } from './file-comparison';
  import type { Locale } from './i18n';

  let {
    reported,
    locale,
    showFlags,
  }: {
    reported?: string;
    locale: Locale;
    showFlags: boolean;
  } = $props();
  const label = $derived(languageLabel(reported, locale));
  let tapped = $state(false);
</script>

{#if showFlags && label.flag}
  <span class="flag-label" class:tapped>
    <button
      type="button"
      aria-label={label.name}
      onclick={() => (tapped = !tapped)}
      onblur={() => (tapped = false)}
    >
      <span aria-hidden="true">{label.flag}</span>
    </button>
    <span class="flag-tooltip" role="tooltip">{label.name}</span>
  </span>
{:else}
  <span>{label.name}</span>
{/if}

<style>
  .flag-label {
    display: inline-block;
    max-width: 100%;
  }
  button {
    min-width: 36px;
    min-height: 36px;
    padding: 2px 6px;
    border: 1px solid var(--ts-border);
    border-radius: 6px;
    font-size: 1rem;
    background: var(--ts-surface);
    color: var(--ts-text);
    cursor: pointer;
  }
  .flag-tooltip {
    display: none;
    margin-top: 4px;
    width: fit-content;
    padding: 4px 6px;
    border-radius: 4px;
    background: var(--ts-selection);
    color: var(--ts-on-selection);
  }
  .flag-label:focus-within .flag-tooltip,
  .tapped .flag-tooltip {
    display: block;
  }
  @media (hover: hover) {
    .flag-label:hover .flag-tooltip {
      display: block;
    }
  }
</style>
