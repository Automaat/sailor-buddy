<script lang="ts">
	import { onDestroy } from 'svelte';
	import { createYacht } from '#lib/api/routes.js';
	import type { Yacht, YachtBody } from '#lib/api/types.js';

	// Picks a yacht from the club list, or adds a new one (e.g. a charter yacht
	// used once) without leaving the surrounding form. `create` defaults to the
	// admin yacht endpoint; the voyage captain's log passes the voyage-scoped one.
	type Props = {
		value: number;
		yachts: Yacht[];
		create?: (body: YachtBody) => Promise<Yacht>;
		id?: string;
	};

	let { value = $bindable(), yachts = $bindable(), create = createYacht, id = 'yacht' }: Props = $props();

	let adding = $state(false);
	let saving = $state(false);
	let error = $state('');
	let draft = $state({ yacht_type: '', name: '', registration_no: '' });
	let select: HTMLSelectElement | undefined = $state();
	// Bumped when a draft is cancelled or the picker unmounts, so a create that
	// resolves late cannot select its yacht or close a newer draft.
	let attempt = 0;
	// The parent form's submit, held back while a create is pending so the form
	// is not saved with the previous yacht; replayed once the new yacht is set.
	let heldSubmit: { submitter: HTMLElement | null } | null = null;

	onDestroy(() => {
		attempt++;
		heldSubmit = null;
	});

	function yachtLabel(y: Yacht) {
		return y.yacht_type ? `${y.name} (${y.yacht_type})` : y.name;
	}

	function openAdd() {
		draft = { yacht_type: '', name: '', registration_no: '' };
		error = '';
		adding = true;
	}

	function cancelAdd() {
		attempt++;
		heldSubmit = null;
		saving = false;
		adding = false;
	}

	function holdParentSubmitWhileSaving(e: SubmitEvent) {
		if (!saving || !select?.form || e.target !== select.form) return;
		e.preventDefault();
		e.stopImmediatePropagation();
		heldSubmit = { submitter: e.submitter };
	}

	function replayHeldSubmit() {
		const held = heldSubmit;
		heldSubmit = null;
		const form = select?.form;
		if (!held || !form?.isConnected) return;
		const submitter = held.submitter && form.contains(held.submitter) ? held.submitter : null;
		form.requestSubmit(submitter);
	}

	function sameYacht(y: Yacht, name: string, type: string) {
		return (
			y.name.trim().toLocaleLowerCase() === name.toLocaleLowerCase() &&
			(y.yacht_type ?? '').trim().toLocaleLowerCase() === type.toLocaleLowerCase()
		);
	}

	async function save() {
		if (saving) return;
		const name = draft.name.trim();
		if (!name) {
			error = 'Podaj nazwę jachtu';
			return;
		}
		const type = draft.yacht_type.trim();
		const existing = yachts.find((y) => sameYacht(y, name, type));
		if (existing) {
			value = existing.id;
			error = '';
			adding = false;
			return;
		}
		const current = ++attempt;
		// The selection when the save started; if the user picks another yacht
		// while the create is pending, their pick wins over the new yacht.
		const chosen = value;
		saving = true;
		error = '';
		try {
			const yacht = await create({
				name,
				yacht_type: type || undefined,
				registration_no: draft.registration_no.trim() || undefined
			});
			if (!yachts.some((y) => y.id === yacht.id)) yachts = [...yachts, yacht];
			if (current !== attempt) return;
			if (value === chosen) value = yacht.id;
			adding = false;
			saving = false;
			replayHeldSubmit();
		} catch (err) {
			if (current !== attempt) return;
			heldSubmit = null;
			error = err instanceof Error ? err.message : 'Nie udało się dodać jachtu';
		} finally {
			if (current === attempt) saving = false;
		}
	}

	function saveOnEnterInsteadOfSubmittingParentForm(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			save();
		}
	}
</script>

<svelte:window onsubmitcapture={holdParentSubmitWhileSaving} />

<select {id} bind:this={select} bind:value class="w-full rounded-lg border px-3 py-2">
	<option value={0}>-- Wybierz --</option>
	{#each yachts as yacht (yacht.id)}
		<option value={yacht.id}>{yachtLabel(yacht)}</option>
	{/each}
</select>
{#if adding}
	<div class="mt-2 space-y-2 rounded-lg border bg-gray-50 p-3">
		<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
			<div>
				<label for="{id}-new-type" class="mb-1 block text-xs font-medium">Typ jachtu</label>
				<input id="{id}-new-type" type="text" bind:value={draft.yacht_type} onkeydown={saveOnEnterInsteadOfSubmittingParentForm} placeholder="np. Bavaria 46" class="w-full rounded-lg border px-3 py-1.5 text-sm" />
			</div>
			<div>
				<label for="{id}-new-name" class="mb-1 block text-xs font-medium">Nazwa jachtu *</label>
				<input id="{id}-new-name" type="text" bind:value={draft.name} onkeydown={saveOnEnterInsteadOfSubmittingParentForm} class="w-full rounded-lg border px-3 py-1.5 text-sm" />
			</div>
			<div class="sm:col-span-2">
				<label for="{id}-new-reg" class="mb-1 block text-xs font-medium">Numer rejestracyjny (opcjonalnie)</label>
				<input id="{id}-new-reg" type="text" bind:value={draft.registration_no} onkeydown={saveOnEnterInsteadOfSubmittingParentForm} class="w-full rounded-lg border px-3 py-1.5 text-sm" />
			</div>
		</div>
		{#if error}
			<p class="text-xs text-red-600">{error}</p>
		{/if}
		<div class="flex gap-2">
			<button type="button" onclick={save} disabled={saving} class="rounded-lg bg-[var(--ocean)] px-3 py-1 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
				{saving ? 'Dodawanie...' : 'Dodaj jacht'}
			</button>
			<button type="button" onclick={cancelAdd} class="rounded-lg border px-3 py-1 text-sm text-[var(--text-muted)] hover:bg-gray-50">
				Anuluj
			</button>
		</div>
	</div>
{:else}
	<button type="button" onclick={openAdd} class="mt-1 text-xs text-[var(--ocean)] hover:underline">
		+ Nowy jacht spoza listy
	</button>
{/if}
