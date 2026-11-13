import { countPersonLinkRequests } from '#lib/api/routes.js';

// linkRequests holds the number of pending "To ja" requests shown next to
// Osoby in the admin nav. Pages that approve or reject a request refresh it.
class LinkRequests {
	pending = $state(0);

	async refresh() {
		try {
			this.pending = await countPersonLinkRequests();
		} catch (err) {
			console.error('Failed to count person link requests:', err);
		}
	}
}

export const linkRequests = new LinkRequests();
