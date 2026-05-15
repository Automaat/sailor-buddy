import type { Organization } from '$lib/api/types';
import { api } from '$lib/api/client';

const LS_KEY = 'sailor-buddy-org';

function createOrgStore() {
	let orgs = $state<Organization[]>([]);
	let currentSlug = $state<string | null>(null);
	let loading = $state(false);

	if (typeof window !== 'undefined') {
		currentSlug = localStorage.getItem(LS_KEY);
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
		get isOrgMode() {
			return currentSlug !== null;
		},
		get isOrgAdmin() {
			if (!currentSlug) return false;
			return orgs.find((o) => o.slug === currentSlug)?.role === 'admin';
		},
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
		async refresh() {
			loading = true;
			try {
				orgs = await api.get<Organization[]>('/orgs');
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
		},
		clear() {
			orgs = [];
			currentSlug = null;
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
