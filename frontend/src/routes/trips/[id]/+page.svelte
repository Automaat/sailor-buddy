<script lang="ts">
	import { api } from '$lib/api/client';
	import { orgStore } from '$lib/stores/org.svelte';
	import type { Trip, CrewAssignment, CrewMember, Voyage, CompleteTripPayload, Cruise } from '$lib/api/types';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import CompleteTripModal from '$lib/components/CompleteTripModal.svelte';

	let trip = $state<Trip | null>(null);
	let cruise = $state<Cruise | null>(null);
	let crew = $state<CrewAssignment[]>([]);
	let allCrewMembers = $state<CrewMember[]>([]);
	let loading = $state(true);

	let assignCrewId = $state('');
	let assignRole = $state('');
	let assigning = $state(false);

	let enrollToken = $state<string | null>(null);
	let togglingEnroll = $state(false);

	let completing = $state(false);
	let showCompleteModal = $state(false);

	const id = $derived(page.params.id);

	onMount(async () => {
		try {
			const prefix = orgStore.apiPrefix();
			trip = await api.get<Trip>(`${prefix}/trips/${id}`);
			enrollToken = trip?.enroll_token ?? null;
			[crew, allCrewMembers] = await Promise.all([
				api.get<CrewAssignment[]>(`${prefix}/trips/${id}/crew`).catch(() => []),
				api.list<CrewMember>(`${prefix}/crew`).catch(() => [])
			]);
			if (trip?.cruise_id) {
				cruise = await api.get<Cruise>(`${prefix}/cruises/${trip.cruise_id}`).catch(() => null);
			}
		} catch (err) {
			console.error('Failed to load trip:', err);
		} finally {
			loading = false;
		}
	});

	async function handleDelete() {
		if (!confirm('Usunąć ten rejs?')) return;
		await api.del(`${orgStore.apiPrefix()}/trips/${id}`);
		goto('/trips');
	}

	async function toggleEnrollment() {
		togglingEnroll = true;
		try {
			if (enrollToken) {
				await api.del(`${orgStore.apiPrefix()}/trips/${id}/enroll-token`);
				enrollToken = null;
			} else {
				const res = await api.post<{ token: string }>(`${orgStore.apiPrefix()}/trips/${id}/enroll-token`);
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

	async function assignCrew(e: Event) {
		e.preventDefault();
		if (!assignCrewId || !assignRole) return;
		assigning = true;
		try {
			await api.post(`${orgStore.apiPrefix()}/trips/${id}/crew`, {
				crew_member_id: Number(assignCrewId),
				role: assignRole
			});
			crew = await api.get<CrewAssignment[]>(`${orgStore.apiPrefix()}/trips/${id}/crew`);
			assignCrewId = '';
			assignRole = '';
		} catch (err) {
			console.error('Failed to assign crew:', err);
		} finally {
			assigning = false;
		}
	}

	async function removeCrew(assignmentId: number) {
		if (!confirm('Usunąć przypisanie załoganta?')) return;
		await api.del(`${orgStore.apiPrefix()}/trips/${id}/crew/${assignmentId}`);
		crew = crew.filter((c) => c.id !== assignmentId);
	}

	async function cancelTrip() {
		if (!confirm('Anulować ten rejs?')) return;
		completing = true;
		try {
			trip = await api.post<Trip>(`${orgStore.apiPrefix()}/trips/${id}/cancel`);
			enrollToken = trip?.enroll_token ?? null;
		} catch (err) {
			console.error('Failed to cancel trip:', err);
		} finally {
			completing = false;
		}
	}

	async function completeTrip(payload: CompleteTripPayload) {
		const voyage = await api.post<Voyage>(`${orgStore.apiPrefix()}/trips/${id}/complete`, payload);
		showCompleteModal = false;
		goto(`/voyages/${voyage.id}`);
	}

	function statusBadge(status: string) {
		return status === 'cancelled' ? 'bg-gray-100 text-gray-500' : 'bg-blue-100 text-blue-700';
	}

	function statusLabel(status: string) {
		return status === 'cancelled' ? 'Anulowany' : 'Planowany';
	}

</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if trip}
	<div class="mx-auto max-w-4xl">
		<div class="mb-6 flex items-center justify-between">
			<div>
				{#if cruise}
					<a href="/cruises/{cruise.id}" class="text-xs text-[var(--ocean)] hover:underline">
						← Część wydarzenia: {cruise.name}
					</a>
				{/if}
				<div class="flex items-center gap-3">
					<h1 class="text-3xl font-bold text-[var(--navy)]">{trip.name}</h1>
					<span class="rounded-full px-2.5 py-1 text-xs font-medium {statusBadge(trip.status)}">
						{statusLabel(trip.status)}
					</span>
				</div>
				<p class="mt-1 text-[var(--text-muted)]">
					{#if trip.start_port && trip.end_port}
						{trip.start_port} → {trip.end_port}
					{/if}
					{#if trip.countries} · {trip.countries}{/if}
				</p>
			</div>
			<div class="flex gap-2">
				{#if trip.status === 'planned'}
					<button
						onclick={() => (showCompleteModal = true)}
						disabled={completing}
						class="rounded-lg bg-green-600 px-4 py-2 text-sm text-white hover:bg-green-700 disabled:opacity-50"
					>
						Zrealizuj
					</button>
					<button
						onclick={cancelTrip}
						disabled={completing}
						class="rounded-lg border border-gray-300 px-4 py-2 text-sm text-gray-600 hover:bg-gray-50 disabled:opacity-50"
					>
						Anuluj
					</button>
				{/if}
				<a href="/trips/{id}/edit" class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">Edytuj</a>
				<button onclick={handleDelete} class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50">
					Usuń
				</button>
			</div>
		</div>

		{#if trip.embark_date || trip.captain_name || trip.cost_total}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Szczegóły</h2>
				<dl class="grid grid-cols-2 gap-2 text-sm">
					{#if trip.embark_date}
						<dt class="text-[var(--text-muted)]">Daty</dt>
						<dd>{trip.embark_date} – {trip.disembark_date ?? '?'}</dd>
					{/if}
					{#if trip.captain_name}
						<dt class="text-[var(--text-muted)]">Kapitan</dt>
						<dd>{trip.captain_name}</dd>
					{/if}
					{#if trip.max_crew}
						<dt class="text-[var(--text-muted)]">Maks. załoga</dt>
						<dd>{trip.max_crew}</dd>
					{/if}
					{#if trip.cost_total}
						<dt class="text-[var(--text-muted)]">Koszt</dt>
						<dd>{trip.cost_total} ({trip.cost_per_person ?? '?'} /os)</dd>
					{/if}
				</dl>
			</div>
		{/if}

		{#if trip.description}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Opis</h2>
				<p class="whitespace-pre-wrap text-sm">{trip.description}</p>
			</div>
		{/if}

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
					<button onclick={copyEnrollLink} class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">
						Kopiuj link
					</button>
					<a href="/trips/{id}/enrollments" class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">
						Zarządzaj zapisami
					</a>
				{/if}
			</div>
		</div>

		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="font-semibold text-[var(--navy)]">Załoga ({crew.length})</h2>
			</div>

			{#if allCrewMembers.length > 0 && trip.status === 'planned'}
				<form onsubmit={assignCrew} class="mb-4 flex flex-wrap items-end gap-2">
					<div>
						<label for="assign-crew" class="block text-xs text-[var(--text-muted)]">Załogant</label>
						<select id="assign-crew" bind:value={assignCrewId} class="rounded-lg border px-3 py-1.5 text-sm">
							<option value="">Wybierz...</option>
							{#each allCrewMembers as member}
								<option value={member.id}>{member.full_name}</option>
							{/each}
						</select>
					</div>
					<div>
						<label for="assign-role" class="block text-xs text-[var(--text-muted)]">Rola</label>
						<input id="assign-role" type="text" bind:value={assignRole} placeholder="np. Sternik" class="rounded-lg border px-3 py-1.5 text-sm" />
					</div>
					<button
						type="submit"
						disabled={!assignCrewId || !assignRole || assigning}
						class="rounded-lg bg-[var(--ocean)] px-4 py-1.5 text-sm text-white hover:bg-[var(--ocean)]/90 disabled:opacity-50"
					>
						{assigning ? 'Dodawanie...' : 'Dodaj'}
					</button>
				</form>
			{/if}

			{#if crew.length === 0}
				<p class="text-sm text-[var(--text-muted)]">Brak przypisanej załogi.</p>
			{:else}
				<div class="space-y-2">
					{#each crew as member}
						<div class="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2">
							<div class="flex items-center gap-2">
								<span class="font-medium">{member.full_name}</span>
								<span class="rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]">
									{member.role}
								</span>
							</div>
							{#if trip.status === 'planned'}
								<button onclick={() => removeCrew(member.id)} class="text-sm text-red-500 hover:underline">
									Usuń
								</button>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</div>

	{#if showCompleteModal}
		<CompleteTripModal
			tripName={trip.name}
			embarkDate={trip.embark_date}
			disembarkDate={trip.disembark_date}
			onClose={() => (showCompleteModal = false)}
			onSubmit={completeTrip}
		/>
	{/if}
{/if}
