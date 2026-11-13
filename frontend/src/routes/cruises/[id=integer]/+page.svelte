<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { patentSummary } from '#lib/profile.js';
	import {
		getCruise,
		getSeries,
		deleteCruise,
		listCruiseTrips,
		listCruiseVoyages,
		listCruiseEnrollments,
		listCruiseTripEnrollments,
		listCruiseLegSignUps,
		generateCruiseEnrollToken,
		clearCruiseEnrollToken,
		updateCruiseEnrollmentStatus,
		assignCruiseEnrollmentTrip,
		deleteCruiseEnrollment
	} from '#lib/api/routes.js';
	import type {
		Cruise,
		CruiseSeries,
		Trip,
		Voyage,
		CruiseEnrollment,
		TripEnrollment,
		LegSignUp
	} from '#lib/api/types.js';
	import { ApiError } from '#lib/api/client.js';
	import NotFound from '#lib/components/NotFound.svelte';
	import PriceInfo from '#lib/components/PriceInfo.svelte';
	import { formatPrice, hasPriceInfo, yourPrice } from '#lib/pricing.js';
	import LegSignUpControl from '#lib/components/LegSignUpControl.svelte';
	import CruiseLogistics from '#lib/components/CruiseLogistics.svelte';
	import { isHttpUrl } from '#lib/cruise.js';
	import { freePlacesLabel, numberLegs, statusLabels } from '#lib/enrollment.js';
	import { legStatus, legStatusBadges, legStatusLabels } from '#lib/rejsy.js';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	type EnrollStatus = 'accepted' | 'rejected' | 'waitlisted' | 'pending';

	let cruise = $state<Cruise | null>(null);
	let series = $state<CruiseSeries | null>(null);
	let trips = $state<Trip[]>([]);
	let voyages = $state<Voyage[]>([]);
	let enrollments = $state<CruiseEnrollment[]>([]);
	let legEnrollments = $state<TripEnrollment[]>([]);
	let signUps = $state<LegSignUp[]>([]);
	let adminLoadedFor = $state<number | null>(null);
	let loading = $state(true);
	let notFound = $state(false);
	let enrollToken = $state<string | null>(null);
	let togglingEnroll = $state(false);

	const id = $derived(Number(page.params.id));
	const isAdmin = $derived(auth.isAdmin);

	const legs = $derived(numberLegs(trips));
	const signUpByTrip = $derived(new Map(signUps.map((s) => [s.trip_id, s])));
	const legGroups = $derived(
		legs.map((l) => ({
			...l,
			signUps: legEnrollments.filter((e) => e.trip_id === l.trip.id),
			legacy: enrollments.filter((e) => e.trip_id === l.trip.id)
		}))
	);
	const unassigned = $derived(
		enrollments.filter((e) => e.trip_id == null || !trips.some((t) => t.id === e.trip_id))
	);

	async function reloadEnrollments() {
		enrollments = (await listCruiseEnrollments(id)) ?? [];
	}

	$effect(() => {
		if (!isAdmin || !cruise || adminLoadedFor === id) return;
		adminLoadedFor = id;
		Promise.all([
			listCruiseEnrollments(id).then((r) => r ?? []).catch(() => []),
			listCruiseTripEnrollments(id).then((r) => r ?? []).catch(() => [])
		]).then(([e, te]) => {
			enrollments = e;
			legEnrollments = te;
		});
	});

	function updateSignUp(next: LegSignUp) {
		signUps = signUps.map((s) => (s.trip_id === next.trip_id ? next : s));
	}

	onMount(async () => {
		try {
			cruise = await getCruise(id);
			enrollToken = cruise?.enroll_token ?? null;
			const seriesID = cruise?.series_id;
			const [t, v, su, s] = await Promise.all([
				listCruiseTrips(id).then((r) => r ?? []).catch(() => []),
				listCruiseVoyages(id).then((r) => r ?? []).catch(() => []),
				listCruiseLegSignUps(id).then((r) => r ?? []).catch(() => []),
				seriesID ? getSeries(seriesID).catch(() => null) : Promise.resolve(null)
			]);
			trips = t;
			voyages = v;
			signUps = su;
			series = s;
		} catch (err) {
			if (err instanceof ApiError && err.status === 404) {
				notFound = true;
			} else {
				console.error('Failed to load cruise:', err);
			}
		} finally {
			loading = false;
		}
	});

	let deleteError = $state('');
	let enrollmentError = $state('');

	// A 409 from a cruise enrollment change means the leg has no free place.
	function enrollmentFailure(err: unknown, fallback: string) {
		enrollmentError =
			err instanceof ApiError && err.status === 409
				? 'Etap jest pełny albo zapis się zmienił. Odśwież stronę i zwolnij miejsce, zanim zaakceptujesz kolejną osobę.'
				: fallback;
	}

	async function handleDelete() {
		if (trips.length > 0 || voyages.length > 0) {
			deleteError = 'Nie można usunąć wydarzenia, które ma etapy. Najpierw usuń jego rejsy.';
			return;
		}
		if (!confirm('Usunąć wydarzenie?')) return;
		deleteError = '';
		try {
			await deleteCruise(id);
			goto('/cruises');
		} catch (err) {
			deleteError = err instanceof Error ? err.message : 'Nie udało się usunąć wydarzenia';
		}
	}

	async function toggleEnrollment() {
		togglingEnroll = true;
		try {
			if (enrollToken) {
				await clearCruiseEnrollToken(id);
				enrollToken = null;
			} else {
				const res = await generateCruiseEnrollToken(id);
				enrollToken = res.token;
			}
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

	async function setEnrollmentStatus(enrollmentId: number, status: EnrollStatus) {
		enrollmentError = '';
		try {
			await updateCruiseEnrollmentStatus(id, enrollmentId, status);
			enrollments = enrollments.map((e) => (e.id === enrollmentId ? { ...e, status } : e));
		} catch (err) {
			enrollmentFailure(err, 'Nie udało się zmienić statusu.');
			console.error('Failed to set status:', err);
		}
	}

	async function assignEnrollmentToTrip(enrollmentId: number, tripId: number | null) {
		enrollmentError = '';
		try {
			await assignCruiseEnrollmentTrip(id, enrollmentId, tripId ?? undefined);
			await reloadEnrollments();
		} catch (err) {
			enrollmentFailure(err, 'Nie udało się przypisać zapisu do etapu.');
			console.error('Failed to assign to trip:', err);
		}
	}

	async function deleteEnrollment(enrollmentId: number) {
		if (!confirm('Usunąć ten zapis?')) return;
		try {
			await deleteCruiseEnrollment(id, enrollmentId);
			enrollments = enrollments.filter((e) => e.id !== enrollmentId);
		} catch (err) {
			console.error('Failed to delete enrollment:', err);
		}
	}

	const statusColors: Record<string, string> = {
		pending: 'bg-[var(--ocean-tint)] text-[var(--ocean)]',
		accepted: 'bg-[var(--ocean)] text-white',
		rejected: 'bg-gray-100 text-gray-600',
		waitlisted: 'bg-[var(--navy)]/10 text-[var(--navy)]'
	};
</script>

{#snippet legacyEnrollment(enrollment: CruiseEnrollment)}
	<div class="rounded-lg bg-gray-50 p-3" data-testid="legacy-enrollment">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex flex-wrap items-center gap-2">
				<span class="font-medium">{enrollment.user_name ?? 'Nieznany'}</span>
				<span class="text-sm text-[var(--text-muted)]">{enrollment.user_email ?? ''}</span>
				{#if enrollment.user_phone}
					<a href="tel:{enrollment.user_phone}" class="text-sm text-[var(--ocean)] hover:underline">{enrollment.user_phone}</a>
				{/if}
				<span class="text-xs text-[var(--text-muted)]">
					{patentSummary(enrollment.user_patent_type, enrollment.user_patent_number)}
				</span>
				<span class="rounded-full px-2 py-0.5 text-xs font-medium {statusColors[enrollment.status] ?? 'bg-gray-100'}">
					{statusLabels[enrollment.status] ?? enrollment.status}
				</span>
				<span class="text-xs text-[var(--text-muted)]">zapis na wydarzenie</span>
			</div>
			<div class="flex flex-wrap gap-1">
				{#if enrollment.status !== 'accepted'}
					<button onclick={() => setEnrollmentStatus(enrollment.id, 'accepted')} class="rounded px-2 py-1 text-xs text-[var(--ocean)] hover:bg-[var(--ocean-tint)]">
						Akceptuj
					</button>
				{/if}
				{#if enrollment.status !== 'waitlisted'}
					<button onclick={() => setEnrollmentStatus(enrollment.id, 'waitlisted')} class="rounded px-2 py-1 text-xs text-[var(--navy)] hover:bg-[var(--ocean-tint)]">
						Rezerwa
					</button>
				{/if}
				{#if enrollment.status !== 'rejected'}
					<button onclick={() => setEnrollmentStatus(enrollment.id, 'rejected')} class="rounded px-2 py-1 text-xs text-red-700 hover:bg-red-100">
						Odrzuć
					</button>
				{/if}
				<button onclick={() => deleteEnrollment(enrollment.id)} class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-100">
					Usuń
				</button>
			</div>
		</div>
		<div class="mt-2 flex items-center gap-2 text-sm">
			<label for={`assign-${enrollment.id}`} class="text-[var(--text-muted)]">Etap:</label>
			<select
				id={`assign-${enrollment.id}`}
				value={enrollment.trip_id ?? ''}
				onchange={(e) => {
					const v = (e.currentTarget as HTMLSelectElement).value;
					assignEnrollmentToTrip(enrollment.id, v ? Number(v) : null);
				}}
				class="rounded border px-2 py-1 text-sm"
			>
				<option value="">— bez etapu —</option>
				{#each trips as t (t.id)}
					<option value={t.id}>{t.name}</option>
				{/each}
			</select>
		</div>
		{#if enrollment.note}
			<p class="mt-2 text-sm text-[var(--text-muted)]">{enrollment.note}</p>
		{/if}
	</div>
{/snippet}

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if cruise}
	<div class="mx-auto max-w-5xl">
		<div class="mb-6 flex items-center justify-between">
			<div>
				<h1 class="text-3xl font-bold text-[var(--navy)]">{cruise.name}</h1>
				{#if series}
					<p class="mt-1 text-sm">
						<a href="/series/{series.id}" class="text-[var(--ocean)] hover:underline">{series.name}</a>
						{#if cruise.edition} · edycja {cruise.edition}{/if}
					</p>
				{/if}
				<p class="mt-1 text-[var(--text-muted)]">
					{#if cruise.start_port && cruise.end_port}
						{cruise.start_port} → {cruise.end_port}
					{/if}
					{#if cruise.countries} · {cruise.countries}{/if}
				</p>
				<p class="mt-1 text-sm text-[var(--text-muted)]">
					{#if cruise.embark_date}
						{cruise.embark_date}
						{#if cruise.disembark_date}– {cruise.disembark_date}{/if}
					{/if}
					{#if cruise.max_crew} · max {cruise.max_crew} os.{/if}
				</p>
			</div>
			{#if isAdmin}
				<div class="flex gap-2">
					<a href="/cruises/{id}/edit" class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">Edytuj</a>
					<button onclick={handleDelete} class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50">
						Usuń
					</button>
				</div>
			{/if}
		</div>

		{#if deleteError}
			<div role="alert" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{deleteError}</div>
		{/if}

		{#if cruise.description}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Opis</h2>
				<p class="whitespace-pre-wrap text-sm">{cruise.description}</p>
			</div>
		{/if}

		{#if hasPriceInfo(cruise, cruise.included, cruise.not_included)}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Cena</h2>
				<PriceInfo
					priced={cruise}
					membership={auth.user?.membership}
					included={cruise.included}
					notIncluded={cruise.not_included}
				/>
			</div>
		{/if}

		{#if cruise.travel_notes || cruise.min_age != null || (cruise.group_chat_url && isHttpUrl(cruise.group_chat_url))}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Informacje dla załogi</h2>
				<CruiseLogistics
					groupChatUrl={cruise.group_chat_url}
					travelNotes={cruise.travel_notes}
					minAge={cruise.min_age}
				/>
			</div>
		{/if}

		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="font-semibold text-[var(--navy)]">Planowane etapy ({trips.length})</h2>
				{#if isAdmin}
					<a
						href="/trips/new?cruise_id={id}"
						class="rounded-lg bg-[var(--ocean)] px-3 py-1.5 text-sm text-white hover:bg-[var(--ocean-dark)]"
					>
						+ Dodaj etap
					</a>
				{/if}
			</div>
			{#if trips.length === 0}
				<p class="text-sm text-[var(--text-muted)]">Brak etapów</p>
			{:else}
				<div class="space-y-2">
					{#each legs as leg (leg.trip.id)}
						{@const trip = leg.trip}
						{@const status = legStatus(trip, cruise)}
						{@const signUp = signUpByTrip.get(trip.id)}
						{@const price = yourPrice(trip, auth.user?.membership)}
						{@const places = signUp ? freePlacesLabel(trip.max_crew, signUp.accepted_count) : null}
						<div class="flex flex-wrap items-start justify-between gap-3 rounded-lg bg-gray-50 px-4 py-3" data-testid="leg-row">
							<div class="min-w-0">
								<div class="flex flex-wrap items-center gap-2">
									<a href="/trips/{trip.id}" class="font-medium hover:underline">{trip.name}</a>
									<span class="rounded-full px-2 py-0.5 text-xs {legStatusBadges[status]}">
										{legStatusLabels[status]}
									</span>
									{#if isAdmin && status === 'overdue'}
										<span class="text-xs font-medium text-[var(--ocean)]">Zrealizuj →</span>
									{/if}
								</div>
								<div class="mt-0.5 flex flex-wrap gap-x-3 text-xs text-[var(--text-muted)]">
									{#if trip.captain_name}<span>kpt. {trip.captain_name}</span>{/if}
									{#if trip.embark_date}
										<span>{trip.embark_date} – {trip.disembark_date ?? '?'}</span>
									{/if}
									{#if price !== null}<span>{formatPrice(price, trip.currency)}</span>{/if}
									{#if places && trip.status !== 'cancelled'}<span data-testid="free-places">{places}</span>{/if}
								</div>
							</div>
							{#if signUp}
								<LegSignUpControl
									{signUp}
									legName={trip.name}
									cancelled={trip.status === 'cancelled'}
									onchange={updateSignUp}
								/>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</div>

		{#if isAdmin}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm" data-testid="admin-sign-ups">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Zapisy na etapy</h2>
				<div class="mb-4 flex flex-wrap items-center gap-3">
					<button
						onclick={toggleEnrollment}
						disabled={togglingEnroll}
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
					<span class="text-xs text-[var(--text-muted)]">Link do podglądu wydarzenia, z którego można zapisać się na etapy.</span>
				</div>

				{#if enrollmentError}
					<p role="alert" class="mb-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{enrollmentError}</p>
				{/if}
				{#if legGroups.length === 0 && unassigned.length === 0}
					<p class="text-sm text-[var(--text-muted)]">Brak zapisów.</p>
				{/if}
				<div class="space-y-4">
					{#each legGroups as group (group.trip.id)}
						<section aria-label={`Zapisy: ${group.trip.name}`}>
							<div class="mb-1 flex items-center justify-between">
								<h3 class="text-sm font-medium">{group.trip.name}</h3>
								<a href="/trips/{group.trip.id}/enrollments" class="text-xs text-[var(--ocean)] hover:underline">
									Zarządzaj zapisami
								</a>
							</div>
							{#if group.signUps.length === 0 && group.legacy.length === 0}
								<p class="text-xs text-[var(--text-muted)]">Brak zapisów.</p>
							{/if}
							<div class="space-y-1">
								{#each group.signUps as e (e.id)}
									<div class="flex flex-wrap items-center gap-2 rounded-lg bg-gray-50 px-3 py-2 text-sm">
										<span class="font-medium">{e.user_name || e.user_email}</span>
										<span class="rounded-full px-2 py-0.5 text-xs font-medium {statusColors[e.status] ?? 'bg-gray-100'}">
											{statusLabels[e.status] ?? e.status}
										</span>
										{#if e.note}<span class="text-xs text-[var(--text-muted)]">{e.note}</span>{/if}
									</div>
								{/each}
								{#each group.legacy as enrollment (enrollment.id)}
									{@render legacyEnrollment(enrollment)}
								{/each}
							</div>
						</section>
					{/each}
					{#if unassigned.length > 0}
						<section aria-label="Zapisy: Bez etapu">
							<h3 class="mb-1 text-sm font-medium">Bez etapu</h3>
							<p class="mb-1 text-xs text-[var(--text-muted)]">Zapisy na całe wydarzenie sprzed zapisów na etapy. Przypisz je do etapu.</p>
							<div class="space-y-1">
								{#each unassigned as enrollment (enrollment.id)}
									{@render legacyEnrollment(enrollment)}
								{/each}
							</div>
						</section>
					{/if}
				</div>
			</div>
		{/if}

		{#if voyages.length > 0 || isAdmin}
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<div class="mb-3 flex items-center justify-between">
					<h2 class="font-semibold text-[var(--navy)]">Zrealizowane etapy ({voyages.length})</h2>
					{#if isAdmin}
						<a href="/voyages/new?cruise_id={id}" class="rounded-lg border px-3 py-1.5 text-sm hover:bg-gray-50">
							+ Wpisz zrealizowany
						</a>
					{/if}
				</div>
				{#if voyages.length === 0}
					<p class="text-sm text-[var(--text-muted)]">Brak zrealizowanych etapów.</p>
				{/if}
				<div class="space-y-2">
					{#each voyages as voyage}
						<a href="/voyages/{voyage.id}" class="block rounded-lg bg-gray-50 px-4 py-3 hover:bg-gray-100">
							<div class="flex items-center justify-between">
								<span class="font-medium">{voyage.name}</span>
								<span class="text-sm text-[var(--text-muted)]">
									{Math.round(voyage.miles)} Mm · {voyage.days} dni
								</span>
							</div>
						</a>
					{/each}
				</div>
			</div>
		{/if}
	</div>
{:else if notFound}
	<NotFound
		title="Nie znaleziono wydarzenia"
		message="To wydarzenie nie istnieje lub nie masz do niego dostępu."
		backHref="/cruises"
		backLabel="Wszystkie rejsy"
	/>
{:else}
	<NotFound
		title="Nie udało się wczytać wydarzenia"
		message="Coś poszło nie tak. Spróbuj ponownie za chwilę."
		backHref="/cruises"
		backLabel="Wszystkie rejsy"
	/>
{/if}
