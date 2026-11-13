<script lang="ts">
	import { resolveEnroll, enroll, enrollLegs } from '#lib/api/routes.js';
	import type { Enrollment, EnrollPageData } from '#lib/api/types.js';
	import {
		freePlaces,
		numberLegs,
		signUpClosedByDeadline,
		signUpDeadlineLabel,
		statusLabels
	} from '#lib/enrollment.js';
	import { warsawToday } from '#lib/rejsy.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import { patentSummary } from '#lib/profile.js';
	import PriceInfo from '#lib/components/PriceInfo.svelte';
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
	const today = warsawToday();
	const pickableLegs = $derived(
		legs.filter(
			(l) => l.trip.status === 'planned' && !l.enrollment && !signUpClosedByDeadline(l.trip.sign_up_deadline, today)
		)
	);
	// A trip link past its sign-up deadline shows "Zapisy zamknięte" instead
	// of the form; cruise legs past theirs are simply not pickable.
	const tripClosed = $derived(
		data?.kind === 'trip' && !data.enrolled && signUpClosedByDeadline(data.trip.sign_up_deadline, today)
	);
	const showForm = $derived(
		data?.kind === 'trip' ? !data.enrolled && !tripClosed : pickableLegs.length > 0
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
			if (data?.kind === 'cruise') {
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

				{#if data.trip.sign_up_deadline}
					<div class="mb-4 text-sm" data-testid="sign-up-deadline">
						{signUpDeadlineLabel(data.trip.sign_up_deadline)}
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

				<div class="mb-4">
					<PriceInfo priced={data.trip} membership={auth.user?.membership} />
				</div>

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

				<div class="mb-4">
					<PriceInfo
						priced={data.cruise}
						membership={auth.user?.membership}
						included={data.cruise.included}
						notIncluded={data.cruise.not_included}
					/>
				</div>
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
								{@const closed = !cancelled && signUpClosedByDeadline(leg.trip.sign_up_deadline, today)}
								{@const free = placesLeft(leg.trip.id, leg.trip.max_crew)}
								<label
									class="flex items-start gap-2 rounded-lg bg-gray-50 px-3 py-2 text-sm"
									class:opacity-60={cancelled}
								>
									{#if auth.isAuthenticated && !leg.enrollment && !cancelled && !closed && showForm && !success}
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
										{#if leg.trip.sign_up_deadline && !cancelled}
											<span class="block text-xs text-[var(--text-muted)]" data-testid="sign-up-deadline">
												{signUpDeadlineLabel(leg.trip.sign_up_deadline)}
											</span>
										{/if}
									</span>
									{#if cancelled}
										<span class="text-xs font-medium text-[var(--text-muted)]">odwołany — niedostępny</span>
									{:else if leg.enrollment}
										<span class="text-xs font-medium text-[var(--ocean)]">
											zapisany: {statusLabels[leg.enrollment.status] ?? leg.enrollment.status}
										</span>
									{:else if closed}
										<span class="text-xs font-medium text-[var(--text-muted)]">Zapisy zamknięte</span>
									{/if}
								</label>
							{/each}
						</div>
						{#if auth.isAuthenticated && showForm && !success}
							<p class="mt-2 text-xs text-[var(--text-muted)]">Zaznacz etapy, na które chcesz się zapisać.</p>
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
				<div class="rounded-lg bg-[var(--ocean-tint)] text-[var(--ocean)] p-4 text-center text-sm">
					Zostałeś zapisany! Organizator rozpatrzy Twoje zgłoszenie.
				</div>
			{:else if tripClosed}
				<div class="rounded-lg bg-gray-50 p-4 text-center text-sm text-[var(--text-muted)]" data-testid="sign-up-closed">
					Zapisy zamknięte
				</div>
			{:else if data.enrolled && !showForm}
				<div class="rounded-lg bg-[var(--ocean-tint)] text-[var(--ocean)] p-4 text-center text-sm">
					Jesteś już zapisany.
					{#if data.enrollment}
						Status: <span class="font-semibold">{statusLabels[data.enrollment.status] ?? data.enrollment.status}</span>
					{/if}
				</div>
			{:else if !auth.isAuthenticated}
				<div class="rounded-lg bg-gray-50 p-4 text-center">
					<p class="mb-3 text-sm text-[var(--text-muted)]">
						Zaloguj się lub załóż konto, aby się zapisać. Wrócisz na tę stronę.
					</p>
					<a
						href={loginHref}
						class="inline-block rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)]"
					>
						Zapisz się
					</a>
				</div>
			{:else}
				<form onsubmit={handleEnroll} class="space-y-4">
					{#if auth.user}
						<div class="rounded-lg bg-gray-50 p-3 text-sm" data-testid="enroll-as">
							<div class="flex items-start justify-between gap-2">
								<div>
									<span class="text-[var(--text-muted)]">Zapisujesz się jako:</span>
									<span class="block font-medium">{auth.user.name}</span>
									<span class="block text-xs text-[var(--text-muted)]">
										{auth.user.phone ?? 'brak telefonu'} · {patentSummary(auth.user.patent_type, auth.user.patent_number)}
									</span>
								</div>
								<a href="/profile" class="text-xs font-medium text-[var(--ocean)] hover:underline">Edytuj</a>
							</div>
						</div>
					{/if}
					{#if data.enrollment}
						<p class="rounded-lg bg-[var(--ocean-tint)] text-[var(--ocean)] p-3 text-sm">
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
						disabled={submitting || (data.kind === 'cruise' && selectedLegs.length === 0)}
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
