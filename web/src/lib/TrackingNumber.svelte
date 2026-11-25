<script lang="ts">
  type Option = { number: number; label: string; detail: string };
  let {
    id,
    label,
    value,
    min,
    max,
    disabled = false,
    options,
    onChange,
  }: {
    id: string;
    label: string;
    value?: number;
    min: number;
    max: number;
    disabled?: boolean;
    options: Option[];
    onChange: (value: number | undefined) => void;
  } = $props();

  let focused = $state(false);
  let query = $state<string | null>(null);
  let active = $state(-1);
  const visible = $derived(
    focused && !disabled
      ? options.filter(
          (option) =>
            query === null ||
            option.label.toLowerCase().includes(query.trim().toLowerCase()),
        )
      : [],
  );

  function close() {
    focused = false;
    query = null;
    active = -1;
  }

  function select(number: number) {
    onChange(number);
    close();
  }

  function update(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    query = input.value;
    focused = true;
    active = -1;
    if (input.value === '') onChange(undefined);
    else if (/^\d+$/.test(input.value)) {
      const number = Number(input.value);
      if (number >= min && number <= max) onChange(number);
    }
  }

  function keydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && focused) {
      event.preventDefault();
      close();
    } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      focused = true;
      if (!visible.length) return;
      event.preventDefault();
      active =
        event.key === 'ArrowDown'
          ? (active + 1) % visible.length
          : active <= 0
            ? visible.length - 1
            : active - 1;
      document
        .getElementById(id + '-option-' + visible[active].number)
        ?.scrollIntoView({ block: 'nearest' });
    } else if (event.key === 'Enter' && visible.length && active >= 0) {
      event.preventDefault();
      select(visible[active].number);
    }
  }
</script>

<div class="scope-option">
  <label for={id}>{label}</label>
  <input
    {id}
    {disabled}
    type="text"
    inputmode={options.length ? undefined : 'numeric'}
    role="combobox"
    autocomplete="off"
    aria-autocomplete="list"
    aria-expanded={visible.length > 0}
    aria-controls={visible.length ? id + '-options' : undefined}
    aria-activedescendant={visible[active]
      ? id + '-option-' + visible[active].number
      : undefined}
    value={query ?? value ?? ''}
    oninput={update}
    onfocus={() => (focused = true)}
    onclick={() => (focused = true)}
    onblur={close}
    onkeydown={keydown}
  />
  {#if visible.length}
    <ul id={id + '-options'} role="listbox" aria-label={label}>
      {#each visible as option, index (option.number)}
        <li role="presentation">
          <button
            id={id + '-option-' + option.number}
            role="option"
            type="button"
            tabindex="-1"
            aria-selected={index === active}
            onpointerdown={(event) => {
              if (event.button === 0) event.preventDefault();
            }}
            onclick={() => select(option.number)}
            >{option.label}<small>{option.detail}</small></button
          >
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .scope-option {
    position: relative;
    width: 10rem;
    max-width: 100%;
  }
  input {
    width: 100%;
  }
  ul {
    position: absolute;
    left: 0;
    z-index: 10;
    width: max-content;
    min-width: 100%;
    max-width: min(22rem, calc(100vw - 4.5rem));
    max-height: 15rem;
    overflow: auto;
    padding: 4px;
    margin: 4px 0 0;
    list-style: none;
    background: var(--ts-surface);
    border: 1px solid var(--ts-border);
    border-radius: 8px;
    box-shadow: 0 6px 18px #00000018;
  }
  button {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 4px 12px;
    width: 100%;
    padding: 8px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--ts-text);
    font: inherit;
    text-align: left;
  }
  button:hover,
  button[aria-selected='true'] {
    background: var(--ts-selection);
    color: var(--ts-on-selection);
  }
</style>
