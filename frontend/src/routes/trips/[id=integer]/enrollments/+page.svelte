<script lang="ts">
	import { formatTimestampDate } from '#lib/dates.js';
	import {
		getTrip,
		getTripSignUp,
		listTripEnrollments,
		updateTripEnrollmentStatus,
		updateTripEnrollmentPaymentStatus,
		deleteTripEnrollment
	} from '#lib/api/routes.js';
	import { ApiError } from '#lib/api/client.js';
	import { noteFailure, settle } from '#lib/api/errors.js';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import type { TripEnrollment, Trip } from '#lib/api/types.js';
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
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	type EnrollStatus = 'accepted' | 'rejected' | 'waitlisted' | 'pending';

	let trip = $state<Trip | null>(null);
	let enrollments = $state<TripEnrollment[]>([]);
	let loading = $state(true);
	let error = $state('');
	let loadErrors = $state<string[]>([]);
	// Accepted crew the list below does not show: cruise enrollments assigned
	// to this leg. They take places too, so free places add them back.
	let otherAccepted = $state(0);
	let paymentError = $state('');
	// Enrollment ids with a payment status save in flight; their select is
	// disabled so two quick changes cannot race.
	let savingPayment = $state<number[]>([]);

	const id = $derived(Number(page.params.id));
	const payments = $derived(paymentCounts(enrollments));

	// Managing enrollments is an admin task; redirect regular members away.
	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			goto(`/trips/${id}`);
		}
	});

	onMount(async () => {
		try {
			// Without the leg's accepted count the list still renders, but free
			// places miss cruise-level crew, so the banner flags the count.
			const [t, e, s] = await Promise.all([
				getTrip(id),
				listTripEnrollments(id),
				settle(getTripSignUp(id), null, noteFailure(loadErrors, 'liczba zajętych miejsc'))
			]);
			trip = t;
			enrollments = e ?? [];
			otherAccepted = s ? Math.max(0, s.accepted_count - countAccepted(enrollments)) : 0;
		} catch (err) {
			console.error('Failed to load enrollments:', err);
		} finally {
			loading = false;
		}
	});

	function countAccepted(list: TripEnrollment[]) {
		return list.filter((e) => e.status === 'accepted').length;
	}

	const accepted = $derived(countAccepted(enrollments) + otherAccepted);
	const places = $derived(trip ? freePlacesLabel(trip.max_crew, accepted) : null);
	// With a place free, the first person on the waiting list is next in line.
	const nextId = $derived(trip ? nextWaitlisted(enrollments, trip.max_crew, accepted) : null);

	async function updateStatus(enrollmentId: number, status: EnrollStatus) {
		error = '';
		try {
			await updateTripEnrollmentStatus(id, enrollmentId, status);
			enrollments = enrollments.map((e) => (e.id === enrollmentId ? { ...e, status } : e));
		} catch (err) {
			if (err instanceof ApiError && err.status === 409) {
				error = 'Etap jest pełny: wszystkie miejsca są zajęte. Zwolnij miejsce, zanim zaakceptujesz kolejną osobę.';
			} else {
				error = 'Nie udało się zmienić statusu.';
			}
			console.error('Failed to update status:', err);
		}
	}

	async function updatePayment(enrollmentId: number, select: HTMLSelectElement) {
		const paymentStatus = select.value as PaymentStatus;
		const previous = enrollments.find((e) => e.id === enrollmentId)?.payment_status ?? 'unpaid';
		paymentError = '';
		savingPayment = [...savingPayment, enrollmentId];
		try {
			const updated = await updateTripEnrollmentPaymentStatus(id, enrollmentId, paymentStatus);
			enrollments = enrollments.map((e) =>
				e.id === enrollmentId ? { ...e, payment_status: updated.payment_status ?? paymentStatus } : e
			);
		} catch (err) {
			console.error('Failed to update payment status:', err);
			select.value = previous;
			paymentError = 'Nie udało się zapisać statusu płatności';
		} finally {
			savingPayment = savingPayment.filter((x) => x !== enrollmentId);
		}
	}

	async function deleteEnrollment(enrollmentId: number) {
		if (!confirm('Usunąć ten zapis?')) return;
		try {
			await deleteTripEnrollment(id, enrollmentId);
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

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else}
	<div class="mx-auto max-w-4xl">
		<LoadErrorBanner errors={loadErrors} />
		<div class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3">
			<div>
				<h1 class="text-3xl font-bold text-[var(--navy)]">Zapisy</h1>
				{#if trip}
					<p class="mt-1 text-[var(--text-muted)]">
						{trip.name}
						{#if trip.max_crew}
							· {accepted} / {trip.max_crew} zaakceptowanych · <span data-testid="free-places">{places}</span>
						{/if}
					</p>
				{/if}
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
				{#each enrollments as enrollment}
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
{/if}
