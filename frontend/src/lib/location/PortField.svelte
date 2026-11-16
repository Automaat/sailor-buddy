<script lang="ts">
	import { onDestroy, type Snippet } from 'svelte';
	import { ApiError } from '#lib/api/client.js';
	import { geocode, suggestPorts, type GeocodeBias } from '#lib/api/routes.js';
	import type { GeocodeResult, PortSuggestion } from '#lib/api/types.js';
	import { hasCoordinates, samePortName, type LocatedPort, type PortDraft } from './port.js';

	type Props = {
		id: string;
		label: string;
		/** The typed name and the coordinates picked for it. */
		value: PortDraft;
		disabled?: boolean;
		placeholder?: string;
		labelClass?: string;
		inputClass?: string;
		/** Shows the picked coordinates under the input. */
		showLocation?: boolean;
		/** The map centre the place lookup ranks nearby towns around. */
		bias?: () => GeocodeBias | undefined;
		/** Runs after a suggestion or place is picked into value. */
		onpick?: (port: PortDraft) => Promise<void> | void;
		/** Controls shown between the label and the input. */
		controls?: Snippet;
	};

	let {
		id,
		label,
		value = $bindable(),
		disabled = false,
		placeholder = 'Wpisz nazwę portu, Enter wyszukuje miejsce',
		labelClass = 'mb-1 block text-sm font-medium',
		inputClass = 'w-full rounded-lg border px-3 py-2',
		showLocation = true,
		bias,
		onpick,
		controls
	}: Props = $props();

	const SUGGEST_DELAY_MS = 250;
	const MIN_QUERY = 2;

	let results = $state<GeocodeResult[]>([]);
	let searching = $state(false);
	let searchError = $state('');
	let suggestions = $state<PortSuggestion[]>([]);
	let suggestError = $state('');
	/** Id of the latest lookup; a reply to an older one is dropped. */
	let searchSeq = 0;
	/** Id of the latest suggestion request; a reply to an older one is dropped. */
	let suggestSeq = 0;
	let suggestTimer: ReturnType<typeof setTimeout> | undefined;
	let destroyed = false;
	/** The located port the name was last picked or loaded as: typing back to its name restores its coordinates. */
	let located: LocatedPort | null = null;
	/** The name this field last wrote; another one means the parent replaced the value, so located no longer applies. */
	let written: string | null = null;

	let root: HTMLDivElement;
	let input: HTMLInputElement;

	onDestroy(() => {
		destroyed = true;
		clearTimeout(suggestTimer);
	});

	const query = $derived(value.name.trim());
	const canSearch = $derived(!disabled && !searching && query.length >= MIN_QUERY);
	const dropdownOpen = $derived(
		searching || results.length > 0 || !!searchError || suggestions.length > 0 || !!suggestError
	);
	const photonUsed = $derived(results.some((r) => r.source === 'photon'));

	/**
	 * onInput keeps coordinates only while the name matches the port they
	 * were picked for. Nominatim's usage policy forbids search as you type,
	 * so typing drops the last lookup and refreshes only the club suggestions.
	 */
	function onInput(e: Event & { currentTarget: HTMLInputElement }) {
		if (value.name !== written) located = null;
		if (hasCoordinates(value)) {
			located = { name: value.name, latitude: value.latitude, longitude: value.longitude };
		}
		const name = e.currentTarget.value;
		written = name;
		value =
			located && samePortName(name, located.name)
				? { name, latitude: located.latitude, longitude: located.longitude }
				: { name };
		searchSeq++;
		searching = false;
		searchError = '';
		results = [];
		scheduleSuggest();
	}

	function scheduleSuggest() {
		clearTimeout(suggestTimer);
		suggestSeq++;
		suggestError = '';
		const q = query;
		if (q.length < MIN_QUERY) {
			suggestions = [];
			return;
		}
		suggestTimer = setTimeout(() => void runSuggest(q), SUGGEST_DELAY_MS);
	}

	async function runSuggest(q: string) {
		const seq = ++suggestSeq;
		try {
			const found = await suggestPorts(q);
			if (seq !== suggestSeq || destroyed) return;
			suggestions = found;
		} catch {
			if (seq !== suggestSeq || destroyed) return;
			suggestions = [];
			suggestError = 'Nie udało się pobrać wcześniej odwiedzonych portów';
		}
	}

	async function runSearch() {
		const q = query;
		if (q.length < MIN_QUERY) return;
		const seq = ++searchSeq;
		searching = true;
		searchError = '';
		try {
			const found = await geocode(q, bias?.());
			if (seq !== searchSeq || destroyed) return;
			results = found;
			if (found.length === 0) searchError = 'Brak wyników';
		} catch (e) {
			if (seq !== searchSeq || destroyed) return;
			searchError =
				e instanceof ApiError && e.status === 503
					? 'Wyszukiwarka jest chwilowo zajęta, spróbuj za chwilę'
					: 'Nie udało się wyszukać miejsca';
			results = [];
		} finally {
			if (seq === searchSeq) searching = false;
		}
	}

	/**
	 * searchFromButton moves focus to the input before the lookup starts: the
	 * button disables itself while searching, and a disabled button loses
	 * focus, which would close the field and drop the results.
	 */
	function searchFromButton() {
		input?.focus();
		return runSearch();
	}

	/** close hides the suggestions and lookup results and drops pending requests. */
	function close() {
		clearTimeout(suggestTimer);
		suggestSeq++;
		searchSeq++;
		searching = false;
		suggestions = [];
		suggestError = '';
		results = [];
		searchError = '';
	}

	function pick(port: PortDraft) {
		value = port;
		written = port.name;
		located = hasCoordinates(port)
			? { name: port.name, latitude: port.latitude, longitude: port.longitude }
			: null;
		close();
		input?.focus();
		return onpick?.(port);
	}

	function pickSuggestion(s: PortSuggestion) {
		return pick(
			hasCoordinates(s)
				? { name: s.name, latitude: s.latitude, longitude: s.longitude }
				: { name: s.name }
		);
	}

	function pickResult(r: GeocodeResult) {
		return pick({ name: r.name, latitude: r.latitude, longitude: r.longitude });
	}

	function options(): HTMLButtonElement[] {
		return Array.from(root?.querySelectorAll<HTMLButtonElement>('[data-port-option]') ?? []);
	}

	/**
	 * onInputKeydown: Escape closes the list, ArrowDown moves into it, and
	 * Enter looks the place up; the field sits inside forms, which Enter must
	 * not submit.
	 */
	function onInputKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			if (dropdownOpen) e.preventDefault();
			close();
			return;
		}
		if (e.key === 'ArrowDown') {
			const first = options()[0];
			if (!first) return;
			e.preventDefault();
			first.focus();
			return;
		}
		if (e.key !== 'Enter') return;
		e.preventDefault();
		if (canSearch) void runSearch();
	}

	/** onOptionKeydown moves between options with the arrows, back to the input above the first, and closes on Escape. */
	function onOptionKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			close();
			input?.focus();
			return;
		}
		if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
		e.preventDefault();
		const all = options();
		const at = all.indexOf(e.currentTarget as HTMLButtonElement);
		const next = at + (e.key === 'ArrowDown' ? 1 : -1);
		if (next < 0) input?.focus();
		else all[Math.min(next, all.length - 1)]?.focus();
	}

	/** onFocusOut closes the list once focus leaves the field, so it does not cover the next one. */
	function onFocusOut(e: FocusEvent) {
		const to = e.relatedTarget;
		if (to instanceof Node && root?.contains(to)) return;
		close();
	}
