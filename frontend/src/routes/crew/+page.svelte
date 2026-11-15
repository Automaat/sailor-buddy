<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { goto } from '$app/navigation';
	import {
		listMembers,
		updateMemberRole,
		updateMemberCaptain,
		updateMemberMembership,
		updateMemberAccess
	} from '#lib/api/routes.js';
	import type { Member } from '#lib/api/types.js';
	import {
		filterCounts,
		matchesFilter,
		matchesQuery,
		memberFilters,
		type MemberFilter
	} from '#lib/members.js';
	import { SvelteSet } from 'svelte/reactivity';
	import Users from '@lucide/svelte/icons/users';
	import { listCrew, createCrew } from '#lib/api/routes.js';
	import {
		accountFilters,
		matchesAccount,
		matchesPerson,
		parseOtherNames,
		type AccountFilter
	} from '#lib/people.js';
	import PersonLinkRequests from '#lib/components/PersonLinkRequests.svelte';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import { settle, noteFailure } from '#lib/api/errors.js';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	let people = $derived(data.people);
	let loadErrors = $state<string[]>([]);
	let showForm = $state(false);
	let form = $state({ full_name: '', other_names: '', email: '', patent_number: '' });
	let adding = $state(false);
	let account = $state<AccountFilter>('all');

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
	const counts = $derived({ ...filterCounts(members), all: people.length });
	const accounts = $derived(new Map(members.map((m) => [m.id, m])));
	const visible = $derived(
		people.filter((person) => {
			const member = person.user_id ? accounts.get(person.user_id) : undefined;
			return (
				matchesAccount(person, account) &&
				(matchesPerson(person, query) || (!!member && matchesQuery(member, query))) &&
				(filter === 'all' || (!!member && matchesFilter(member, filter)))
			);
		})
	);

	async function loadMembers() {
		loading = true;
		const errors: string[] = [];
		members = (await settle(listMembers(), [], noteFailure(errors, 'konta i dostęp'))) ?? [];
		loadErrors = errors;
		loading = false;
	}

	$effect(() => {
		if (auth.isAdmin) void loadMembers();
		else members = [];
	});

	$effect(() => {
		if (auth.user && !auth.isAdmin) void goto('/');
	});

	async function refreshPeople() {
		people = (await settle(listCrew(), people, noteFailure(loadErrors, 'osoby'))) ?? [];
	}

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
		if (
			member.role === 'admin' &&
			!confirm(`Odebrać uprawnienia administratora: ${member.name}?`)
		) {
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
	async function setSuspended(member: Member, suspended: boolean, el: HTMLSelectElement) {
		if (member.suspended === suspended) return;
		const ok = await save(
			member,
			() => updateMemberAccess(member.id, { suspended }),
			`Zapisano dostęp: ${member.name} — ${suspended ? 'Zawieszone' : 'Aktywne'}`,
			'Nie udało się zmienić dostępu',
			() => (el.value = stored(member).suspended ? 'suspended' : 'active')
		);
		if (ok) patch(member.id, { suspended });
	}
	async function handleAdd(e: Event) {
		e.preventDefault();
		adding = true;
		failure = '';
		try {
			const person = await createCrew({
				full_name: form.full_name,
				other_names: parseOtherNames(form.other_names),
				email: form.email || undefined,
				patent_number: form.patent_number || undefined
			});
			people = [...people, person].sort((a, b) => a.full_name.localeCompare(b.full_name, 'pl'));
			form = { full_name: '', other_names: '', email: '', patent_number: '' };
			showForm = false;
		} catch (err) {
			failure = errorText(err, 'Nie udało się dodać osoby');
		} finally {
			adding = false;
		}
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Osoby</h1>
		{#if auth.isAdmin}
			<button
				onclick={() => (showForm = !showForm)}
				class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--ocean-dark)]"
			>
				{showForm ? 'Anuluj' : '+ Dodaj osobę'}
			</button>
		{/if}
	</div>

	{#if auth.isAdmin}
		<PersonLinkRequests ondecided={refreshPeople} />
	{/if}

	{#if showForm && auth.isAdmin}
		<form onsubmit={handleAdd} class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<div>
					<label for="name" class="mb-1 block text-sm font-medium">Imię i nazwisko *</label>
					<input
						id="name"
						type="text"
						bind:value={form.full_name}
						required
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="other-names" class="mb-1 block text-sm font-medium"
						>Inne nazwiska / pisownie</label
					>
					<input
						id="other-names"
						type="text"
						bind:value={form.other_names}
						placeholder="oddzielone przecinkami"
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="email" class="mb-1 block text-sm font-medium">Email</label>
					<input
						id="email"
						type="email"
						bind:value={form.email}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="patent" class="mb-1 block text-sm font-medium">Numer patentu</label>
					<input
						id="patent"
						type="text"
						bind:value={form.patent_number}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
			</div>
			<button
				type="submit"
				disabled={adding}
				class="mt-4 rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{adding ? 'Dodawanie...' : 'Dodaj'}
			</button>
		</form>
	{/if}

	<div class="fixed right-4 bottom-4 z-50 flex max-w-sm flex-col gap-2">
		{#if failure}
			<div
				role="alert"
				class="flex items-start gap-3 rounded-lg bg-red-50 p-3 text-sm text-red-600 shadow-lg"
			>
				<span class="flex-1">{failure}</span>
				<button
					type="button"
					onclick={() => (failure = '')}
					class="text-xs font-medium hover:underline"
				>
					Zamknij
				</button>
			</div>
		{/if}
		{#if success}
			<div
				role="status"
				class="rounded-lg bg-[var(--ocean-tint)] p-3 text-sm text-[var(--ocean)] shadow-lg"
			>
				{success}
			</div>
		{/if}
	</div>

	<LoadErrorBanner errors={loadErrors} />
	<div class="mb-4 flex flex-wrap items-center gap-3">
		<input
			type="search"
			bind:value={query}
			placeholder="Szukaj po nazwisku, innym nazwisku lub e-mailu"
			aria-label="Szukaj osób"
			class="min-w-0 flex-1 rounded-lg border px-3 py-2"
		/>
		<div class="flex gap-1" role="group" aria-label="Konto">
			{#each accountFilters as f (f.value)}
				<button
					type="button"
					onclick={() => (account = f.value)}
					aria-pressed={account === f.value}
					class="rounded-full px-3 py-1 text-sm {account === f.value
						? 'bg-[var(--ocean)] text-white'
						: 'bg-white text-[var(--text-muted)] hover:bg-gray-50'}"
				>
					{f.label}
				</button>
			{/each}
		</div>
	</div>
	{#if auth.isAdmin && !loading && !loadErrors.includes('konta i dostęp')}
		<div class="mb-4 flex flex-wrap gap-2" role="group" aria-label="Członkostwo i kadra">
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
	{/if}

	{#if people.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Users class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p>Brak osób</p>
		</div>
	{:else if visible.length === 0}
		<p class="py-12 text-center text-[var(--text-muted)]">Nikt nie pasuje do wyszukiwania</p>
	{:else}
		<div class="overflow-x-auto rounded-2xl bg-white shadow-sm">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b text-left text-[var(--text-muted)]">
						<th class="p-4">Imię i nazwisko</th>
						<th class="p-4">E-mail</th>
						{#if auth.isAdmin}<th class="p-4">Dostęp do konta</th>
							<th class="p-4">Rola</th>
							<th class="p-4">Członkostwo</th>
							<th class="p-4">Kadra kapitańska</th>{/if}
					</tr>
				</thead>
				<tbody>
					{#each visible as person (person.id)}
						{@const member = person.user_id ? accounts.get(person.user_id) : undefined}
						<tr class="border-b border-gray-50" data-testid="person">
							<td class="p-4 font-medium text-[var(--navy)]">
								<a href="/crew/{person.id}" class="hover:underline"
									><span>{person.full_name}</span></a
								>
								{#if person.other_names?.length}<div class="text-xs text-[var(--text-muted)]">
										{person.other_names.join(', ')}
									</div>{/if}
								{#if person.patent_number}<div class="text-xs text-[var(--text-muted)]">
										Patent: {person.patent_number}
									</div>{/if}
								<span class="ml-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600"
									>{person.has_account ? 'ma konto' : 'bez konta'}</span
								>
								{#if member && !member.onboarded}<span class="ml-1 text-xs text-[var(--text-muted)]"
										>profil niekompletny</span
									>{/if}
							</td>
							<td class="p-4 text-[var(--text-muted)]">{person.email || member?.email || '—'}</td>
							{#if auth.isAdmin}
								{#if member}
									{@const isLastAdmin = member.role === 'admin' && adminCount <= 1}
									{@const blocker = captainBlocker(member)}
									{@const busy = saving.has(member.id)}

									<td class="p-4"
										><select
											aria-label="Dostęp: {member.name}"
											disabled={busy || member.role === 'admin'}
											value={member.suspended ? 'suspended' : 'active'}
											title={member.role === 'admin'
												? 'Odbierz rolę administratora przed zawieszeniem konta'
												: ''}
											onchange={(e) =>
												setSuspended(
													member,
													e.currentTarget.value === 'suspended',
													e.currentTarget
												)}
											class="rounded-lg border px-2 py-1 text-xs"
											><option value="active">Aktywne</option><option value="suspended"
												>Zawieszone</option
											></select
										></td
									>
									<td class="p-4">
										<select
											aria-label="Rola: {member.name}"
											value={member.role}
											disabled={busy || isLastAdmin}
											title={isLastAdmin ? 'Klub musi mieć co najmniej jednego administratora' : ''}
											onchange={(e) =>
												setRole(member, e.currentTarget.value as Role, e.currentTarget)}
											class="rounded-lg border px-2 py-1 text-xs disabled:opacity-40 {member.role ===
											'admin'
												? 'bg-[var(--ocean-tint)] text-[var(--ocean)]'
												: ''}"
										>
											{#each Object.entries(roleLabels) as [value, label] (value)}
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
											{#each Object.entries(membershipLabels) as [value, label] (value)}
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
								{:else}
									<td colspan="4" class="p-4 text-[var(--text-muted)]"
										>{person.has_account
											? loading
												? 'Wczytywanie konta...'
												: 'Dane konta niedostępne'
											: 'Brak konta — dostęp i uprawnienia nie dotyczą tej osoby'}</td
									>
								{/if}
							{/if}
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
