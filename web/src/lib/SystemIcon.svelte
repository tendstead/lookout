<script lang="ts">
  import Icon from './Icon.svelte';
  let {
    size = 22,
    'aria-hidden': hidden = true,
  }: { size?: number | string; 'aria-hidden'?: boolean | 'true' | 'false' } =
    $props();
</script>

<Icon kind="system" {size} aria-hidden={hidden} />
