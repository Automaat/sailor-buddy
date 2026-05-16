<script lang="ts">
	import { orgStore } from '$lib/stores/org.svelte';
	import {
		getCruise,
		deleteCruise,
		listCruiseTrips,
		listCruiseVoyages,
		listCruiseEnrollments,
		generateCruiseEnrollToken,
		clearCruiseEnrollToken,
		updateCruiseEnrollmentStatus,
		assignCruiseEnrollmentTrip,
		deleteCruiseEnrollment
	} from '$lib/api/routes';
	import type { Cruise, Trip, Voyage, CruiseEnrollment } from '$lib/api/types';
	import { statusLabels } from '$lib/enrollment';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	type EnrollStatus = 'accepted' | 'rejected' | 'waitlisted' | 'pending';

	let cruise = $state<Cruise | null>(null);
	let trips = $state<Trip[]>([]);
	let voyages = $state<Voyage[]>([]);
	let enrollments = $state<CruiseEnrollment[]>([]);
	let loading = $state(true);
	let enrollToken = $state<string | null>(null);
	let togglingEnroll = $state(false);

	const id = $derived(Number(page.params.id));
	const isAdmin = $derived(orgStore.isOrgAdmin);

	async function reloadEnrollments() {
		enrollments = (await listCruiseEnrollments(id)) ?? [];
	}

	onMount(async () => {
		try {
			cruise = await getCruise(id);
			enrollToken = cruise?.enroll_token ?? null;
			const [t, v, e] = await Promise.all([
				listCruiseTrips(id).then((r) => r ?? []).catch(() => []),
				listCruiseVoyages(id).then((r) => r ?? []).catch(() => []),
				listCruiseEnrollments(id).then((r) => r ?? []).catch(() => [])
			]);
			trips = t;
			voyages = v;
			enrollments = e;
		} catch (err) {
			console.error('Failed to load cruise:', err);
		} finally {
			loading = false;
		}
	});

	async function handleDelete() {
		if (!confirm('Usunąć wydarzenie? Trips/voyages stracą powiązanie ale zostaną.')) return;
		await deleteCruise(id);
		goto('/cruises');
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
		try {
			await updateCruiseEnrollmentStatus(id, enrollmentId, status);
			enrollments = enrollments.map((e) => (e.id === enrollmentId ? { ...e, status } : e));
		} catch (err) {
			console.error('Failed to set status:', err);
		}
	}

	async function assignEnrollmentToTrip(enrollmentId: number, tripId: number | null) {
		try {
			await assignCruiseEnrollmentTrip(id, enrollmentId, tripId ?? undefined);
			await reloadEnrollments();
		} catch (err) {
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
		pending: 'bg-yellow-100 text-yellow-800',
		accepted: 'bg-green-100 text-green-800',
		rejected: 'bg-red-100 text-red-800',
		waitlisted: 'bg-purple-100 text-purple-800'
	};
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if cruise}
	<div class="mx-auto max-w-5xl">
		<div class="mb-6 flex items-center justify-between">
			<div>
				<h1 class="text-3xl font-bold text-[var(--navy)]">{cruise.name}</h1>
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

		{#if cruise.description}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Opis</h2>
				<p class="whitespace-pre-wrap text-sm">{cruise.description}</p>
			</div>
		{/if}

		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Zapisy</h2>
			<div class="mb-4 flex flex-wrap items-center gap-3">
				{#if isAdmin}
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
				{/if}
				<span class="text-sm text-[var(--text-muted)]">
					{enrollments.filter((e) => e.status === 'accepted').length} / {enrollments.length} zaakceptowanych
				</span>
			</div>

			{#if enrollments.length === 0}
				<p class="text-sm text-[var(--text-muted)]">Brak zapisów.</p>
			{:else}
				<div class="space-y-2">
					{#each enrollments as enrollment}
						<div class="rounded-lg bg-gray-50 p-3">
							<div class="flex flex-wrap items-center justify-between gap-2">
								<div class="flex items-center gap-2">
									<span class="font-medium">{enrollment.user_name ?? 'Nieznany'}</span>
									<span class="text-sm text-[var(--text-muted)]">{enrollment.user_email ?? ''}</span>
									<span class="rounded-full px-2 py-0.5 text-xs font-medium {statusColors[enrollment.status] ?? 'bg-gray-100'}">
										{statusLabels[enrollment.status] ?? enrollment.status}
									</span>
									{#if enrollment.trip_name}
										<span class="rounded-full bg-blue-100 px-2 py-0.5 text-xs text-blue-700">
											{enrollment.trip_name}
										</span>
									{/if}
								</div>
								{#if isAdmin}
									<div class="flex flex-wrap gap-1">
										{#if enrollment.status !== 'accepted'}
											<button onclick={() => setEnrollmentStatus(enrollment.id, 'accepted')} class="rounded px-2 py-1 text-xs text-green-700 hover:bg-green-100">
												Akceptuj
											</button>
										{/if}
										{#if enrollment.status !== 'waitlisted'}
											<button onclick={() => setEnrollmentStatus(enrollment.id, 'waitlisted')} class="rounded px-2 py-1 text-xs text-purple-700 hover:bg-purple-100">
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
								{/if}
							</div>
							{#if isAdmin && enrollment.status === 'accepted'}
								<div class="mt-2 flex items-center gap-2 text-sm">
									<span class="text-[var(--text-muted)]">Przypisz do rejsu:</span>
									<select
										value={enrollment.trip_id ?? ''}
										onchange={(e) => {
											const v = (e.currentTarget as HTMLSelectElement).value;
											assignEnrollmentToTrip(enrollment.id, v ? Number(v) : null);
										}}
										class="rounded border px-2 py-1 text-sm"
									>
										<option value="">— niewybrane —</option>
										{#each trips as t}
											<option value={t.id}>{t.name}</option>
										{/each}
									</select>
								</div>
							{/if}
							{#if enrollment.note}
								<p class="mt-2 text-sm text-[var(--text-muted)]">{enrollment.note}</p>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="font-semibold text-[var(--navy)]">Planowane rejsy ({trips.length})</h2>
				{#if isAdmin}
					<a
						href="/trips/new?cruise_id={id}"
						class="rounded-lg bg-[var(--ocean)] px-3 py-1.5 text-sm text-white hover:bg-[var(--ocean-dark)]"
					>
						+ Dodaj jacht
					</a>
				{/if}
			</div>
			{#if trips.length === 0}
				<p class="text-sm text-[var(--text-muted)]">Brak rejsów. Dodaj jachty, którymi popłyną uczestnicy.</p>
			{:else}
				<div class="space-y-2">
					{#each trips as trip}
						<a href="/trips/{trip.id}" class="block rounded-lg bg-gray-50 px-4 py-3 hover:bg-gray-100">
							<div class="flex items-center justify-between">
								<div>
									<span class="font-medium">{trip.name}</span>
									{#if trip.captain_name}
										<span class="ml-2 text-sm text-[var(--text-muted)]">kpt. {trip.captain_name}</span>
									{/if}
								</div>
								<span class="rounded-full px-2 py-0.5 text-xs {trip.status === 'cancelled' ? 'bg-gray-200 text-gray-600' : 'bg-blue-100 text-blue-700'}">
									{trip.status === 'cancelled' ? 'Anulowany' : 'Planowany'}
								</span>
							</div>
						</a>
					{/each}
				</div>
			{/if}
		</div>

		{#if voyages.length > 0}
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Zrealizowane rejsy ({voyages.length})</h2>
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
{/if}
