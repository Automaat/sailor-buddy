<script lang="ts">
	import { formatDateRangeWithDays } from '#lib/dates.js';
	import {
		deleteVoyage,
		listVoyageCrew,
		assignVoyageCrew,
		removeVoyageCrew,
		listVoyageOpinions,
		generateVoyageOpinion,
		deleteVoyageOpinion,
		downloadVoyageOpinion,
		addVoyagePort,
		deleteVoyagePort,
		reorderVoyagePorts,
		updateVoyageLog,
		addVoyageYacht
	} from '#lib/api/routes.js';
	import type { VoyagePortBody, YachtBody } from '#lib/api/types.js';
	import { auth } from '#lib/stores/auth.svelte.js';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import PortPicker from '#lib/components/PortPicker.svelte';
	import YachtSelect from '#lib/components/YachtSelect.svelte';
	import { goto } from '$app/navigation';

	// data comes from +page.ts, which reruns when the URL's voyage changes, so
	// navigating between two voyages swaps every value derived from it.
	let { data } = $props();

	const cruise = $derived(data.cruise);
	const allCrewMembers = $derived(data.allCrewMembers);
	const loadErrors = $derived(data.loadErrors);
	const adminLoadErrors = $derived(data.adminLoadErrors);
	// These change in place after a log save or a crew, opinion, port or
	// yacht edit, and reset to the loaded value on navigation.
	let voyage = $derived(data.voyage);
	let crew = $derived(data.crew);
	let opinions = $derived(data.opinions);
	let ports = $derived(data.ports);
	let yachts = $derived(data.yachts);

	const id = $derived(data.voyage.id);

	let genCrewId = $state('');
	let genFormat = $state<'pdf' | 'docx'>('pdf');
	let assignCrewId = $state('');
	let assignRole = $state('');
	// The *For values name the voyage whose request is in flight, so a button
	// stays busy only on that voyage, not on one a link moved to.
	let generatingFor = $state<number | null>(null);
	let assigningFor = $state<number | null>(null);
	let logSavingFor = $state<number | null>(null);
	const generating = $derived(generatingFor === id);
	const assigning = $derived(assigningFor === id);
	const logSaving = $derived(logSavingFor === id);

	// Voyages are club-managed: only admins manage crew & opinions.
	const canManageCrew = $derived(auth.isAdmin);
	// Failures of admin-only sections join the banner only for admins. The
	// check is reactive: on a direct load the role arrives after the load.
	const visibleLoadErrors = $derived(canManageCrew ? [...loadErrors, ...adminLoadErrors] : loadErrors);
	// The captain, through the person linked to their account, logs the
	// post-cruise data: hours, miles, ports, note.
	const isVoyageCaptain = $derived(
		!!auth.user?.person_id && voyage.captain_person_id === auth.user.person_id
	);
	const canLog = $derived(auth.isAdmin || isVoyageCaptain);
	const yacht = $derived(yachts.find((y) => y.id === voyage.yacht_id) ?? null);

	let logOpen = $state(false);
	let logError = $state('');
	let logForm = $state({
		hours_total: 0,
		hours_sail: 0,
		hours_engine: 0,
		hours_over_6bf: 0,
		miles: 0,
		days: 0,
		tidal_waters: false,
		captain_notes: '',
		yacht_id: 0,
		track_url: '',
		voyage_card: ''
	});

	// Form input and alerts belong to one voyage: a link to another clears them.
	$effect(() => {
		void id;
		genCrewId = '';
		assignCrewId = '';
		assignRole = '';
		logOpen = false;
		logError = '';
	});

	// A request started on one voyage may answer after a link has moved the
	// page to another; its reply then belongs to a page the user has left
	// and is dropped.
	function leftVoyage(voyageId: number) {
		return voyageId !== id;
	}

	function openLog() {
		logForm = {
			hours_total: voyage.hours_total,
			hours_sail: voyage.hours_sail,
			hours_engine: voyage.hours_engine,
			hours_over_6bf: voyage.hours_over_6bf,
			miles: voyage.miles,
			days: voyage.days,
			tidal_waters: !!voyage.tidal_waters,
			captain_notes: voyage.captain_notes ?? '',
			yacht_id: voyage.yacht_id ?? 0,
			track_url: voyage.track_url ?? '',
			voyage_card: voyage.voyage_card ?? ''
		};
		logError = '';
		logOpen = true;
	}

	async function saveLog(e: Event) {
		e.preventDefault();
		const voyageId = id;
		logSavingFor = voyageId;
		logError = '';
		try {
			const body = {
				...logForm,
				tidal_waters: logForm.tidal_waters ? 1 : 0,
				captain_notes: logForm.captain_notes || undefined,
				yacht_id: logForm.yacht_id || undefined,
				track_url: logForm.track_url.trim() || undefined,
				voyage_card: logForm.voyage_card.trim() || undefined
			};
			await updateVoyageLog(voyageId, body);
			if (leftVoyage(voyageId)) return;
			voyage = { ...voyage, ...body };
			logOpen = false;
		} catch (err) {
			if (leftVoyage(voyageId)) return;
			logError = err instanceof Error ? err.message : 'Nie udało się zapisać dziennika';
		} finally {
			if (logSavingFor === voyageId) logSavingFor = null;
		}
	}

	function addYachtForVoyage(body: YachtBody) {
		return addVoyageYacht(id, body);
	}

	async function handleDelete() {
		if (!confirm('Usunąć ten rejs?')) return;
		const voyageId = id;
		const cruiseId = voyage.cruise_id;
		await deleteVoyage(voyageId);
		if (leftVoyage(voyageId)) return;
		void goto(`/cruises/${cruiseId}`);
	}

	async function generateOpinion() {
		if (!genCrewId) return;
		const voyageId = id;
		generatingFor = voyageId;
		try {
			await generateVoyageOpinion(voyageId, {
				crew_member_id: Number(genCrewId),
				format: genFormat
			});
			const list = (await listVoyageOpinions(voyageId)) ?? [];
			if (leftVoyage(voyageId)) return;
			opinions = list;
			genCrewId = '';
		} catch (err) {
			console.error('Failed to generate opinion:', err);
		} finally {
			if (generatingFor === voyageId) generatingFor = null;
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
		const voyageId = id;
		await deleteVoyageOpinion(voyageId, opId);
		if (leftVoyage(voyageId)) return;
		opinions = opinions.filter((o) => o.id !== opId);
	}

	async function assignCrew(e: Event) {
		e.preventDefault();
		if (!assignCrewId || !assignRole) return;
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
			console.error('Failed to assign crew:', err);
		} finally {
			if (assigningFor === voyageId) assigningFor = null;
		}
	}

	async function removeCrew(assignmentId: number) {
		if (!confirm('Usunąć przypisanie załoganta?')) return;
		const voyageId = id;
		await removeVoyageCrew(voyageId, assignmentId);
		if (leftVoyage(voyageId)) return;
		crew = crew.filter((c) => c.id !== assignmentId);
	}

	async function addPort(body: VoyagePortBody) {
		const voyageId = id;
		// Derive the next position from the max so it stays unique after deletions.
		const nextPosition = ports.length > 0 ? Math.max(...ports.map((p) => p.position)) + 1 : 0;
		const port = await addVoyagePort(voyageId, { ...body, position: nextPosition });
		if (leftVoyage(voyageId)) return;
		ports = [...ports, port];
	}

	async function removePort(index: number) {
		const voyageId = id;
		const port = ports[index];
		if (!port) return;
		await deleteVoyagePort(voyageId, port.id);
		if (leftVoyage(voyageId)) return;
		ports = ports.filter((p) => p.id !== port.id);
	}

	async function reorderPorts(from: number, to: number) {
		const voyageId = id;
		const next = [...ports];
		const [moved] = next.splice(from, 1);
		next.splice(to, 0, moved);
		// The endpoint returns the ports with their persisted positions.
		const saved = (await reorderVoyagePorts(voyageId, next.map((p) => p.id))) ?? next;
		if (leftVoyage(voyageId)) return;
		ports = saved;
	}
