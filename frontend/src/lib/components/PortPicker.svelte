<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { Map as LeafletMap, LayerGroup } from 'leaflet';
	import 'leaflet/dist/leaflet.css';
	import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
	import markerIcon from 'leaflet/dist/images/marker-icon.png';
	import markerShadow from 'leaflet/dist/images/marker-shadow.png';
	import { geocode } from '$lib/api/routes';
	import type { GeocodeResult, VoyagePortBody } from '$lib/api/types';

	// A port only needs a name and coordinates to be drawn; both VoyagePort and
	// VoyagePortBody satisfy this, so the picker works for the completion modal
	// (unsaved bodies) and the voyage page (persisted ports) alike.
	type PortLike = { name: string; latitude: number; longitude: number };

	type Props = {
		ports: PortLike[];
		readonly?: boolean;
		onAdd?: (port: VoyagePortBody) => Promise<void> | void;
		onRemove?: (index: number) => Promise<void> | void;
		// onReorder moves the port at `from` to `to`; drag-and-drop is only
		// offered when a parent supplies it.
		onReorder?: (from: number, to: number) => Promise<void> | void;
		// debounceMs is overridable so tests can search without waiting.
		debounceMs?: number;
	};

	let {
		ports,
		readonly = false,
		onAdd,
		onRemove,
		onReorder,
		debounceMs = 400
	}: Props = $props();

	let query = $state('');
	let results = $state<GeocodeResult[]>([]);
	let searching = $state(false);
	let searchError = $state('');
	let busy = $state(false);
	let debounce: ReturnType<typeof setTimeout> | undefined;
	// Monotonic id of the latest search; stale responses are discarded.
	let searchSeq = 0;

	let mapEl: HTMLDivElement;
	// Leaflet touches `window`, so it is imported only in the browser (onMount).
	let L: typeof import('leaflet') | undefined;
	let map: LeafletMap | undefined;
	let layer: LayerGroup | undefined;
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
		renderMarkers();
	});

	onDestroy(() => {
		destroyed = true;
		clearTimeout(debounce);
		map?.remove();
	});

	// renderMarkers redraws every port marker plus the route polyline whenever
	// the ports list changes. $effect re-runs it; the map-ready guard skips the
	// pre-mount run.
	function renderMarkers() {
		if (!L || !map || !layer) return;
		layer.clearLayers();
		const points: [number, number][] = [];
		ports.forEach((p, i) => {
			const point: [number, number] = [p.latitude, p.longitude];
			points.push(point);
			L!.marker(point)
				.bindTooltip(`${i + 1}. ${p.name}`, { permanent: true, direction: 'top' })
				.addTo(layer!);
		});
		if (points.length > 1) {
			L.polyline(points, { color: '#0369a1', weight: 2, dashArray: '4 6' }).addTo(layer);
		}
		if (points.length > 0) {
			map.fitBounds(L.latLngBounds(points), { padding: [40, 40], maxZoom: 9 });
		}
	}

	$effect(() => {
		// Track the full ports content so edits/reorders (not just length
		// changes) trigger a redraw.
		ports.map((p) => `${p.name}:${p.latitude}:${p.longitude}`).join('|');
		renderMarkers();
	});

	function onInput() {
		clearTimeout(debounce);
		searchError = '';
		const q = query.trim();
		if (q.length < 2) {
			results = [];
			return;
		}
		// Debounced so typing does not hammer the Nominatim proxy (1 req/s policy).
		debounce = setTimeout(runSearch, debounceMs);
	}

	async function runSearch() {
		const q = query.trim();
		if (q.length < 2) return;
		const seq = ++searchSeq;
		searching = true;
		searchError = '';
		try {
			const found = await geocode(q);
			// Discard a response that a newer search has superseded.
			if (seq !== searchSeq || destroyed) return;
			results = found;
			if (found.length === 0) searchError = 'Brak wyników';
		} catch {
			if (seq !== searchSeq || destroyed) return;
			searchError = 'Nie udało się wyszukać miejsca';
			results = [];
		} finally {
			if (seq === searchSeq) searching = false;
		}
	}

	async function pick(r: GeocodeResult) {
		busy = true;
		searchError = '';
		try {
			await onAdd?.({ name: r.name, latitude: r.latitude, longitude: r.longitude });
			query = '';
			results = [];
		} catch {
			searchError = 'Nie udało się dodać portu';
		} finally {
			busy = false;
		}
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
			<input
				id="port-search"
				type="text"
				bind:value={query}
				oninput={onInput}
				disabled={busy}
				placeholder="Wpisz nazwę miasta / portu..."
				class="w-full rounded-lg border px-3 py-1.5 text-sm"
				autocomplete="off"
			/>
			{#if searching || results.length > 0 || searchError}
				<div
					class="absolute z-[1100] mt-1 w-full overflow-hidden rounded-lg border bg-white shadow-lg"
				>
					{#if searching}
						<div class="px-3 py-2 text-sm text-[var(--text-muted)]">Szukanie...</div>
					{:else if searchError}
						<div class="px-3 py-2 text-sm text-[var(--text-muted)]">{searchError}</div>
					{:else}
						{#each results as r}
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
					{/if}
				</div>
			{/if}
		</div>
	{/if}

	<div bind:this={mapEl} class="h-64 w-full rounded-lg border" aria-label="Mapa portów"></div>

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
						<span><span class="text-[var(--text-muted)]">{i + 1}.</span> {port.name}</span>
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
