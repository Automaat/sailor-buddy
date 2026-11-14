<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { updateTrip } from '#lib/api/routes.js';
	import type { Trip } from '#lib/api/types.js';
	import { goto } from '$app/navigation';
	import CaptainSelect from '#lib/components/CaptainSelect.svelte';
	import CruiseSelect from '#lib/components/CruiseSelect.svelte';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import YachtSelect from '#lib/components/YachtSelect.svelte';
	import PriceFields from '#lib/components/PriceFields.svelte';
	import { priceBody, priceFormFrom } from '#lib/pricing.js';
	import { perLoad } from '#lib/state.svelte.js';

	let { data } = $props();

	const id = $derived(data.trip.id);
	const cruises = $derived(data.cruises);
	const loadErrors = $derived(data.loadErrors);
	let yachts = $derived(data.yachts);
	const legacyCaptainName = $derived(
		data.trip.captain_person_id ? '' : (data.trip.captain_name ?? '')
	);

	let form = $derived(perLoad(data, tripForm(data.trip)));
	let price = $derived(perLoad(data, priceFormFrom(data.trip)));
	let submit = $derived(perLoad(data, { error: '', saving: false }));

	// Editing trips is an admin task; redirect regular members away.
	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			void goto(`/trips/${id}`);
		}
	});

	function tripForm(trip: Trip) {
		return {
			name: trip.name,
			embark_date: trip.embark_date ?? '',
			disembark_date: trip.disembark_date ?? '',
			sign_up_deadline: trip.sign_up_deadline ?? '',
			countries: trip.countries ?? '',
			start_port: trip.start_port ?? '',
			end_port: trip.end_port ?? '',
			cruise_id: trip.cruise_id,
			captain_person_id: trip.captain_person_id ?? 0,
			yacht_id: trip.yacht_id ?? 0,
			cost_total: trip.cost_total ?? 0,
			max_crew: trip.max_crew ?? 0,
			description: trip.description ?? '',
			image_logo_url: trip.image_logo_url ?? '',
			image_photo_url: trip.image_photo_url ?? '',
			image_route_url: trip.image_route_url ?? ''
		};
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		const run = submit;
		if (!form.cruise_id) {
			run.error = 'Wybierz wydarzenie, do którego należy rejs';
			return;
		}
		const tripId = id;
		run.saving = true;
		run.error = '';
		try {
			const payload = {
				...form,
				...priceBody(price),
				yacht_id: form.yacht_id || undefined,
				sign_up_deadline: form.sign_up_deadline || undefined,
				image_logo_url: form.image_logo_url || undefined,
				image_photo_url: form.image_photo_url || undefined,
				image_route_url: form.image_route_url || undefined,
				// captain_name of a linked captain is the server's snapshot, so only a
				// legacy free-text name survives clearing the captain.
				captain_person_id: form.captain_person_id || undefined,
				captain_name: form.captain_person_id ? undefined : legacyCaptainName || undefined
			};
			await updateTrip(tripId, payload);
			if (run === submit) void goto(`/trips/${tripId}`);
		} catch (err) {
			run.error = err instanceof Error ? err.message : 'Nie udało się zapisać';
		} finally {
			run.saving = false;
		}
	}
</script>

<div class="mx-auto max-w-3xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Edytuj rejs</h1>
	<LoadErrorBanner errors={loadErrors} />

	{#if submit.error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{submit.error}</div>
	{/if}

	<!-- Remounts the pickers for each trip, so a yacht still being added for
	the previous trip is never picked, or its held save replayed, on this one. -->
	{#key data}
		<form onsubmit={handleSubmit} class="space-y-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<div class="sm:col-span-2">
					<label for="name" class="mb-1 block text-sm font-medium">Nazwa rejsu *</label>
					<input
						id="name"
						type="text"
						bind:value={form.name}
						required
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div class="sm:col-span-2">
					<label for="cruise" class="mb-1 block text-sm font-medium">Wydarzenie klubu *</label>
					<CruiseSelect bind:value={form.cruise_id} {cruises} />
				</div>
				<div>
					<label for="yacht" class="mb-1 block text-sm font-medium">Jacht</label>
					<YachtSelect bind:value={form.yacht_id} bind:yachts />
				</div>
				<div>
					<label for="captain" class="mb-1 block text-sm font-medium">Kapitan</label>
					<CaptainSelect bind:value={form.captain_person_id} legacyName={legacyCaptainName} />
				</div>
				<div>
					<label for="embark" class="mb-1 block text-sm font-medium">Data zaokrętowania</label>
					<input
						id="embark"
						type="date"
						bind:value={form.embark_date}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="disembark" class="mb-1 block text-sm font-medium">Data wyokrętowania</label>
					<input
						id="disembark"
						type="date"
						bind:value={form.disembark_date}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div class="col-span-2">
					<label for="sign_up_deadline" class="mb-1 block text-sm font-medium"
						>Zapisy do (włącznie)</label
					>
					<input
						id="sign_up_deadline"
						type="date"
						bind:value={form.sign_up_deadline}
						max={form.embark_date || undefined}
						class="w-full rounded-lg border px-3 py-2"
					/>
					<p class="mt-1 text-xs text-[var(--text-muted)]">
						Puste — zapisy otwarte, dopóki ich nie wyłączysz.
					</p>
				</div>
				<div>
					<label for="start_port" class="mb-1 block text-sm font-medium">Port wyjścia</label>
					<input
						id="start_port"
						type="text"
						bind:value={form.start_port}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="end_port" class="mb-1 block text-sm font-medium">Port docelowy</label>
					<input
						id="end_port"
						type="text"
						bind:value={form.end_port}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div class="sm:col-span-2">
					<label for="countries" class="mb-1 block text-sm font-medium">Kraje</label>
					<input
						id="countries"
						type="text"
						bind:value={form.countries}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
			</div>

			<hr />
			<h3 class="font-semibold text-[var(--navy)]">Koszty i załoga</h3>
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<PriceFields bind:price />
				<div>
					<label for="ct" class="mb-1 block text-sm font-medium">Koszt całkowity</label>
					<input
						id="ct"
						type="number"
						step="0.01"
						bind:value={form.cost_total}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="max_crew" class="mb-1 block text-sm font-medium">Maks. załoga</label>
					<input
						id="max_crew"
						type="number"
						bind:value={form.max_crew}
						class="w-full rounded-lg border px-3 py-2"
						min="0"
						placeholder="0 = bez limitu"
					/>
				</div>
			</div>

			<div>
				<label for="desc" class="mb-1 block text-sm font-medium">Opis</label>
				<textarea
					id="desc"
					bind:value={form.description}
					rows="4"
					class="w-full rounded-lg border px-3 py-2"></textarea>
			</div>

			<div class="flex gap-3">
				<button
					type="submit"
					disabled={submit.saving}
					class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
				>
					{submit.saving ? 'Zapisywanie...' : 'Zapisz zmiany'}
				</button>
				<a
					href="/trips/{id}"
					class="tap-target rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50"
					>Anuluj</a
				>
			</div>
		</form>
	{/key}
</div>
