<script lang="ts">
  import { onMount } from 'svelte';
  import { translate, type Locale } from './i18n';
  import { api, type Server } from './server-types';
  let {
    locale,
    value = $bindable(''),
    onchange,
  }: { locale: Locale; value?: string; onchange: () => void } = $props();
  let servers = $state<Server[]>([]);
  onMount(() => {
    void api('/api/v1/servers')
      .then((v) => (servers = v.items))
      .catch(() => {});
  });
</script>

<label class="server-filter"
  >{translate(locale, 'servers')}<select bind:value onchange={() => onchange()}
    ><option value="">{translate(locale, 'allServers')}</option
    >{#each servers as server}<option value={server.id}>{server.name}</option
      >{/each}</select
  ></label
>
