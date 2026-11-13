<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import {
		getTrip,
		deleteTrip,
		listTripCrew,
		assignTripCrew,
		removeTripCrew,
		cancelTrip,
		completeTrip,
		listAssignableCrew,
		getCruise,
		generateTripEnrollToken,
		clearTripEnrollToken,
		getTripSignUp
	} from '#lib/api/routes.js';
	import type {
		Trip,
		CrewAssignment,
		CrewMember,
		Cruise,
		CompleteTripPayload,
		LegSignUp
	} from '#lib/api/types.js';
	import { ApiError } from '#lib/api/client.js';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import CompleteTripModal from '#lib/components/CompleteTripModal.svelte';
	import ActionsMenu from '#lib/components/ActionsMenu.svelte';
	import NotFound from '#lib/components/NotFound.svelte';
	import PriceInfo from '#lib/components/PriceInfo.svelte';
	import { formatPrice, hasPriceInfo } from '#lib/pricing.js';
	import LegSignUpControl from '#lib/components/LegSignUpControl.svelte';
	import { freePlacesLabel } from '#lib/enrollment.js';
	import { legStatus, legStatusBadges, legStatusLabels } from '#lib/rejsy.js';

	let trip = $state<Trip | null>(null);
	let cruise = $state<Cruise | null>(null);
	let crew = $state<CrewAssignment[]>([]);
	let allCrewMembers = $state<CrewMember[]>([]);
	let loading = $state(true);
	let notFound = $state(false);

	let assignCrewId = $state('');
	let assignRole = $state('');
	let assigning = $state(false);

	let enrollToken = $state<string | null>(null);
	let signUp = $state<LegSignUp | null>(null);
	const places = $derived(trip && signUp ? freePlacesLabel(trip.max_crew, signUp.accepted_count) : null);
	let togglingEnroll = $state(false);

	let completing = $state(false);
	let showCompleteModal = $state(false);

	const id = $derived(Number(page.params.id));
	// Trips are club-managed: only admins create, edit and crew them.
	const canManageCrew = $derived(auth.isAdmin);

	onMount(async () => {
		try {
			trip = await getTrip(id);
			enrollToken = trip?.enroll_token ?? null;
			[crew, allCrewMembers, signUp] = await Promise.all([
				listTripCrew(id).then((c) => c ?? []).catch(() => []),
				listAssignableCrew().catch(() => []),
				getTripSignUp(id).catch(() => null)
			]);
			if (trip?.cruise_id) {
				cruise = await getCruise(trip.cruise_id).catch(() => null);
			}
		} catch (err) {
			if (err instanceof ApiError && err.status === 404) {
				notFound = true;
			} else {
				console.error('Failed to load trip:', err);
			}
		} finally {
			loading = false;
		}
	});

	async function handleDelete() {
		if (!confirm('Usunąć ten rejs?')) return;
		await deleteTrip(id);
		goto(trip ? `/cruises/${trip.cruise_id}` : '/cruises');
	}

	async function toggleEnrollment() {
		togglingEnroll = true;
		try {
			if (enrollToken) {
				await clearTripEnrollToken(id);
				enrollToken = null;
			} else {
				const res = await generateTripEnrollToken(id);
				enrollToken = res.token;
			}
			if (signUp) signUp = { ...signUp, open: enrollToken !== null && trip?.status === 'planned' };
		} catch (err) {
			console.error('Failed to toggle enrollment:', err);
		} finally {
			togglingEnroll = false;
		}
	}

	function copyEnrollLink() {
		if (!enrollToken) return;
		navigator.clipboard.writeText(`${window.location.origin}/enroll/${enrollToken}`);
	}

	async function assignCrew(e: Event) {
		e.preventDefault();
		if (!assignCrewId || !assignRole) return;
		assigning = true;
		try {
			await assignTripCrew(id, {
				crew_member_id: Number(assignCrewId),
				role: assignRole
			});
			crew = (await listTripCrew(id)) ?? [];
			assignCrewId = '';
			assignRole = '';
		} catch (err) {
			console.error('Failed to assign crew:', err);
		} finally {
			assigning = false;
		}
	}

	async function removeCrew(assignmentId: number) {
		if (!confirm('Usunąć przypisanie załoganta?')) return;
		await removeTripCrew(id, assignmentId);
		crew = crew.filter((c) => c.id !== assignmentId);
	}

	async function handleCancel() {
		if (!confirm('Odwołać ten etap?')) return;
		completing = true;
		try {
			trip = await cancelTrip(id);
			enrollToken = trip?.enroll_token ?? null;
			signUp = await getTripSignUp(id).catch(() => signUp);
		} catch (err) {
			console.error('Failed to cancel trip:', err);
		} finally {
			completing = false;
		}
	}

	async function handleComplete(payload: CompleteTripPayload) {
		const voyage = await completeTrip(id, payload);
		showCompleteModal = false;
		goto(`/voyages/${voyage.id}`);
	}

	const status = $derived(trip ? legStatus(trip, cruise) : 'planned');

