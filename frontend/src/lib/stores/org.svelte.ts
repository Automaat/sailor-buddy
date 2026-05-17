import type { Organization } from '$lib/api/types';
import { api } from '$lib/api/client';

const LS_KEY = 'sailor-buddy-org';

function createOrgStore() {
	let orgs = $state<Organization[]>([]);
	let currentSlug = $state<string | null>(null);
	let loading = $state(false);
	// true once the org list has been fetched at least once — guards wait for this
	let loaded = $state(false);
	// in-flight refresh, so ensureLoaded reuses it instead of firing another
	let refreshPromise: Promise<void> | null = null;

	if (typeof window !== 'undefined') {
		currentSlug = localStorage.getItem(LS_KEY);
	}

	async function load(): Promise<void> {
		loading = true;
		try {
			orgs = (await api.get('/orgs')) ?? [];
			loaded = true;
			if (currentSlug && !orgs.find((o) => o.slug === currentSlug)) {
				currentSlug = null;
			}
			if (!currentSlug && orgs.length > 0) {
				currentSlug = orgs[0].slug;
			}
			if (typeof window !== 'undefined') {
				if (currentSlug) {
					localStorage.setItem(LS_KEY, currentSlug);
				} else {
					localStorage.removeItem(LS_KEY);
				}
			}
		} finally {
			loading = false;
		}
	}

	return {
		get orgs() {
			return orgs;
		},
		get current(): Organization | null {
			if (!currentSlug) return null;
			return orgs.find((o) => o.slug === currentSlug) ?? null;
		},
		get currentSlug() {
			return currentSlug;
		},
		get loading() {
			return loading;
		},
		get loaded() {
			return loaded;
		},
		get isOrgMode() {
			return currentSlug !== null;
		},
		get isOrgAdmin() {
			if (!currentSlug) return false;
			return orgs.find((o) => o.slug === currentSlug)?.role === 'admin';
		},
		// Switching org context is an admin tool — intentionally unavailable to
		// non-admin members, even when they belong to several orgs.
		get canSwitch() {
			return orgs.length > 1 && orgs.some((o) => o.role === 'admin');
		},
		select(slug: string | null) {
			currentSlug = slug;
			if (typeof window !== 'undefined') {
				if (slug) {
					localStorage.setItem(LS_KEY, slug);
				} else {
					localStorage.removeItem(LS_KEY);
				}
			}
		},
		refresh(): Promise<void> {
			refreshPromise = load();
			return refreshPromise;
		},
		// ensureLoaded resolves once the org list has been fetched at least
		// once. Callers that scope on isOrgAdmin await this so they don't race
		// the initial load and fall back to the wrong (personal) endpoint.
		async ensureLoaded(): Promise<void> {
			if (loaded) return;
			refreshPromise ??= load();
			await refreshPromise;
		},
		clear() {
			orgs = [];
			currentSlug = null;
			loaded = false;
			refreshPromise = null;
			if (typeof window !== 'undefined') {
				localStorage.removeItem(LS_KEY);
			}
		},
		apiPrefix(): string {
			if (!currentSlug) return '';
			return `/orgs/${currentSlug}`;
		}
	};
}

export const orgStore = createOrgStore();
