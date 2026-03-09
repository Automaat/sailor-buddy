<script lang="ts">
	import { api } from '$lib/api/client';
	import type { Cruise, CrewAssignment, CrewMember, VoyageOpinion } from '$lib/api/types';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let cruise = $state<Cruise | null>(null);
	let crew = $state<CrewAssignment[]>([]);
	let opinions = $state<VoyageOpinion[]>([]);
	let allCrewMembers = $state<CrewMember[]>([]);
	let loading = $state(true);

	let genCrewId = $state('');
	let genFormat = $state('pdf');
	let generating = $state(false);

	let assignCrewId = $state('');
	let assignRole = $state('');
	let assigning = $state(false);

	let enrollToken = $state<string | null>(null);
	let togglingEnroll = $state(false);

	const id = $derived(page.params.id);

	onMount(async () => {
		try {
			[cruise, crew, opinions, allCrewMembers] = await Promise.all([
				api.get<Cruise>(`/cruises/${id}`),
				api.get<CrewAssignment[]>(`/cruises/${id}/crew`),
				api.get<VoyageOpinion[]>(`/cruises/${id}/opinions`),
				api.get<CrewMember[]>('/crew')
			]);
			enrollToken = cruise?.enroll_token ?? null;
		} catch (err) {
			console.error('Failed to load cruise:', err);
		} finally {
			loading = false;
		}
	});

	async function handleDelete() {
		if (!confirm('Usunąć ten rejs?')) return;
		await api.del(`/cruises/${id}`);
		goto('/cruises');
	}

	async function generateOpinion() {
		if (!genCrewId) return;
		generating = true;
		try {
			await api.post(`/cruises/${id}/opinions`, {
				crew_member_id: Number(genCrewId),
				format: genFormat
			});
			opinions = await api.get<VoyageOpinion[]>(`/cruises/${id}/opinions`);
			genCrewId = '';
		} catch (err) {
			console.error('Failed to generate opinion:', err);
		} finally {
			generating = false;
		}
	}

	async function deleteOpinion(opId: number) {
		if (!confirm('Usunąć tę opinię?')) return;
		await api.del(`/cruises/${id}/opinions/${opId}`);
		opinions = opinions.filter((o) => o.id !== opId);
	}

	async function toggleEnrollment() {
		togglingEnroll = true;
		try {
			if (enrollToken) {
				await api.del(`/cruises/${id}/enroll-token`);
				enrollToken = null;
			} else {
				const res = await api.post<{ token: string }>(`/cruises/${id}/enroll-token`);
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
			await api.post(`/cruises/${id}/crew`, {
				crew_member_id: Number(assignCrewId),
				role: assignRole
			});
			crew = await api.get<CrewAssignment[]>(`/cruises/${id}/crew`);
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
		await api.del(`/cruises/${id}/crew/${assignmentId}`);
		crew = crew.filter((c) => c.id !== assignmentId);
	}
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if cruise}
	<div class="mx-auto max-w-4xl">
		<div class="mb-6 flex items-center justify-between">
			<div>
				<h1 class="text-3xl font-bold text-[var(--navy)]">{cruise.name}</h1>
				<p class="mt-1 text-[var(--text-muted)]">
					{#if cruise.start_port && cruise.end_port}
						{cruise.start_port} → {cruise.end_port}
					{/if}
					{#if cruise.countries} · {cruise.countries}{/if}
					{#if cruise.year} · {cruise.year}{/if}
				</p>
			</div>
			<div class="flex gap-2">
				<a
					href="/cruises/{id}/edit"
					class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50"
				>
					Edytuj
				</a>
				<button
					onclick={handleDelete}
					class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50"
				>
					Usuń
				</button>
			</div>
		</div>

		<div class="mb-6 grid grid-cols-2 gap-4 md:grid-cols-4">
			<div class="rounded-xl bg-white p-4 shadow-sm">
				<div class="text-xs text-[var(--text-muted)]">Godziny łącznie</div>
				<div class="text-2xl font-bold text-[var(--ocean)]">
					{cruise.hours_total ?? 0}
				</div>
				<div class="text-xs text-[var(--text-muted)]">
					{cruise.hours_sail ?? 0}h żagle / {cruise.hours_engine ?? 0}h silnik
				</div>
			</div>
			<div class="rounded-xl bg-white p-4 shadow-sm">
				<div class="text-xs text-[var(--text-muted)]">Mile</div>
				<div class="text-2xl font-bold text-[var(--sand)]">{cruise.miles ?? 0}</div>
			</div>
			<div class="rounded-xl bg-white p-4 shadow-sm">
				<div class="text-xs text-[var(--text-muted)]">Dni</div>
				<div class="text-2xl font-bold text-[var(--navy)]">{cruise.days ?? 0}</div>
			</div>
			<div class="rounded-xl bg-white p-4 shadow-sm">
				<div class="text-xs text-[var(--text-muted)]">Godziny &gt;6Bf</div>
				<div class="text-2xl font-bold">{cruise.hours_over_6bf ?? 0}</div>
			</div>
		</div>

		{#if cruise.embark_date || cruise.captain_name || cruise.tidal_waters}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Szczegóły</h2>
				<dl class="grid grid-cols-2 gap-2 text-sm">
					{#if cruise.embark_date}
						<dt class="text-[var(--text-muted)]">Daty</dt>
						<dd>{cruise.embark_date} – {cruise.disembark_date ?? '?'}</dd>
					{/if}
					{#if cruise.captain_name}
						<dt class="text-[var(--text-muted)]">Kapitan</dt>
						<dd>{cruise.captain_name}</dd>
					{/if}
					{#if cruise.tidal_waters}
						<dt class="text-[var(--text-muted)]">Wody pływowe</dt>
						<dd>Tak</dd>
					{/if}
					{#if cruise.cost_total}
						<dt class="text-[var(--text-muted)]">Koszt</dt>
						<dd>{cruise.cost_total} ({cruise.cost_per_person ?? '?'} /os)</dd>
					{/if}
				</dl>
			</div>
		{/if}

		{#if cruise.description}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Opis</h2>
				<p class="whitespace-pre-wrap text-sm">{cruise.description}</p>
			</div>
		{/if}

		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Zapisy</h2>
			<div class="flex flex-wrap items-center gap-3">
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
					<a
						href="/cruises/{id}/enrollments"
						class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50"
					>
						Zarządzaj zapisami
					</a>
				{/if}
			</div>
		</div>

		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="font-semibold text-[var(--navy)]">Załoga ({crew.length})</h2>
			</div>

			{#if allCrewMembers.length > 0}
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
					{#each crew as member}
						<div class="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2">
							<div class="flex items-center gap-2">
								<span class="font-medium">{member.full_name}</span>
								<span class="rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]">
									{member.role}
								</span>
							</div>
							<button
								onclick={() => removeCrew(member.id)}
								class="text-sm text-red-500 hover:underline"
							>
								Usuń
							</button>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Opinie z rejsu</h2>

			{#if crew.length > 0}
				<div class="mb-4 flex flex-wrap items-end gap-2">
					<div>
						<label for="gen-crew" class="block text-xs text-[var(--text-muted)]"
							>Załogant</label
						>
						<select
							id="gen-crew"
							bind:value={genCrewId}
							class="rounded-lg border px-3 py-1.5 text-sm"
						>
							<option value="">Wybierz...</option>
							{#each crew as member}
								<option value={member.crew_member_id}>{member.full_name}</option>
							{/each}
						</select>
					</div>
					<div>
						<label for="gen-format" class="block text-xs text-[var(--text-muted)]"
							>Format</label
						>
						<select
							id="gen-format"
							bind:value={genFormat}
							class="rounded-lg border px-3 py-1.5 text-sm"
						>
							<option value="pdf">PDF</option>
							<option value="docx">DOCX</option>
						</select>
					</div>
					<button
						onclick={generateOpinion}
						disabled={!genCrewId || generating}
						class="rounded-lg bg-[var(--ocean)] px-4 py-1.5 text-sm text-white hover:bg-[var(--ocean)]/90 disabled:opacity-50"
					>
						{generating ? 'Generowanie...' : 'Generuj'}
					</button>
				</div>
			{/if}

			{#if opinions.length === 0}
				<p class="text-sm text-[var(--text-muted)]">Brak wygenerowanych opinii.</p>
			{:else}
				<div class="space-y-2">
					{#each opinions as op}
						<div class="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2">
							<div class="flex items-center gap-2">
								<span class="font-medium">{op.full_name}</span>
								<span
									class="rounded-full bg-[var(--sand)]/20 px-2 py-0.5 text-xs uppercase text-[var(--sand)]"
								>
									{op.file_format}
								</span>
							</div>
							<div class="flex gap-2">
								<a
									href="/api/cruises/{id}/opinions/{op.id}/download"
									class="text-sm text-[var(--ocean)] hover:underline"
								>
									Pobierz
								</a>
								<button
									onclick={() => deleteOpinion(op.id)}
									class="text-sm text-red-500 hover:underline"
								>
									Usuń
								</button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</div>
{/if}
