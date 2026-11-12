<script lang="ts">
	import { resolveEnroll, enroll, enrollLegs } from '#lib/api/routes.js';
	import type { Enrollment, EnrollPageData } from '#lib/api/types.js';
	import { freePlaces, numberLegs, statusLabels } from '#lib/enrollment.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import CruiseLogistics from '#lib/components/CruiseLogistics.svelte';
	import { page } from '$app/state';
	import { onMount } from 'svelte';

	let data = $state<EnrollPageData | null>(null);
	let loading = $state(true);
	let error = $state('');
	let note = $state('');
	let submitting = $state(false);
	let success = $state(false);
	let selectedLegs = $state<number[]>([]);

	// Cruise legs (trips) numbered by embark date, each with the caller's
	// enrollment for it, if any.
	const legs = $derived.by(() => {
		if (data?.kind !== 'cruise' || !data.trips) return [];
		const byTrip = new Map<number, Enrollment>();
		for (const e of data.leg_enrollments ?? []) {
			if (e.trip_id != null) byTrip.set(e.trip_id, e);
		}
		return numberLegs(data.trips).map((l) => ({ ...l, enrollment: byTrip.get(l.trip.id) }));
	});
	const pickableLegs = $derived(
		legs.filter((l) => l.trip.status === 'planned' && !l.enrollment)
	);
	// A whole-cruise sign-up (no legs picked) is offered only to callers with
	// no enrollment of either kind yet.
	const canEnrollCruise = $derived(
		data?.kind === 'cruise' && !data.enrollment && (data.leg_enrollments ?? []).length === 0
	);
	const showForm = $derived(
		data?.kind === 'trip' ? !data.enrolled : canEnrollCruise || pickableLegs.length > 0
	);

	const token = $derived((page.params as Record<string, string>).token);
	const tripAccepted = $derived(
		new Map((data?.trip_accepted ?? []).map((t) => [t.trip_id, t.accepted_count]))
	);
	function placesLeft(tripId: number, maxCrew: number | null | undefined): number | null {
		const accepted = tripAccepted.get(tripId);
		return accepted === undefined ? null : freePlaces(maxCrew, accepted);
	}
	const tripFree = $derived(
		data?.kind === 'trip'
			? placesLeft(data.trip.id, data.trip.max_crew)
			: null
	);
	const loginHref = $derived(`/login?redirect=${encodeURIComponent(`/enroll/${token}`)}`);

	onMount(async () => {
		try {
			data = await resolveEnroll(token);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nieprawidłowy link zapisu';
		} finally {
			loading = false;
		}
	});

	async function handleEnroll(e: Event) {
		e.preventDefault();
		submitting = true;
		error = '';
		try {
			if (selectedLegs.length > 0) {
				await enrollLegs(token, selectedLegs, note || undefined);
			} else {
				await enroll(token, note || undefined);
			}
			success = true;
			selectedLegs = [];
			data = await resolveEnroll(token);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się zapisać';
		} finally {
			submitting = false;
		}
	}
</script>

