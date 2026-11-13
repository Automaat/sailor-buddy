<script lang="ts">
	import { formatDateRangeWithDays } from '#lib/dates.js';
	import { getDashboard } from '#lib/api/routes.js';
	import type { DashboardStats, SeaTime, YearStats } from '#lib/api/types.js';

	let stats = $state<DashboardStats | null>(null);
	let loading = $state(true);
	let error = $state('');

	async function load() {
		loading = true;
		error = '';
		try {
			stats = await getDashboard();
		} catch (err) {
			console.error('Failed to load dashboard:', err);
			error = 'Nie udało się wczytać pulpitu. Odśwież stronę, aby spróbować ponownie.';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});

	const roleLabels = { captain: 'Kapitan', crew: 'Załoga' } as const;
</script>

{#snippet totals(t: SeaTime)}
	<div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<div class="text-sm text-[var(--text-muted)]">Zrealizowane rejsy</div>
			<div class="mt-1 text-4xl font-bold text-[var(--navy)]">{t.voyage_count}</div>
		</div>
		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<div class="text-sm text-[var(--text-muted)]">Godziny na morzu</div>
			<div class="mt-1 text-4xl font-bold text-[var(--ocean)]">{Math.round(t.total_hours)}</div>
			<div class="mt-1 text-xs text-[var(--text-muted)]">
				{Math.round(t.total_hours_sail)}h żagle / {Math.round(t.total_hours_engine)}h silnik
			</div>
		</div>
		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<div class="text-sm text-[var(--text-muted)]">Mile morskie</div>
			<div class="mt-1 text-4xl font-bold text-[var(--sand)]">
				{Math.round(t.total_miles).toLocaleString()}
			</div>
		</div>
		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<div class="text-sm text-[var(--text-muted)]">Dni na morzu</div>
			<div class="mt-1 text-4xl font-bold text-[var(--navy)]">{t.total_days}</div>
		</div>
	</div>
{/snippet}

{#snippet roleSplit(label: string, t: SeaTime)}
	<div class="rounded-2xl bg-white p-4 shadow-sm">
		<div class="text-sm font-semibold text-[var(--navy)]">{label}</div>
		<div class="mt-1 text-sm text-[var(--text-muted)]">
			{t.voyage_count} rejsy · {Math.round(t.total_miles)} Mm · {Math.round(t.total_hours)} h · {t.total_days}
			dni
		</div>
	</div>
{/snippet}

{#snippet yearTable(rows: YearStats[])}
	<div class="rounded-2xl bg-white p-6 shadow-sm">
		<h3 class="mb-4 text-lg font-semibold text-[var(--navy)]">Wg roku</h3>
		<div class="overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b text-left text-[var(--text-muted)]">
						<th class="pb-2 pr-4">Rok</th>
						<th class="pb-2 pr-4">Rejsy</th>
						<th class="pb-2 pr-4">Godziny</th>
						<th class="pb-2 pr-4">Mile</th>
						<th class="pb-2">Dni</th>
					</tr>
				</thead>
				<tbody>
					{#each rows as row (row.year ?? 'none')}
						<tr class="border-b border-gray-50">
							<td class="py-2 pr-4 font-medium">{row.year ?? '—'}</td>
							<td class="py-2 pr-4">{row.voyage_count}</td>
							<td class="py-2 pr-4">{Math.round(row.total_hours)}</td>
							<td class="py-2 pr-4">{Math.round(row.total_miles)}</td>
							<td class="py-2">{row.total_days}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{/snippet}

<div>
	<h1 class="mb-8 text-3xl font-bold text-[var(--navy)]">Pulpit</h1>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if error}
		<div role="alert" class="rounded-2xl bg-red-50 p-6 text-red-700">{error}</div>
	{:else if stats}
		{@const me = stats.personal}
		<section aria-labelledby="my-sea-time" class="mb-10 space-y-6">
			<h2 id="my-sea-time" class="text-xl font-semibold text-[var(--navy)]">Mój staż</h2>
			{#if me.total.voyage_count === 0}
				<div class="rounded-2xl bg-white p-6 text-center shadow-sm">
					<p class="text-[var(--text-muted)]">Nie masz jeszcze zrealizowanych rejsów z klubem.</p>
					<a href="/cruises" class="tap-target mt-3 inline-block font-medium text-[var(--ocean)] underline">
						Zobacz rejsy klubu
					</a>
				</div>
			{:else}
				{@render totals(me.total)}
				<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
					{@render roleSplit('Jako kapitan', me.as_captain)}
					{@render roleSplit('Jako załoga', me.as_crew)}
				</div>
				{#if me.by_year?.length}
					{@render yearTable(me.by_year)}
				{/if}
				<div class="rounded-2xl bg-white p-6 shadow-sm">
					<h3 class="mb-4 text-lg font-semibold text-[var(--navy)]">Moje rejsy</h3>
					<ul class="divide-y divide-gray-100">
						{#each me.voyages ?? [] as v (v.id)}
							<li>
								<a
									href="/voyages/{v.id}"
									class="flex flex-wrap items-baseline justify-between gap-2 py-3 hover:bg-gray-50"
								>
									<span>
										<span class="font-medium text-[var(--navy)]">{v.name}</span>
										<span class="block text-xs text-[var(--text-muted)]">
											{#if v.embark_date}
												{formatDateRangeWithDays(v.embark_date, v.disembark_date)}
											{:else if v.year}
												{v.year}
											{/if}
											{#if v.yacht_name}· {v.yacht_name}{/if}
										</span>
									</span>
									<span class="text-sm text-[var(--text-muted)]">
										{roleLabels[v.role]} · {Math.round(v.miles)} Mm · {Math.round(v.hours_total)} h
									</span>
								</a>
							</li>
						{/each}
					</ul>
				</div>
			{/if}
		</section>

		{#if stats.club}
			<section aria-labelledby="club-sea-time" class="space-y-6">
				<h2 id="club-sea-time" class="text-xl font-semibold text-[var(--navy)]">Klub</h2>
				{@render totals(stats.club.total)}
				{#if stats.club.by_year?.length}
					{@render yearTable(stats.club.by_year)}
				{/if}
			</section>
		{/if}
	{/if}
</div>
