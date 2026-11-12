import { defineParams } from '@sveltejs/kit/params';

// integer constrains [id] route segments to digits, so malformed URLs such
// as /trips/foo are a routing-level 404 instead of an /api/trips/NaN request.
export const params = defineParams({
	integer: (param: string) => (/^\d+$/.test(param) ? param : undefined)
});