<div class="min-h-screen bg-[var(--surface)] px-4 py-12">
{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if error && !data}
	<div class="mx-auto max-w-lg text-center">
		<div class="rounded-2xl bg-white p-8 shadow-sm">
			<h1 class="mb-2 text-2xl font-bold text-red-600">Nieprawidłowy link</h1>
			<p class="text-[var(--text-muted)]">{error}</p>
		</div>
	</div>
{:else if data}
	<div class="mx-auto max-w-lg">
		<div class="rounded-2xl bg-white p-8 shadow-sm">
			{#if data.kind === 'trip'}
				<p class="mb-1 text-xs uppercase tracking-wide text-[var(--text-muted)]">Rejs</p>
				<h1 class="mb-1 text-2xl font-bold text-[var(--navy)]">{data.trip.name}</h1>
				<p class="mb-4 text-sm text-[var(--text-muted)]">
					{#if data.trip.start_port && data.trip.end_port}
						{data.trip.start_port} → {data.trip.end_port}
					{/if}
					{#if data.trip.countries} · {data.trip.countries}{/if}
				</p>

				{#if data.trip.embark_date}
					<div class="mb-4 text-sm">
						<span class="text-[var(--text-muted)]">Daty:</span>
						{data.trip.embark_date} – {data.trip.disembark_date ?? '?'}
					</div>
				{/if}

				{#if data.trip.captain_name}
					<div class="mb-4 text-sm">
						<span class="text-[var(--text-muted)]">Kapitan:</span>
						{data.trip.captain_name}
					</div>
				{/if}

				{#if data.trip.description}
					<p class="mb-4 whitespace-pre-wrap text-sm">{data.trip.description}</p>
				{/if}

				<div class="mb-6 flex gap-4 text-sm">
					<div class="rounded-lg bg-gray-50 px-3 py-2">
						<span class="text-[var(--text-muted)]">Zapisanych:</span>
						<span class="font-semibold">{data.total_count}</span>
						{#if data.trip.max_crew}
							<span class="text-[var(--text-muted)]">/ {data.trip.max_crew}</span>
						{/if}
					</div>
					<div class="rounded-lg bg-gray-50 px-3 py-2">
						<span class="text-[var(--text-muted)]">Zaakceptowanych:</span>
						<span class="font-semibold">{data.accepted_count}</span>
					</div>
					{#if tripFree !== null}
						<div class="rounded-lg bg-gray-50 px-3 py-2" data-testid="free-places">
							<span class="text-[var(--text-muted)]">Wolne miejsca:</span>
							<span class="font-semibold">{tripFree}</span>
						</div>
					{/if}
				</div>
			{:else if data.kind === 'cruise'}
				<p class="mb-1 text-xs uppercase tracking-wide text-[var(--text-muted)]">Wydarzenie klubu</p>
				<h1 class="mb-1 text-2xl font-bold text-[var(--navy)]">{data.cruise.name}</h1>
				<p class="mb-4 text-sm text-[var(--text-muted)]">
					{#if data.cruise.start_port && data.cruise.end_port}
						{data.cruise.start_port} → {data.cruise.end_port}
					{/if}
					{#if data.cruise.countries} · {data.cruise.countries}{/if}
				</p>

				{#if data.cruise.embark_date}
					<div class="mb-4 text-sm">
						<span class="text-[var(--text-muted)]">Daty:</span>
						{data.cruise.embark_date} – {data.cruise.disembark_date ?? '?'}
					</div>
				{/if}

				{#if data.cruise.description}
					<p class="mb-4 whitespace-pre-wrap text-sm">{data.cruise.description}</p>
				{/if}

				<CruiseLogistics
					class="mb-4"
					groupChatUrl={data.cruise.group_chat_url}
					travelNotes={data.cruise.travel_notes}
					minAge={data.cruise.min_age}
				/>

				{#if legs.length > 0}
					<fieldset class="mb-4">
						<legend class="mb-2 text-xs uppercase tracking-wide text-[var(--text-muted)]">Etapy</legend>
						<div class="space-y-1">
							{#each legs as leg (leg.trip.id)}
								{@const cancelled = leg.trip.status === 'cancelled'}
								{@const free = placesLeft(leg.trip.id, leg.trip.max_crew)}
								<label
									class="flex items-start gap-2 rounded-lg bg-gray-50 px-3 py-2 text-sm"
									class:opacity-60={cancelled}
								>
									{#if auth.isAuthenticated && !leg.enrollment && !cancelled && showForm && !success}
										<input
											type="checkbox"
											class="mt-1"
											value={leg.trip.id}
											bind:group={selectedLegs}
											aria-label={`Etap ${leg.number}: ${leg.trip.name}`}
										/>
									{/if}
									<span class="flex-1">
										<span class="font-medium">Etap {leg.number}: {leg.trip.name}</span>
										{#if leg.trip.embark_date}
											<span class="block text-xs text-[var(--text-muted)]">
												{leg.trip.embark_date} – {leg.trip.disembark_date ?? '?'}
												{#if leg.trip.start_port && leg.trip.end_port}
													· {leg.trip.start_port} → {leg.trip.end_port}
												{/if}
											</span>
										{/if}
										{#if leg.trip.captain_name}
											<span class="block text-xs text-[var(--text-muted)]">kpt. {leg.trip.captain_name}</span>
										{/if}
										{#if free !== null && !cancelled}
											<span class="block text-xs text-[var(--text-muted)]" data-testid="free-places">wolne miejsca: {free}</span>
										{/if}
									</span>
									{#if cancelled}
										<span class="text-xs font-medium text-red-600">odwołany — niedostępny</span>
									{:else if leg.enrollment}
										<span class="text-xs font-medium text-blue-700">
											zapisany: {statusLabels[leg.enrollment.status] ?? leg.enrollment.status}
										</span>
									{/if}
								</label>
							{/each}
						</div>
						{#if canEnrollCruise}
							<p class="mt-2 text-xs text-[var(--text-muted)]">
								Zaznacz etapy, na które chcesz się zapisać. Bez zaznaczenia zgłosisz się na całe
								wydarzenie, a organizator przydzieli Cię do jachtu.
							</p>
						{/if}
					</fieldset>
				{/if}

				<div class="mb-6 flex gap-4 text-sm">
					<div class="rounded-lg bg-gray-50 px-3 py-2">
						<span class="text-[var(--text-muted)]">Zapisanych:</span>
						<span class="font-semibold">{data.total_count}</span>
						{#if data.cruise.max_crew}
							<span class="text-[var(--text-muted)]">/ {data.cruise.max_crew}</span>
						{/if}
					</div>
					<div class="rounded-lg bg-gray-50 px-3 py-2">
						<span class="text-[var(--text-muted)]">Zaakceptowanych:</span>
						<span class="font-semibold">{data.accepted_count}</span>
					</div>
				</div>
			{/if}

			{#if error}
				<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
			{/if}

			{#if success}
				<div class="rounded-lg bg-green-50 p-4 text-center text-sm text-green-700">
					Zostałeś zapisany! Organizator rozpatrzy Twoje zgłoszenie.
				</div>
			{:else if data.enrolled && !showForm}
				<div class="rounded-lg bg-blue-50 p-4 text-center text-sm text-blue-700">
					Jesteś już zapisany.
					{#if data.enrollment}
						Status: <span class="font-semibold">{statusLabels[data.enrollment.status] ?? data.enrollment.status}</span>
					{/if}
				</div>
			{:else if !auth.isAuthenticated}
				<div class="rounded-lg bg-gray-50 p-4 text-center">
					<p class="mb-3 text-sm text-[var(--text-muted)]">
						Zaloguj się lub załóż konto, aby dołączyć do tego rejsu.
					</p>
					<a
						href={loginHref}
						class="inline-block rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)]"
					>
						Zaloguj się, aby dołączyć
					</a>
				</div>
			{:else}
				<form onsubmit={handleEnroll} class="space-y-4">
					{#if data.enrollment}
						<p class="rounded-lg bg-blue-50 p-3 text-sm text-blue-700">
							Jesteś zapisany na wydarzenie. Status:
							<span class="font-semibold">{statusLabels[data.enrollment.status] ?? data.enrollment.status}</span>
						</p>
					{/if}
					<div>
						<label for="note" class="mb-1 block text-sm font-medium">Notatka (opcjonalnie)</label>
						<textarea
							id="note"
							bind:value={note}
							rows="3"
							placeholder="Wiadomość dla organizatora..."
							class="w-full rounded-lg border px-3 py-2 text-sm"
						></textarea>
					</div>
					<button
						type="submit"
						disabled={submitting || (!canEnrollCruise && data.kind === 'cruise' && selectedLegs.length === 0)}
						class="w-full rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
					>
						{submitting
							? 'Zapisywanie...'
							: selectedLegs.length > 0
								? `Zapisz się na wybrane etapy (${selectedLegs.length})`
								: 'Zapisz się'}
					</button>
				</form>
			{/if}
		</div>
	</div>
{/if}
</div>
