<script lang="ts">
	import { api } from '$lib/api/client';
	import { page } from '$app/state';
	import type { OrgMember, OrgInvite } from '$lib/api/types';
	import { orgStore } from '$lib/stores/org.svelte';

	let slug = $derived((page.params as Record<string, string>).slug);
	let members = $state<OrgMember[]>([]);
	let invites = $state<OrgInvite[]>([]);
	let loading = $state(true);
	let error = $state('');

	let isAdmin = $derived(orgStore.current?.role === 'admin');

	let showInviteForm = $state(false);
	let inviteRole = $state('crew');
	let inviteMaxUses = $state('');
	let inviteExpiresHours = $state('');
	let creatingInvite = $state(false);

	const roleLabels: Record<string, string> = {
		admin: 'Admin',
		captain: 'Kapitan',
		crew: 'Załogant'
	};

	async function load() {
		loading = true;
		error = '';
		try {
			members = await api.get<OrgMember[]>(`/orgs/${slug}/members`);
			if (isAdmin) {
				invites = await api.get<OrgInvite[]>(`/orgs/${slug}/invites`);
			} else {
				invites = [];
			}
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function updateRole(memberId: number, role: string) {
		try {
			await api.put(`/orgs/${slug}/members/${memberId}/role`, { role });
			await load();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function removeMember(memberId: number) {
		if (!confirm('Usunąć tego członka z klubu?')) return;
		try {
			await api.del(`/orgs/${slug}/members/${memberId}`);
			await load();
		} catch (e: any) {
			error = e.message;
		}
	}

	async function createInvite(e: Event) {
		e.preventDefault();
		creatingInvite = true;
		error = '';
		const maxUses = inviteMaxUses ? parseInt(inviteMaxUses) : undefined;
		const expiresInHours = inviteExpiresHours ? parseInt(inviteExpiresHours) : undefined;
		if (maxUses !== undefined && (isNaN(maxUses) || maxUses < 1)) {
			error = 'Max użyć musi być większe od 0';
			creatingInvite = false;
			return;
		}
		if (expiresInHours !== undefined && (isNaN(expiresInHours) || expiresInHours < 1)) {
			error = 'Czas wygaśnięcia musi być większy od 0';
			creatingInvite = false;
			return;
		}
		try {
			await api.post(`/orgs/${slug}/invites`, {
				role: inviteRole,
				max_uses: maxUses,
				expires_in_hours: expiresInHours
			});
			showInviteForm = false;
			inviteRole = 'crew';
			inviteMaxUses = '';
			inviteExpiresHours = '';
			await load();
		} catch (e: any) {
			error = e.message;
		} finally {
			creatingInvite = false;
		}
	}

	async function deleteInvite(inviteId: number) {
		try {
			await api.del(`/orgs/${slug}/invites/${inviteId}`);
			await load();
		} catch (e: any) {
			error = e.message;
		}
	}

	function copyInviteLink(token: string) {
		const url = `${window.location.origin}/join/${token}`;
		navigator.clipboard.writeText(url);
	}

	$effect(() => {
		slug;
		load();
	});
</script>

<div class="mx-auto max-w-4xl">
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-bold text-[var(--navy)]">Członkowie</h1>
		{#if isAdmin}
			<button
				onclick={() => (showInviteForm = !showInviteForm)}
				class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-white transition-colors hover:bg-[var(--ocean)]/80"
			>
				{showInviteForm ? 'Anuluj' : 'Zaproś'}
			</button>
		{/if}
	</div>

	{#if error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
	{/if}

	{#if showInviteForm && isAdmin}
		<div class="mb-6 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
			<h2 class="mb-4 text-lg font-semibold text-[var(--navy)]">Nowe zaproszenie</h2>
			<form onsubmit={createInvite} class="space-y-4">
				<div class="grid grid-cols-3 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium text-gray-700" for="invite-role"
							>Rola</label
						>
						<select
							id="invite-role"
							bind:value={inviteRole}
							class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
						>
							<option value="crew">Załogant</option>
							<option value="captain">Kapitan</option>
							<option value="admin">Admin</option>
						</select>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium text-gray-700" for="invite-max"
							>Max użyć</label
						>
						<input
							id="invite-max"
							type="number"
							bind:value={inviteMaxUses}
							placeholder="Bez limitu"
							min="1"
							step="1"
							class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
						/>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium text-gray-700" for="invite-exp"
							>Wygasa za (h)</label
						>
						<input
							id="invite-exp"
							type="number"
							bind:value={inviteExpiresHours}
							placeholder="Nigdy"
							min="1"
							step="1"
							class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
						/>
					</div>
				</div>
				<button
					type="submit"
					disabled={creatingInvite}
					class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-white transition-colors hover:bg-[var(--ocean)]/80 disabled:opacity-50"
				>
					{creatingInvite ? 'Tworzenie...' : 'Utwórz link'}
				</button>
			</form>
		</div>
	{/if}

	{#if isAdmin && invites.length > 0}
		<div class="mb-6 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
			<h2 class="mb-4 text-lg font-semibold text-[var(--navy)]">Aktywne zaproszenia</h2>
			<div class="space-y-3">
				{#each invites as invite}
					<div class="flex items-center justify-between rounded-lg bg-gray-50 p-3">
						<div>
							<span class="text-sm font-medium">{roleLabels[invite.role] ?? invite.role}</span>
							<span class="ml-2 text-xs text-gray-500">
								{invite.use_count}{invite.max_uses ? `/${invite.max_uses}` : ''} użyć
							</span>
							{#if invite.expires_at}
								<span class="ml-2 text-xs text-gray-400">
									wygasa: {new Date(invite.expires_at).toLocaleDateString('pl')}
								</span>
							{/if}
						</div>
						<div class="flex gap-2">
							<button
								onclick={() => copyInviteLink(invite.token)}
								class="rounded bg-[var(--ocean)]/10 px-3 py-1 text-xs text-[var(--ocean)] transition-colors hover:bg-[var(--ocean)]/20"
							>
								Kopiuj link
							</button>
							<button
								onclick={() => deleteInvite(invite.id)}
								class="rounded bg-red-50 px-3 py-1 text-xs text-red-600 transition-colors hover:bg-red-100"
							>
								Usuń
							</button>
						</div>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	{#if loading}
		<p class="text-gray-500">Ładowanie...</p>
	{:else}
		<div class="rounded-lg border border-gray-200 bg-white shadow-sm">
			<table class="w-full">
				<thead>
					<tr class="border-b border-gray-100 text-left text-sm text-gray-500">
						<th class="px-6 py-3">Imię</th>
						<th class="px-6 py-3">Email</th>
						<th class="px-6 py-3">Rola</th>
						{#if isAdmin}
							<th class="px-6 py-3"></th>
						{/if}
					</tr>
				</thead>
				<tbody>
					{#each members as member}
						<tr class="border-b border-gray-50">
							<td class="px-6 py-3 text-sm">{member.user_name}</td>
							<td class="px-6 py-3 text-sm text-gray-500">{member.user_email}</td>
							<td class="px-6 py-3">
								{#if isAdmin}
									<select
										value={member.role}
										onchange={(e) =>
											updateRole(member.id, (e.target as HTMLSelectElement).value)}
										class="rounded border border-gray-300 px-2 py-1 text-sm"
									>
										<option value="crew">Załogant</option>
										<option value="captain">Kapitan</option>
										<option value="admin">Admin</option>
									</select>
								{:else}
									<span
										class="rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]"
									>
										{roleLabels[member.role] ?? member.role}
									</span>
								{/if}
							</td>
							{#if isAdmin}
								<td class="px-6 py-3 text-right">
									<button
										onclick={() => removeMember(member.id)}
										class="text-sm text-red-500 transition-colors hover:text-red-700"
									>
										Usuń
									</button>
								</td>
							{/if}
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
