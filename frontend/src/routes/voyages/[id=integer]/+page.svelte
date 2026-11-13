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
		updateVoyageLog,
		getCruise
	} from '#lib/api/routes.js';
	import type {
		Voyage,
		CrewAssignment,
		CrewMember,
		VoyageOpinion,
		VoyagePort,
		VoyagePortBody,
		Cruise
	} from '#lib/api/types.js';
	import { ApiError } from '#lib/api/client.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import NotFound from '#lib/components/NotFound.svelte';
	import PortPicker from '#lib/components/PortPicker.svelte';
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
	// Voyages are club-managed: only admins manage crew & opinions.
	const canManageCrew = $derived(auth.isAdmin);
	// The assigned captain logs the post-cruise data: hours, miles, ports, note.
	const isVoyageCaptain = $derived(
		!!auth.user && !!voyage?.captain_user_id && voyage.captain_user_id === auth.user.id
	);
	const canLog = $derived(auth.isAdmin || isVoyageCaptain);

	let logOpen = $state(false);
	let logSaving = $state(false);
	let logError = $state('');
	let logForm = $state({
		hours_total: 0,
		hours_sail: 0,
		hours_engine: 0,
		hours_over_6bf: 0,
		miles: 0,
		days: 0,
		tidal_waters: false,
		captain_notes: ''
	});

	function openLog() {
		if (!voyage) return;
		logForm = {
			hours_total: voyage.hours_total,
			hours_sail: voyage.hours_sail,
			hours_engine: voyage.hours_engine,
			hours_over_6bf: voyage.hours_over_6bf,
			miles: voyage.miles,
			days: voyage.days,
			tidal_waters: !!voyage.tidal_waters,
			captain_notes: voyage.captain_notes ?? ''
		};
		logError = '';
		logOpen = true;
	}

	async function saveLog(e: Event) {
		e.preventDefault();
		if (!voyage) return;
		logSaving = true;
		logError = '';
		try {
			const body = {
				...logForm,
				tidal_waters: logForm.tidal_waters ? 1 : 0,
				captain_notes: logForm.captain_notes || undefined
			};
			await updateVoyageLog(id, body);
			voyage = { ...voyage, ...body };
			logOpen = false;
		} catch (err) {
			logError = err instanceof Error ? err.message : 'Nie udało się zapisać dziennika';
		} finally {
			logSaving = false;
		}
	}

	onMount(async () => {
		try {
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
		goto(voyage ? `/cruises/${voyage.cruise_id}` : '/cruises?tab=completed');
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
		<div class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3">
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
			{#if canManageCrew}
				<div class="flex gap-2 max-md:flex-wrap">
					<a href="/voyages/{id}/edit" class="tap-target rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">Edytuj</a>
					<button onclick={handleDelete} class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50">
						Usuń
					</button>
				</div>
			{/if}
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
				<dl class="grid grid-cols-1 text-sm sm:grid-cols-2 sm:gap-2 max-sm:[&>dd]:mb-2">
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

		{#if canLog || voyage.captain_notes}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<div class="mb-3 flex items-center justify-between">
					<h2 class="font-semibold text-[var(--navy)]">Raport kapitana</h2>
					{#if canLog && !logOpen}
						<button onclick={openLog} class="rounded-lg border px-3 py-1 text-sm hover:bg-gray-50">
							Uzupełnij po rejsie
						</button>
					{/if}
				</div>
				{#if logOpen}
					{#if logError}
						<div class="mb-3 rounded-lg bg-red-50 p-3 text-sm text-red-600">{logError}</div>
					{/if}
					<form onsubmit={saveLog} class="space-y-4">
						<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
							<div>
								<label for="log_hours_total" class="mb-1 block text-xs font-medium">Godziny łącznie</label>
								<input id="log_hours_total" type="number" step="0.1" min="0" bind:value={logForm.hours_total} class="w-full rounded-lg border px-3 py-2" />
							</div>
							<div>
								<label for="log_hours_sail" class="mb-1 block text-xs font-medium">Godziny na żaglach</label>
								<input id="log_hours_sail" type="number" step="0.1" min="0" bind:value={logForm.hours_sail} class="w-full rounded-lg border px-3 py-2" />
							</div>
							<div>
								<label for="log_hours_engine" class="mb-1 block text-xs font-medium">Godziny na silniku</label>
								<input id="log_hours_engine" type="number" step="0.1" min="0" bind:value={logForm.hours_engine} class="w-full rounded-lg border px-3 py-2" />
							</div>
							<div>
								<label for="log_hours_6bf" class="mb-1 block text-xs font-medium">Godziny &gt;6Bf</label>
								<input id="log_hours_6bf" type="number" step="0.1" min="0" bind:value={logForm.hours_over_6bf} class="w-full rounded-lg border px-3 py-2" />
							</div>
							<div>
								<label for="log_miles" class="mb-1 block text-xs font-medium">Mile</label>
								<input id="log_miles" type="number" step="0.1" min="0" bind:value={logForm.miles} class="w-full rounded-lg border px-3 py-2" />
							</div>
							<div>
								<label for="log_days" class="mb-1 block text-xs font-medium">Dni</label>
								<input id="log_days" type="number" min="0" bind:value={logForm.days} class="w-full rounded-lg border px-3 py-2" />
							</div>
							<label class="col-span-2 flex items-center gap-2 text-sm">
								<input type="checkbox" bind:checked={logForm.tidal_waters} />
								Wody pływowe
							</label>
						</div>
						<div>
							<label for="log_notes" class="mb-1 block text-xs font-medium">Notatka z rejsu</label>
							<textarea id="log_notes" rows="4" bind:value={logForm.captain_notes} class="w-full rounded-lg border px-3 py-2"></textarea>
						</div>
						<p class="text-xs text-[var(--text-muted)]">Odwiedzone mariny dodasz w sekcji „Odwiedzone porty” poniżej.</p>
						<div class="flex gap-2">
							<button type="submit" disabled={logSaving} class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
								{logSaving ? 'Zapisywanie...' : 'Zapisz raport'}
							</button>
							<button type="button" onclick={() => (logOpen = false)} class="rounded-lg border px-4 py-2 text-sm text-[var(--text-muted)] hover:bg-gray-50">
								Anuluj
							</button>
						</div>
					</form>
				{:else if voyage.captain_notes}
					<p class="whitespace-pre-wrap text-sm">{voyage.captain_notes}</p>
				{:else}
					<p class="text-sm text-[var(--text-muted)]">Kapitan nie dodał jeszcze raportu.</p>
				{/if}
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
			<PortPicker {ports} readonly={!canLog} onAdd={addPort} onRemove={removePort} onReorder={reorderPorts} />
		</div>

		{#if canManageCrew}
			<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
				<div class="mb-3 flex items-center justify-between">
					<h2 class="font-semibold text-[var(--navy)]">Załoga ({crew.length})</h2>
				</div>

				{#if allCrewMembers.length > 0}
					<form onsubmit={assignCrew} class="mb-4 flex flex-wrap items-end gap-2">
						<div class="min-w-0 max-w-full">
							<label for="assign-crew" class="block text-xs text-[var(--text-muted)]">Załogant</label>
							<select id="assign-crew" bind:value={assignCrewId} class="max-w-full rounded-lg border px-3 py-1.5 text-sm">
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
									<span class="rounded-full bg-[var(--ocean-tint)] px-2 py-0.5 text-xs uppercase text-[var(--ocean)]">
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
		{/if}
	</div>
{:else if notFound}
	<NotFound
		title="Nie znaleziono rejsu"
		message="Ten zrealizowany rejs nie istnieje lub nie masz do niego dostępu."
		backHref="/cruises?tab=completed"
		backLabel="Zrealizowane rejsy"
	/>
{:else}
	<NotFound
		title="Nie udało się wczytać rejsu"
		message="Coś poszło nie tak. Spróbuj ponownie za chwilę."
		backHref="/cruises?tab=completed"
		backLabel="Zrealizowane rejsy"
	/>
{/if}
