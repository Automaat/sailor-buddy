<script lang="ts">
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

	function yachtLabel(y: Yacht) {
		return y.yacht_type ? `${y.name} (${y.yacht_type})` : y.name;
	}

	function openAdd() {
		draft = { yacht_type: '', name: '', registration_no: '' };
		error = '';
		adding = true;
	}

	async function save() {
		const name = draft.name.trim();
		if (!name) {
			error = 'Podaj nazwę jachtu';
			return;
		}
		saving = true;
		error = '';
		try {
			const yacht = await create({
				name,
				yacht_type: draft.yacht_type.trim() || undefined,
				registration_no: draft.registration_no.trim() || undefined
			});
			yachts = [...yachts, yacht];
			value = yacht.id;
			adding = false;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się dodać jachtu';
		} finally {
			saving = false;
		}
	}

	function saveOnEnterInsteadOfSubmittingParentForm(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			save();
		}
	}
</script>

<select {id} bind:value class="w-full rounded-lg border px-3 py-2">
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
			<button type="button" onclick={() => (adding = false)} class="rounded-lg border px-3 py-1 text-sm text-[var(--text-muted)] hover:bg-gray-50">
				Anuluj
			</button>
		</div>
	</div>
{:else}
	<button type="button" onclick={openAdd} class="mt-1 text-xs text-[var(--ocean)] hover:underline">
		+ Nowy jacht spoza listy
	</button>
{/if}
