<script lang="ts">
	import { formatDateRangeWithDays } from '#lib/dates.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import { listCruises, listMyCruiseIds, listTrips, listVoyages } from '#lib/api/routes.js';
	import { cruiseKinds, cruiseKindLabels, type CruiseKind } from '#lib/cruise.js';
	import {
		buildRejsEntries,
		filterRejsEntries,
		isMineFilter,
		onlyMine,
		parseRejsTab,
		rejsListUrl,
		rejsStatusLabels,
		rejsTabs,
		type RejsEntry,
		type RejsTab
	} from '#lib/rejsy.js';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import Sailboat from '@lucide/svelte/icons/sailboat';

	let entries = $state<RejsEntry[]>([]);
	let loading = $state(true);
	/** null when the user's cruises could not be loaded. */
	let myCruiseIds = $state<ReadonlySet<number> | null>(new Set());
	let tab = $state<RejsTab>(parseRejsTab(page.url.searchParams.get('tab')));
	let mine = $state(isMineFilter(page.url.searchParams.get('mine')));
	$effect(() => {
		tab = parseRejsTab(page.url.searchParams.get('tab'));
		mine = isMineFilter(page.url.searchParams.get('mine'));
	});
	let kindFilter = $state<CruiseKind | 'all'>('all');

	const mineUnavailable = $derived(mine && myCruiseIds === null);
	const inTab = $derived(
		filterRejsEntries(mine && myCruiseIds ? onlyMine(entries, myCruiseIds) : entries, tab)
	);
	const visible = $derived(
		kindFilter === 'all' ? inTab : inTab.filter((e) => e.kind === kindFilter)
	);
	const kindFilters: { value: CruiseKind | 'all'; label: string }[] = [
		{ value: 'all', label: 'Wszystkie' },
		...cruiseKinds.map((k) => ({ value: k, label: cruiseKindLabels[k] }))
	];
	const emptyLabels: Record<RejsTab, string> = {
		upcoming: 'Brak nadchodzących rejsów',
		completed: 'Brak zrealizowanych rejsów',
		all: 'Brak rejsów'
	};
	const mineEmptyLabels: Record<RejsTab, string> = {
		upcoming: 'Nie masz nadchodzących rejsów',
		completed: 'Nie masz jeszcze zrealizowanych rejsów',
		all: 'Nie masz jeszcze żadnych rejsów'
	};

	function navigate() {
		void goto(rejsListUrl(tab, mine), { replace: true, shallow: true });
	}

	function selectTab(next: RejsTab) {
		tab = next;
		navigate();
	}

	function toggleMine() {
		mine = !mine;
		navigate();
	}

	function statusBadge(status: RejsEntry['status']) {
		if (status === 'completed') return 'bg-[var(--navy)]/10 text-[var(--navy)]';
		if (status === 'cancelled') return 'bg-gray-100 text-gray-500';
		if (status === 'overdue') return 'bg-[var(--navy)] text-white';
		return 'bg-[var(--ocean-tint)] text-[var(--ocean)]';
	}

	async function load() {
		loading = true;
		try {
			const [cruises, trips, voyages, mineIds] = await Promise.all([
				listCruises(),
				listTrips(),
				listVoyages(),
				listMyCruiseIds().catch((err) => {
					console.error('Failed to load your cruises:', err);
					return null;
				})
			]);
			entries = buildRejsEntries(cruises ?? [], trips ?? [], voyages ?? []);
			myCruiseIds = mineIds ? new Set(mineIds) : null;
		} catch (err) {
			console.error('Failed to load cruises:', err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		void load();
	});
</script>

<div>
	<div class="mb-6 flex flex-wrap items-center justify-between gap-3">
		<div>
			<h1 class="text-3xl font-bold text-[var(--navy)]">Rejsy</h1>
			<p class="mt-1 text-sm text-[var(--text-muted)]">
				Wydarzenia klubu: planowane i zrealizowane
			</p>
		</div>
		{#if auth.isAdmin}
			<a
				href="/cruises/new"
				class="tap-target rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--ocean-dark)]"
			>
				+ Nowe wydarzenie
			</a>
		{/if}
	</div>

	<div class="mb-3 flex gap-1 border-b" role="tablist" aria-label="Stan rejsu">
		{#each rejsTabs as t (t.value)}
			<button
				type="button"
				role="tab"
				aria-selected={tab === t.value}
				onclick={() => selectTab(t.value)}
				class="-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors {tab === t.value
					? 'border-[var(--ocean)] text-[var(--ocean)]'
					: 'border-transparent text-[var(--text-muted)] hover:text-[var(--navy)]'}"
			>
				{t.label}
			</button>
		{/each}
	</div>

	<div class="mb-4 flex flex-wrap items-center gap-2">
		<button
			type="button"
			aria-pressed={mine}
			onclick={toggleMine}
			class="rounded-full border px-3 py-1 text-sm font-medium transition-colors {mine
				? 'border-[var(--navy)] bg-[var(--navy)] text-white'
				: 'bg-white text-[var(--navy)] hover:bg-gray-50'}"
		>
			Moje rejsy
		</button>
		<span class="h-5 w-px bg-gray-200" aria-hidden="true"></span>
		<div class="flex flex-wrap gap-2" role="group" aria-label="Rodzaj rejsu">
			{#each kindFilters as f (f.value)}
				<button
					type="button"
					aria-pressed={kindFilter === f.value}
					onclick={() => (kindFilter = f.value)}
					class="rounded-full border px-3 py-1 text-sm transition-colors {kindFilter === f.value
						? 'border-[var(--ocean)] bg-[var(--ocean)] text-white'
						: 'bg-white text-[var(--text-muted)] hover:bg-gray-50'}"
				>
					{f.label}
				</button>
			{/each}
		</div>
	</div>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if mineUnavailable}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm" role="alert">
			<p class="text-lg text-red-600">Nie udało się wczytać Twoich rejsów</p>
			<button type="button" onclick={load} class="mt-2 text-[var(--ocean)] hover:underline">
				Spróbuj ponownie
			</button>
		</div>
	{:else if inTab.length > 0 && visible.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-lg text-[var(--text-muted)]">Brak rejsów tego rodzaju</p>
		</div>
	{:else if visible.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Sailboat class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">
				{mine ? mineEmptyLabels[tab] : emptyLabels[tab]}
			</p>
			{#if auth.isAdmin && entries.length === 0}
				<a href="/cruises/new" class="mt-2 inline-block text-[var(--ocean)] hover:underline">
					Utwórz pierwsze wydarzenie
				</a>
			{/if}
		</div>
	{:else}
		<div class="grid gap-4">
			{#each visible as entry (entry.key)}
				<a
					href={entry.href}
					class="flex items-center justify-between rounded-2xl bg-white p-6 shadow-sm transition-shadow hover:shadow-md"
				>
					<div>
						<div class="flex flex-wrap items-center gap-2">
							<h3 class="font-semibold text-[var(--navy)]">{entry.name}</h3>
							<span
								class="rounded-full px-2 py-0.5 text-xs font-medium {statusBadge(entry.status)}"
							>
								{rejsStatusLabels[entry.status]}
							</span>
							{#if entry.kind}
								<span
									class="rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]"
								>
									{cruiseKindLabels[entry.kind]}
								</span>
							{/if}
						</div>
						<div class="mt-1 text-sm text-[var(--text-muted)]">
							{#if entry.startPort && entry.endPort}
								{entry.startPort} → {entry.endPort}
							{/if}
							{#if entry.countries}
								· {entry.countries}{/if}
						</div>
						<div class="mt-1 text-xs text-[var(--text-muted)]">
							{#if entry.embarkDate}
								{formatDateRangeWithDays(entry.embarkDate, entry.disembarkDate)}
							{/if}
							{#if entry.plannedLegs + entry.completedLegs > 0}
								· etapy: {entry.completedLegs}/{entry.plannedLegs + entry.completedLegs} zrealizowane
							{/if}
						</div>
					</div>
					<div class="text-right text-sm text-[var(--text-muted)]">
						{#if entry.completedLegs > 0}
							<div class="flex gap-4">
								<div>
									<div class="text-lg font-bold text-[var(--ocean)]">
										{Math.round(entry.hoursTotal)}h
									</div>
									<div class="text-xs">godziny</div>
								</div>
								<div>
									<div class="text-lg font-bold text-[var(--sand)]">{Math.round(entry.miles)}</div>
									<div class="text-xs">Mm</div>
								</div>
							</div>
						{/if}
						{#if entry.maxCrew && entry.status === 'upcoming'}<div>
								max {entry.maxCrew} os.
							</div>{/if}
						{#if entry.enrollOpen && entry.status === 'upcoming'}
							<div
								class="mt-1 inline-block rounded-full bg-[var(--ocean)] px-2 py-0.5 text-xs text-white"
							>
								Zapisy otwarte
							</div>
						{/if}
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
