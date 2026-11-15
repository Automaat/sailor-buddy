<script lang="ts">
	import { formatDateRangeWithDays } from '#lib/dates.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import { deleteCruise } from '#lib/api/routes.js';
	import type { LegSignUp } from '#lib/api/types.js';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import CruiseEnrollmentAdmin from './CruiseEnrollmentAdmin.svelte';
	import PriceInfo from '#lib/components/PriceInfo.svelte';
	import { formatPrice, hasPriceInfo, yourPrice } from '#lib/pricing.js';
	import LegSignUpControl from '#lib/components/LegSignUpControl.svelte';
	import CruiseLogistics from '#lib/components/CruiseLogistics.svelte';
	import { isHttpUrl } from '#lib/cruise.js';
	import { freePlacesLabel, numberLegs, signUpDeadlineLabel } from '#lib/enrollment.js';
	import { legStatus, legStatusBadges, legStatusLabels } from '#lib/rejsy.js';
	import { goto } from '$app/navigation';

	// data comes from +page.ts, which reruns when the URL's cruise changes, so
	// navigating between two cruises swaps every value derived from it.
	let { data } = $props();

	const cruise = $derived(data.cruise);
	const series = $derived(data.series);
	const trips = $derived(data.trips);
	const voyages = $derived(data.voyages);
	// signUps and enrollToken change in place after a sign-up or a token
	// toggle, and reset to the loaded value on navigation.
	let signUps = $derived(data.signUps);
	const id = $derived(data.cruise.id);
	const isAdmin = $derived(auth.isAdmin);

	const legs = $derived(numberLegs(trips));
	const signUpByTrip = $derived(new Map(signUps.map((s) => [s.trip_id, s])));
	function updateSignUp(next: LegSignUp) {
		signUps = signUps.map((s) => (s.trip_id === next.trip_id ? next : s));
	}

	let deleteError = $state('');
	$effect(() => {
		id;
		deleteError = '';
	});
	async function handleDelete() {
		if (trips.length > 0 || voyages.length > 0) {
			deleteError = 'Nie można usunąć wydarzenia, które ma etapy. Najpierw usuń jego rejsy.';
			return;
		}
		if (!confirm('Usunąć wydarzenie?')) return;
		deleteError = '';
		try {
			await deleteCruise(id);
			void goto('/cruises');
		} catch (err) {
			deleteError = err instanceof Error ? err.message : 'Nie udało się usunąć wydarzenia';
		}
	}
</script>

<div class="mx-auto max-w-5xl">
	<LoadErrorBanner errors={data.loadErrors} />
	<div class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3">
		<div>
			<h1 class="text-3xl font-bold text-[var(--navy)]">{cruise.name}</h1>
			{#if series}
				<p class="mt-1 text-sm">
					<a href="/series/{series.id}" class="text-[var(--ocean)] hover:underline">{series.name}</a
					>
					{#if cruise.edition}
						· edycja {cruise.edition}{/if}
				</p>
			{/if}
			<p class="mt-1 text-[var(--text-muted)]">
				{#if cruise.start_port && cruise.end_port}
					{cruise.start_port} → {cruise.end_port}
				{/if}
				{#if cruise.countries}
					· {cruise.countries}{/if}
			</p>
			<p class="mt-1 text-sm text-[var(--text-muted)]">
				{#if cruise.embark_date}
					{formatDateRangeWithDays(cruise.embark_date, cruise.disembark_date)}
				{/if}
				{#if cruise.max_crew}
					· max {cruise.max_crew} os.{/if}
			</p>
		</div>
		{#if isAdmin}
			<div class="flex gap-2">
				<a
					href="/cruises/{id}/edit"
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
					class="tap-target rounded-lg bg-[var(--ocean)] px-3 py-1.5 text-sm text-white hover:bg-[var(--ocean-dark)]"
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
					<div
						class="flex flex-wrap items-start justify-between gap-3 rounded-lg bg-gray-50 px-4 py-3"
						data-testid="leg-row"
					>
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
									<span>{formatDateRangeWithDays(trip.embark_date, trip.disembark_date)}</span>
								{/if}
								{#if price !== null}<span>{formatPrice(price, trip.currency)}</span>{/if}
								{#if places && trip.status !== 'cancelled'}<span data-testid="free-places"
										>{places}</span
									>{/if}
								{#if trip.sign_up_deadline && trip.status !== 'cancelled'}
									<span data-testid="sign-up-deadline"
										>{signUpDeadlineLabel(trip.sign_up_deadline)}</span
									>
								{/if}
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

	<CruiseEnrollmentAdmin {cruise} {trips} />
	{#if voyages.length > 0 || isAdmin}
		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="font-semibold text-[var(--navy)]">Zrealizowane etapy ({voyages.length})</h2>
				{#if isAdmin}
					<a
						href="/voyages/new?cruise_id={id}"
						class="tap-target rounded-lg border px-3 py-1.5 text-sm hover:bg-gray-50"
					>
						+ Wpisz zrealizowany
					</a>
				{/if}
			</div>
			{#if voyages.length === 0}
				<p class="text-sm text-[var(--text-muted)]">Brak zrealizowanych etapów.</p>
			{/if}
			<div class="space-y-2">
				{#each voyages as voyage (voyage.id)}
					<a
						href="/voyages/{voyage.id}"
						class="block rounded-lg bg-gray-50 px-4 py-3 hover:bg-gray-100"
					>
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
