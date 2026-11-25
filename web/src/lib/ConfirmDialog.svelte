<script lang="ts">
  import type { Snippet } from 'svelte';
  let {
    open,
    title,
    cancelLabel,
    confirmLabel,
    onCancel,
    onConfirm,
    children,
  }: {
    open: boolean;
    title: string;
    cancelLabel: string;
    confirmLabel: string;
    onCancel: () => void;
    onConfirm: () => void;
    children?: Snippet;
  } = $props();
  const id = $props.id();
  let dialog = $state<HTMLDialogElement>();
  let cancelButton: HTMLButtonElement;
  $effect(() => {
    if (!dialog) return;
    if (open && !dialog.open) {
      dialog.showModal();
      cancelButton?.focus();
    } else if (!open && dialog.open) {
      dialog.close();
    }
  });
</script>

<dialog
  bind:this={dialog}
  aria-labelledby={id}
  oncancel={(event) => {
    event.preventDefault();
    onCancel();
  }}
>
  <h2 {id}>{title}</h2>
  {@render children?.()}
  <div class="actions">
    <button
      type="button"
      class="cancel"
      bind:this={cancelButton}
      onclick={onCancel}>{cancelLabel}</button
    >
    <button type="button" class="confirm" onclick={onConfirm}
      >{confirmLabel}</button
    >
  </div>
</dialog>

<style>
  dialog {
    width: min(24rem, calc(100% - 2rem));
    box-sizing: border-box;
    padding: 20px;
    border: 1px solid var(--ts-border);
    border-radius: 12px;
    background: var(--ts-surface);
    color: var(--ts-text);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 45%);
  }
  button {
    min-height: 44px;
    padding: 6px 13px;
    border-radius: 8px;
    border: 1px solid var(--ts-border);
    font: inherit;
    cursor: pointer;
  }
  .cancel {
    background: var(--ts-surface);
    color: var(--ts-text);
  }
  .confirm {
    background: var(--ts-primary);
    color: var(--ts-on-primary);
    border-color: var(--ts-primary);
  }
  h2 {
    margin: 0 0 8px;
    font-size: 1.125rem;
  }
  .actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }
</style>
