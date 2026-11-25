<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';

  let {
    name,
    category,
    categoryLabel,
    artwork,
    href,
    onOpen,
    selected = false,
    children,
    actions,
  }: {
    name: string;
    category: string;
    categoryLabel: string;
    artwork?: string;
    href?: string;
    onOpen?: () => void;
    selected?: boolean;
    children?: Snippet;
    actions?: Snippet;
  } = $props();
</script>

{#snippet poster()}
  <div class="title-art" aria-hidden="true">
    {#if artwork}
      {#key artwork}
        <img
          src={artwork}
          alt=""
          loading="lazy"
          onerror={(event) =>
            ((event.currentTarget as HTMLImageElement).hidden = true)}
        />
      {/key}
    {/if}
    <Icon kind={category?.includes('series') ? 'system' : 'library'} size={34} />
    <span>{categoryLabel}</span>
  </div>
{/snippet}

{#if href && !onOpen && !actions}
  <a class="title-card" {href}>
    {@render poster()}
    <div class="title-card-text">
      <h3>{name}</h3>
      {@render children?.()}
    </div>
  </a>
{:else}
  <article class="title-card" class:selected>
    {@render poster()}
    <div class="title-card-text">
      <h3>
        {#if onOpen}
          <button class="card-heading" type="button" onclick={onOpen}
            >{name}</button
          >
        {:else}{name}{/if}
      </h3>
      {@render children?.()}
      {@render actions?.()}
    </div>
  </article>
{/if}

<style>
  .card-heading {
    padding: 0;
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    text-align: start;
    cursor: pointer;
  }
  .card-heading:hover {
    text-decoration: underline;
  }
  .selected {
    border-color: var(--ts-primary);
  }
</style>
