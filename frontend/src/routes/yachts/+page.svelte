<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { listYachts, createYacht, deleteYacht } from '#lib/api/routes.js';
	import type { Yacht } from '#lib/api/types.js';
	import Ship from '@lucide/svelte/icons/ship';

	let yachts = $state<Yacht[]>([]);
	let loading = $state(true);
	let showForm = $state(false);
	const emptyForm = () => ({
		name: '',
		registration_no: '',
		yacht_type: '',
		berths: null as number | null,
		length_m: null as number | null,
		build_year: null as number | null,
		engine: '',
		owner_name: ''
	});
	let form = $state(emptyForm());
	let saving = $state(false);

	async function load() {
		loading = true;
		try {
			yachts = await listYachts();
		} catch (err) {
			console.error('Failed to load yachts:', err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		void load();
	});

	async function handleAdd(e: Event) {
		e.preventDefault();
		saving = true;
		try {
			const yacht = await createYacht({
				name: form.name,
				registration_no: form.registration_no || undefined,
				yacht_type: form.yacht_type || undefined,
				berths: form.berths ?? undefined,
				length_m: form.length_m ?? undefined,
				build_year: form.build_year ?? undefined,
				engine: form.engine || undefined,
				owner_name: form.owner_name || undefined
			});
			yachts = [...yachts, yacht];
			form = emptyForm();
			showForm = false;
		} catch (err) {
			console.error('Failed to add yacht:', err);
		} finally {
			saving = false;
		}
	}

	function yachtSpecs(y: Yacht): string {
		return [
			y.length_m != null ? `${y.length_m} m` : null,
			y.berths != null ? `${y.berths} koi` : null,
			y.build_year != null ? `rok ${y.build_year}` : null,
			y.engine,
			y.owner_name ? `armator: ${y.owner_name}` : null
		]
			.filter(Boolean)
			.join(' · ');
	}

	async function handleDelete(id: number) {
		if (!confirm('Usunąć ten jacht?')) return;
		await deleteYacht(id);
		yachts = yachts.filter((y) => y.id !== id);
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Jachty</h1>
		{#if auth.isAdmin}
			<button
				onclick={() => (showForm = !showForm)}
				class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--ocean-dark)]"
			>
				{showForm ? 'Anuluj' : '+ Dodaj jacht'}
			</button>
		{/if}
	</div>

	{#if showForm && auth.isAdmin}
		<form onsubmit={handleAdd} class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
				<div>
					<label for="name" class="mb-1 block text-sm font-medium">Nazwa *</label>
					<input
						id="name"
						type="text"
						bind:value={form.name}
						required
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="reg" class="mb-1 block text-sm font-medium">Nr rejestracyjny</label>
					<input
						id="reg"
						type="text"
						bind:value={form.registration_no}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="type" class="mb-1 block text-sm font-medium">Typ</label>
					<input
						id="type"
						type="text"
						bind:value={form.yacht_type}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="berths" class="mb-1 block text-sm font-medium">Koje</label>
					<input
						id="berths"
						type="number"
						min="1"
						step="1"
						bind:value={form.berths}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="length" class="mb-1 block text-sm font-medium">Długość (m)</label>
					<input
						id="length"
						type="number"
						min="0.1"
						step="0.1"
						bind:value={form.length_m}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="build_year" class="mb-1 block text-sm font-medium">Rok budowy</label>
					<input
						id="build_year"
						type="number"
						min="1800"
						max="2100"
						step="1"
						bind:value={form.build_year}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="engine" class="mb-1 block text-sm font-medium">Silnik</label>
					<input
						id="engine"
						type="text"
						bind:value={form.engine}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="owner" class="mb-1 block text-sm font-medium">Armator</label>
					<input
						id="owner"
						type="text"
						bind:value={form.owner_name}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
			</div>
			<button
				type="submit"
				disabled={saving}
				class="mt-4 rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{saving ? 'Dodawanie...' : 'Dodaj'}
			</button>
		</form>
	{/if}

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if yachts.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Ship class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak jachtów</p>
		</div>
	{:else}
		<div class="grid gap-3">
			{#each yachts as yacht (yacht.id)}
				<div class="flex items-center justify-between rounded-2xl bg-white px-6 py-4 shadow-sm">
					<div>
						<span class="font-semibold text-[var(--navy)]">{yacht.name}</span>
						{#if yacht.yacht_type}
							<span class="ml-2 text-sm text-[var(--text-muted)]">{yacht.yacht_type}</span>
						{/if}
						{#if yachtSpecs(yacht)}
							<div class="text-sm text-[var(--text-muted)]">{yachtSpecs(yacht)}</div>
						{/if}
					</div>
					<div class="flex items-center gap-4">
						{#if yacht.registration_no}
							<span class="text-sm text-[var(--text-muted)]">{yacht.registration_no}</span>
						{/if}
						{#if auth.isAdmin}
							<button
								onclick={() => handleDelete(yacht.id)}
								class="text-sm text-red-400 hover:text-red-600">Usuń</button
							>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
