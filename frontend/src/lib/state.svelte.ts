/**
 * perLoad returns value as deeply reactive state that belongs to one page
 * load. Declared as `let form = $derived(perLoad(data, initialForm(data)))`,
 * it starts over from the new value whenever the page's load data changes:
 * SvelteKit reuses a page component when a link moves it to another record,
 * so state set once on mount would keep the previous record's form. Fields
 * can be bound (bind:value={form.name}) like any $state.
 *
 * A request started before the change still holds the old state object, so
 * what it writes there (an error, a busy flag) is never shown on the new
 * record; compare the held object with the current one before acting on its
 * reply.
 */
export function perLoad<T>(data: unknown, value: T): T {
	void data;
	const state = $state(value);
	return state;
}
