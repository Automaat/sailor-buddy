<script lang="ts">
	import { api } from '$lib/api/client';
	import type { EnrollPageData } from '$lib/api/types';
	import { page } from '$app/state';
	import { onMount } from 'svelte';

	let data = $state<EnrollPageData | null>(null);
	let loading = $state(true);
	let error = $state('');
	let note = $state('');
	let submitting = $state(false);
	let success = $state(false);

	const token = $derived((page.params as Record<string, string>).token);

	const statusLabels: Record<string, string> = {
		pending: 'oczekujący',
		accepted: 'zaakceptowany',
		rejected: 'odrzucony',
		waitlisted: 'lista rezerwowa'
	};

	onMount(async () => {
		try {
			data = await api.get<EnrollPageData>(`/enroll/${token}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nieprawidłowy link zapisu';
		} finally {
			loading = false;
		}
	});

	async function handleEnroll(e: Event) {
		e.preventDefault();
		submitting = true;
		error = '';
		try {
			await api.post(`/enroll/${token}`, { note: note || undefined });
			success = true;
			data = await api.get<EnrollPageData>(`/enroll/${token}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się zapisać';
		} finally {
			submitting = false;
		}
	}
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if error && !data}
	<div class="mx-auto max-w-lg py-12 text-center">
		<div class="rounded-2xl bg-white p-8 shadow-sm">
			<h1 class="mb-2 text-2xl font-bold text-red-600">Nieprawidłowy link</h1>
			<p class="text-[var(--text-muted)]">{error}</p>
		</div>
	</div>
{:else if data}
	<div class="mx-auto max-w-lg">
		<div class="rounded-2xl bg-white p-8 shadow-sm">
			<h1 class="mb-1 text-2xl font-bold text-[var(--navy)]">{data.cruise.name}</h1>
			<p class="mb-4 text-sm text-[var(--text-muted)]">
				{#if data.cruise.start_port && data.cruise.end_port}
					{data.cruise.start_port} → {data.cruise.end_port}
				{/if}
				{#if data.cruise.countries} · {data.cruise.countries}{/if}
				{#if data.cruise.year} · {data.cruise.year}{/if}
			</p>

			{#if data.cruise.embark_date}
				<div class="mb-4 text-sm">
					<span class="text-[var(--text-muted)]">Daty:</span>
					{data.cruise.embark_date} – {data.cruise.disembark_date ?? '?'}
				</div>
			{/if}

			{#if data.cruise.captain_name}
				<div class="mb-4 text-sm">
					<span class="text-[var(--text-muted)]">Kapitan:</span>
					{data.cruise.captain_name}
				</div>
			{/if}

			{#if data.cruise.description}
				<p class="mb-4 whitespace-pre-wrap text-sm">{data.cruise.description}</p>
			{/if}

			<div class="mb-6 flex gap-4 text-sm">
				<div class="rounded-lg bg-gray-50 px-3 py-2">
					<span class="text-[var(--text-muted)]">Zapisanych:</span>
					<span class="font-semibold">{data.total_count}</span>
					{#if data.cruise.max_crew}
						<span class="text-[var(--text-muted)]">/ {data.cruise.max_crew}</span>
					{/if}
				</div>
				<div class="rounded-lg bg-gray-50 px-3 py-2">
					<span class="text-[var(--text-muted)]">Zaakceptowanych:</span>
					<span class="font-semibold">{data.accepted_count}</span>
				</div>
			</div>

			{#if error}
				<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
			{/if}

			{#if success}
				<div class="rounded-lg bg-green-50 p-4 text-center text-sm text-green-700">
					Zostałeś zapisany! Organizator rozpatrzy Twoje zgłoszenie.
				</div>
			{:else if data.enrolled}
				<div class="rounded-lg bg-blue-50 p-4 text-center text-sm text-blue-700">
					Jesteś już zapisany.
					{#if data.enrollment}
						Status: <span class="font-semibold">{statusLabels[data.enrollment.status] ?? data.enrollment.status}</span>
					{/if}
				</div>
			{:else}
				<form onsubmit={handleEnroll} class="space-y-4">
					<div>
						<label for="note" class="mb-1 block text-sm font-medium">Notatka (opcjonalnie)</label>
						<textarea
							id="note"
							bind:value={note}
							rows="3"
							placeholder="Wiadomość dla organizatora..."
							class="w-full rounded-lg border px-3 py-2 text-sm"
						></textarea>
					</div>
					<button
						type="submit"
						disabled={submitting}
						class="w-full rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
					>
						{submitting ? 'Zapisywanie...' : 'Zapisz się'}
					</button>
				</form>
			{/if}
		</div>
	</div>
{/if}
