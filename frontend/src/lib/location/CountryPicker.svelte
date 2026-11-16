<script lang="ts">
	import { countryName, searchCountries, type Country } from './countries.js';

	/**
	 * A multi-select of countries by Polish name. The parent labels it with
	 * `<label for={id}>`; value holds the picked ISO codes in picking order.
	 * The search box is a combobox: arrows move through the matches, Enter
	 * picks one without submitting the form, Escape closes the list and
	 * Backspace in an empty box removes the last country.
	 */
	type Props = {
		id: string;
		value: string[];
		countries: readonly Country[];
		disabled?: boolean;
	};

	let { id, value = $bindable(), countries, disabled = false }: Props = $props();

	let query = $state('');
	let open = $state(false);
	let active = $state(0);

	const listId = $derived(`${id}-options`);
	const options = $derived(
		searchCountries(countries, query).filter((c) => !value.includes(c.code))
	);
	const expanded = $derived(open && options.length > 0);
	const activeId = $derived(
		expanded && options[active] ? `${id}-option-${options[active].code}` : undefined
	);

	$effect(() => {
		if (activeId) document.getElementById(activeId)?.scrollIntoView?.({ block: 'nearest' });
	});

	function pick(code: string) {
		value = [...value, code];
		query = '';
		active = 0;
	}

	function remove(code: string) {
		value = value.filter((c) => c !== code);
	}

	function onKeydown(e: KeyboardEvent) {
		switch (e.key) {
			case 'ArrowDown':
				e.preventDefault();
				if (!open) open = true;
				else active = Math.min(active + 1, options.length - 1);
				break;
			case 'ArrowUp':
				e.preventDefault();
				active = Math.max(active - 1, 0);
				break;
			case 'Enter':
				if (!query.trim() && !expanded) return;
				e.preventDefault();
				if (expanded && options[active]) pick(options[active].code);
				break;
			case 'Escape':
				if (open) e.preventDefault();
				open = false;
				break;
			case 'Backspace':
				if (!query && value.length > 0) remove(value[value.length - 1]);
				break;
		}
	}
</script>

<div>
	{#if value.length > 0}
		<ul class="mb-2 flex flex-wrap gap-2" aria-label="Wybrane kraje">
			{#each value as code (code)}
				<li
					class="flex items-center gap-1 rounded-full bg-gray-100 py-0.5 pl-3 pr-1 text-sm text-[var(--navy)]"
				>
					{countryName(code, countries)}
					<button
						type="button"
						onclick={() => remove(code)}
						{disabled}
						aria-label="Usuń kraj: {countryName(code, countries)}"
						class="rounded-full px-2 text-[var(--text-muted)] hover:bg-gray-200 hover:text-[var(--navy)] disabled:opacity-50"
					>
						×
					</button>
				</li>
			{/each}
		</ul>
	{/if}
	<div class="relative">
		<input
			{id}
			type="text"
			role="combobox"
			aria-expanded={expanded}
			aria-controls={listId}
			aria-autocomplete="list"
			aria-activedescendant={activeId}
			autocomplete="off"
			placeholder="Wpisz nazwę kraju…"
			bind:value={query}
			{disabled}
			oninput={() => {
				open = true;
				active = 0;
			}}
			onfocus={() => (open = true)}
			onblur={() => (open = false)}
			onkeydown={onKeydown}
			class="w-full rounded-lg border px-3 py-2"
		/>
		<ul
			id={listId}
			role="listbox"
			aria-label="Pasujące kraje"
			hidden={!expanded}
			class="absolute z-20 mt-1 max-h-60 w-full overflow-y-auto rounded-lg border bg-white shadow-lg"
		>
			{#each options as country, i (country.code)}
				<li
					id="{id}-option-{country.code}"
					role="option"
					aria-selected={i === active}
					tabindex="-1"
					onmousedown={(e) => e.preventDefault()}
					onclick={() => pick(country.code)}
					onkeydown={onKeydown}
					class="cursor-pointer px-3 py-3 text-sm sm:py-2 {i === active ? 'bg-gray-100' : ''}"
				>
					{country.name}
				</li>
			{/each}
		</ul>
		{#if open && query.trim() && options.length === 0}
			<p class="mt-1 text-sm text-[var(--text-muted)]" role="status">Brak pasującego kraju</p>
		{/if}
	</div>
</div>
