<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { Map as LeafletMap, LayerGroup, LeafletMouseEvent } from 'leaflet';
	import 'leaflet/dist/leaflet.css';
	import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
	import markerIcon from 'leaflet/dist/images/marker-icon.png';
	import markerShadow from 'leaflet/dist/images/marker-shadow.png';
	import type { GeocodeBias } from '#lib/api/routes.js';
	import type { VoyagePortBody } from '#lib/api/types.js';
	import PortField from '#lib/location/PortField.svelte';
	import { hasCoordinates, normalizePortName, type PortDraft } from '#lib/location/port.js';

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

	let draft = $state<PortDraft>({ name: '' });
	let anchorage = $state(false);
	let error = $state('');
	let busy = $state(false);

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
		map?.remove();
	});

	function onMapClick(e: LeafletMouseEvent) {
		if (readonly || busy || !map) return;
		const at = map.wrapLatLng(e.latlng);
		pin = { lat: roundCoord(at.lat), lon: roundCoord(at.lng) };
		pinName = draft.name.trim();
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
			if (!hasCoordinates(p)) return;
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

	$effect(() => {
		// Track the full ports content so edits/reorders (not just length
		// changes) trigger a redraw.
		ports.map((p) => `${p.name}:${p.latitude}:${p.longitude}:${p.anchorage}`).join('|');
		renderMarkers();
	});

	async function add(port: VoyagePortBody) {
		busy = true;
		error = '';
		try {
			await onAdd?.(anchorage ? { ...port, anchorage: true } : port);
			clearPin();
			draft = { name: '' };
			anchorage = false;
		} catch {
			error = 'Nie udało się dodać portu';
		} finally {
			busy = false;
		}
	}

	function addPicked(port: PortDraft) {
		const name = normalizePortName(port.name);
		return add(
			hasCoordinates(port) ? { name, latitude: port.latitude, longitude: port.longitude } : { name }
		);
	}

	function addNameOnly() {
		const name = normalizePortName(draft.name);
		if (!name) return;
		return add({ name });
	}

	async function remove(i: number) {
		busy = true;
		try {
			await onRemove?.(i);
		} catch {
			error = 'Nie udało się usunąć portu';
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
			error = 'Nie udało się zmienić kolejności portów';
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
			error = 'Nie udało się zmienić kolejności portów';
		} finally {
			busy = false;
		}
	}
</script>

<div class="space-y-3">
	{#if !readonly}
		<div>
			<PortField
				id="port-search"
				label="Odwiedzone porty"
				bind:value={draft}
				disabled={busy}
				placeholder="Wpisz nazwę miasta / portu i naciśnij Enter..."
				labelClass="mb-1 block text-xs font-medium"
				inputClass="w-full rounded-lg border px-3 py-1.5 text-sm"
				showLocation={false}
				bias={mapBias}
				onpick={addPicked}
			>
				{#snippet controls()}
					<div class="mb-1 flex flex-wrap items-center gap-3 text-xs">
						<label class="flex items-center gap-1">
							<input type="checkbox" bind:checked={anchorage} disabled={busy} />
							⚓ Kotwicowisko
						</label>
						<button
							type="button"
							onclick={addNameOnly}
							disabled={busy || !draft.name.trim()}
							class="text-[var(--ocean)] hover:underline disabled:opacity-40"
						>
							Dodaj bez lokalizacji na mapie
						</button>
					</div>
				{/snippet}
			</PortField>
			{#if error}
				<p class="mt-1 text-xs text-red-600" role="alert">{error}</p>
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
							{#if !hasCoordinates(port)}<span class="text-xs text-[var(--text-muted)]"
									>(bez mapy)</span
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
