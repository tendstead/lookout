<script lang="ts">
  let {
    kind = 'library',
    size = 22,
    'aria-hidden': hidden = true,
  }: {
    kind?: string;
    size?: number | string;
    'aria-hidden'?: boolean | 'true' | 'false';
  } = $props();
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.7"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden={hidden}
>
  {#if kind === 'system'}<rect x="3" y="3" width="18" height="7" rx="2" /><rect
      x="3"
      y="14"
      width="18"
      height="7"
      rx="2"
    /><path d="M7 6.5h.01M7 17.5h.01M11 6.5h6M11 17.5h6" />
  {:else if kind === 'refresh'}<path
      d="M20 7v5h-5M4 17v-5h5M6.1 7a7 7 0 0 1 11.8-1L20 9M4 15l2.1 3a7 7 0 0 0 11.8-1"
    />
  {:else if kind === 'moon'}<path
      d="M20.7 13.5A9 9 0 0 1 10.5 3.3a9 9 0 1 0 10.2 10.2Z"
    />
  {:else if kind === 'sun'}<circle cx="12" cy="12" r="4" /><path
      d="M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.5 1.5M17.5 17.5 19 19M5 19l1.5-1.5M17.5 6.5 19 5"
    />
  {:else if kind === 'eye'}<path
      d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"
    /><circle cx="12" cy="12" r="3" />
  {:else}<rect x="3" y="4" width="7" height="16" rx="1.5" /><rect
      x="14"
      y="4"
      width="7"
      height="16"
      rx="1.5"
    /><path d="M6 8h1M6 12h1M17 8h1M17 12h1" />{/if}
</svg>
