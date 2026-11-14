<script lang="ts">
	import { formatDateRangeWithDays } from '#lib/dates.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import {
		deleteTrip,
		listTripCrew,
		assignTripCrew,
		removeTripCrew,
		cancelTrip,
		completeTrip,
		generateTripEnrollToken,
		clearTripEnrollToken,
		getTripSignUp
	} from '#lib/api/routes.js';
	import type { CompleteTripPayload, LegSignUp } from '#lib/api/types.js';
	import { clearFailure, errorMessage, noteFailure, settle } from '#lib/api/errors.js';
	import { goto } from '$app/navigation';
	import CompleteTripModal from '#lib/components/CompleteTripModal.svelte';
	import ActionsMenu from '#lib/components/ActionsMenu.svelte';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import PriceInfo from '#lib/components/PriceInfo.svelte';
	import { formatPrice, hasPriceInfo } from '#lib/pricing.js';
	import LegSignUpControl from '#lib/components/LegSignUpControl.svelte';
	import { freePlacesLabel, signUpClosedByDeadline, signUpDeadlineLabel } from '#lib/enrollment.js';
	import { legStatus, legStatusBadges, legStatusLabels, warsawToday } from '#lib/rejsy.js';

	// data comes from +page.ts, which reruns when the URL's trip changes, so
	// navigating between two trips swaps every value derived from it.
	let { data } = $props();

	const cruise = $derived(data.cruise);
	const allCrewMembers = $derived(data.allCrewMembers);
	const adminLoadErrors = $derived(data.adminLoadErrors);
	// These change in place after a cancel, a crew edit, a sign-up or an
	// enroll-link toggle, and reset to the loaded value on navigation.
	let trip = $derived(data.trip);
	let crew = $derived(data.crew);
	let signUp = $derived(data.signUp);
	let loadErrors = $derived(data.loadErrors);
	let enrollToken = $derived(data.trip.enroll_token ?? null);

	const id = $derived(data.trip.id);

	let assignCrewId = $state('');
	let assignRole = $state('');
	let showCompleteModal = $state(false);
	let deleteError = $state('');
	// The *For values name the trip whose request is in flight, so a button
	// stays busy only on that trip, not on one a link moved to.
	let assigningFor = $state<number | null>(null);
	let togglingFor = $state<number | null>(null);
	let completingFor = $state<number | null>(null);
	const assigning = $derived(assigningFor === id);
	const togglingEnroll = $derived(togglingFor === id);
	const completing = $derived(completingFor === id);

	const places = $derived(signUp ? freePlacesLabel(trip.max_crew, signUp.accepted_count) : null);
	// Trips are club-managed: only admins create, edit and crew them.
	const canManageCrew = $derived(auth.isAdmin);
	// Failures of admin-only sections join the banner only for admins. The
	// check is reactive: on a direct load the role arrives after the load.
	const visibleLoadErrors = $derived(
		canManageCrew ? [...loadErrors, ...adminLoadErrors] : loadErrors
	);

	// Form input and the complete dialog belong to one trip: a link to
	// another clears them.
	$effect(() => {
		void id;
		assignCrewId = '';
		assignRole = '';
		showCompleteModal = false;
		deleteError = '';
	});

	// A request started on one trip may answer after a link has moved the
	// page to another; its reply then belongs to a page the user has left
	// and is dropped.
	function leftTrip(tripId: number) {
		return tripId !== id;
	}

	// A sign-up reply names its trip; one for a trip the page has left is
	// dropped.
	function updateSignUp(next: LegSignUp) {
		if (!leftTrip(next.trip_id)) signUp = next;
	}

	async function handleDelete() {
		if (!confirm('Usunąć ten rejs?')) return;
		const tripId = id;
		const cruiseId = trip.cruise_id;
		deleteError = '';
		try {
			await deleteTrip(tripId);
		} catch (err) {
			console.error('Failed to delete trip:', err);
			if (!leftTrip(tripId)) deleteError = `Nie udało się usunąć etapu: ${errorMessage(err)}`;
			return;
		}
		if (leftTrip(tripId)) return;
		void goto(`/cruises/${cruiseId}`);
	}

	async function toggleEnrollment() {
		const tripId = id;
		togglingFor = tripId;
		try {
			let token: string | null;
			if (enrollToken) {
				await clearTripEnrollToken(tripId);
				token = null;
			} else {
				token = (await generateTripEnrollToken(tripId)).token;
			}
			if (leftTrip(tripId)) return;
			enrollToken = token;
			if (signUp) {
				signUp = {
					...signUp,
					open:
						token !== null &&
						trip.status === 'planned' &&
						!signUpClosedByDeadline(trip.sign_up_deadline, warsawToday())
				};
			}
		} catch (err) {
			console.error('Failed to toggle enrollment:', err);
		} finally {
			if (togglingFor === tripId) togglingFor = null;
		}
	}

	async function copyEnrollLink() {
		if (!enrollToken) return;
		try {
			await navigator.clipboard.writeText(`${window.location.origin}/enroll/${enrollToken}`);
		} catch (err) {
			console.error('Failed to copy enrollment link:', err);
		}
	}

	async function assignCrew(e: Event) {
		e.preventDefault();
		if (!assignCrewId || !assignRole) return;
		const tripId = id;
		assigningFor = tripId;
		try {
			await assignTripCrew(tripId, {
				crew_member_id: Number(assignCrewId),
				role: assignRole
			});
			const list = (await listTripCrew(tripId)) ?? [];
			if (leftTrip(tripId)) return;
			crew = list;
			assignCrewId = '';
			assignRole = '';
		} catch (err) {
			console.error('Failed to assign crew:', err);
		} finally {
			if (assigningFor === tripId) assigningFor = null;
		}
	}

	async function removeCrew(assignmentId: number) {
		if (!confirm('Usunąć przypisanie załoganta?')) return;
		const tripId = id;
		await removeTripCrew(tripId, assignmentId);
		if (leftTrip(tripId)) return;
		crew = crew.filter((c) => c.id !== assignmentId);
	}

	async function handleCancel() {
		if (!confirm('Odwołać ten etap?')) return;
		const tripId = id;
		completingFor = tripId;
		try {
			const cancelled = await cancelTrip(tripId);
			if (leftTrip(tripId)) return;
			trip = cancelled;
			enrollToken = cancelled.enroll_token ?? null;
			const errors = [...loadErrors];
			const next = await settle(
				getTripSignUp(tripId).then((s) => {
					clearFailure(errors, 'zapis na etap');
					return s;
				}),
				signUp,
				noteFailure(errors, 'zapis na etap')
			);
			if (leftTrip(tripId)) return;
			signUp = next;
			loadErrors = errors;
		} catch (err) {
			console.error('Failed to cancel trip:', err);
		} finally {
			if (completingFor === tripId) completingFor = null;
		}
	}

	async function handleComplete(payload: CompleteTripPayload) {
		const tripId = id;
		const voyage = await completeTrip(tripId, payload);
		if (leftTrip(tripId)) return;
		showCompleteModal = false;
		void goto(`/voyages/${voyage.id}`);
	}

	const status = $derived(legStatus(trip, cruise));
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
				{#if trip.countries}
					· {trip.countries}{/if}
			</p>
		</div>
		{#if canManageCrew}
			<div class="flex gap-2 max-md:flex-wrap">
				{#if trip.status === 'planned'}
					<button
						onclick={() => (showCompleteModal = true)}
						disabled={completing}
						class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
					>
						Zrealizuj
					</button>
				{/if}
				<a
					href="/trips/{id}/edit"
					class="tap-target rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">Edytuj</a
				>
				<ActionsMenu>
					{#snippet children(close)}
						{#if trip.status === 'planned'}
							<button
								type="button"
								role="menuitem"
								disabled={completing}
								onclick={() => {
									close();
									void handleCancel();
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
								void handleDelete();
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

	{#if deleteError}
		<div role="alert" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{deleteError}</div>
	{/if}

	{#if canManageCrew && status === 'overdue'}
		<div
			role="status"
			class="mb-6 flex items-center justify-between gap-4 rounded-2xl border border-[var(--ocean)]/30 bg-[var(--ocean-tint)] p-4 text-sm text-[var(--navy)]"
		>
			<p>
				Rejs się zakończył, ale nadal czeka na rozliczenie. Zrealizuj go, aby wpisać przebieg do
				dziennika.
			</p>
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
			<dl class="grid grid-cols-1 text-sm sm:grid-cols-2 sm:gap-2 max-sm:[&>dd]:mb-2">
				{#if trip.embark_date}
					<dt class="text-[var(--text-muted)]">Daty</dt>
					<dd>{formatDateRangeWithDays(trip.embark_date, trip.disembark_date)}</dd>
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
		<div
			class="mb-6 flex flex-wrap items-start justify-between gap-3 rounded-2xl bg-white p-6 shadow-sm"
			data-testid="my-sign-up"
		>
			<div>
				<h2 class="font-semibold text-[var(--navy)]">Twój zapis</h2>
				{#if places && trip.status !== 'cancelled'}
					<p class="mt-1 text-sm text-[var(--text-muted)]" data-testid="free-places">{places}</p>
				{/if}
				{#if trip.sign_up_deadline && trip.status !== 'cancelled'}
					<p class="mt-1 text-sm text-[var(--text-muted)]" data-testid="sign-up-deadline">
						{signUpDeadlineLabel(trip.sign_up_deadline)}
					</p>
				{/if}
			</div>
			<!-- Keyed on the trip so a link to another trip starts the control
			afresh: no busy flag, alert or note draft from the previous trip. -->
			{#key id}
				<LegSignUpControl
					{signUp}
					legName={trip.name}
					cancelled={trip.status === 'cancelled'}
					onchange={updateSignUp}
				/>
			{/key}
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
					<button
						onclick={copyEnrollLink}
						class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50"
					>
						Kopiuj link
					</button>
				{/if}
				<a
					href="/trips/{id}/enrollments"
					class="tap-target rounded-lg border px-4 py-2 text-sm hover:bg-gray-50"
				>
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
					<div class="min-w-0 max-w-full">
						<label for="assign-crew" class="block text-xs text-[var(--text-muted)]">Załogant</label>
						<select
							id="assign-crew"
							bind:value={assignCrewId}
							class="max-w-full rounded-lg border px-3 py-1.5 text-sm"
						>
							<option value="">Wybierz...</option>
							{#each allCrewMembers as member (member.id)}
								<option value={member.id}>{member.full_name}</option>
							{/each}
						</select>
					</div>
					<div>
						<label for="assign-role" class="block text-xs text-[var(--text-muted)]">Rola</label>
						<input
							id="assign-role"
							type="text"
							bind:value={assignRole}
							placeholder="np. Sternik"
							class="rounded-lg border px-3 py-1.5 text-sm"
						/>
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
					{#each crew as member (member.id)}
						<div class="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2">
							<div class="flex items-center gap-2">
								<span class="font-medium">{member.full_name}</span>
								<span
									class="rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]"
								>
									{member.role}
								</span>
							</div>
							{#if trip.status === 'planned'}
								<button
									onclick={() => removeCrew(member.id)}
									class="text-sm text-red-500 hover:underline"
								>
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