</script>

<div class="relative" bind:this={root} onfocusout={onFocusOut}>
	<label for={id} class={labelClass}>{label}</label>
	{@render controls?.()}
	<div class="flex gap-2">
		<input
			{id}
			bind:this={input}
			type="text"
			value={value.name}
			oninput={onInput}
			onkeydown={onInputKeydown}
			{disabled}
			{placeholder}
			aria-describedby="{id}-hint"
			class={inputClass}
			autocomplete="off"
		/>
		<button
			type="button"
			onclick={searchFromButton}
			disabled={!canSearch}
			aria-label="Szukaj miejsca: {label}"
			class="shrink-0 rounded-lg border px-3 text-sm text-[var(--ocean)] hover:bg-gray-50 disabled:opacity-40"
		>
			Szukaj
		</button>
	</div>
	<p id="{id}-hint" class="sr-only">
		Podpowiedzi portów klubu pojawiają się podczas pisania; strzałka w dół przechodzi do listy.
		Enter wyszukuje miejsce na mapie.
	</p>
	{#if showLocation && hasCoordinates(value)}
		<p class="mt-1 text-xs text-[var(--text-muted)]">
			Na mapie: <span class="font-mono"
				>{value.latitude.toFixed(5)}, {value.longitude.toFixed(5)}</span
			>
		</p>
	{/if}
	{#if dropdownOpen}
		<div
			class="absolute z-[1100] mt-1 max-h-96 w-full overflow-y-auto rounded-lg border bg-white shadow-lg"
		>
			{#if suggestions.length > 0}
				<div
					class="px-3 pt-2 text-xs font-semibold uppercase text-[var(--text-muted)]"
					id="{id}-suggestions-heading"
				>
					Wcześniej odwiedzone
				</div>
				<div role="group" aria-labelledby="{id}-suggestions-heading">
					{#each suggestions as sug, i (i)}
						<button
							type="button"
							data-port-option
							onclick={() => pickSuggestion(sug)}
							onkeydown={onOptionKeydown}
							onmousedown={(e) => e.preventDefault()}
							{disabled}
							class="flex w-full items-baseline justify-between gap-2 px-3 py-2 text-left hover:bg-gray-50 focus:bg-gray-50 disabled:opacity-50"
						>
							<span class="text-sm font-medium">{sug.name}</span>
							<span class="text-xs text-[var(--text-muted)]">
								{#if !hasCoordinates(sug)}bez mapy ·
								{/if}{sug.uses}×
							</span>
						</button>
					{/each}
				</div>
			{:else if suggestError}
				<div class="px-3 py-2 text-xs text-[var(--text-muted)]">{suggestError}</div>
			{/if}
			{#if searching}
				<div class="border-t px-3 py-2 text-sm text-[var(--text-muted)]">Szukanie...</div>
			{:else if searchError}
				<div class="border-t px-3 py-2 text-sm text-[var(--text-muted)]">{searchError}</div>
			{:else if results.length > 0}
				<div
					class="border-t px-3 pt-2 text-xs font-semibold uppercase text-[var(--text-muted)]"
					id="{id}-results-heading"
				>
					Wyniki wyszukiwania
				</div>
				<div role="group" aria-labelledby="{id}-results-heading">
					{#each results as r, i (i)}
						<button
							type="button"
							data-port-option
							onclick={() => pickResult(r)}
							onkeydown={onOptionKeydown}
							onmousedown={(e) => e.preventDefault()}
							{disabled}
							class="block w-full px-3 py-2 text-left hover:bg-gray-50 focus:bg-gray-50 disabled:opacity-50"
						>
							<span class="block text-sm font-medium">{r.name}</span>
							{#if r.label && r.label !== r.name}
								<span class="block text-xs text-[var(--text-muted)]">{r.label}</span>
							{/if}
						</button>
					{/each}
				</div>
				<p class="border-t px-3 py-1 text-[10px] text-[var(--text-muted)]">
					Dane ©
					<a
						href="https://www.openstreetmap.org/copyright"
						target="_blank"
						rel="noopener noreferrer"
						class="underline">współtwórcy OpenStreetMap</a
					>{#if photonUsed}, wyszukiwanie: Photon (komoot){/if}
				</p>
			{/if}
		</div>
	{/if}
</div>
