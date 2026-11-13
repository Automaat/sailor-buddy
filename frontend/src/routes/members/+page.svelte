<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { goto } from '$app/navigation';
	import {
		listMembers,
		updateMemberRole,
		updateMemberCaptain,
		updateMemberMembership
	} from '#lib/api/routes.js';
	import type { Member } from '#lib/api/types.js';
	import { filterCounts, filterMembers, memberFilters, type MemberFilter } from '#lib/members.js';
	import { SvelteSet } from 'svelte/reactivity';
	import Users from '@lucide/svelte/icons/users';
	import Search from '@lucide/svelte/icons/search';

	type Role = 'admin' | 'member';
	const SUCCESS_MS = 3000;

	let members = $state<Member[]>([]);
	let loading = $state(true);
	const saving = new SvelteSet<number>();
	let query = $state('');
	let filter = $state<MemberFilter>('all');
	let success = $state('');
	let failure = $state('');
	let successTimer: ReturnType<typeof setTimeout> | undefined;

	const adminCount = $derived(members.filter((m) => m.role === 'admin').length);
	const counts = $derived(filterCounts(members));
	const visible = $derived(filterMembers(members, query, filter));

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

	$effect(() => () => clearTimeout(successTimer));

	// A success never replaces an error: with saves on several rows in flight,
	// the failure must stay visible until the admin dismisses it or saves again.
	function showSuccess(text: string) {
		clearTimeout(successTimer);
		success = text;
		successTimer = setTimeout(() => (success = ''), SUCCESS_MS);
	}

	function errorText(err: unknown, fallback: string): string {
		return err instanceof Error && err.message ? err.message : fallback;
	}

	// stored returns the member as currently held, so a failed save reverts its
	// control to the latest value rather than the one captured at its start.
	function stored(member: Member): Member {
		return members.find((m) => m.id === member.id) ?? member;
	}

	function patch(id: number, change: Partial<Member>) {
		members = members.map((m) => (m.id === id ? { ...m, ...change } : m));
	}

	// save runs one member update, reports the outcome and puts the control
	// back to the stored value when the update fails or is cancelled.
	async function save(
		member: Member,
		run: () => Promise<unknown>,
		done: string,
		failed: string,
		revert: () => void
	) {
		saving.add(member.id);
		failure = '';
		try {
			await run();
			showSuccess(done);
			return true;
		} catch (err) {
			revert();
			failure = errorText(err, failed);
			return false;
		} finally {
			saving.delete(member.id);
		}
	}

	const roleLabels: Record<Role, string> = { admin: 'Administrator', member: 'Użytkownik' };

	async function setRole(member: Member, role: Role, el: HTMLSelectElement) {
		if (member.role === role) return;
		const revert = () => (el.value = stored(member).role);
		if (member.role === 'admin' && !confirm(`Odebrać uprawnienia administratora: ${member.name}?`)) {
			revert();
			return;
		}
		const ok = await save(
			member,
			() => updateMemberRole(member.id, role),
			`Zapisano rolę: ${member.name} — ${roleLabels[role]}`,
			'Nie udało się zmienić roli',
			revert
		);
		if (ok) patch(member.id, { role });
	}

	const membershipLabels: Record<Member['membership'], string> = {
		czlonek: 'Członek',
		sympatyk: 'Sympatyk',
		gosc: 'Gość'
	};

	// Captains must be club members with a patent number on their account;
	// the server enforces this, the tooltip explains a missing or disabled toggle.
	function captainBlocker(member: Member): string {
		if (member.is_captain) return '';
		if (!member.patent_number) return 'Brak numeru patentu na koncie';
		if (member.membership !== 'czlonek') return 'Kapitanem może być tylko członek klubu';
		return '';
	}

	async function setCaptain(member: Member, isCaptain: boolean, el: HTMLInputElement) {
		const ok = await save(
			member,
			() => updateMemberCaptain(member.id, isCaptain),
			isCaptain ? `Dodano do kadry: ${member.name}` : `Usunięto z kadry: ${member.name}`,
			'Nie udało się zmienić statusu kapitana',
			() => (el.checked = stored(member).is_captain)
		);
		if (ok) patch(member.id, { is_captain: isCaptain });
	}

	async function setMembership(
		member: Member,
		membership: Member['membership'],
		el: HTMLSelectElement
	) {
		if (member.membership === membership) return;
		const ok = await save(
			member,
			() => updateMemberMembership(member.id, membership),
			`Zapisano członkostwo: ${member.name} — ${membershipLabels[membership]}`,
			'Nie udało się zmienić statusu członkostwa',
			() => (el.value = stored(member).membership)
		);
		if (ok) patch(member.id, { membership });
	}
</script>

