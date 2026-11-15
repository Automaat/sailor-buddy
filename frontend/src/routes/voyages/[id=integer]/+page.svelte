<script lang="ts">
	import { formatDateRangeWithDays } from '#lib/dates.js';
	import { deleteVoyage, publishVoyage, unpublishVoyage } from '#lib/api/routes.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import VoyageLog from './VoyageLog.svelte';
	import VoyageCrew from './VoyageCrew.svelte';
	import VoyageOpinions from './VoyageOpinions.svelte';
	import VoyagePorts from './VoyagePorts.svelte';
	import { goto } from '$app/navigation';
	import { errorMessage } from '#lib/api/errors.js';
	import { perLoad } from '#lib/state.svelte.js';
	import HiddenBadge from '#lib/components/HiddenBadge.svelte';
	import { canSeeHidden, isHidden, isLegHidden } from '#lib/publication.js';
	let { data } = $props();
	// The cruise changes in place when publishing the voyage publishes it too.
	let cruise = $derived(data.cruise);
	let voyage = $derived(data.voyage);
	let yachts = $derived(data.yachts);
	let crew = $derived(data.crew);
	const id = $derived(data.voyage.id);
	const canManageCrew = $derived(auth.isAdmin);
	const showHidden = $derived(canSeeHidden(auth));
	const isVoyageCaptain = $derived(
		!!auth.user?.person_id && voyage.captain_person_id === auth.user.person_id
	);
	// The voyage captain reads the crew to write its opinions; only admins
	// change the crew.
	const canSeeCrew = $derived(canManageCrew || isVoyageCaptain);
	const visibleLoadErrors = $derived([
		...data.loadErrors,
		...(canSeeCrew ? data.crewLoadErrors : []),
		...(canManageCrew ? data.adminLoadErrors : [])
	]);
	const canLog = $derived(auth.isAdmin || isVoyageCaptain);
	const yacht = $derived(yachts.find((y) => y.id === voyage.yacht_id) ?? null);
	let deletion = $derived(perLoad(data, { error: '' }));
	let publication = $derived(perLoad(data, { busy: false, error: '' }));
	async function handlePublish() {
		const held = publication;
		held.busy = true;
		held.error = '';
		try {
			const next = await publishVoyage(id);
			if (held !== publication) return;
			voyage = next;
			// The API publishes a hidden cruise without a series along with its leg.
			if (cruise && isHidden(cruise) && cruise.series_id == null) {
				cruise = { ...cruise, published_at: next.published_at };
			}
		} catch (err) {
			held.error = `Nie udało się opublikować rejsu: ${errorMessage(err)}`;
		} finally {
			held.busy = false;
		}
	}
	async function handleUnpublish() {
		const held = publication;
		held.busy = true;
		held.error = '';
		try {
			const next = await unpublishVoyage(id);
			if (held !== publication) return;
			voyage = next;
		} catch (err) {
			held.error = `Nie udało się ukryć rejsu: ${errorMessage(err)}`;
		} finally {
			held.busy = false;
		}
	}
	async function handleDelete() {
		if (!confirm('Usunąć ten rejs?')) return;
		const held = deletion;
		const voyageId = id;
		const cruiseId = voyage.cruise_id;
		held.error = '';
		try {
			await deleteVoyage(voyageId);
		} catch (err) {
			console.error('Failed to delete voyage:', err);
			held.error = `Nie udało się usunąć rejsu: ${errorMessage(err)}`;
			return;
		}
		if (held !== deletion) return;
		void goto(`/cruises/${cruiseId}`);
	}
</script>

