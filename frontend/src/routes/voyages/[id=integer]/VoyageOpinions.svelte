<script lang="ts">
	import {
		generateVoyageOpinion,
		listVoyageOpinions,
		downloadVoyageOpinion,
		deleteVoyageOpinion
	} from '#lib/api/routes.js';

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
		crew,
		initialOpinions
	}: { id: number; crew: PageData['crew']; initialOpinions: PageData['opinions'] } = $props();
	let opinions = $derived(initialOpinions);
	let genCrewId = $state('');
	let genFormat = $state<'pdf' | 'docx'>('pdf');
	let generatingFor = $state<number | null>(null);
	const generating = $derived(generatingFor === id);

	async function generateOpinion() {
		if (!genCrewId) return;
		sectionError = '';
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
			if (active) sectionError = errorMessage(err);
			console.error('Failed to generate opinion:', err);
		} finally {
			if (generatingFor === voyageId) generatingFor = null;
		}
	}

	async function downloadOpinion(opId: number) {
		try {
			await downloadVoyageOpinion(id, opId);
		} catch (err) {
			if (active) sectionError = errorMessage(err);
			console.error('Failed to download opinion:', err);
		}
	}

	async function deleteOpinion(opId: number) {
		if (!confirm('Usunąć tę opinię?')) return;
		sectionError = '';
		const voyageId = id;
		try {
			await deleteVoyageOpinion(voyageId, opId);
		} catch (err) {
			if (active) sectionError = errorMessage(err);
			return;
		}
		if (leftVoyage(voyageId)) return;
		opinions = opinions.filter((o) => o.id !== opId);
	}
</script>

<div class="rounded-2xl bg-white p-6 shadow-sm">
	<h2 class="mb-3 font-semibold text-[var(--navy)]">Opinie z rejsu</h2>
	{#if sectionError}<div role="alert" class="mb-3 rounded-lg bg-red-50 p-3 text-sm text-red-600">
			{sectionError}
		</div>{/if}

	{#if crew.length > 0}
		<div class="mb-4 flex flex-wrap items-end gap-2">
			<div class="min-w-0 max-w-full">
				<label for="gen-crew" class="block text-xs text-[var(--text-muted)]">Załogant</label>
				<select
					id="gen-crew"
					bind:value={genCrewId}
					class="max-w-full rounded-lg border px-3 py-1.5 text-sm"
				>
					<option value="">Wybierz...</option>
					{#each crew as member (member.id)}
						<option value={member.crew_member_id}>{member.full_name}</option>
					{/each}
				</select>
			</div>
			<div>
				<label for="gen-format" class="block text-xs text-[var(--text-muted)]">Format</label>
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
			{#each opinions as op (op.id)}
				<div class="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2">
					<div class="flex items-center gap-2">
						<span class="font-medium">{op.full_name}</span>
						<span
							class="rounded-full bg-[var(--ocean-tint)] px-2 py-0.5 text-xs uppercase text-[var(--ocean)]"
						>
							{op.file_format}
						</span>
					</div>
					<div class="flex gap-2">
						<button
							onclick={() => downloadOpinion(op.id)}
							class="text-sm text-[var(--ocean)] hover:underline"
						>
							Pobierz
						</button>
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
