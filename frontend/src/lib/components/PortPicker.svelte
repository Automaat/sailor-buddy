<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { Map as LeafletMap, LayerGroup, LeafletMouseEvent } from 'leaflet';
	import 'leaflet/dist/leaflet.css';
	import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
	import markerIcon from 'leaflet/dist/images/marker-icon.png';
	import markerShadow from 'leaflet/dist/images/marker-shadow.png';
	import { ApiError } from '#lib/api/client.js';
	import { geocode, suggestPorts, type GeocodeBias } from '#lib/api/routes.js';
	import type { GeocodeResult, PortSuggestion, VoyagePortBody } from '#lib/api/types.js';

	// Both VoyagePort and VoyagePortBody satisfy this, so the picker works for
	// the completion modal (unsaved bodies) and the voyage page (persisted
	// ports) alike. A port recorded by name only has no coordinates: it stays in
	// the route list but is not drawn on the map.
	type PortLike = {
		name: string;
		latitude?: number | null;
		longitude?: number | null;
		anchorage?: boolean;
	};

	type Props = {
		ports: PortLike[];
		readonly?: boolean;
		onAdd?: (port: VoyagePortBody) => Promise<void> | void;
		onRemove?: (index: number) => Promise<void> | void;
		// onReorder moves the port at `from` to `to`; drag-and-drop is only
		// offered when a parent supplies it.
		onReorder?: (from: number, to: number) => Promise<void> | void;
	};

	let { ports, readonly = false, onAdd, onRemove, onReorder }: Props = $props();

	let query = $state('');
	let anchorage = $state(false);
	let results = $state<GeocodeResult[]>([]);
	let searching = $state(false);
	let searchError = $state('');
	let busy = $state(false);
	// Monotonic id of the latest search; stale responses are discarded.
	let searchSeq = 0;

	// Club ports matching the typed text, fetched while typing. Only this
	// local endpoint runs as you type; the external geocoder does not.
	const SUGGEST_DELAY_MS = 250;
	let suggestions = $state<PortSuggestion[]>([]);
	let suggestError = $state('');
	let suggestSeq = 0;
	let suggestTimer: ReturnType<typeof setTimeout> | undefined;

	// A point picked by clicking the map, saved under a name the user types.
	let pin = $state<{ lat: number; lon: number } | null>(null);
	let pinName = $state('');

	let mapEl: HTMLDivElement;
	// Leaflet touches `window`, so it is imported only in the browser (onMount).
	let L: typeof import('leaflet') | undefined;
	let map: LeafletMap | undefined;
	let layer: LayerGroup | undefined;
	let pinLayer: LayerGroup | undefined;
	let destroyed = false;

	onMount(async () => {
		L = await import('leaflet');
		// The dynamic import is async; bail if the component unmounted meanwhile.
		if (destroyed || !mapEl) return;
		L.Icon.Default.mergeOptions({
			iconRetinaUrl: markerIcon2x,
			iconUrl: markerIcon,
			shadowUrl: markerShadow
		});
		map = L.map(mapEl).setView([54, 15], 4);
		L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
			attribution: '© OpenStreetMap',
			maxZoom: 18
		}).addTo(map);
		layer = L.layerGroup().addTo(map);
		pinLayer = L.layerGroup().addTo(map);
		map.on('click', onMapClick);
		renderMarkers();
	});

	onDestroy(() => {
		destroyed = true;
		clearTimeout(suggestTimer);
		map?.remove();
	});

	function onMapClick(e: LeafletMouseEvent) {
		if (readonly || busy || !map) return;
		const at = map.wrapLatLng(e.latlng);
		pin = { lat: roundCoord(at.lat), lon: roundCoord(at.lng) };
		pinName = query.trim();
	}

	// Five decimals locate a port to about a metre.
	function roundCoord(v: number): number {
		return Math.round(v * 1e5) / 1e5;
	}

	$effect(() => {
		const at = pin;
		if (!L || !map || !pinLayer) return;
		pinLayer.clearLayers();
		if (at) {
			L.circleMarker([at.lat, at.lon], { radius: 8, color: '#dc2626', weight: 2 }).addTo(pinLayer);
		}
	});

	function clearPin() {
		pin = null;
		pinName = '';
	}

	function addPin() {
		const name = pinName.trim();
		if (!pin || !name) return;
		return add({ name, latitude: pin.lat, longitude: pin.lon });
	}

	// The map centre, wrapped into [-180, 180] after panning across the
	// antimeridian, ranks nearby towns first.
	function mapBias(): GeocodeBias | undefined {
		if (!map) return undefined;
		const c = map.wrapLatLng(map.getCenter());
		if (!Number.isFinite(c.lat) || !Number.isFinite(c.lng)) return undefined;
		return { lat: Math.round(c.lat * 1e4) / 1e4, lon: Math.round(c.lng * 1e4) / 1e4 };
	}

	// renderMarkers redraws every port marker plus the route polyline whenever
	// the ports list changes. $effect re-runs it; the map-ready guard skips the
	// pre-mount run.
	function renderMarkers() {
		if (!L || !map || !layer) return;
		layer.clearLayers();
		const points: [number, number][] = [];
		ports.forEach((p, i) => {
			if (!hasCoords(p)) return;
			const point: [number, number] = [p.latitude, p.longitude];
			points.push(point);
			L!
				.marker(point)
				.bindTooltip(tooltipLabel(`${i + 1}. ${p.anchorage ? '⚓ ' : ''}${p.name}`), {
					permanent: true,
					direction: 'top'
				})
				.addTo(layer!);
		});
		if (points.length > 1) {
			L.polyline(points, { color: '#0369a1', weight: 2, dashArray: '4 6' }).addTo(layer);
		}
		if (points.length > 0) {
			map.fitBounds(L.latLngBounds(points), { padding: [40, 40], maxZoom: 9 });
		}
	}

	// Leaflet inserts string tooltip content as HTML; port names are
	// user-entered, so they go in as a text node.
	function tooltipLabel(text: string): HTMLElement {
		const el = document.createElement('span');
		el.textContent = text;
		return el;
	}

	function hasCoords(p: PortLike): p is PortLike & { latitude: number; longitude: number } {
		return typeof p.latitude === 'number' && typeof p.longitude === 'number';
	}

	$effect(() => {
		// Track the full ports content so edits/reorders (not just length
		// changes) trigger a redraw.
		ports.map((p) => `${p.name}:${p.latitude}:${p.longitude}:${p.anchorage}`).join('|');
		renderMarkers();
	});

	const canSearch = $derived(!busy && !searching && query.trim().length >= 2);

	// Typing invalidates the previous search and refreshes the club
	// suggestions after a short pause. Nominatim's usage policy forbids
	// search-as-you-type, so a geocoder lookup runs on Enter or "Szukaj".
	function onInput() {
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
		const q = query.trim();
		if (q.length < 2) {
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

	function closeDropdown() {
		clearTimeout(suggestTimer);
		suggestSeq++;
		searchSeq++;
		searching = false;
		suggestions = [];
		suggestError = '';
		results = [];
		searchError = '';
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			closeDropdown();
			return;
		}
		if (e.key !== 'Enter') return;
		// The picker sits inside the trip-completion form; Enter must search,
		// not submit it.
		e.preventDefault();
		if (canSearch) void runSearch();
	}

	async function runSearch() {
		const q = query.trim();
		if (q.length < 2) return;
		const seq = ++searchSeq;
		searching = true;
		searchError = '';
		try {
			const found = await geocode(q, mapBias());
			// Discard a response that a newer search has superseded.
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

	async function add(port: VoyagePortBody) {
		busy = true;
		searchError = '';
		try {
			await onAdd?.(anchorage ? { ...port, anchorage: true } : port);
			closeDropdown();
			clearPin();
			query = '';
			anchorage = false;
		} catch {
			searchError = 'Nie udało się dodać portu';
		} finally {
			busy = false;
		}
	}

	function pick(r: GeocodeResult) {
		return add({ name: r.name, latitude: r.latitude, longitude: r.longitude });
	}

	function pickSuggestion(s: PortSuggestion) {
		if (typeof s.latitude === 'number' && typeof s.longitude === 'number') {
			return add({ name: s.name, latitude: s.latitude, longitude: s.longitude });
		}
		return add({ name: s.name });
	}

	const photonUsed = $derived(results.some((r) => r.source === 'photon'));
	const dropdownOpen = $derived(
		searching || results.length > 0 || !!searchError || suggestions.length > 0 || !!suggestError
	);

	function addNameOnly() {
		const name = query.trim();
		if (!name) return;
		return add({ name });
	}

	async function remove(i: number) {
		busy = true;
		try {
			await onRemove?.(i);
		} catch {
			searchError = 'Nie udało się usunąć portu';
		} finally {
			busy = false;
		}
	}

	// Drag-and-drop reordering of the visited list.
	const reorderable = $derived(!readonly && !!onReorder);
	let dragIndex = $state<number | null>(null);
	let overIndex = $state<number | null>(null);

	function onDragStart(i: number, e: DragEvent) {
		dragIndex = i;
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = 'move';
			e.dataTransfer.setData('text/plain', String(i));
		}
	}

	function onDragOver(i: number, e: DragEvent) {
		if (dragIndex === null) return;
		// preventDefault marks the row as a valid drop target.
		e.preventDefault();
		overIndex = i;
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
	}

	function onDragEnd() {
		dragIndex = null;
		overIndex = null;
	}

	async function onDrop(i: number) {
		const from = dragIndex;
		dragIndex = null;
		overIndex = null;
		if (from === null || from === i) return;
		busy = true;
		try {
			await onReorder?.(from, i);
		} catch {
			searchError = 'Nie udało się zmienić kolejności portów';
		} finally {
			busy = false;
		}
	}

	// moveBy reorders via keyboard so the list is operable without a mouse.
	async function moveBy(i: number, delta: number) {
		const target = i + delta;
		if (target < 0 || target >= ports.length) return;
		busy = true;
		try {
			await onReorder?.(i, target);
		} catch {
			searchError = 'Nie udało się zmienić kolejności portów';
		} finally {
			busy = false;
		}
	}
</script>

<div class="space-y-3">
	{#if !readonly}
		<div class="relative">
			<label for="port-search" class="mb-1 block text-xs font-medium">Odwiedzone porty</label>
			<div class="mb-1 flex flex-wrap items-center gap-3 text-xs">
				<label class="flex items-center gap-1">
					<input type="checkbox" bind:checked={anchorage} disabled={busy} />
					⚓ Kotwicowisko
				</label>
				<button
					type="button"
					onclick={addNameOnly}
					disabled={busy || !query.trim()}
					class="text-[var(--ocean)] hover:underline disabled:opacity-40"
				>
					Dodaj bez lokalizacji na mapie
				</button>
			</div>
			<div class="flex gap-2">
				<input
					id="port-search"
					type="text"
					bind:value={query}
					oninput={onInput}
					onkeydown={onKeydown}
					disabled={busy}
					placeholder="Wpisz nazwę miasta / portu i naciśnij Enter..."
					class="w-full rounded-lg border px-3 py-1.5 text-sm"
					autocomplete="off"
				/>
				<button
					type="button"
					onclick={runSearch}
					disabled={!canSearch}
					class="rounded-lg border px-3 py-1.5 text-sm text-[var(--ocean)] hover:bg-gray-50 disabled:opacity-40"
				>
					Szukaj
				</button>
			</div>
			{#if dropdownOpen}
				<div
					class="absolute z-[1100] mt-1 max-h-96 w-full overflow-y-auto rounded-lg border bg-white shadow-lg"
				>
					{#if suggestions.length > 0}
						<div
							class="px-3 pt-2 text-xs font-semibold uppercase text-[var(--text-muted)]"
							id="port-suggestions-heading"
						>
							Wcześniej odwiedzone
						</div>
						<div role="group" aria-labelledby="port-suggestions-heading">
							{#each suggestions as sug, i (i)}
								<button
									type="button"
									onclick={() => pickSuggestion(sug)}
									disabled={busy}
									class="flex w-full items-baseline justify-between gap-2 px-3 py-2 text-left hover:bg-gray-50 disabled:opacity-50"
								>
									<span class="text-sm font-medium">{sug.name}</span>
									<span class="text-xs text-[var(--text-muted)]">
										{#if typeof sug.latitude !== 'number'}bez mapy ·
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
							id="port-results-heading"
						>
							Wyniki wyszukiwania
						</div>
						<div role="group" aria-labelledby="port-results-heading">
							{#each results as r, i (i)}
								<button
									type="button"
									onclick={() => pick(r)}
									disabled={busy}
									class="block w-full px-3 py-2 text-left hover:bg-gray-50 disabled:opacity-50"
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
			<p class="mt-1 text-xs text-[var(--text-muted)]">
				Nie ma miejsca na liście? Kliknij na mapie, aby wskazać punkt.
			</p>
		</div>
		{#if pin}
			<div
				class="space-y-2 rounded-lg border bg-gray-50 p-3 text-sm"
				role="group"
				aria-label="Punkt z mapy"
			>
				<p>
					Wybrany punkt: <span class="font-mono">{pin.lat.toFixed(5)}, {pin.lon.toFixed(5)}</span>
				</p>
				<div class="flex flex-wrap gap-2">
					<label for="port-pin-name" class="sr-only">Nazwa punktu z mapy</label>
					<input
						id="port-pin-name"
						type="text"
						bind:value={pinName}
						disabled={busy}
						placeholder="Nazwa portu"
						class="min-w-0 flex-1 rounded-lg border px-3 py-1.5 text-sm"
						autocomplete="off"
						onkeydown={(e) => {
							if (e.key !== 'Enter') return;
							e.preventDefault();
							void addPin();
						}}
					/>
					<button
						type="button"
						onclick={addPin}
						disabled={busy || !pinName.trim()}
						class="rounded-lg border px-3 py-1.5 text-sm text-[var(--ocean)] hover:bg-white disabled:opacity-40"
					>
						Dodaj punkt
					</button>
					<button
						type="button"
						onclick={clearPin}
						disabled={busy}
						class="px-2 py-1.5 text-sm text-[var(--text-muted)] hover:underline"
					>
						Anuluj
					</button>
				</div>
			</div>
		{/if}
	{/if}

	<div
		bind:this={mapEl}
		class="port-map h-64 w-full rounded-lg border"
		aria-label="Mapa portów"
	></div>

	{#if ports.length === 0}
		<p class="text-sm text-[var(--text-muted)]">Brak dodanych portów.</p>
	{:else}
		<ol class="space-y-1">
			{#each ports as port, i (port.name + i)}
				<li
					draggable={reorderable && !busy}
					ondragstart={(e) => onDragStart(i, e)}
					ondragover={(e) => onDragOver(i, e)}
					ondrop={() => onDrop(i)}
					ondragend={onDragEnd}
					class="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-1.5 text-sm
						{reorderable ? 'cursor-move' : ''}
						{dragIndex === i ? 'opacity-40' : ''}
						{overIndex === i && dragIndex !== i ? 'ring-2 ring-[var(--ocean)]' : ''}"
				>
					<span class="flex items-center gap-2">
						{#if reorderable}
							<span class="select-none leading-none text-[var(--text-muted)]" aria-hidden="true"
								>⠿</span
							>
						{/if}
						<span>
							<span class="text-[var(--text-muted)]">{i + 1}.</span>
							{#if port.anchorage}<span title="Kotwicowisko" aria-label="Kotwicowisko">⚓</span
								>{/if}
							{port.name}
							{#if !hasCoords(port)}<span class="text-xs text-[var(--text-muted)]">(bez mapy)</span
								>{/if}
						</span>
					</span>
					<span class="flex items-center gap-2">
						{#if reorderable}
							<button
								type="button"
								onclick={() => moveBy(i, -1)}
								disabled={busy || i === 0}
								aria-label="Przesuń port {port.name} w górę"
								class="text-[var(--text-muted)] hover:text-[var(--navy)] disabled:opacity-30"
							>
								↑
							</button>
							<button
								type="button"
								onclick={() => moveBy(i, 1)}
								disabled={busy || i === ports.length - 1}
								aria-label="Przesuń port {port.name} w dół"
								class="text-[var(--text-muted)] hover:text-[var(--navy)] disabled:opacity-30"
							>
								↓
							</button>
						{/if}
						{#if !readonly}
							<button
								type="button"
								onclick={() => remove(i)}
								disabled={busy}
								class="text-red-500 hover:underline disabled:opacity-50"
							>
								Usuń
							</button>
						{/if}
					</span>
				</li>
			{/each}
		</ol>
	{/if}
</div>

<style>
	/* Leaflet's zoom links are 30px; phones need a 44px tap target. */
	@media (width < 48rem) {
		.port-map :global(.leaflet-bar a) {
			width: 2.75rem;
			height: 2.75rem;
			line-height: 2.75rem;
		}
	}
</style>
