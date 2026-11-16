<script lang="ts">
	import { formatDateRangeWithDays } from '#lib/dates.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import {
		deleteTrip,
		removeTripCrew,
		updateTripCrewRole,
		cancelTrip,
		completeTrip,
		generateTripEnrollToken,
		clearTripEnrollToken,
		getTripSignUp,
		publishTrip,
		unpublishTrip
	} from '#lib/api/routes.js';
	import type { CompleteTripPayload, LegSignUp } from '#lib/api/types.js';
	import { clearFailure, errorMessage, noteFailure, settle } from '#lib/api/errors.js';
	import { goto } from '$app/navigation';
	import CompleteTripModal from '#lib/components/CompleteTripModal.svelte';
	import ActionsMenu from '#lib/components/ActionsMenu.svelte';
	import BackLink from '#lib/components/BackLink.svelte';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import { formatCountries } from '#lib/location/countries.js';
	import PriceInfo from '#lib/components/PriceInfo.svelte';
	import { formatPrice, hasPriceInfo } from '#lib/pricing.js';
	import LegSignUpControl from '#lib/components/LegSignUpControl.svelte';
	import { freePlacesLabel, signUpClosedByDeadline, signUpDeadlineLabel } from '#lib/enrollment.js';
	import { legStatus, legStatusBadges, legStatusLabels, warsawToday } from '#lib/rejsy.js';
	import HiddenBadge from '#lib/components/HiddenBadge.svelte';
	import { canSeeHidden, isHidden, isLegHidden } from '#lib/publication.js';
	import { perLoad } from '#lib/state.svelte.js';

	// data comes from +page.ts, which reruns when the URL's trip changes, so
	// navigating between two trips swaps every value derived from it.
	let { data } = $props();

	let cruise = $derived(data.cruise);
	const crewLoadErrors = $derived(data.crewLoadErrors);
	// These change in place after a cancel, a crew role edit or removal, a sign-up, an
	// enroll-link toggle or a publish, and reset to the loaded value on
	// navigation.
	let trip = $derived(data.trip);
	let crew = $derived(data.crew);
	let signUp = $derived(data.signUp);
	let loadErrors = $derived(data.loadErrors);
	let enrollToken = $derived(data.trip.enroll_token ?? null);
	// Only the trip read carries the yacht; a cancel or publish replies without it.
	const yacht = $derived(data.trip.yacht ?? null);

	const id = $derived(data.trip.id);

	let showCompleteModal = $state(false);
	let deleteError = $state('');
	let removeCrewError = $state('');
	// The *For values name the trip whose request is in flight, so a button
	// stays busy only on that trip, not on one a link moved to.
	let togglingFor = $state<number | null>(null);
	let completingFor = $state<number | null>(null);
	const togglingEnroll = $derived(togglingFor === id);
	const completing = $derived(completingFor === id);

	const places = $derived(signUp ? freePlacesLabel(trip.max_crew, signUp.accepted_count) : null);
	// Trips are club-managed: only admins create and edit them and manage
	// their crew, which comes from accepted sign-ups.
	const canManageCrew = $derived(auth.isAdmin);
	// The leg's captain reads the crew with contacts but changes nothing.
	const isLegCaptain = $derived(
		!!auth.user?.person_id && trip.captain_person_id === auth.user.person_id
	);
	const canSeeCrew = $derived(canManageCrew || isLegCaptain);
	const showHidden = $derived(canSeeHidden(auth));
	// Crew failures join the banner only for those who see the crew. The
	// check is reactive: on a direct load the role arrives after the load.
	const visibleLoadErrors = $derived(canSeeCrew ? [...loadErrors, ...crewLoadErrors] : loadErrors);

	// Form input and the complete dialog belong to one trip: a link to
	// another clears them.
	$effect(() => {
		void id;
		showCompleteModal = false;
		deleteError = '';
		removeCrewError = '';
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

	let publication = $derived(perLoad(data, { busy: false, error: '' }));
	// Whether the leg takes sign-ups depends on its and the cruise's
	// publication, so the sign-up control reloads after either changes.
	async function refreshSignUp(held: typeof publication, tripId: number) {
		const errors = [...loadErrors];
		const label = 'zapis na etap';
		const next = await settle(
			getTripSignUp(tripId).then((s) => {
				clearFailure(errors, label);
				return s;
			}),
			signUp,
			noteFailure(errors, label)
		);
		if (held !== publication) return;
		signUp = next;
		loadErrors = errors;
	}
	async function handlePublish() {
		const held = publication;
		const tripId = id;
		held.busy = true;
		held.error = '';
		try {
			const next = await publishTrip(tripId);
			if (held !== publication) return;
			trip = next;
			// The API publishes a hidden cruise without a series along with its leg.
			if (cruise && isHidden(cruise) && cruise.series_id == null) {
				cruise = { ...cruise, published_at: next.published_at };
			}
			await refreshSignUp(held, tripId);
		} catch (err) {
			held.error = `Nie udało się opublikować etapu: ${errorMessage(err)}`;
		} finally {
			held.busy = false;
		}
	}
	async function handleUnpublish() {
		const held = publication;
		const tripId = id;
		held.busy = true;
		held.error = '';
		try {
			const next = await unpublishTrip(tripId);
			if (held !== publication) return;
			trip = next;
			await refreshSignUp(held, tripId);
		} catch (err) {
			held.error = `Nie udało się ukryć etapu: ${errorMessage(err)}`;
		} finally {
			held.busy = false;
		}
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

	type CrewEntry = (typeof crew)[number];

	// Role drafts by crew entry, the entry whose role is saving and the last
	// failure; they start over on another trip.
	let roles = $derived(
		perLoad(data, {
			drafts: {} as Record<number, string>,
			saving: null as number | null,
			error: ''
		})
	);

	function roleDraft(member: CrewEntry) {
		return roles.drafts[member.id] ?? member.role;
	}

	function roleChanged(member: CrewEntry) {
		const next = roleDraft(member).trim();
		return next !== '' && next !== member.role;
	}

	async function saveRole(e: Event, member: CrewEntry) {
		e.preventDefault();
		if (!roleChanged(member)) return;
		const held = roles;
		const tripId = id;
		const role = roleDraft(member).trim();
		held.saving = member.id;
		held.error = '';
		try {
			const updated = await updateTripCrewRole(tripId, member.id, role);
			if (held !== roles) return;
			crew = crew.map((c) => (c.id === member.id ? { ...c, role: updated.role } : c));
			delete held.drafts[member.id];
		} catch (err) {
			console.error('Failed to update crew role:', err);
			if (held === roles) held.error = `Nie udało się zmienić roli: ${errorMessage(err)}`;
		} finally {
			if (held.saving === member.id) held.saving = null;
		}
	}

	async function removeCrew(assignmentId: number) {
		if (!confirm('Usunąć przypisanie załoganta?')) return;
		const tripId = id;
		removeCrewError = '';
		try {
			await removeTripCrew(tripId, assignmentId);
		} catch (err) {
			console.error('Failed to remove crew:', err);
			if (!leftTrip(tripId)) {
				removeCrewError = `Nie udało się usunąć załoganta: ${errorMessage(err)}`;
			}
			return;
		}
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

	const status = $derived(legStatus(trip));
	// Admins complete any planned leg; its captain only once it ended, which
	// the API checks again.
	const canComplete = $derived(
		trip.status === 'planned' && (canManageCrew || (isLegCaptain && status === 'overdue'))
	);
</script>

<div class="mx-auto max-w-4xl">
	<LoadErrorBanner errors={visibleLoadErrors} />
	<div class="mb-6 flex flex-wrap items-center justify-between gap-3">
		<div class="min-w-0 grow basis-72">
			{#if cruise}
				<BackLink href="/cruises/{cruise.id}" label="Część wydarzenia: {cruise.name}" />
			{/if}
			<div class="flex items-center gap-3">
				<h1 class="min-w-0 text-3xl font-bold text-[var(--navy)]">{trip.name}</h1>
				<span class="rounded-full px-2.5 py-1 text-xs font-medium {legStatusBadges[status]}">
					{legStatusLabels[status]}
				</span>
				{#if showHidden && isLegHidden(trip, cruise)}
					<HiddenBadge reason={isHidden(trip) ? undefined : 'Ukryte, bo wydarzenie jest ukryte'} />
				{/if}
			</div>
			<p class="mt-1 text-[var(--text-muted)]">
				{#if trip.start_port && trip.end_port}
					{trip.start_port} → {trip.end_port}
				{/if}
				{#if trip.countries?.length}
					· {formatCountries(trip.countries, data.countries)}{/if}
			</p>
		</div>
		{#if canManageCrew || canComplete}
			<div class="flex flex-wrap gap-2">
				{#if canComplete}
					<button
						onclick={() => (showCompleteModal = true)}
						disabled={completing}
						class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
					>
						Zrealizuj
					</button>
				{/if}
				{#if canManageCrew}
					{#if isHidden(trip)}
						<button
							onclick={handlePublish}
							disabled={publication.busy}
							class="rounded-lg border border-[var(--ocean)] px-4 py-2 text-sm text-[var(--ocean)] hover:bg-[var(--ocean-tint)] disabled:opacity-50"
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
				{/if}
			</div>
		{/if}
	</div>

	{#if deleteError}
		<div role="alert" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{deleteError}</div>
	{/if}
	{#if publication.error}
		<div role="alert" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">
			{publication.error}
		</div>
	{/if}

	{#if canComplete && status === 'overdue'}
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

	{#if trip.embark_date || trip.captain_name || yacht || trip.cost_total}
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
				{#if yacht}
					<dt class="text-[var(--text-muted)]">Jacht</dt>
					<dd>{yacht.yacht_type ? `${yacht.yacht_type} ` : ''}{yacht.name}</dd>
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

	{#if canSeeCrew}
		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm" data-testid="trip-crew">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="font-semibold text-[var(--navy)]">Załoga ({crew.length})</h2>
			</div>

			{#if canManageCrew && trip.status === 'planned'}
				<p class="mb-4 text-sm text-[var(--text-muted)]" data-testid="crew-hint">
					Załoga powstaje z zaakceptowanych zapisów. Aby dodać osobę, zapisz ją na etap i zaakceptuj
					w
					<a href="/trips/{id}/enrollments" class="text-[var(--ocean)] hover:underline"
						>Zarządzaj zapisami</a
					>.
				</p>
			{/if}

			{#if removeCrewError}
				<div role="alert" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">
					{removeCrewError}
				</div>
			{/if}
			{#if roles.error}
				<div role="alert" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">
					{roles.error}
				</div>
			{/if}

			{#if crew.length === 0}
				<p class="text-sm text-[var(--text-muted)]">Brak przypisanej załogi.</p>
			{:else}
				<div class="space-y-2">
					{#each crew as member (member.id)}
						<div class="rounded-lg bg-gray-50 px-4 py-2" data-testid="crew-member">
							<div class="flex items-center justify-between gap-2 max-md:flex-wrap">
								<div class="flex min-w-0 items-center gap-2 max-md:flex-wrap">
									<span class="min-w-0 font-medium">{member.full_name}</span>
									{#if canManageCrew && trip.status === 'planned'}
										<form class="flex items-center gap-1" onsubmit={(e) => saveRole(e, member)}>
											<label class="sr-only" for="crew-role-{member.id}"
												>Rola: {member.full_name}</label
											>
											<input
												id="crew-role-{member.id}"
												type="text"
												maxlength="100"
												value={roleDraft(member)}
												oninput={(e) => (roles.drafts[member.id] = e.currentTarget.value)}
												class="w-36 rounded-lg border px-2 py-1 text-sm"
											/>
											<button
												type="submit"
												disabled={roles.saving === member.id || !roleChanged(member)}
												aria-label="Zapisz rolę: {member.full_name}"
												class="rounded-lg border px-2 py-1 text-xs hover:bg-white disabled:opacity-50"
											>
												{roles.saving === member.id ? 'Zapisywanie...' : 'Zapisz'}
											</button>
										</form>
									{:else}
										<span
											class="rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]"
										>
											{member.role}
										</span>
									{/if}
								</div>
								{#if canManageCrew && trip.status === 'planned'}
									<button
										onclick={() => removeCrew(member.id)}
										class="text-sm text-red-500 hover:underline"
									>
										Usuń
									</button>
								{/if}
							</div>
							{#if member.phone || member.email || member.emergency_contact_name || member.emergency_contact_phone}
								<dl
									class="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--text-muted)] [&_dd]:min-w-0"
									data-testid="crew-contacts"
								>
									{#if member.phone}
										<div class="flex gap-1">
											<dt>Telefon:</dt>
											<dd>
												<a href="tel:{member.phone}" class="text-[var(--ocean)] hover:underline"
													>{member.phone}</a
												>
											</dd>
										</div>
									{/if}
									{#if member.email}
										<div class="flex gap-1">
											<dt>E-mail:</dt>
											<dd>
												<a
													href="mailto:{member.email}"
													class="break-all text-[var(--ocean)] hover:underline">{member.email}</a
												>
											</dd>
										</div>
									{/if}
									{#if member.emergency_contact_name || member.emergency_contact_phone}
										<div class="flex gap-1">
											<dt>Kontakt awaryjny:</dt>
											<dd>
												{member.emergency_contact_name ?? ''}
												{#if member.emergency_contact_phone}
													<a
														href="tel:{member.emergency_contact_phone}"
														class="text-[var(--ocean)] hover:underline"
														>{member.emergency_contact_phone}</a
													>
												{/if}
											</dd>
										</div>
									{/if}
								</dl>
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
