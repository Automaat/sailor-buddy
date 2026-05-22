<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import { listMembers, updateMemberRole } from '$lib/api/routes';
	import type { Member } from '$lib/api/types';
	import Users from '@lucide/svelte/icons/users';

	let members = $state<Member[]>([]);
	let loading = $state(true);
	let error = $state('');
	let savingId = $state<number | null>(null);

	const adminCount = $derived(members.filter((m) => m.role === 'admin').length);

	// Managing members is an admin task; redirect regular members away.
	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			goto('/');
		}
	});

	async function load() {
		loading = true;
		try {
			members = (await listMembers()) ?? [];
		} catch (err) {
			console.error('Failed to load members:', err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});

	async function setRole(member: Member, role: 'admin' | 'member') {
		if (member.role === role) return;
		savingId = member.id;
		error = '';
		try {
			await updateMemberRole(member.id, role);
			members = members.map((m) => (m.id === member.id ? { ...m, role } : m));
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się zmienić roli';
		} finally {
			savingId = null;
		}
	}
</script>

<div>
	<div class="mb-6">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Członkowie</h1>
		<p class="mt-1 text-sm text-[var(--text-muted)]">
			Każdy zalogowany użytkownik dołącza automatycznie. Administratorzy zarządzają danymi klubu.
		</p>
	</div>

	{#if error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
	{/if}

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if members.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Users class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak członków</p>
		</div>
	{:else}
		<div class="overflow-hidden rounded-2xl bg-white shadow-sm">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b text-left text-[var(--text-muted)]">
						<th class="p-4">Imię i nazwisko</th>
						<th class="p-4">E-mail</th>
						<th class="p-4">Rola</th>
						<th class="p-4">Akcje</th>
					</tr>
				</thead>
				<tbody>
					{#each members as member}
						{@const isLastAdmin = member.role === 'admin' && adminCount <= 1}
						<tr class="border-b border-gray-50">
							<td class="p-4 font-medium text-[var(--navy)]">{member.name}</td>
							<td class="p-4 text-[var(--text-muted)]">{member.email}</td>
							<td class="p-4">
								<span
									class="rounded-full px-2 py-0.5 text-xs {member.role === 'admin'
										? 'bg-blue-100 text-blue-700'
										: 'bg-gray-100 text-gray-600'}"
								>
									{member.role === 'admin' ? 'Administrator' : 'Członek'}
								</span>
							</td>
							<td class="p-4">
								{#if member.role === 'admin'}
									<button
										onclick={() => setRole(member, 'member')}
										disabled={savingId === member.id || isLastAdmin}
										title={isLastAdmin ? 'Klub musi mieć co najmniej jednego administratora' : ''}
										class="rounded-lg border px-3 py-1 text-xs hover:bg-gray-50 disabled:opacity-40"
									>
										Odbierz uprawnienia
									</button>
								{:else}
									<button
										onclick={() => setRole(member, 'admin')}
										disabled={savingId === member.id}
										class="rounded-lg border px-3 py-1 text-xs hover:bg-gray-50 disabled:opacity-40"
									>
										Nadaj uprawnienia admina
									</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
