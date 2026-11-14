<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '#lib/stores/auth.svelte.js';
	import { createTrip } from '#lib/api/routes.js';
	import type { Cruise } from '#lib/api/types.js';
	import CaptainSelect from '#lib/components/CaptainSelect.svelte';
	import CruiseSelect from '#lib/components/CruiseSelect.svelte';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import PriceFields from '#lib/components/PriceFields.svelte';
	import { priceBody, priceFormFrom, type PriceForm } from '#lib/pricing.js';
	import { perLoad } from '#lib/state.svelte.js';

	let { data } = $props();

	const cruises = $derived(data.cruises);
	const loadErrors = $derived(data.loadErrors);

	// Planning trips is an admin task; redirect regular members away.
	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			void goto('/cruises');
		}
	});

	type Prefillable = 'embark_date' | 'disembark_date' | 'countries' | 'start_port' | 'end_port';
	const prefillable: Prefillable[] = ['embark_date', 'disembark_date', 'countries', 'start_port', 'end_port'];

	type Draft = {
		form: {
			name: string;
			embark_date: string;
			disembark_date: string;
			sign_up_deadline: string;
			countries: string;
			start_port: string;
			end_port: string;
			cruise_id: number;
			captain_person_id: number;
			max_crew: number;
			description: string;
		};
		price: PriceForm;
		prefilled: Partial<Record<Prefillable, string>>;
		prefilledPrice: string;
		error: string;
		saving: boolean;
	};

	/** A fresh planning form, prefilled from the event the link preselects. */
	function newDraft(cruiseID: number, list: Cruise[]): Draft {
		const draft: Draft = {
			form: {
				name: '',
				embark_date: '',
				disembark_date: '',
				sign_up_deadline: '',
				countries: '',
				start_port: '',
				end_port: '',
				cruise_id: cruiseID,
				captain_person_id: 0,
				max_crew: 0,
				description: ''
			},
			price: priceFormFrom(),
			prefilled: {},
			prefilledPrice: JSON.stringify(priceFormFrom()),
			error: '',
			saving: false
		};
		if (cruiseID) prefill(draft, list.find((c) => c.id === cruiseID));
		return draft;
	}

	/** Copies the event's defaults into fields the admin has not edited since the last prefill. */
	function prefill(target: Draft, c: Cruise | undefined) {
		if (!c) return;
		const next: Partial<Record<Prefillable, string>> = {};
		for (const field of prefillable) {
			const untouched = !target.form[field] || target.form[field] === target.prefilled[field];
			if (untouched) {
				target.form[field] = c[field] ?? '';
				next[field] = target.form[field];
			}
		}
		target.prefilled = next;
		if (JSON.stringify(target.price) === target.prefilledPrice) {
			target.price = priceFormFrom(c);
			target.prefilledPrice = JSON.stringify(target.price);
		}
	}

	/** Starts over when a link moves the page to another event's planning (?cruise_id=): SvelteKit reuses this component across them. */
	let draft = $derived(perLoad(data, newDraft(data.cruiseId, data.cruises)));
	const form = $derived(draft.form);

	function prefillFromCruise(cruiseID: number) {
		prefill(draft, cruises.find((c) => c.id === cruiseID));
	}

	/** The draft whose captain was preselected: a captain planning their own trip is preselected once per draft, so picking "Brak" afterwards sticks. */
	let captainDefaultedFor: Draft | null = null;
	$effect(() => {
		const current = draft;
		if (captainDefaultedFor === current || !auth.user) return;
		captainDefaultedFor = current;
		if (auth.user.is_captain && auth.user.person_id) current.form.captain_person_id = auth.user.person_id;
	});

	const cancelHref = $derived(form.cruise_id ? `/cruises/${form.cruise_id}` : '/cruises');

	async function handleSubmit(e: Event) {
		e.preventDefault();
		const run = draft;
		if (!run.form.cruise_id) {
			run.error = 'Wybierz wydarzenie, do którego należy rejs';
			return;
		}
		run.saving = true;
		run.error = '';
		try {
			const payload = {
				...run.form,
				...priceBody(run.price),
				sign_up_deadline: run.form.sign_up_deadline || undefined,
				captain_person_id: run.form.captain_person_id || undefined
			};
			const trip = await createTrip(payload);
			if (run === draft) void goto(`/trips/${trip.id}`);
		} catch (err) {
			run.error = err instanceof Error ? err.message : 'Nie udało się zaplanować rejsu';
		} finally {
			run.saving = false;
		}
	}
</script>

<div class="mx-auto max-w-3xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Zaplanuj rejs</h1>
	<LoadErrorBanner errors={loadErrors} />

	{#if draft.error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{draft.error}</div>
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
				<CaptainSelect bind:value={form.captain_person_id} />
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
			<PriceFields bind:price={draft.price} />
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
			<button type="submit" disabled={draft.saving} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
				{draft.saving ? 'Tworzenie...' : 'Zaplanuj rejs'}
			</button>
			<a href={cancelHref} class="tap-target rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">Anuluj</a>
		</div>
	</form>
</div>
