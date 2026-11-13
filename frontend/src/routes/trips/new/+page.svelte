<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '#lib/stores/auth.svelte.js';
	import { page } from '$app/state';
	import { listCruises, createTrip } from '#lib/api/routes.js';
	import type { Cruise } from '#lib/api/types.js';
	import { onMount } from 'svelte';
	import CaptainSelect from '#lib/components/CaptainSelect.svelte';
	import CruiseSelect from '#lib/components/CruiseSelect.svelte';
	import PriceFields from '#lib/components/PriceFields.svelte';
	import { priceBody, priceFormFrom } from '#lib/pricing.js';

	let cruises = $state<Cruise[]>([]);
	let error = $state('');
	let loading = $state(false);

	// Planning trips is an admin task; redirect regular members away.
	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			goto('/cruises');
		}
	});

	const initialCruiseID = $derived.by(() => {
		const v = page.url.searchParams.get('cruise_id');
		const n = v ? Number(v) : 0;
		return Number.isFinite(n) ? n : 0;
	});

	let form = $state({
		name: '',
		embark_date: '',
		disembark_date: '',
		sign_up_deadline: '',
		countries: '',
		start_port: '',
		end_port: '',
		cruise_id: 0,
		captain_user_id: 0,
		max_crew: 0,
		description: ''
	});
	let price = $state(priceFormFrom());

	// A captain planning their own trip is preselected, once, so picking
	// "Brak" afterwards sticks.
	let captainDefaulted = false;
	$effect(() => {
		if (captainDefaulted || !auth.user) return;
		captainDefaulted = true;
		if (auth.user.is_captain) form.captain_user_id = auth.user.id;
	});

	type Prefillable = 'embark_date' | 'disembark_date' | 'countries' | 'start_port' | 'end_port';
	const prefillable: Prefillable[] = ['embark_date', 'disembark_date', 'countries', 'start_port', 'end_port'];
	let prefilled: Partial<Record<Prefillable, string>> = {};
	let prefilledPrice = JSON.stringify(priceFormFrom());

	/** Copies the event's defaults into fields the admin has not edited since the last prefill. */
	function prefillFromCruise(cruiseID: number) {
		const c = cruises.find((x) => x.id === cruiseID);
		if (!c) return;
		const next: Partial<Record<Prefillable, string>> = {};
		for (const field of prefillable) {
			const untouched = !form[field] || form[field] === prefilled[field];
			if (untouched) {
				form[field] = c[field] ?? '';
				next[field] = form[field];
			}
		}
		prefilled = next;
		if (JSON.stringify(price) === prefilledPrice) {
			price = priceFormFrom(c);
			prefilledPrice = JSON.stringify(price);
		}
	}

	onMount(async () => {
		cruises = await listCruises().catch(() => []);
		if (initialCruiseID) {
			form.cruise_id = initialCruiseID;
			prefillFromCruise(initialCruiseID);
		}
	});

	const cancelHref = $derived(form.cruise_id ? `/cruises/${form.cruise_id}` : '/cruises');

	async function handleSubmit(e: Event) {
		e.preventDefault();
		if (!form.cruise_id) {
			error = 'Wybierz wydarzenie, do którego należy rejs';
			return;
		}
		loading = true;
		error = '';
		try {
			const payload = {
				...form,
				...priceBody(price),
				sign_up_deadline: form.sign_up_deadline || undefined,
				captain_user_id: form.captain_user_id || undefined
			};
			const trip = await createTrip(payload);
			goto(`/trips/${trip.id}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się zaplanować rejsu';
		} finally {
			loading = false;
		}
	}
</script>

<div class="mx-auto max-w-3xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Zaplanuj rejs</h1>

	{#if error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
	{/if}

	<form onsubmit={handleSubmit} class="space-y-6 rounded-2xl bg-white p-6 shadow-sm">
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			<div class="sm:col-span-2">
				<label for="name" class="mb-1 block text-sm font-medium">Nazwa rejsu *</label>
				<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div class="sm:col-span-2">
				<label for="cruise" class="mb-1 block text-sm font-medium">Wydarzenie klubu *</label>
				<CruiseSelect bind:value={form.cruise_id} {cruises} onchange={prefillFromCruise} />
			</div>
			<div class="sm:col-span-2">
				<label for="captain" class="mb-1 block text-sm font-medium">Kapitan</label>
				<CaptainSelect bind:value={form.captain_user_id} />
			</div>
			<div>
				<label for="embark" class="mb-1 block text-sm font-medium">Data zaokrętowania</label>
				<input id="embark" type="date" bind:value={form.embark_date} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div>
				<label for="disembark" class="mb-1 block text-sm font-medium">Data wyokrętowania</label>
				<input id="disembark" type="date" bind:value={form.disembark_date} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div class="col-span-2">
				<label for="sign_up_deadline" class="mb-1 block text-sm font-medium">Zapisy do (włącznie)</label>
				<input
					id="sign_up_deadline"
					type="date"
					bind:value={form.sign_up_deadline}
					max={form.embark_date || undefined}
					class="w-full rounded-lg border px-3 py-2"
				/>
				<p class="mt-1 text-xs text-[var(--text-muted)]">Puste — zapisy otwarte, dopóki ich nie wyłączysz.</p>
			</div>
			<div>
				<label for="start_port" class="mb-1 block text-sm font-medium">Port wyjścia</label>
				<input id="start_port" type="text" bind:value={form.start_port} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div>
				<label for="end_port" class="mb-1 block text-sm font-medium">Port docelowy</label>
				<input id="end_port" type="text" bind:value={form.end_port} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div class="sm:col-span-2">
				<label for="countries" class="mb-1 block text-sm font-medium">Kraje</label>
				<input id="countries" type="text" bind:value={form.countries} class="w-full rounded-lg border px-3 py-2" />
			</div>
		</div>

		<hr />
		<h3 class="font-semibold text-[var(--navy)]">Koszty i załoga</h3>
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			<PriceFields bind:price />
			<div>
				<label for="max_crew" class="mb-1 block text-sm font-medium">Maks. załoga</label>
				<input id="max_crew" type="number" bind:value={form.max_crew} class="w-full rounded-lg border px-3 py-2" min="0" placeholder="0 = bez limitu" />
			</div>
		</div>

		<div>
			<label for="description" class="mb-1 block text-sm font-medium">Opis</label>
			<textarea id="description" bind:value={form.description} rows="4" class="w-full rounded-lg border px-3 py-2"></textarea>
		</div>

		<div class="flex gap-3">
			<button type="submit" disabled={loading} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
				{loading ? 'Tworzenie...' : 'Zaplanuj rejs'}
			</button>
			<a href={cancelHref} class="tap-target rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">Anuluj</a>
		</div>
	</form>
</div>