</script>

<div class="mx-auto max-w-4xl">
	<LoadErrorBanner errors={visibleLoadErrors} />
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

	{#if voyage.embark_date || voyage.captain_name || yacht || voyage.track_url || voyage.voyage_card || voyage.tidal_waters || voyage.cost_total}
		<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Szczegóły</h2>
			<dl class="grid grid-cols-1 text-sm sm:grid-cols-2 sm:gap-2 max-sm:[&>dd]:mb-2">
				{#if voyage.embark_date}
					<dt class="text-[var(--text-muted)]">Daty</dt>
					<dd>{formatDateRangeWithDays(voyage.embark_date, voyage.disembark_date)}</dd>
				{/if}
				{#if voyage.captain_name}
					<dt class="text-[var(--text-muted)]">Kapitan</dt>
					<dd>{voyage.captain_name}</dd>
				{/if}
				{#if yacht}
					<dt class="text-[var(--text-muted)]">Jacht</dt>
					<dd>{yacht.yacht_type ? `${yacht.yacht_type} ` : ''}{yacht.name}</dd>
				{/if}
				{#if voyage.track_url}
					<dt class="text-[var(--text-muted)]">Ślad rejsu</dt>
					<dd>
						<a href={voyage.track_url} target="_blank" rel="noopener noreferrer" class="break-all text-[var(--ocean)] hover:underline">
							{voyage.track_url}
						</a>
					</dd>
				{/if}
				{#if voyage.voyage_card}
					<dt class="text-[var(--text-muted)]">Karta rejsu</dt>
					<dd>{voyage.voyage_card}</dd>
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
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
						<div class="sm:col-span-2">
							<label for="log_yacht" class="mb-1 block text-xs font-medium">Jacht</label>
							<YachtSelect id="log_yacht" bind:value={logForm.yacht_id} bind:yachts create={addYachtForVoyage} />
						</div>
						<div>
							<label for="log_track_url" class="mb-1 block text-xs font-medium">Link do śladu rejsu</label>
							<input id="log_track_url" type="url" bind:value={logForm.track_url} placeholder="https://..." class="w-full rounded-lg border px-3 py-2" />
						</div>
						<div>
							<label for="log_voyage_card" class="mb-1 block text-xs font-medium">Karta rejsu</label>
							<input id="log_voyage_card" type="text" bind:value={logForm.voyage_card} placeholder="np. u kapitana, w klubie" class="w-full rounded-lg border px-3 py-2" />
						</div>
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
		<!-- Keyed on the voyage so a link to another voyage starts the picker
	afresh: no typed search, alert or busy flag from the previous voyage. -->
	{#key id}
		<PortPicker {ports} readonly={!canLog} onAdd={addPort} onRemove={removePort} onReorder={reorderPorts} />
	{/key}
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
							{#each allCrewMembers as member (member.id)}
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
					{#each crew as member (member.id)}
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
					<div class="min-w-0 max-w-full">
						<label for="gen-crew" class="block text-xs text-[var(--text-muted)]">Załogant</label>
						<select id="gen-crew" bind:value={genCrewId} class="max-w-full rounded-lg border px-3 py-1.5 text-sm">
							<option value="">Wybierz...</option>
							{#each crew as member (member.id)}
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
					{#each opinions as op (op.id)}
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