</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if trip}
	<div class="mx-auto max-w-4xl">
		<div class="mb-6 flex items-center justify-between">
			<div>
				{#if cruise}
					<a href="/cruises/{cruise.id}" class="text-xs text-[var(--ocean)] hover:underline">
						← Część wydarzenia: {cruise.name}
					</a>
				{/if}
				<div class="flex items-center gap-3">
					<h1 class="text-3xl font-bold text-[var(--navy)]">{trip.name}</h1>
					<span class="rounded-full px-2.5 py-1 text-xs font-medium {legStatusBadges[status]}">
						{legStatusLabels[status]}
					</span>
				</div>
				<p class="mt-1 text-[var(--text-muted)]">
					{#if trip.start_port && trip.end_port}
						{trip.start_port} → {trip.end_port}
					{/if}
					{#if trip.countries} · {trip.countries}{/if}
				</p>
			</div>
			{#if canManageCrew}
				<div class="flex gap-2">
					{#if trip.status === 'planned'}
						<button
							onclick={() => (showCompleteModal = true)}
							disabled={completing}
							class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
						>
							Zrealizuj
						</button>
					{/if}
					<a href="/trips/{id}/edit" class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">Edytuj</a>
					<ActionsMenu>
						{#snippet children(close)}
							{#if trip?.status === 'planned'}
								<button
									type="button"
									role="menuitem"
									disabled={completing}
									onclick={() => {
										close();
										handleCancel();
									}}
									class="block w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-50"
								>
									Odwołaj etap
								</button>
							{/if}
							<button
								type="button"
								role="menuitem"
								onclick={() => {
									close();
									handleDelete();
								}}
								class="block w-full px-4 py-2 text-left text-sm text-red-600 hover:bg-red-50"
							>
								Usuń
							</button>
						{/snippet}
					</ActionsMenu>
				</div>
			{/if}
		</div>

		{#if canManageCrew && status === 'overdue'}
			<div role="status" class="mb-6 flex items-center justify-between gap-4 rounded-2xl border border-[var(--ocean)]/30 bg-[var(--ocean-tint)] p-4 text-sm text-[var(--navy)]">
				<p>Rejs się zakończył, ale nadal czeka na rozliczenie. Zrealizuj go, aby wpisać przebieg do dziennika.</p>
				<button
					onclick={() => (showCompleteModal = true)}
					disabled={completing}
					class="shrink-0 rounded-lg bg-[var(--ocean)] px-4 py-2 text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
				>
					Zrealizuj teraz
				</button>
			</div>
		{/if}

		{#if trip.embark_date || trip.captain_name || trip.cost_total}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Szczegóły</h2>
				<dl class="grid grid-cols-2 gap-2 text-sm">
					{#if trip.embark_date}
						<dt class="text-[var(--text-muted)]">Daty</dt>
						<dd>{trip.embark_date} – {trip.disembark_date ?? '?'}</dd>
					{/if}
					{#if trip.captain_name}
						<dt class="text-[var(--text-muted)]">Kapitan</dt>
						<dd>{trip.captain_name}</dd>
					{/if}
					{#if trip.max_crew}
						<dt class="text-[var(--text-muted)]">Maks. załoga</dt>
						<dd>{trip.max_crew}</dd>
					{/if}
					{#if trip.cost_total}
						<dt class="text-[var(--text-muted)]">Koszt całkowity</dt>
						<dd>{formatPrice(trip.cost_total, trip.currency)}</dd>
					{/if}
				</dl>
			</div>
		{/if}

		{#if hasPriceInfo(trip)}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Cena</h2>
				<PriceInfo priced={trip} membership={auth.user?.membership} />
			</div>
		{/if}

		{#if trip.description}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Opis</h2>
				<p class="whitespace-pre-wrap text-sm">{trip.description}</p>
			</div>
		{/if}

		{#if signUp}
			<div class="mb-6 flex flex-wrap items-start justify-between gap-3 rounded-2xl bg-white p-6 shadow-sm" data-testid="my-sign-up">
				<div>
					<h2 class="font-semibold text-[var(--navy)]">Twój zapis</h2>
					{#if places && trip.status !== 'cancelled'}
						<p class="mt-1 text-sm text-[var(--text-muted)]" data-testid="free-places">{places}</p>
					{/if}
				</div>
				<LegSignUpControl
					{signUp}
					legName={trip.name}
					cancelled={trip.status === 'cancelled'}
					onchange={(next) => (signUp = next)}
				/>
			</div>
		{/if}

		{#if canManageCrew}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Zapisy</h2>
				<div class="flex flex-wrap items-center gap-3">
					<button
						onclick={toggleEnrollment}
						disabled={togglingEnroll || trip.status !== 'planned'}
						class="rounded-lg px-4 py-2 text-sm font-medium {enrollToken
							? 'border border-red-200 text-red-600 hover:bg-red-50'
							: 'bg-[var(--ocean)] text-white hover:bg-[var(--ocean-dark)]'} disabled:opacity-50"
					>
						{enrollToken ? 'Wyłącz zapisy' : 'Włącz zapisy'}
					</button>
					{#if enrollToken}
						<button onclick={copyEnrollLink} class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">
							Kopiuj link
						</button>
					{/if}
					<a href="/trips/{id}/enrollments" class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">
						Zarządzaj zapisami
					</a>
				</div>
			</div>
		{/if}

		{#if canManageCrew}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<div class="mb-3 flex items-center justify-between">
					<h2 class="font-semibold text-[var(--navy)]">Załoga ({crew.length})</h2>
				</div>

				{#if allCrewMembers.length > 0 && trip.status === 'planned'}
					<form onsubmit={assignCrew} class="mb-4 flex flex-wrap items-end gap-2">
						<div>
							<label for="assign-crew" class="block text-xs text-[var(--text-muted)]">Załogant</label>
							<select id="assign-crew" bind:value={assignCrewId} class="rounded-lg border px-3 py-1.5 text-sm">
								<option value="">Wybierz...</option>
								{#each allCrewMembers as member}
									<option value={member.id}>{member.full_name}</option>
								{/each}
							</select>
						</div>
						<div>
							<label for="assign-role" class="block text-xs text-[var(--text-muted)]">Rola</label>
							<input id="assign-role" type="text" bind:value={assignRole} placeholder="np. Sternik" class="rounded-lg border px-3 py-1.5 text-sm" />
						</div>
						<button
							type="submit"
							disabled={!assignCrewId || !assignRole || assigning}
							class="rounded-lg bg-[var(--ocean)] px-4 py-1.5 text-sm text-white hover:bg-[var(--ocean)]/90 disabled:opacity-50"
						>
							{assigning ? 'Dodawanie...' : 'Dodaj'}
						</button>
					</form>
				{/if}

				{#if crew.length === 0}
					<p class="text-sm text-[var(--text-muted)]">Brak przypisanej załogi.</p>
				{:else}
					<div class="space-y-2">
						{#each crew as member}
							<div class="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2">
								<div class="flex items-center gap-2">
									<span class="font-medium">{member.full_name}</span>
									<span class="rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]">
										{member.role}
									</span>
								</div>
								{#if trip.status === 'planned'}
									<button onclick={() => removeCrew(member.id)} class="text-sm text-red-500 hover:underline">
										Usuń
									</button>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</div>
		{/if}
	</div>

	{#if showCompleteModal}
		<CompleteTripModal
			tripName={trip.name}
			embarkDate={trip.embark_date}
			disembarkDate={trip.disembark_date}
			onClose={() => (showCompleteModal = false)}
			onSubmit={handleComplete}
		/>
	{/if}
{:else if notFound}
	<NotFound
		title="Nie znaleziono rejsu"
		message="Ten rejs nie istnieje lub nie masz do niego dostępu."
		backHref="/cruises"
		backLabel="Wszystkie rejsy"
	/>
{:else}
	<NotFound
		title="Nie udało się wczytać rejsu"
		message="Coś poszło nie tak. Spróbuj ponownie za chwilę."
		backHref="/cruises"
		backLabel="Wszystkie rejsy"
	/>
{/if}
