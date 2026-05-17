<script lang="ts">
	import {
		getVoyage,
		deleteVoyage,
		listVoyageCrew,
		assignVoyageCrew,
		removeVoyageCrew,
		listVoyageOpinions,
		generateVoyageOpinion,
		deleteVoyageOpinion,
		downloadVoyageOpinion,
		listAssignableCrew,
		listVoyagePorts,
		addVoyagePort,
		deleteVoyagePort,
		reorderVoyagePorts,
		getCruise
	} from '$lib/api/routes';
	import type {
		Voyage,
		CrewAssignment,
		CrewMember,
		VoyageOpinion,
		VoyagePort,
		VoyagePortBody,
		Cruise
	} from '$lib/api/types';
	import { ApiError } from '$lib/api/client';
	import { orgStore } from '$lib/stores/org.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import PortPicker from '$lib/components/PortPicker.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let voyage = $state<Voyage | null>(null);
	let cruise = $state<Cruise | null>(null);
	let crew = $state<CrewAssignment[]>([]);
	let opinions = $state<VoyageOpinion[]>([]);
	let ports = $state<VoyagePort[]>([]);
	let allCrewMembers = $state<CrewMember[]>([]);
	let loading = $state(true);
	let notFound = $state(false);

	let genCrewId = $state('');
	let genFormat = $state<'pdf' | 'docx'>('pdf');
	let generating = $state(false);

	let assignCrewId = $state('');
	let assignRole = $state('');
	let assigning = $state(false);

	const id = $derived(Number(page.params.id));

	onMount(async () => {
		try {
			// getVoyage scopes on isOrgAdmin, which is only known once the org
			// list has loaded — wait so a direct visit hits the right endpoint.
			await orgStore.ensureLoaded();
			voyage = await getVoyage(id);
			[crew, opinions, ports, allCrewMembers] = await Promise.all([
				listVoyageCrew(id).then((c) => c ?? []).catch(() => []),
				listVoyageOpinions(id).then((o) => o ?? []).catch(() => []),
				listVoyagePorts(id).then((p) => p ?? []).catch(() => []),
				listAssignableCrew().catch(() => [])
			]);
			if (voyage?.cruise_id) {
				cruise = await getCruise(voyage.cruise_id).catch(() => null);
			}
		} catch (err) {
			if (err instanceof ApiError && err.status === 404) {
				notFound = true;
			} else {
				console.error('Failed to load voyage:', err);
			}
		} finally {
			loading = false;
		}
	});

	async function handleDelete() {
		if (!confirm('Usunąć ten rejs?')) return;
		await deleteVoyage(id);
		goto('/voyages');
	}

	async function generateOpinion() {
		if (!genCrewId) return;
		generating = true;
		try {
			await generateVoyageOpinion(id, {
				crew_member_id: Number(genCrewId),
				format: genFormat
			});
			opinions = (await listVoyageOpinions(id)) ?? [];
			genCrewId = '';
		} catch (err) {
			console.error('Failed to generate opinion:', err);
		} finally {
			generating = false;
		}
	}

	async function downloadOpinion(opId: number) {
		try {
			await downloadVoyageOpinion(id, opId);
		} catch (err) {
			console.error('Failed to download opinion:', err);
		}
	}

	async function deleteOpinion(opId: number) {
		if (!confirm('Usunąć tę opinię?')) return;
		await deleteVoyageOpinion(id, opId);
		opinions = opinions.filter((o) => o.id !== opId);
	}

	async function assignCrew(e: Event) {
		e.preventDefault();
		if (!assignCrewId || !assignRole) return;
		assigning = true;
		try {
			await assignVoyageCrew(id, {
				crew_member_id: Number(assignCrewId),
				role: assignRole
			});
			crew = (await listVoyageCrew(id)) ?? [];
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
		await removeVoyageCrew(id, assignmentId);
		crew = crew.filter((c) => c.id !== assignmentId);
	}

	async function addPort(body: VoyagePortBody) {
		// Derive the next position from the max so it stays unique after deletions.
		const nextPosition = ports.length > 0 ? Math.max(...ports.map((p) => p.position)) + 1 : 0;
		const port = await addVoyagePort(id, { ...body, position: nextPosition });
		ports = [...ports, port];
	}

	async function removePort(index: number) {
		const port = ports[index];
		if (!port) return;
		await deleteVoyagePort(id, port.id);
		ports = ports.filter((_, i) => i !== index);
	}

	async function reorderPorts(from: number, to: number) {
		const next = [...ports];
		const [moved] = next.splice(from, 1);
		next.splice(to, 0, moved);
		// The endpoint returns the ports with their persisted positions.
		ports = (await reorderVoyagePorts(id, next.map((p) => p.id))) ?? next;
	}
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if voyage}
	<div class="mx-auto max-w-4xl">
		<div class="mb-6 flex items-center justify-between">
			<div>
				{#if cruise}
					<a href="/cruises/{cruise.id}" class="text-xs text-[var(--ocean)] hover:underline">
						← Część wydarzenia: {cruise.name}
					</a>
				{/if}
				<h1 class="text-3xl font-bold text-[var(--navy)]">{voyage.name}</h1>
				<p class="mt-1 text-[var(--text-muted)]">
					{#if voyage.start_port && voyage.end_port}
						{voyage.start_port} → {voyage.end_port}
					{/if}
					{#if voyage.countries} · {voyage.countries}{/if}
					{#if voyage.year} · {voyage.year}{/if}
				</p>
			</div>
			<div class="flex gap-2">
				<a href="/voyages/{id}/edit" class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">Edytuj</a>
				<button onclick={handleDelete} class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50">
					Usuń
				</button>
			</div>
		</div>

		<div class="mb-6 grid grid-cols-2 gap-4 md:grid-cols-4">
			<div class="rounded-xl bg-white p-4 shadow-sm">
				<div class="text-xs text-[var(--text-muted)]">Godziny łącznie</div>
				<div class="text-2xl font-bold text-[var(--ocean)]">{voyage.hours_total}</div>
				<div class="text-xs text-[var(--text-muted)]">
					{voyage.hours_sail}h żagle / {voyage.hours_engine}h silnik
				</div>
			</div>
			<div class="rounded-xl bg-white p-4 shadow-sm">
				<div class="text-xs text-[var(--text-muted)]">Mile</div>
				<div class="text-2xl font-bold text-[var(--sand)]">{voyage.miles}</div>
			</div>
			<div class="rounded-xl bg-white p-4 shadow-sm">
				<div class="text-xs text-[var(--text-muted)]">Dni</div>
				<div class="text-2xl font-bold text-[var(--navy)]">{voyage.days}</div>
			</div>
			<div class="rounded-xl bg-white p-4 shadow-sm">
				<div class="text-xs text-[var(--text-muted)]">Godziny &gt;6Bf</div>
				<div class="text-2xl font-bold">{voyage.hours_over_6bf}</div>
			</div>
		</div>

		{#if voyage.embark_date || voyage.captain_name || voyage.tidal_waters || voyage.cost_total}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Szczegóły</h2>
				<dl class="grid grid-cols-2 gap-2 text-sm">
					{#if voyage.embark_date}
						<dt class="text-[var(--text-muted)]">Daty</dt>
						<dd>{voyage.embark_date} – {voyage.disembark_date ?? '?'}</dd>
					{/if}
					{#if voyage.captain_name}
						<dt class="text-[var(--text-muted)]">Kapitan</dt>
						<dd>{voyage.captain_name}</dd>
					{/if}
					{#if voyage.tidal_waters}
						<dt class="text-[var(--text-muted)]">Wody pływowe</dt>
						<dd>Tak</dd>
					{/if}
					{#if voyage.cost_total}
						<dt class="text-[var(--text-muted)]">Koszt</dt>
						<dd>{voyage.cost_total} ({voyage.cost_per_person ?? '?'} /os)</dd>
					{/if}
				</dl>
			</div>
		{/if}

		{#if voyage.description}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-3 font-semibold text-[var(--navy)]">Opis</h2>
				<p class="whitespace-pre-wrap text-sm">{voyage.description}</p>
			</div>
		{/if}

		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Odwiedzone porty ({ports.length})</h2>
			<PortPicker {ports} onAdd={addPort} onRemove={removePort} onReorder={reorderPorts} />
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
						<input id="assign-role" type="text" bind:value={assignRole} placeholder="np. Sternik" class="rounded-lg border px-3 py-1.5 text-sm" />
					</div>
					<button type="submit" disabled={!assignCrewId || !assignRole || assigning} class="rounded-lg bg-[var(--ocean)] px-4 py-1.5 text-sm text-white hover:bg-[var(--ocean)]/90 disabled:opacity-50">
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
							<button onclick={() => removeCrew(member.id)} class="text-sm text-red-500 hover:underline">
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
						<label for="gen-crew" class="block text-xs text-[var(--text-muted)]">Załogant</label>
						<select id="gen-crew" bind:value={genCrewId} class="rounded-lg border px-3 py-1.5 text-sm">
							<option value="">Wybierz...</option>
							{#each crew as member}
								<option value={member.crew_member_id}>{member.full_name}</option>
							{/each}
						</select>
					</div>
					<div>
						<label for="gen-format" class="block text-xs text-[var(--text-muted)]">Format</label>
						<select id="gen-format" bind:value={genFormat} class="rounded-lg border px-3 py-1.5 text-sm">
							<option value="pdf">PDF</option>
							<option value="docx">DOCX</option>
						</select>
					</div>
					<button onclick={generateOpinion} disabled={!genCrewId || generating} class="rounded-lg bg-[var(--ocean)] px-4 py-1.5 text-sm text-white hover:bg-[var(--ocean)]/90 disabled:opacity-50">
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
								<span class="rounded-full bg-[var(--sand)]/20 px-2 py-0.5 text-xs uppercase text-[var(--sand)]">
									{op.file_format}
								</span>
							</div>
							<div class="flex gap-2">
								<button onclick={() => downloadOpinion(op.id)} class="text-sm text-[var(--ocean)] hover:underline">
									Pobierz
								</button>
								<button onclick={() => deleteOpinion(op.id)} class="text-sm text-red-500 hover:underline">
									Usuń
								</button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</div>
{:else if notFound}
	<NotFound
		title="Nie znaleziono rejsu"
		message="Ten zrealizowany rejs nie istnieje lub nie masz do niego dostępu."
		backHref="/voyages"
		backLabel="Zrealizowane rejsy"
	/>
{:else}
	<NotFound
		title="Nie udało się wczytać rejsu"
		message="Coś poszło nie tak. Spróbuj ponownie za chwilę."
		backHref="/voyages"
		backLabel="Zrealizowane rejsy"
	/>
{/if}
