<script lang="ts">
	import { api } from '$lib/api/client';
	import type { CruiseEnrollment, Cruise } from '$lib/api/types';
	import { statusLabels } from '$lib/enrollment';
	import { page } from '$app/state';
	import { onMount } from 'svelte';

	let cruise = $state<Cruise | null>(null);
	let enrollments = $state<CruiseEnrollment[]>([]);
	let loading = $state(true);

	const id = $derived(page.params.id);

	onMount(async () => {
		try {
			[cruise, enrollments] = await Promise.all([
				api.get<Cruise>(`/cruises/${id}`),
				api.get<CruiseEnrollment[]>(`/cruises/${id}/enrollments`)
			]);
		} catch (err) {
			console.error('Failed to load enrollments:', err);
		} finally {
			loading = false;
		}
	});

	async function updateStatus(enrollmentId: number, status: string) {
		try {
			await api.put(`/cruises/${id}/enrollments/${enrollmentId}/status`, { status });
			enrollments = enrollments.map((e) =>
				e.id === enrollmentId ? { ...e, status } : e
			);
		} catch (err) {
			console.error('Failed to update status:', err);
		}
	}

	async function deleteEnrollment(enrollmentId: number) {
		if (!confirm('Usunąć ten zapis?')) return;
		try {
			await api.del(`/cruises/${id}/enrollments/${enrollmentId}`);
			enrollments = enrollments.filter((e) => e.id !== enrollmentId);
		} catch (err) {
			console.error('Failed to delete enrollment:', err);
		}
	}

	const statusColors: Record<string, string> = {
		pending: 'bg-yellow-100 text-yellow-800',
		accepted: 'bg-green-100 text-green-800',
		rejected: 'bg-red-100 text-red-800',
		waitlisted: 'bg-purple-100 text-purple-800'
	};
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else}
	<div class="mx-auto max-w-4xl">
		<div class="mb-6 flex items-center justify-between">
			<div>
				<h1 class="text-3xl font-bold text-[var(--navy)]">Zapisy</h1>
				{#if cruise}
					<p class="mt-1 text-[var(--text-muted)]">
						{cruise.name}
						{#if cruise.max_crew}
							· {enrollments.filter((e) => e.status === 'accepted').length} / {cruise.max_crew} zaakceptowanych
						{/if}
					</p>
				{/if}
			</div>
			<a
				href="/cruises/{id}"
				class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50"
			>
				Wróć do rejsu
			</a>
		</div>

		{#if enrollments.length === 0}
			<div class="rounded-2xl bg-white p-6 text-center text-sm text-[var(--text-muted)] shadow-sm">
				Brak zapisów.
			</div>
		{:else}
			<div class="space-y-3">
				{#each enrollments as enrollment}
					<div class="rounded-xl bg-white p-4 shadow-sm">
						<div class="flex items-center justify-between">
							<div>
								<span class="font-medium">{enrollment.user_name ?? 'Nieznany'}</span>
								<span class="ml-2 text-sm text-[var(--text-muted)]">{enrollment.user_email ?? ''}</span>
								<span class="ml-2 inline-block rounded-full px-2 py-0.5 text-xs font-medium {statusColors[enrollment.status] ?? 'bg-gray-100'}">
									{statusLabels[enrollment.status] ?? enrollment.status}
								</span>
							</div>
							<div class="flex gap-1">
								{#if enrollment.status !== 'accepted'}
									<button
										onclick={() => updateStatus(enrollment.id, 'accepted')}
										class="rounded px-2 py-1 text-xs text-green-700 hover:bg-green-50"
									>
										Akceptuj
									</button>
								{/if}
								{#if enrollment.status !== 'waitlisted'}
									<button
										onclick={() => updateStatus(enrollment.id, 'waitlisted')}
										class="rounded px-2 py-1 text-xs text-purple-700 hover:bg-purple-50"
									>
										Rezerwa
									</button>
								{/if}
								{#if enrollment.status !== 'rejected'}
									<button
										onclick={() => updateStatus(enrollment.id, 'rejected')}
										class="rounded px-2 py-1 text-xs text-red-700 hover:bg-red-50"
									>
										Odrzuć
									</button>
								{/if}
								<button
									onclick={() => deleteEnrollment(enrollment.id)}
									class="rounded px-2 py-1 text-xs text-red-500 hover:bg-red-50"
								>
									Usuń
								</button>
							</div>
						</div>
						{#if enrollment.note}
							<p class="mt-2 text-sm text-[var(--text-muted)]">{enrollment.note}</p>
						{/if}
						<p class="mt-1 text-xs text-[var(--text-muted)]">
							Zapisano: {new Date(enrollment.created_at).toLocaleDateString('pl-PL')}
						</p>
					</div>
				{/each}
			</div>
		{/if}
	</div>
{/if}
