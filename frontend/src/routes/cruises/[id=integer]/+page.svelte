<script lang="ts">
	import { formatDateRangeWithDays } from '#lib/dates.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import {
		deleteCruise,
		listCruiseLegSignUps,
		publishCruise,
		unpublishCruise
	} from '#lib/api/routes.js';
	import { clearFailure, errorMessage, noteFailure, settle } from '#lib/api/errors.js';
	import type { LegSignUp } from '#lib/api/types.js';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import CruiseEnrollmentAdmin from './CruiseEnrollmentAdmin.svelte';
	import PriceCoverage from '#lib/components/PriceCoverage.svelte';
	import { formatPrice, yourPrice } from '#lib/pricing.js';
	import LegSignUpControl from '#lib/components/LegSignUpControl.svelte';
	import CruiseLogistics from '#lib/components/CruiseLogistics.svelte';
	import { freePlacesLabel, numberLegs, signUpDeadlineLabel } from '#lib/enrollment.js';
	import { legStatus, legStatusBadges, legStatusLabels } from '#lib/rejsy.js';
	import HiddenBadge from '#lib/components/HiddenBadge.svelte';
	import { canSeeHidden, isHidden, publishLegsQuestion } from '#lib/publication.js';
	import { perLoad } from '#lib/state.svelte.js';
	import { onDestroy } from 'svelte';
	import { goto } from '$app/navigation';

	// data comes from +page.ts, which reruns when the URL's cruise changes, so
	// navigating between two cruises swaps every value derived from it.
	let { data } = $props();

	// The cruise and its legs change in place after a publish or hide, and
	// reset to the loaded value on navigation.
	let cruise = $derived(data.cruise);
	const series = $derived(data.series);
	let trips = $derived(data.trips);
	let voyages = $derived(data.voyages);
	// Sign-ups reset to the loaded value on navigation.
	let signUps = $derived(data.signUps);
	let loadErrors = $derived(data.loadErrors);
	const id = $derived(data.cruise.id);
	const isAdmin = $derived(auth.isAdmin);
	const showHidden = $derived(canSeeHidden(auth));

	const legs = $derived(numberLegs(trips));
	const signUpByTrip = $derived(new Map(signUps.map((s) => [s.trip_id, s])));
	function updateSignUp(next: LegSignUp) {
		signUps = signUps.map((s) => (s.trip_id === next.trip_id ? next : s));
	}

	let adminLoadErrors = $state<string[]>([]);
	let mounted = true;
	onDestroy(() => {
		mounted = false;
	});
	let deleteError = $state('');
	$effect(() => {
		if (id) deleteError = '';
	});
	let publication = $derived(perLoad(data, { busy: false, error: '' }));
	// Whether a leg takes sign-ups depends on its and the cruise's publication,
	// so the user's sign-up controls reload after either changes.
	async function refreshSignUps(held: typeof publication, cruiseId: number) {
		const errors = [...loadErrors];
		const label = 'twoje zapisy na etapy';
		const next = await settle(
			listCruiseLegSignUps(cruiseId).then((r) => {
				clearFailure(errors, label);
				return r ?? [];
			}),
			signUps,
			noteFailure(errors, label)
		);
		if (held !== publication) return;
		signUps = next;
		loadErrors = errors;
	}
	async function handlePublish() {
		const held = publication;
		const cruiseId = id;
		const hiddenLegs = trips.filter(isHidden).length + voyages.filter(isHidden).length;
		const includeLegs = hiddenLegs > 0 && confirm(publishLegsQuestion(hiddenLegs));
		held.busy = true;
		held.error = '';
		try {
			const next = await publishCruise(cruiseId, includeLegs);
			if (held !== publication) return;
			cruise = next;
			if (includeLegs) {
				// The API stamped the hidden legs in the same transaction; the
				// cruise's time stands in for theirs until the next load.
				const at = next.published_at;
				trips = trips.map((t) => (isHidden(t) ? { ...t, published_at: at } : t));
				voyages = voyages.map((v) => (isHidden(v) ? { ...v, published_at: at } : v));
			}
			await refreshSignUps(held, cruiseId);
		} catch (err) {
			held.error = `Nie udało się opublikować wydarzenia: ${errorMessage(err)}`;
		} finally {
			held.busy = false;
		}
	}
	async function handleUnpublish() {
		const held = publication;
		const cruiseId = id;
		held.busy = true;
		held.error = '';
		try {
			const next = await unpublishCruise(cruiseId);
			if (held !== publication) return;
			cruise = next;
			await refreshSignUps(held, cruiseId);
		} catch (err) {
			held.error = `Nie udało się ukryć wydarzenia: ${errorMessage(err)}`;
		} finally {
			held.busy = false;
		}
	}

	async function handleDelete() {
		if (trips.length > 0 || voyages.length > 0) {
			deleteError = 'Nie można usunąć wydarzenia, które ma etapy. Najpierw usuń jego rejsy.';
			return;
		}
		if (!confirm('Usunąć wydarzenie?')) return;
		const cruiseId = id;
		deleteError = '';
		try {
			await deleteCruise(cruiseId);
			if (!mounted || cruiseId !== id) return;
			void goto('/cruises');
		} catch (err) {
			if (!mounted || cruiseId !== id) return;
			deleteError = err instanceof Error ? err.message : 'Nie udało się usunąć wydarzenia';
		}
	}
</script>

<div class="mx-auto max-w-5xl">
	<LoadErrorBanner errors={[...loadErrors, ...adminLoadErrors]} />
	<div
		class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3"
		data-testid="cruise-header"
	>
		<div>
			<div class="flex flex-wrap items-center gap-3">
				<h1 class="text-3xl font-bold text-[var(--navy)]">{cruise.name}</h1>
				{#if showHidden && isHidden(cruise)}<HiddenBadge />{/if}
			</div>
			{#if series}
				<p class="mt-1 text-sm">
					<a href="/series/{series.id}" class="text-[var(--ocean)] hover:underline">{series.name}</a
					>
					{#if cruise.edition}
						· edycja {cruise.edition}{/if}
				</p>
			{/if}
			{#if cruise.countries}
				<p class="mt-1 text-[var(--text-muted)]">{cruise.countries}</p>
			{/if}
		</div>
		{#if isAdmin}
			<div class="flex gap-2">
				{#if isHidden(cruise)}
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
	{#if publication.error}
		<div role="alert" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">
			{publication.error}
		</div>
	{/if}

	{#if cruise.description}
		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Opis</h2>
			<p class="whitespace-pre-wrap text-sm">{cruise.description}</p>
		</div>
	{/if}

	{#if cruise.included?.length || cruise.not_included?.length}
		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Co obejmuje cena etapu</h2>
			<PriceCoverage included={cruise.included} notIncluded={cruise.not_included} />
		</div>
	{/if}

	{#if cruise.travel_notes || cruise.min_age != null}
		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Informacje dla załogi</h2>
			<CruiseLogistics travelNotes={cruise.travel_notes} minAge={cruise.min_age} />
		</div>
	{/if}

	<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
		<div class="mb-3 flex items-center justify-between">
			<h2 class="font-semibold text-[var(--navy)]">Nadchodzące etapy ({trips.length})</h2>
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
					{@const status = legStatus(trip)}
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
								{#if showHidden && isHidden(trip)}<HiddenBadge />{/if}
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

	<CruiseEnrollmentAdmin {cruise} {trips} bind:adminLoadErrors />
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
							<span class="flex flex-wrap items-center gap-2">
								<span class="font-medium">{voyage.name}</span>
								{#if showHidden && isHidden(voyage)}<HiddenBadge />{/if}
							</span>
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
