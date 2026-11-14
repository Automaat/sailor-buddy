<script lang="ts">
	import { formatTimestampDate } from '#lib/dates.js';
	import {
		updateTripEnrollmentStatus,
		updateTripEnrollmentPaymentStatus,
		deleteTripEnrollment
	} from '#lib/api/routes.js';
	import { ApiError } from '#lib/api/client.js';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import type { TripEnrollment } from '#lib/api/types.js';
	import {
		freePlacesLabel,
		nextWaitlisted,
		paymentCounts,
		paymentLabels,
		paymentStatuses,
		statusLabels,
		type PaymentStatus
	} from '#lib/enrollment.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import { goto } from '$app/navigation';

	type EnrollStatus = 'accepted' | 'rejected' | 'waitlisted' | 'pending';

	// data comes from +page.ts, which reruns when the URL's trip changes, so
	// navigating between two trips swaps every value derived from it.
	let { data } = $props();

	const trip = $derived(data.trip);
	const loadErrors = $derived(data.loadErrors);
	// Accepted crew the list below does not show: cruise enrollments assigned
	// to this leg. They take places too, so free places add them back.
	const otherAccepted = $derived(data.otherAccepted);
	// Changes in place after a status, payment or delete, and resets to the
	// loaded list on navigation.
	let enrollments = $derived(data.enrollments);
	let error = $state('');
	let paymentError = $state('');
	// Enrollment ids with a payment status save in flight; their select is
	// disabled so two quick changes cannot race.
	let savingPayment = $state<number[]>([]);

	const id = $derived(data.trip.id);
	const payments = $derived(paymentCounts(enrollments));

	// Alerts belong to one trip: a link to another clears them.
	$effect(() => {
		void id;
		error = '';
		paymentError = '';
	});

	// Managing enrollments is an admin task; redirect regular members away.
	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			goto(`/trips/${id}`);
		}
	});

	// A request started on one trip may answer after a link has moved the
	// page to another; its reply then belongs to a page the user has left
	// and is dropped.
	function leftTrip(tripId: number) {
		return tripId !== id;
	}

	function countAccepted(list: TripEnrollment[]) {
		return list.filter((e) => e.status === 'accepted').length;
	}

	const accepted = $derived(countAccepted(enrollments) + otherAccepted);
	const places = $derived(freePlacesLabel(trip.max_crew, accepted));
	// With a place free, the first person on the waiting list is next in line.
	const nextId = $derived(nextWaitlisted(enrollments, trip.max_crew, accepted));

	async function updateStatus(enrollmentId: number, status: EnrollStatus) {
		const tripId = id;
		error = '';
		try {
			await updateTripEnrollmentStatus(tripId, enrollmentId, status);
			if (leftTrip(tripId)) return;
			enrollments = enrollments.map((e) => (e.id === enrollmentId ? { ...e, status } : e));
		} catch (err) {
			console.error('Failed to update status:', err);
			if (leftTrip(tripId)) return;
			if (err instanceof ApiError && err.status === 409) {
				error = 'Etap jest pełny: wszystkie miejsca są zajęte. Zwolnij miejsce, zanim zaakceptujesz kolejną osobę.';
			} else {
				error = 'Nie udało się zmienić statusu.';
			}
		}
	}

	async function updatePayment(enrollmentId: number, select: HTMLSelectElement) {
		const paymentStatus = select.value as PaymentStatus;
		const previous = enrollments.find((e) => e.id === enrollmentId)?.payment_status ?? 'unpaid';
		const tripId = id;
		paymentError = '';
		savingPayment = [...savingPayment, enrollmentId];
		try {
			const updated = await updateTripEnrollmentPaymentStatus(tripId, enrollmentId, paymentStatus);
			if (leftTrip(tripId)) return;
			enrollments = enrollments.map((e) =>
				e.id === enrollmentId ? { ...e, payment_status: updated.payment_status ?? paymentStatus } : e
			);
		} catch (err) {
			console.error('Failed to update payment status:', err);
			if (leftTrip(tripId)) return;
			select.value = previous;
			paymentError = 'Nie udało się zapisać statusu płatności';
		} finally {
			savingPayment = savingPayment.filter((x) => x !== enrollmentId);
		}
	}

	async function deleteEnrollment(enrollmentId: number) {
		if (!confirm('Usunąć ten zapis?')) return;
		const tripId = id;
		try {
			await deleteTripEnrollment(tripId, enrollmentId);
			if (leftTrip(tripId)) return;
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

<div class="mx-auto max-w-4xl">
	<LoadErrorBanner errors={loadErrors} />
	<div class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3">
		<div>
			<h1 class="text-3xl font-bold text-[var(--navy)]">Zapisy</h1>
			<p class="mt-1 text-[var(--text-muted)]">
				{trip.name}
				{#if trip.max_crew}
					· {accepted} / {trip.max_crew} zaakceptowanych · <span data-testid="free-places">{places}</span>
				{/if}
			</p>
		</div>
		<a href="/trips/{id}" class="tap-target rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">
			Wróć do rejsu
		</a>
	</div>

	{#if error}
		<p role="alert" class="mb-4 rounded-lg bg-red-50 px-4 py-2 text-sm text-red-700">{error}</p>
	{/if}
	{#if enrollments.length > 0}
		<div class="mb-4 flex flex-wrap gap-2 text-sm" data-testid="payment-counts">
			{#each paymentStatuses as ps}
				<span class="rounded-full bg-white px-3 py-1 shadow-sm" data-testid={`payment-count-${ps}`}>
					{paymentLabels[ps]}: <strong>{payments[ps]}</strong>
				</span>
			{/each}
		</div>
	{/if}
	{#if paymentError}
		<p role="alert" class="mb-4 text-sm text-red-600">{paymentError}</p>
	{/if}

	{#if enrollments.length === 0}
		<div class="rounded-2xl bg-white p-6 text-center text-sm text-[var(--text-muted)] shadow-sm">
			Brak zapisów.
		</div>
	{:else}
		<div class="space-y-3">
			<!-- Keyed so a link to another trip builds fresh rows: an unkeyed
			     select would keep a choice still saving on the previous trip. -->
			{#each enrollments as enrollment (enrollment.id)}
				<div class="rounded-xl bg-white p-4 shadow-sm">
					<div class="flex items-center justify-between max-md:flex-wrap max-md:gap-2">
						<div class="min-w-0 break-words">
							<span class="font-medium">{enrollment.user_name ?? 'Nieznany'}</span>
							<span class="ml-2 text-sm text-[var(--text-muted)]">{enrollment.user_email ?? ''}</span>
							<span class="ml-2 inline-block rounded-full px-2 py-0.5 text-xs font-medium {statusColors[enrollment.status] ?? 'bg-gray-100'}">
								{statusLabels[enrollment.status] ?? enrollment.status}
							</span>
							{#if enrollment.id === nextId}
								<span class="ml-2 inline-block rounded-full bg-[var(--ocean)] px-2 py-0.5 text-xs font-medium text-white" data-testid="next-waitlisted">
									następny w kolejce
								</span>
							{/if}
						</div>
						<div class="flex items-center gap-1 max-md:flex-wrap">
							<label class="sr-only" for={`payment-${enrollment.id}`}>
								Płatność: {enrollment.user_name ?? 'Nieznany'}
							</label>
							<select
								id={`payment-${enrollment.id}`}
								value={enrollment.payment_status ?? 'unpaid'}
								onchange={(e) => updatePayment(enrollment.id, e.currentTarget)}
								disabled={savingPayment.includes(enrollment.id)}
								class="rounded border px-2 py-1 text-xs"
							>
								{#each paymentStatuses as ps}
									<option value={ps}>{paymentLabels[ps]}</option>
								{/each}
							</select>
							{#if enrollment.status !== 'accepted'}
								<button onclick={() => updateStatus(enrollment.id, 'accepted')} class="rounded px-2 py-1 text-xs text-[var(--ocean)] hover:bg-[var(--ocean-tint)]">
									Akceptuj
								</button>
							{/if}
							{#if enrollment.status !== 'waitlisted'}
								<button onclick={() => updateStatus(enrollment.id, 'waitlisted')} class="rounded px-2 py-1 text-xs text-[var(--navy)] hover:bg-[var(--ocean-tint)]">
									Rezerwa
								</button>
							{/if}
							{#if enrollment.status !== 'rejected'}
								<button onclick={() => updateStatus(enrollment.id, 'rejected')} class="rounded px-2 py-1 text-xs text-red-700 hover:bg-red-50">
									Odrzuć
								</button>
							{/if}
							<button onclick={() => deleteEnrollment(enrollment.id)} class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-50">
								Usuń
							</button>
						</div>
					</div>
					{#if enrollment.note}
						<p class="mt-2 text-sm text-[var(--text-muted)]">{enrollment.note}</p>
					{/if}
					<p class="mt-1 text-xs text-[var(--text-muted)]">
						Zapisano: {formatTimestampDate(enrollment.created_at)}
					</p>
				</div>
			{/each}
		</div>
	{/if}
</div>