<div>
	<div class="mb-6">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Członkowie</h1>
		<p class="mt-1 text-sm text-[var(--text-muted)]">
			Każdy zalogowany użytkownik dołącza automatycznie. Administratorzy (zarząd i opiekun) zarządzają
			danymi klubu, a kapitanowie z kadry uzupełniają raporty po swoich rejsach.
		</p>
	</div>

	<div class="fixed right-4 bottom-4 z-50 flex max-w-sm flex-col gap-2">
		{#if failure}
			<div
				role="alert"
				class="flex items-start gap-3 rounded-lg bg-red-50 p-3 text-sm text-red-600 shadow-lg"
			>
				<span class="flex-1">{failure}</span>
				<button type="button" onclick={() => (failure = '')} class="text-xs font-medium hover:underline">
					Zamknij
				</button>
			</div>
		{/if}
		{#if success}
			<div role="status" class="rounded-lg bg-[var(--ocean-tint)] p-3 text-sm text-[var(--ocean)] shadow-lg">
				{success}
			</div>
		{/if}
	</div>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if members.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Users class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak członków</p>
		</div>
	{:else}
		<div class="mb-4 flex flex-col gap-3">
			<label class="relative block max-w-md">
				<span class="sr-only">Szukaj członków</span>
				<Search
					class="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-[var(--text-muted)]"
				/>
				<input
					type="search"
					bind:value={query}
					placeholder="Szukaj po imieniu, nazwisku lub e-mailu"
					class="w-full rounded-lg border bg-white py-2 pr-3 pl-9 text-sm"
				/>
			</label>
			<div class="flex flex-wrap gap-2" role="group" aria-label="Filtr członków">
				{#each memberFilters as f (f.value)}
					<button
						type="button"
						aria-pressed={filter === f.value}
						onclick={() => (filter = f.value)}
						class="rounded-full border px-3 py-1 text-sm transition-colors {filter === f.value
							? 'border-[var(--ocean)] bg-[var(--ocean)] text-white'
							: 'bg-white text-[var(--text-muted)] hover:bg-gray-50'}"
					>
						{f.label} <span class="tabular-nums opacity-80">({counts[f.value]})</span>
					</button>
				{/each}
			</div>
		</div>

		{#if visible.length === 0}
			<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
				<p class="text-lg text-[var(--text-muted)]">Brak osób spełniających kryteria</p>
			</div>
		{:else}
			<div class="overflow-x-auto rounded-2xl bg-white shadow-sm">
				<table class="w-full text-sm">
					<thead>
						<tr class="border-b text-left text-[var(--text-muted)]">
							<th class="p-4">Imię i nazwisko</th>
							<th class="p-4">E-mail</th>
							<th class="p-4">Rola</th>
							<th class="p-4">Członkostwo</th>
							<th class="p-4">Kadra kapitańska</th>
						</tr>
					</thead>
					<tbody>
						{#each visible as member (member.id)}
							{@const isLastAdmin = member.role === 'admin' && adminCount <= 1}
							{@const blocker = captainBlocker(member)}
							{@const busy = saving.has(member.id)}
							<tr class="border-b border-gray-50">
								<td class="p-4 font-medium text-[var(--navy)]">
									{member.name}
									{#if !member.onboarded}
										<span
											class="ml-1 rounded-full bg-[var(--navy)]/10 px-2 py-0.5 text-[10px] font-normal text-[var(--navy)]"
										>
											profil niekompletny
										</span>
									{/if}
								</td>
								<td class="p-4 text-[var(--text-muted)]">{member.email}</td>
								<td class="p-4">
									<select
										aria-label="Rola: {member.name}"
										value={member.role}
										disabled={busy || isLastAdmin}
										title={isLastAdmin ? 'Klub musi mieć co najmniej jednego administratora' : ''}
										onchange={(e) => setRole(member, e.currentTarget.value as Role, e.currentTarget)}
										class="rounded-lg border px-2 py-1 text-xs disabled:opacity-40 {member.role ===
										'admin'
											? 'bg-[var(--ocean-tint)] text-[var(--ocean)]'
											: ''}"
									>
										{#each Object.entries(roleLabels) as [value, label]}
											<option {value}>{label}</option>
										{/each}
									</select>
								</td>
								<td class="p-4">
									<select
										aria-label="Członkostwo: {member.name}"
										value={member.membership}
										disabled={busy}
										onchange={(e) =>
											setMembership(
												member,
												e.currentTarget.value as Member['membership'],
												e.currentTarget
											)}
										class="rounded-lg border px-2 py-1 text-xs disabled:opacity-40"
									>
										{#each Object.entries(membershipLabels) as [value, label]}
											<option {value}>{label}</option>
										{/each}
									</select>
								</td>
								<td class="p-4">
									{#if member.patent_number || member.is_captain}
										<label class="flex items-center gap-2 text-xs" title={blocker}>
											<input
												type="checkbox"
												aria-label="Kapitan: {member.name}"
												checked={member.is_captain}
												disabled={busy || !!blocker}
												onchange={(e) =>
													setCaptain(member, e.currentTarget.checked, e.currentTarget)}
											/>
											{member.is_captain ? 'Kapitan' : 'Nie'}
										</label>
									{:else}
										<span
											class="cursor-help text-[var(--text-muted)]"
											title={blocker}
											aria-label="Kapitan: {member.name} — {blocker}"
										>
											—
										</span>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</div>
