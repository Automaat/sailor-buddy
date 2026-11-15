<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { patentSummary } from '#lib/profile.js';
	import {
		listCruiseEnrollments,
		listCruiseTripEnrollments,
		generateCruiseEnrollToken,
		clearCruiseEnrollToken,
		updateCruiseEnrollmentStatus,
		assignCruiseEnrollmentTrip,
		deleteCruiseEnrollment
	} from '#lib/api/routes.js';
	import type { Cruise, Trip, CruiseEnrollment, TripEnrollment } from '#lib/api/types.js';
	import { ApiError } from '#lib/api/client.js';
	import { noteFailure, settle } from '#lib/api/errors.js';
	import { numberLegs, statusLabels } from '#lib/enrollment.js';
	let {
		cruise,
		trips,
		adminLoadErrors = $bindable([])
	}: { cruise: Cruise; trips: Trip[]; adminLoadErrors?: string[] } = $props();
	const id = $derived(cruise.id);
	const isAdmin = $derived(auth.isAdmin);
	type EnrollStatus = 'accepted' | 'rejected' | 'waitlisted' | 'pending';
	let enrollToken = $derived(cruise.enroll_token ?? null);
	let enrollments = $state<CruiseEnrollment[]>([]);
	let legEnrollments = $state<TripEnrollment[]>([]);
	// togglingFor is the cruise whose enroll-link request is in flight; the
	// toggle is disabled only on that cruise, not on one a link moved to.
	let togglingFor = $state<number | null>(null);
	const togglingEnroll = $derived(togglingFor === cruise.id);

	const legs = $derived(numberLegs(trips));
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

	// A request started on one cruise may answer after a link has moved the
	// page to another; its reply (or error) then belongs to a page the user
	// has left and is dropped.
	function leftCruise(cruiseId: number) {
		return cruiseId !== id;
	}

	async function reloadEnrollments(cruiseId: number) {
		const list = (await listCruiseEnrollments(cruiseId)) ?? [];
		if (!leftCruise(cruiseId)) enrollments = list;
	}

	// Enrollment lists are admin-only, and auth.isAdmin turns true only once
	// /auth/me answers, so they load here, keyed on the cruise id, instead of
	// in +page.ts. A reply for a cruise the page has left is dropped.
	$effect(() => {
		const cruiseId = id;
		enrollments = [];
		legEnrollments = [];
		adminLoadErrors = [];
		enrollmentError = '';
		if (!isAdmin) return;
		let current = true;
		const errors: string[] = [];
		void Promise.all([
			settle(
				listCruiseEnrollments(cruiseId).then((r) => r ?? []),
				[],
				noteFailure(errors, 'zgłoszenia na wydarzenie')
			),
			settle(
				listCruiseTripEnrollments(cruiseId).then((r) => r ?? []),
				[],
				noteFailure(errors, 'zapisy na etapy')
			)
		]).then(([e, te]) => {
			if (!current) return;
			enrollments = e;
			legEnrollments = te;
			adminLoadErrors = errors;
		});
		return () => {
			current = false;
		};
	});

	let enrollmentError = $state('');
	// A 409 from a cruise enrollment change means the leg has no free place.
	function enrollmentFailure(err: unknown, fallback: string) {
		enrollmentError =
			err instanceof ApiError && err.status === 409
				? 'Etap jest pełny albo zapis się zmienił. Odśwież stronę i zwolnij miejsce, zanim zaakceptujesz kolejną osobę.'
				: fallback;
	}

	async function toggleEnrollment() {
		const cruiseId = id;
		togglingFor = cruiseId;
		try {
			if (enrollToken) {
				await clearCruiseEnrollToken(cruiseId);
				if (!leftCruise(cruiseId)) enrollToken = null;
			} else {
				const res = await generateCruiseEnrollToken(cruiseId);
				if (!leftCruise(cruiseId)) enrollToken = res.token;
			}
		} catch (err) {
			console.error('Failed to toggle enrollment:', err);
		} finally {
			if (togglingFor === cruiseId) togglingFor = null;
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

	async function setEnrollmentStatus(enrollmentId: number, status: EnrollStatus) {
		const cruiseId = id;
		enrollmentError = '';
		try {
			await updateCruiseEnrollmentStatus(cruiseId, enrollmentId, status);
			if (leftCruise(cruiseId)) return;
			enrollments = enrollments.map((e) => (e.id === enrollmentId ? { ...e, status } : e));
		} catch (err) {
			if (!leftCruise(cruiseId)) enrollmentFailure(err, 'Nie udało się zmienić statusu.');
			console.error('Failed to set status:', err);
		}
	}

	async function assignEnrollmentToTrip(enrollmentId: number, tripId: number | null) {
		const cruiseId = id;
		enrollmentError = '';
		try {
			await assignCruiseEnrollmentTrip(cruiseId, enrollmentId, tripId ?? undefined);
			await reloadEnrollments(cruiseId);
		} catch (err) {
			if (!leftCruise(cruiseId)) enrollmentFailure(err, 'Nie udało się przypisać zapisu do etapu.');
			console.error('Failed to assign to trip:', err);
		}
	}

	async function deleteEnrollment(enrollmentId: number) {
		if (!confirm('Usunąć ten zapis?')) return;
		const cruiseId = id;
		try {
			await deleteCruiseEnrollment(cruiseId, enrollmentId);
			if (leftCruise(cruiseId)) return;
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
					<a href="tel:{enrollment.user_phone}" class="text-sm text-[var(--ocean)] hover:underline"
						>{enrollment.user_phone}</a
					>
				{/if}
				<span class="text-xs text-[var(--text-muted)]">
					{patentSummary(enrollment.user_patent_type, enrollment.user_patent_number)}
				</span>
				<span
					class="rounded-full px-2 py-0.5 text-xs font-medium {statusColors[enrollment.status] ??
						'bg-gray-100'}"
				>
					{statusLabels[enrollment.status] ?? enrollment.status}
				</span>
				<span class="text-xs text-[var(--text-muted)]">zapis na wydarzenie</span>
			</div>
			<div class="flex flex-wrap gap-1">
				{#if enrollment.status !== 'accepted'}
					<button
						onclick={() => setEnrollmentStatus(enrollment.id, 'accepted')}
						class="rounded px-2 py-1 text-xs text-[var(--ocean)] hover:bg-[var(--ocean-tint)]"
					>
						Akceptuj
					</button>
				{/if}
				{#if enrollment.status !== 'waitlisted'}
					<button
						onclick={() => setEnrollmentStatus(enrollment.id, 'waitlisted')}
						class="rounded px-2 py-1 text-xs text-[var(--navy)] hover:bg-[var(--ocean-tint)]"
					>
						Rezerwa
					</button>
				{/if}
				{#if enrollment.status !== 'rejected'}
					<button
						onclick={() => setEnrollmentStatus(enrollment.id, 'rejected')}
						class="rounded px-2 py-1 text-xs text-red-700 hover:bg-red-100"
					>
						Odrzuć
					</button>
				{/if}
				<button
					onclick={() => deleteEnrollment(enrollment.id)}
					class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-100"
				>
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
					void assignEnrollmentToTrip(enrollment.id, v ? Number(v) : null);
				}}
				class="min-w-0 max-w-full rounded border px-2 py-1 text-sm"
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
				<button
					onclick={copyEnrollLink}
					class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50"
				>
					Kopiuj link
				</button>
			{/if}
			<span class="text-xs text-[var(--text-muted)]"
				>Link do podglądu wydarzenia, z którego można zapisać się na etapy.</span
			>
		</div>

		{#if enrollmentError}
			<p role="alert" class="mb-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
				{enrollmentError}
			</p>
		{/if}
		{#if legGroups.length === 0 && unassigned.length === 0}
			<p class="text-sm text-[var(--text-muted)]">Brak zapisów.</p>
		{/if}
		<div class="space-y-4">
			{#each legGroups as group (group.trip.id)}
				<section aria-label={`Zapisy: ${group.trip.name}`}>
					<div class="mb-1 flex items-center justify-between gap-2">
						<h3 class="min-w-0 text-sm font-medium [overflow-wrap:anywhere]">{group.trip.name}</h3>
						<a
							href="/trips/{group.trip.id}/enrollments"
							class="tap-target shrink-0 whitespace-nowrap text-xs text-[var(--ocean)] hover:underline"
						>
							Zarządzaj zapisami
						</a>
					</div>
					{#if group.signUps.length === 0 && group.legacy.length === 0}
						<p class="text-xs text-[var(--text-muted)]">Brak zapisów.</p>
					{/if}
					<div class="space-y-1">
						{#each group.signUps as e (e.id)}
							<div
								class="flex flex-wrap items-center gap-2 rounded-lg bg-gray-50 px-3 py-2 text-sm [overflow-wrap:anywhere] [&>*]:min-w-0"
							>
								<span class="font-medium">{e.person_name || e.email}</span>
								{#if e.user_id == null}
									<span class="text-xs text-[var(--text-muted)]">bez konta</span>
								{/if}
								<span
									class="rounded-full px-2 py-0.5 text-xs font-medium {statusColors[e.status] ??
										'bg-gray-100'}"
								>
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
					<p class="mb-1 text-xs text-[var(--text-muted)]">
						Zapisy na całe wydarzenie sprzed zapisów na etapy. Przypisz je do etapu.
					</p>
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
