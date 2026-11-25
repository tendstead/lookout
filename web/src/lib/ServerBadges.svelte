<script lang="ts">
  import { translate, type Locale } from './i18n';
  type Badge = { server_id: string; name: string; status?: string };
  let {
    servers = [],
    locale,
    selected,
    onSelect,
    disabled = false,
  }: {
    servers?: Badge[];
    locale: Locale;
    selected?: string;
    onSelect?: (id: string) => void;
    disabled?: boolean;
  } = $props();
  const label = (server: Badge) =>
    translate(
      locale,
      server.status === 'available'
        ? 'availabilityAvailable'
        : server.status === 'missing'
          ? 'availabilityMissing'
          : 'availabilityUncertain',
    );
</script>

<div class="server-badges">
  {#each servers as server (server.server_id)}
    {#if onSelect}
      <button
        type="button"
        class="server-badge"
        class:uncertain={server.status === 'uncertain'}
        title={server.status ? label(server) : undefined}
        aria-pressed={selected === server.server_id}
        {disabled}
        onclick={() => onSelect?.(server.server_id)}
        >{server.name}<span aria-hidden="true"
          >{server.status === 'uncertain'
            ? ' ?'
            : server.status === 'missing'
              ? ' −'
              : ''}</span
        ></button
      >
    {:else}
      <span
        class="server-badge"
        class:uncertain={server.status === 'uncertain'}
        title={label(server)}
        >{server.name}{server.status === 'uncertain'
          ? ' ?'
          : server.status === 'missing'
            ? ' −'
            : ''}</span
      >
    {/if}
  {/each}
</div>

<style>
  button.server-badge {
    cursor: pointer;
    font: inherit;
    font-size: 11px;
  }
  button.server-badge[aria-pressed='true'] {
    border-color: var(--ts-primary);
    background: var(--ts-selection);
    color: var(--ts-on-selection);
  }
</style>