<div class="mx-auto max-w-4xl">
	<LoadErrorBanner errors={visibleLoadErrors} />
	<div class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3">
		<div>
			{#if cruise}
				<a href="/cruises/{cruise.id}" class="text-xs text-[var(--ocean)] hover:underline">
					← Część wydarzenia: {cruise.name}
				</a>
			{/if}
			<div class="flex flex-wrap items-center gap-3">
				<h1 class="text-3xl font-bold text-[var(--navy)]">{voyage.name}</h1>
				{#if showHidden && isLegHidden(voyage, cruise)}
					<HiddenBadge
						reason={isHidden(voyage) ? undefined : 'Ukryte, bo wydarzenie jest ukryte'}
					/>
				{/if}
			</div>
			<p class="mt-1 text-[var(--text-muted)]">
				{#if voyage.start_port && voyage.end_port}
					{voyage.start_port} → {voyage.end_port}
				{/if}
				{#if voyage.countries}
					· {voyage.countries}{/if}
				{#if voyage.year}
					· {voyage.year}{/if}
			</p>
		</div>
		{#if canManageCrew}
			<div class="flex gap-2 max-md:flex-wrap">
				{#if isHidden(voyage)}
					<button
						onclick={handlePublish}
						disabled={publication.busy}
						class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
					>
						Opublikuj
					</button>
				{:else}
					<button
						onclick={handleUnpublish}
						disabled={publication.busy}
						class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50 disabled:opacity-50"
					>
						Ukryj
					</button>
				{/if}
				<a
					href="/voyages/{id}/edit"
					class="tap-target rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">Edytuj</a
				>
				<button
					onclick={handleDelete}
					class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50"
				>
					Usuń
				</button>
			</div>
		{/if}
	</div>

	{#if publication.error}
		<div role="alert" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">
			{publication.error}
		</div>
	{/if}
	{#if deletion.error}
		<div role="alert" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">
			{deletion.error}
		</div>
	{/if}

	<div class="mb-6 grid grid-cols-2 gap-4 md:grid-cols-4">
		<div class="rounded-xl bg-white p-4 shadow-sm">
			<div class="text-xs text-[var(--text-muted)]">Godziny łącznie</div>
			<div class="text-2xl font-bold text-[var(--ocean)]">{voyage.hours_total}</div>
			<div class="text-xs text-[var(--text-muted)]">
				{voyage.hours_sail}h żagle / {voyage.hours_engine}h silnik
			</div>
		</div>
		<div class="rounded-xl bg-white p-4 shadow-sm">
			<div class="text-xs text-[var(--text-muted)]">Mile</div>
			<div class="text-2xl font-bold text-[var(--sand)]">{voyage.miles}</div>
		</div>
		<div class="rounded-xl bg-white p-4 shadow-sm">
			<div class="text-xs text-[var(--text-muted)]">Dni</div>
			<div class="text-2xl font-bold text-[var(--navy)]">{voyage.days}</div>
		</div>
		<div class="rounded-xl bg-white p-4 shadow-sm">
			<div class="text-xs text-[var(--text-muted)]">Godziny &gt;6Bf</div>
			<div class="text-2xl font-bold">{voyage.hours_over_6bf}</div>
		</div>
	</div>

	{#if voyage.embark_date || voyage.captain_name || yacht || voyage.track_url || voyage.voyage_card || voyage.tidal_waters || voyage.cost_total}
		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Szczegóły</h2>
			<dl class="grid grid-cols-1 text-sm sm:grid-cols-2 sm:gap-2 max-sm:[&>dd]:mb-2">
				{#if voyage.embark_date}
					<dt class="text-[var(--text-muted)]">Daty</dt>
					<dd>{formatDateRangeWithDays(voyage.embark_date, voyage.disembark_date)}</dd>
				{/if}
				{#if voyage.captain_name}
					<dt class="text-[var(--text-muted)]">Kapitan</dt>
					<dd>{voyage.captain_name}</dd>
				{/if}
				{#if yacht}
					<dt class="text-[var(--text-muted)]">Jacht</dt>
					<dd>{yacht.yacht_type ? `${yacht.yacht_type} ` : ''}{yacht.name}</dd>
				{/if}
				{#if voyage.track_url}
					<dt class="text-[var(--text-muted)]">Ślad rejsu</dt>
					<dd>
						<a
							href={voyage.track_url}
							target="_blank"
							rel="noopener noreferrer"
							class="break-all text-[var(--ocean)] hover:underline"
						>
							{voyage.track_url}
						</a>
					</dd>
				{/if}
				{#if voyage.voyage_card}
					<dt class="text-[var(--text-muted)]">Karta rejsu</dt>
					<dd>{voyage.voyage_card}</dd>
				{/if}
				{#if voyage.tidal_waters}
					<dt class="text-[var(--text-muted)]">Wody pływowe</dt>
					<dd>Tak</dd>
				{/if}
				{#if voyage.cost_total}
					<dt class="text-[var(--text-muted)]">Koszt</dt>
					<dd>{voyage.cost_total} ({voyage.cost_per_person ?? '?'} /os)</dd>
				{/if}
			</dl>
		</div>
	{/if}

	{#key data}
		<VoyageLog bind:voyage bind:yachts {canLog} />
	{/key}
	{#if voyage.description}
		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Opis</h2>
			<p class="whitespace-pre-wrap text-sm">{voyage.description}</p>
		</div>
	{/if}

	{#key data}
		<VoyagePorts {id} initialPorts={data.ports} {canLog} />
		{#if canSeeCrew}
			<VoyageCrew {id} bind:crew allCrewMembers={data.allCrewMembers} readonly={!canManageCrew} />
			<VoyageOpinions {id} {crew} initialOpinions={data.opinions} />
		{/if}
	{/key}
</div>
