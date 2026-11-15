<script lang="ts">
  import { translate, type Locale } from './i18n';
  import type { Availability } from './server-types';
  let { servers = [], locale }: { servers?: Availability[]; locale: Locale } =
    $props();
</script>

<div class="server-badges">
  {#each servers as server}<span
      class="server-badge"
      class:uncertain={server.status === 'uncertain'}
      title={translate(
        locale,
        server.status === 'available'
          ? 'availabilityAvailable'
          : server.status === 'missing'
            ? 'availabilityMissing'
            : 'availabilityUncertain',
      )}
      >{server.name}{server.status === 'uncertain'
        ? ' ?'
        : server.status === 'missing'
          ? ' −'
          : ''}</span
    >{/each}
</div>
