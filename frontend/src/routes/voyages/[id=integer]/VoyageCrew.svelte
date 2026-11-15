<script lang="ts">
	import { assignVoyageCrew, listVoyageCrew, removeVoyageCrew } from '#lib/api/routes.js';

	import { errorMessage } from '#lib/api/errors.js';
	import { onDestroy } from 'svelte';
	import type { PageData } from './$types';
	let sectionError = $state('');
	let active = true;
	onDestroy(() => {
		active = false;
	});
	function leftVoyage(voyageId: number) {
		return !active || voyageId !== id;
	}
	let {
		id,
		crew = $bindable(),
		allCrewMembers,
		readonly = false
	}: {
		id: number;
		crew: PageData['crew'];
		allCrewMembers: PageData['allCrewMembers'];
		readonly?: boolean;
	} = $props();
	let assignCrewId = $state('');
	let assignRole = $state('');
	let assigningFor = $state<number | null>(null);
	const assigning = $derived(assigningFor === id);

	async function assignCrew(e: Event) {
		e.preventDefault();
		if (!assignCrewId || !assignRole) return;
		sectionError = '';
		const voyageId = id;
		assigningFor = voyageId;
		try {
			await assignVoyageCrew(voyageId, {
				crew_member_id: Number(assignCrewId),
				role: assignRole
			});
			const list = (await listVoyageCrew(voyageId)) ?? [];
			if (leftVoyage(voyageId)) return;
			crew = list;
			assignCrewId = '';
			assignRole = '';
		} catch (err) {
			if (active) sectionError = errorMessage(err);
			console.error('Failed to assign crew:', err);
		} finally {
			if (assigningFor === voyageId) assigningFor = null;
		}
	}

	async function removeCrew(assignmentId: number) {
		if (!confirm('Usunąć przypisanie załoganta?')) return;
		sectionError = '';
		const voyageId = id;
		try {
			await removeVoyageCrew(voyageId, assignmentId);
		} catch (err) {
			if (active) sectionError = errorMessage(err);
			return;
		}
		if (leftVoyage(voyageId)) return;
		crew = crew.filter((c) => c.id !== assignmentId);
	}
</script>

<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm" data-testid="voyage-crew">
	<div class="mb-3 flex items-center justify-between">
		<h2 class="font-semibold text-[var(--navy)]">Załoga ({crew.length})</h2>
		{#if sectionError}<div role="alert" class="mb-3 rounded-lg bg-red-50 p-3 text-sm text-red-600">
				{sectionError}
			</div>{/if}
	</div>

	{#if !readonly && allCrewMembers.length > 0}
		<form onsubmit={assignCrew} class="mb-4 flex flex-wrap items-end gap-2">
			<div class="min-w-0 max-w-full">
				<label for="assign-crew" class="block text-xs text-[var(--text-muted)]">Załogant</label>
				<select
					id="assign-crew"
					bind:value={assignCrewId}
					class="max-w-full rounded-lg border px-3 py-1.5 text-sm"
				>
					<option value="">Wybierz...</option>
					{#each allCrewMembers as member (member.id)}
						<option value={member.id}>{member.full_name}</option>
					{/each}
				</select>
			</div>
			<div>
				<label for="assign-role" class="block text-xs text-[var(--text-muted)]">Rola</label>
				<input
					id="assign-role"
					type="text"
					bind:value={assignRole}
					placeholder="np. Sternik"
					class="rounded-lg border px-3 py-1.5 text-sm"
				/>
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
			{#each crew as member (member.id)}
				<div class="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2">
					<div class="flex min-w-0 items-center gap-2">
						<span class="min-w-0 font-medium">{member.full_name}</span>
						<span class="rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]">
							{member.role}
						</span>
					</div>
					{#if !readonly}
						<button
							onclick={() => removeCrew(member.id)}
							class="text-sm text-red-500 hover:underline"
						>
							Usuń
						</button>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>
