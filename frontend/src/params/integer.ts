import type { ParamMatcher } from '@sveltejs/kit';

// integer constrains [id] route segments to digits, so malformed URLs such
// as /trips/foo are a routing-level 404 instead of an /api/trips/NaN request.
export const match: ParamMatcher = (param) => /^\d+$/.test(param);
